package resolver

import (
	"context"

	"github.com/aws/aws-sdk-go/service/route53"
	"github.com/go-logr/logr"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

type AWSClients interface {
	NewResolverClient(region, arn string) (ResolverClient, error)
	NewResolverClientWithExternalId(region, roleToAssume, externalRoleToAssume, externalId string) (ResolverClient, error)
	NewEC2Client(region, arn string) (EC2Client, error)
	NewEC2ClientWithExternalId(region, arn, externalId string) (EC2Client, error)
	NewRAMClient(region, arn string) (RAMClient, error)
	NewRAMClientWithExternalId(region, arn, externalId string) (RAMClient, error)
	NewRoute53Client(region, arn string) (Route53Client, error)
}

//counterfeiter:generate . EC2Client
type EC2Client interface {
	CreateSecurityGroupForResolverEndpoints(ctx context.Context, vpcId, groupName string) (string, error)
	DeleteSecurityGroupForResolverEndpoints(ctx context.Context, logger logr.Logger, vpcId, groupName string) error
}

//counterfeiter:generate . RAMClient
type RAMClient interface {
	CreateResourceShareWithContext(ctx context.Context, logger logr.Logger, resourceShareName string, resourceArns, principals string) (string, error)
	DeleteResourceShareWithContext(ctx context.Context, logger logr.Logger, resourceShareName string) error
}

//counterfeiter:generate . Route53Client
type Route53Client interface {
	CreateHostedZone(ctx context.Context, logger logr.Logger, dnsZone DnsZone) (string, error)
	DeleteHostedZone(ctx context.Context, logger logr.Logger, zoneId string) error
	GetHostedZoneIdByName(ctx context.Context, logger logr.Logger, zoneName string) (string, error)
	GetHostedZoneNSRecords(ctx context.Context, zoneId string) (*route53.ResourceRecordSet, error)
	AddDelegationToParentZone(ctx context.Context, logger logr.Logger, parentZoneId string, resourceRecord *route53.ResourceRecordSet) error
	DeleteDelegationFromParentZone(ctx context.Context, logger logr.Logger, parentZoneId, zoneId string) error
	AddDnsRecordsToHostedZone(ctx context.Context, logger logr.Logger, hostedZoneId string, dnsRecords []DNSRecord) error
	DeleteDnsRecordsFromHostedZone(ctx context.Context, logger logr.Logger, hostedZoneId string) error
}

//counterfeiter:generate . ResolverClient
type ResolverClient interface {
	GetResolverRuleByName(ctx context.Context, resolverRuleName, resolverRuleType string) (ResolverRule, error)
	CreateResolverRule(ctx context.Context, logger logr.Logger, cluster Cluster, securityGroupId, domainName, resolverRuleName string) (ResolverRule, error)
	DeleteResolverRule(ctx context.Context, logger logr.Logger, cluster Cluster, resolverRuleId string) error
	AssociateResolverRuleWithContext(ctx context.Context, logger logr.Logger, associationName, vpcID, resolverRuleId string) error
	DisassociateResolverRuleWithContext(ctx context.Context, logger logr.Logger, vpcID, resolverRuleId string) error
	FindResolverRulesByAWSAccountId(ctx context.Context, logger logr.Logger, awsAccountId string) ([]ResolverRule, error)
	FindResolverRuleIdsAssociatedWithVPCId(ctx context.Context, logger logr.Logger, vpcId string) ([]string, error)
}
