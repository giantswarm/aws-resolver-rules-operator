package resolver

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type Resolver struct {
	awsClients                AWSClients
	dnsServerAWSAccountId     string
	dnsServerResolverClient   ResolverClient
	dnsServerVPCId            string
	workloadClusterBaseDomain string
}

type AWSClients interface {
	NewResolverClient(region, arn, externalId string) (ResolverClient, error)
	NewEC2Client(region, arn string) (EC2Client, error)
	NewRAMClient(region, arn string) (RAMClient, error)
}

//counterfeiter:generate . EC2Client
type EC2Client interface {
	CreateSecurityGroupWithContext(ctx context.Context, vpcId, description, groupName string) (string, error)
	AuthorizeSecurityGroupIngressWithContext(ctx context.Context, securityGroupId, protocol string, port int) error
}

//counterfeiter:generate . RAMClient
type RAMClient interface {
	CreateResourceShareWithContext(ctx context.Context, resourceShareName string, allowExternalPrincipals bool, resourceArns, principals []string) (string, error)
}

//counterfeiter:generate . ResolverClient
type ResolverClient interface {
	CreateResolverRuleWithContext(ctx context.Context, domainName, resolverRuleName, endpointId, kind string, targetIPs []string) (string, string, error)
	AssociateResolverRuleWithContext(ctx context.Context, associationName, vpcID, resolverRuleId string) (string, error)
	CreateResolverEndpointWithContext(ctx context.Context, direction, name string, securityGroupIds, subnetIds []string) (string, error)
}

type AssociatedResolverRule struct {
	RuleArn       string
	RuleName      string
	AssociationId string
}

const (
	DNSPort                  = 53
	SecurityGroupDescription = "Security group for resolver rule endpoints"
)

func NewResolver(awsClients AWSClients, dnsServerResolverClient ResolverClient, dnsServerAWSAccountId, dnsServerVPCId, workloadClusterBaseDomain string) (Resolver, error) {
	return Resolver{
		awsClients:                awsClients,
		dnsServerResolverClient:   dnsServerResolverClient,
		dnsServerAWSAccountId:     dnsServerAWSAccountId,
		dnsServerVPCId:            dnsServerVPCId,
		workloadClusterBaseDomain: workloadClusterBaseDomain,
	}, nil
}

func (r *Resolver) CreateRule(ctx context.Context, clusterName, clusterRegion, clusterArn, clusterVPCId string, clusterSubnetIds []string) (AssociatedResolverRule, error) {
	resolverRuleARN, resolverRuleId, err := r.createRule(ctx, clusterName, clusterRegion, clusterArn, clusterVPCId, clusterSubnetIds)
	if err != nil {
		return AssociatedResolverRule{}, errors.WithStack(err)
	}

	associationId, err := r.associateRule(ctx, clusterName, clusterRegion, clusterArn, resolverRuleARN, resolverRuleId)
	if err != nil {
		return AssociatedResolverRule{}, errors.WithStack(err)
	}

	return AssociatedResolverRule{resolverRuleARN, resolverRuleId, associationId}, nil
}

func (r *Resolver) createRule(ctx context.Context, clusterName, clusterRegion, clusterArn, clusterVPCId string, clusterSubnetIds []string) (string, string, error) {
	logger := log.FromContext(ctx)

	ec2Client, err := r.awsClients.NewEC2Client(clusterRegion, clusterArn)
	if err != nil {
		return "", "", errors.WithStack(err)
	}
	resolverClient, err := r.awsClients.NewResolverClient(clusterRegion, clusterArn, "")
	if err != nil {
		return "", "", errors.WithStack(err)
	}

	logger.Info("Creating security group for the Resolver endpoints")
	securityGroupId, err := ec2Client.CreateSecurityGroupWithContext(ctx, clusterVPCId, SecurityGroupDescription, getSecurityGroupName(clusterName))
	if err != nil {
		return "", "", errors.WithStack(err)
	}

	logger.WithValues("securityGroupId", securityGroupId)

	logger.Info("Creating ingress rules in the security group")
	err = ec2Client.AuthorizeSecurityGroupIngressWithContext(ctx, securityGroupId, "udp", DNSPort)
	if err != nil {
		return "", "", errors.WithStack(err)
	}

	err = ec2Client.AuthorizeSecurityGroupIngressWithContext(ctx, securityGroupId, "tcp", DNSPort)
	if err != nil {
		return "", "", errors.WithStack(err)
	}

	logger.Info("Creating inbound resolver endpoint")
	inboundEndpointId, err := resolverClient.CreateResolverEndpointWithContext(ctx, "INBOUND", getInboundEndpointName(clusterName), []string{securityGroupId}, clusterSubnetIds)
	if err != nil {
		return "", "", errors.WithStack(err)
	}

	logger.WithValues("inboundEndpointId", inboundEndpointId)

	logger.Info("Creating outbound resolver endpoint")
	outboundEndpointId, err := resolverClient.CreateResolverEndpointWithContext(ctx, "OUTBOUND", getOutboundEndpointName(clusterName), []string{securityGroupId}, clusterSubnetIds)
	if err != nil {
		return "", "", errors.WithStack(err)
	}

	logger.WithValues("outboundEndpointId", outboundEndpointId)

	logger.Info("Creating resolver rule", "type", "FORWARD", "domainName", getResolverRuleDomainName(clusterName, r.workloadClusterBaseDomain))
	resolverRuleArn, resolverRuleId, err := resolverClient.CreateResolverRuleWithContext(ctx, getResolverRuleDomainName(clusterName, r.workloadClusterBaseDomain), getResolverRuleName(clusterName), outboundEndpointId, "FORWARD", clusterSubnetIds)
	if err != nil {
		return "", "", errors.WithStack(err)
	}

	logger.WithValues("resolverRuleId", resolverRuleId, "resolverRuleArn", resolverRuleArn, "inboundEndpointId", inboundEndpointId, "outboundEndpointId", outboundEndpointId, "securityGroupId", securityGroupId)

	return resolverRuleArn, resolverRuleId, nil
}

func (r *Resolver) associateRule(ctx context.Context, clusterName, clusterRegion, clusterArn, resolverRuleARN, resolverRuleId string) (string, error) {
	logger := log.FromContext(ctx)

	ramClient, err := r.awsClients.NewRAMClient(clusterRegion, clusterArn)
	if err != nil {
		return "", errors.WithStack(err)
	}

	logger.Info("Creating resource share item so we can share resolver rule with a different aws account", "awsAccount", r.dnsServerAWSAccountId)
	_, err = ramClient.CreateResourceShareWithContext(ctx, getResourceShareName(clusterName), true, []string{resolverRuleARN}, []string{r.dnsServerAWSAccountId})
	if err != nil {
		return "", errors.WithStack(err)
	}

	logger.Info("Associating resolver rule with VPC", "vpcId", r.dnsServerVPCId)
	associationId, err := r.dnsServerResolverClient.AssociateResolverRuleWithContext(ctx, getAssociationName(clusterName), r.dnsServerVPCId, resolverRuleId)
	if err != nil {
		return "", errors.WithStack(err)
	}

	logger.WithValues("associationId", associationId)

	return associationId, nil
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

func getInboundEndpointName(clusterName string) string {
	return fmt.Sprintf("%s-inbound", clusterName)
}

func getOutboundEndpointName(clusterName string) string {
	return fmt.Sprintf("%s-outbound", clusterName)
}

func getResolverRuleDomainName(clusterName, baseDomain string) string {
	return fmt.Sprintf("%s.%s", clusterName, baseDomain)
}