package aws

import (
	"context"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/route53resolver"
)

type AWSResolver struct {
	client *route53resolver.Route53Resolver
}

func (a *AWSResolver) CreateResolverRuleWithContext(ctx context.Context, domainName, resolverRuleName, endpointId, kind string, targetIPs []string) (string, string, error) {
	var targetAddress []*route53resolver.TargetAddress
	for _, ip := range targetIPs {
		targetAddress = append(targetAddress, &route53resolver.TargetAddress{
			Ip: aws.String(ip),
		})
	}
	response, err := a.client.CreateResolverRuleWithContext(ctx, &route53resolver.CreateResolverRuleInput{
		DomainName:         aws.String(domainName),
		Name:               aws.String(resolverRuleName),
		ResolverEndpointId: aws.String(endpointId),
		RuleType:           aws.String(kind),
		TargetIps:          targetAddress,
	})
	if err != nil {
		return "", "", err
	}

	return *response.ResolverRule.Arn, *response.ResolverRule.Id, nil
}

func (a *AWSResolver) AssociateResolverRuleWithContext(ctx context.Context, associationName, vpcID, resolverRuleId string) (string, error) {
	response, err := a.client.AssociateResolverRuleWithContext(ctx, &route53resolver.AssociateResolverRuleInput{
		Name:           aws.String(associationName),
		ResolverRuleId: aws.String(resolverRuleId),
		VPCId:          aws.String(vpcID),
	})
	if err != nil {
		return "", err
	}

	return *response.ResolverRuleAssociation.Id, nil
}

func (a *AWSResolver) CreateResolverEndpointWithContext(ctx context.Context, direction, name string, securityGroupIds, subnetIds []string) (string, error) {
	var ipAddresses []*route53resolver.IpAddressRequest
	for _, id := range subnetIds {
		ipAddresses = append(ipAddresses, &route53resolver.IpAddressRequest{
			SubnetId: aws.String(id),
		})
	}
	response, err := a.client.CreateResolverEndpointWithContext(ctx, &route53resolver.CreateResolverEndpointInput{
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
