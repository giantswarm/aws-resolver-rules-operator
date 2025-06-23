package resolver

import (
	"context"

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
	NewS3Client(region, arn string) (S3Client, error)
	NewTransitGatewayClient(region, arn string) (TransitGatewayClient, error)
	NewPrefixListClient(region, arn string) (PrefixListClient, error)
	NewRouteTableClient(region, arn string) (RouteTableClient, error)
}

//counterfeiter:generate . EC2Client
type EC2Client interface {
	CreateSecurityGroupForResolverEndpoints(ctx context.Context, vpcId, groupName string, tags map[string]string) (string, error)
	DeleteSecurityGroupForResolverEndpoints(ctx context.Context, logger logr.Logger, vpcId, groupName string) error
	TerminateInstancesByTag(ctx context.Context, logger logr.Logger, tagKey, tagValue string) ([]string, error)
}

type ResourceShare struct {
	Name              string
	ResourceArns      []string
	ExternalAccountID string
}

//counterfeiter:generate . RAMClient
type RAMClient interface {
	ApplyResourceShare(context.Context, ResourceShare) error
	DeleteResourceShare(context.Context, string) error
}

//counterfeiter:generate . Route53Client
type Route53Client interface {
	CreateHostedZone(ctx context.Context, logger logr.Logger, dnsZone DnsZone) (string, error)
	DeleteHostedZone(ctx context.Context, logger logr.Logger, zoneId string) error
	GetHostedZoneIdByName(ctx context.Context, logger logr.Logger, zoneName string) (string, error)
	GetHostedZoneNSRecord(ctx context.Context, logger logr.Logger, zoneId string, zoneName string) (*DNSRecord, error)
	AddDelegationToParentZone(ctx context.Context, logger logr.Logger, parentZoneId string, resourceRecord *DNSRecord) error
	DeleteDelegationFromParentZone(ctx context.Context, logger logr.Logger, parentZoneId string, resourceRecord *DNSRecord) error
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

type TransitGatewayAttachment struct {
	TransitGatewayARN string
	SubnetIDs         []string
	VPCID             string
	Tags              map[string]string
}

//counterfeiter:generate . TransitGatewayClient
type TransitGatewayClient interface {
	Apply(context.Context, string, map[string]string) (string, error)
	ApplyAttachment(context.Context, TransitGatewayAttachment) error
	Detach(context.Context, TransitGatewayAttachment) error
	Delete(context.Context, string) error
}

type PrefixListEntry struct {
	PrefixListARN string
	CIDR          string
	Description   string
}

//counterfeiter:generate . PrefixListClient
type PrefixListClient interface {
	Apply(context.Context, string, map[string]string) (string, error)
	ApplyEntry(context.Context, PrefixListEntry) error
	DeleteEntry(context.Context, PrefixListEntry) error
	Delete(context.Context, string) error
}

type Filter []string

type RouteRule struct {
	DestinationPrefixListId string
	TransitGatewayId        string
}

//counterfeiter:generate . RouteTableClient
type RouteTableClient interface {
	AddRoutes(ctx context.Context, routeRule RouteRule, filter Filter) error
	RemoveRoutes(ctx context.Context, routeRule RouteRule, filter Filter) error
}

//counterfeiter:generate . S3Client
type S3Client interface {
	Put(ctx context.Context, bucket, path string, data []byte) error
}
