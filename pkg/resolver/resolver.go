package resolver

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
)

type Resolver struct {
	// awsClients is a factory to retrieve clients to talk to the AWS API using the right credentials.
	awsClients AWSClients
	// dnsServer contains details about the DNS server that needs to resolve the domain.
	dnsServer DNSServer
	// workloadClusterBaseDomain is the root hosted zone used to create the workload cluster hosted zone, i.e. gaws.gigantic.io
	workloadClusterBaseDomain string
}

type AWSClients interface {
	NewResolverClient(region, arn string) (ResolverClient, error)
	NewResolverClientWithExternalId(region, roleToAssume, externalRoleToAssume, externalId string) (ResolverClient, error)
	NewEC2Client(region, arn string) (EC2Client, error)
	NewEC2ClientWithExternalId(region, arn, externalId string) (EC2Client, error)
	NewRAMClient(region, arn string) (RAMClient, error)
	NewRAMClientWithExternalId(region, arn, externalId string) (RAMClient, error)
}

//counterfeiter:generate . EC2Client
type EC2Client interface {
	CreateSecurityGroupForResolverEndpoints(ctx context.Context, vpcId, groupName string) (string, error)
	DeleteSecurityGroupForResolverEndpoints(ctx context.Context, logger logr.Logger, vpcId, groupName string) error
}

//counterfeiter:generate . RAMClient
type RAMClient interface {
	CreateResourceShareWithContext(ctx context.Context, resourceShareName string, allowExternalPrincipals bool, resourceArns, principals []string) (string, error)
	DeleteResourceShareWithContext(ctx context.Context, logger logr.Logger, resourceShareName string) error
}

//counterfeiter:generate . ResolverClient
type ResolverClient interface {
	GetResolverRuleByName(ctx context.Context, resolverRuleName, resolverRuleType string) (ResolverRule, error)
	CreateResolverRule(ctx context.Context, logger logr.Logger, cluster Cluster, securityGroupId, domainName, resolverRuleName string) (ResolverRule, error)
	DeleteResolverRule(ctx context.Context, logger logr.Logger, cluster Cluster, resolverRuleId string) error
	AssociateResolverRuleWithContext(ctx context.Context, associationName, vpcID, resolverRuleId string) error
	DisassociateResolverRuleWithContext(ctx context.Context, logger logr.Logger, vpcID, resolverRuleId string) error
}

type Cluster struct {
	Name       string
	Region     string
	VPCId      string
	IAMRoleARN string
	Subnets    []string
}

type ResolverRule struct {
	RuleId  string
	RuleArn string
}

func NewResolver(awsClients AWSClients, dnsServer DNSServer, workloadClusterBaseDomain string) (Resolver, error) {
	return Resolver{
		awsClients:                awsClients,
		dnsServer:                 dnsServer,
		workloadClusterBaseDomain: workloadClusterBaseDomain,
	}, nil
}

// CreateRule will create a route53 resolver Rule and associate it with a VPC where a DNS server is running.
// Clients of the DNS server need to be able to resolve the Cluster domain, so we need to associate a resolver rule to
// the VPC where the DNS server is running.
func (r *Resolver) CreateRule(ctx context.Context, logger logr.Logger, cluster Cluster) (ResolverRule, error) {
	resolverRule, err := r.createRule(ctx, logger, cluster)
	if err != nil {
		return ResolverRule{}, errors.WithStack(err)
	}

	err = r.associateRule(ctx, logger, cluster, resolverRule)
	if err != nil {
		return ResolverRule{}, errors.WithStack(err)
	}

	return resolverRule, nil
}

func (r *Resolver) DeleteRule(ctx context.Context, logger logr.Logger, cluster Cluster) error {
	resolverClient, err := r.awsClients.NewResolverClient(cluster.Region, cluster.IAMRoleARN)
	if err != nil {
		return errors.WithStack(err)
	}

	resolverRule, err := resolverClient.GetResolverRuleByName(ctx, getResolverRuleName(cluster.Name), "FORWARD")
	if errors.Is(err, &ResolverRuleNotFoundError{}) {
		logger.Info("The Resolver Rule was not found, it must have been already deleted", "resolverRuleName", getResolverRuleName(cluster.Name), "dnsServerVPCId", r.dnsServer.VPCId)
		return nil
	}
	if err != nil {
		return errors.WithStack(err)
	}

	dnsServerResolverClient, err := r.awsClients.NewResolverClientWithExternalId(r.dnsServer.AWSRegion, cluster.IAMRoleARN, r.dnsServer.IAMRoleToAssume, r.dnsServer.IAMExternalId)
	if err != nil {
		return errors.WithStack(err)
	}

	err = dnsServerResolverClient.DisassociateResolverRuleWithContext(ctx, logger, r.dnsServer.VPCId, resolverRule.RuleId)
	if err != nil {
		return errors.WithStack(err)
	}

	ramClient, err := r.awsClients.NewRAMClient(cluster.Region, cluster.IAMRoleARN)
	if err != nil {
		return errors.WithStack(err)
	}

	err = ramClient.DeleteResourceShareWithContext(ctx, logger, getResourceShareName(cluster.Name))
	if err != nil {
		return errors.WithStack(err)
	}

	err = resolverClient.DeleteResolverRule(ctx, logger, cluster, resolverRule.RuleId)
	if err != nil {
		return errors.WithStack(err)
	}

	ec2Client, err := r.awsClients.NewEC2Client(cluster.Region, cluster.IAMRoleARN)
	if err != nil {
		return errors.WithStack(err)
	}

	err = ec2Client.DeleteSecurityGroupForResolverEndpoints(ctx, logger, cluster.VPCId, getSecurityGroupName(cluster.Name))
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (r *Resolver) createRule(ctx context.Context, logger logr.Logger, cluster Cluster) (ResolverRule, error) {
	ec2Client, err := r.awsClients.NewEC2Client(cluster.Region, cluster.IAMRoleARN)
	if err != nil {
		return ResolverRule{}, errors.WithStack(err)
	}
	resolverClient, err := r.awsClients.NewResolverClient(cluster.Region, cluster.IAMRoleARN)
	if err != nil {
		return ResolverRule{}, errors.WithStack(err)
	}

	logger.Info("Creating security group for the Resolver endpoints")
	securityGroupId, err := ec2Client.CreateSecurityGroupForResolverEndpoints(ctx, cluster.VPCId, getSecurityGroupName(cluster.Name))
	if err != nil {
		return ResolverRule{}, errors.WithStack(err)
	}

	logger.Info("Creating resolver rule", "domainName", getResolverRuleDomainName(cluster.Name, r.workloadClusterBaseDomain))
	resolverRule, err := resolverClient.CreateResolverRule(ctx, logger, cluster, securityGroupId, getResolverRuleDomainName(cluster.Name, r.workloadClusterBaseDomain), getResolverRuleName(cluster.Name))
	if err != nil {
		return ResolverRule{}, errors.WithStack(err)
	}

	return resolverRule, nil
}

func (r *Resolver) associateRule(ctx context.Context, logger logr.Logger, cluster Cluster, resolverRule ResolverRule) error {
	logger = logger.WithValues("resolverRuleId", resolverRule.RuleId, "awsAccount", r.dnsServer.AWSAccountId, "vpcId", r.dnsServer.VPCId)

	ramClient, err := r.awsClients.NewRAMClient(cluster.Region, cluster.IAMRoleARN)
	if err != nil {
		return errors.WithStack(err)
	}

	dnsServerResolverClient, err := r.awsClients.NewResolverClientWithExternalId(r.dnsServer.AWSRegion, cluster.IAMRoleARN, r.dnsServer.IAMRoleToAssume, r.dnsServer.IAMExternalId)
	if err != nil {
		return errors.WithStack(err)
	}

	logger.Info("Creating resource share item so we can share resolver rule with a different aws account")
	_, err = ramClient.CreateResourceShareWithContext(ctx, getResourceShareName(cluster.Name), true, []string{resolverRule.RuleArn}, []string{r.dnsServer.AWSAccountId})
	if err != nil {
		return errors.WithStack(err)
	}

	logger.Info("Associating resolver rule with VPC")
	err = dnsServerResolverClient.AssociateResolverRuleWithContext(ctx, getAssociationName(cluster.Name), r.dnsServer.VPCId, resolverRule.RuleId)
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func getSecurityGroupName(clusterName string) string {
	return fmt.Sprintf("%s-resolverrules-endpoints", clusterName)
}

func getResolverRuleName(clusterName string) string {
	return fmt.Sprintf("giantswarm-%s", clusterName)
}

func getResourceShareName(clusterName string) string {
	return fmt.Sprintf("giantswarm-%s-rr", clusterName)
}

func getAssociationName(clusterName string) string {
	return fmt.Sprintf("giantswarm-%s-rr-association", clusterName)
}

func getResolverRuleDomainName(clusterName, baseDomain string) string {
	return fmt.Sprintf("%s.%s", clusterName, baseDomain)
}
