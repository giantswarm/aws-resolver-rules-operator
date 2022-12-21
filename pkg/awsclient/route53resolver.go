package awsclient

import (
	"context"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/route53resolver"
)

type AWSRoute53Resolver struct {
	Route53resolverClient *route53resolver.Route53Resolver
}

func (a *AWSRoute53Resolver) CreateResolverRuleWithContext(ctx context.Context, domainName, resolverRuleName, endpointId, kind string, targetIPs []string) (string, error) {
	var targetAddress []*route53resolver.TargetAddress
	for _, ip := range targetIPs {
		targetAddress = append(targetAddress, &route53resolver.TargetAddress{
			Ip: aws.String(ip),
		})
	}
	response, err := a.Route53resolverClient.CreateResolverRuleWithContext(ctx, &route53resolver.CreateResolverRuleInput{
		DomainName:         aws.String(domainName),
		Name:               aws.String(resolverRuleName),
		ResolverEndpointId: aws.String(endpointId),
		RuleType:           aws.String(kind),
		TargetIps:          targetAddress,
	})
	if err != nil {
		return "", err
	}

	return *response.ResolverRule.Id, nil
}

func (a *AWSRoute53Resolver) AssociateResolverRuleWithContext(ctx context.Context, associationName, vpcID, resolverRuleId string) (string, error) {
	panic("implement me")
}

func (a *AWSRoute53Resolver) CreateResolverEndpointWithContext(ctx context.Context, direction, name string, securityGroupIds, subnetIds []string) (string, error) {
	var ipAddresses []*route53resolver.IpAddressRequest
	for _, id := range subnetIds {
		ipAddresses = append(ipAddresses, &route53resolver.IpAddressRequest{
			SubnetId: aws.String(id),
		})
	}
	response, err := a.Route53resolverClient.CreateResolverEndpointWithContext(ctx, &route53resolver.CreateResolverEndpointInput{
		Direction:        aws.String(direction),
		IpAddresses:      ipAddresses,
		Name:             aws.String(name),
		SecurityGroupIds: aws.StringSlice(securityGroupIds),
	})

	if err != nil {
		return "", err
	}

	return *response.ResolverEndpoint.Id, nil
}
