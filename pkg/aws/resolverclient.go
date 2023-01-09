package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/route53resolver"
	"github.com/pkg/errors"

	"github.com/aws-resolver-rules-operator/pkg/resolver"
)

type AWSResolver struct {
	client *route53resolver.Route53Resolver
}

func (a *AWSResolver) CreateResolverRule(ctx context.Context, cluster resolver.Cluster, securityGroupId, domainName, resolverRuleName string) (string, string, error) {
	inboundEndpointId, err := a.createResolverEndpointWithContext(ctx, "INBOUND", getInboundEndpointName(cluster.Name), []string{securityGroupId}, cluster.Subnets)
	if err != nil {
		return "", "", errors.WithStack(err)
	}

	outboundEndpointId, err := a.createResolverEndpointWithContext(ctx, "OUTBOUND", getOutboundEndpointName(cluster.Name), []string{securityGroupId}, cluster.Subnets)
	if err != nil {
		return "", "", errors.WithStack(err)
	}

	endpointIpsResponse, err := a.client.ListResolverEndpointIpAddressesWithContext(ctx, &route53resolver.ListResolverEndpointIpAddressesInput{
		ResolverEndpointId: aws.String(inboundEndpointId),
	})
	if err != nil {
		return "", "", errors.WithStack(err)
	}

	var targetAddress []*route53resolver.TargetAddress
	for _, ip := range endpointIpsResponse.IpAddresses {
		targetAddress = append(targetAddress, &route53resolver.TargetAddress{
			Ip: ip.Ip,
		})
	}
	resolverRuleArn, resolverRuleId, err := a.createResolverRule(ctx, domainName, resolverRuleName, outboundEndpointId, targetAddress)
	if err != nil {
		return "", "", errors.WithStack(err)
	}

	return resolverRuleArn, resolverRuleId, nil
}

// createResolverRule will create a new Resolver Rule.
// If the Rule already exists, it will try to fetch it to return the Rule ARN and ID.
func (a *AWSResolver) createResolverRule(ctx context.Context, domainName, resolverRuleName, endpointId string, targetIps []*route53resolver.TargetAddress) (string, string, error) {
	response, err := a.client.CreateResolverRuleWithContext(ctx, &route53resolver.CreateResolverRuleInput{
		DomainName:         aws.String(domainName),
		Name:               aws.String(resolverRuleName),
		ResolverEndpointId: aws.String(endpointId),
		RuleType:           aws.String("FORWARD"),
		TargetIps:          targetIps,
	})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case route53resolver.ErrCodeResourceExistsException:
				listRulesResponse, err := a.client.ListResolverRulesWithContext(ctx, &route53resolver.ListResolverRulesInput{
					Filters: []*route53resolver.Filter{
						{
							Name:   aws.String("Name"),
							Values: aws.StringSlice([]string{resolverRuleName}),
						},
					},
				})
				if err != nil {
					return "", "", errors.WithStack(err)
				}
				return *listRulesResponse.ResolverRules[0].Arn, *listRulesResponse.ResolverRules[0].Id, nil
			default:
				return "", "", errors.WithStack(err)
			}
		}

		return "", "", errors.WithStack(err)
	}

	return *response.ResolverRule.Arn, *response.ResolverRule.Id, nil
}

func (a *AWSResolver) AssociateResolverRuleWithContext(ctx context.Context, associationName, vpcID, resolverRuleId string) error {
	_, err := a.client.AssociateResolverRuleWithContext(ctx, &route53resolver.AssociateResolverRuleInput{
		Name:           aws.String(associationName),
		ResolverRuleId: aws.String(resolverRuleId),
		VPCId:          aws.String(vpcID),
	})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case route53resolver.ErrCodeResourceExistsException:
				return nil
			default:
				return errors.WithStack(err)
			}
		}

		return errors.WithStack(err)
	}

	return nil
}

// CreateResolverEndpointWithContext creates a Resolver endpoint.
// It won't return an error if the endpoint already exists. Errors can be found here
// https://docs.aws.amazon.com/Route53/latest/APIReference/API_route53resolver_CreateResolverEndpoint.html#API_route53resolver_CreateResolverEndpoint_Errors
func (a *AWSResolver) createResolverEndpointWithContext(ctx context.Context, direction, name string, securityGroupIds, subnetIds []string) (string, error) {
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
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case route53resolver.ErrCodeResourceExistsException:
				endpointsResponse, err := a.client.ListResolverEndpointsWithContext(ctx, &route53resolver.ListResolverEndpointsInput{
					Filters: []*route53resolver.Filter{
						{
							Name:   aws.String("Name"),
							Values: aws.StringSlice([]string{name}),
						},
					},
				})
				if err != nil {
					return "", errors.WithStack(err)
				}

				return *endpointsResponse.ResolverEndpoints[0].Id, nil
			default:
				return "", errors.WithStack(err)
			}
		}

		return "", errors.WithStack(err)
	}

	return *response.ResolverEndpoint.Id, nil
}

func getInboundEndpointName(clusterName string) string {
	return fmt.Sprintf("%s-inbound", clusterName)
}

func getOutboundEndpointName(clusterName string) string {
	return fmt.Sprintf("%s-outbound", clusterName)
}
