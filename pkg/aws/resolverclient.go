package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/route53resolver"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	"github.com/aws-resolver-rules-operator/pkg/resolver"
)

type AWSResolver struct {
	client *route53resolver.Route53Resolver
}

func (a *AWSResolver) CreateResolverRule(ctx context.Context, logger logr.Logger, cluster resolver.Cluster, securityGroupId, domainName, resolverRuleName string) (resolver.ResolverRule, error) {
	resolverRule, err := a.GetResolverRuleByName(ctx, resolverRuleName, "FORWARD")
	if err != nil {
		if !errors.Is(err, &resolver.ResolverRuleNotFoundError{}) {
			return resolver.ResolverRule{}, errors.WithStack(err)
		}
	}

	// If we find it, we just return it.
	if err == nil {
		logger.Info("Resolver rule was already there, no need to create it", "rule", resolverRule)
		return resolverRule, nil
	}

	// Otherwise we create it.
	inboundEndpointId, err := a.createResolverEndpointWithContext(ctx, logger, "INBOUND", getInboundEndpointName(cluster.Name), []string{securityGroupId}, cluster.Subnets)
	if err != nil {
		return resolver.ResolverRule{}, errors.WithStack(err)
	}

	outboundEndpointId, err := a.createResolverEndpointWithContext(ctx, logger, "OUTBOUND", getOutboundEndpointName(cluster.Name), []string{securityGroupId}, cluster.Subnets)
	if err != nil {
		return resolver.ResolverRule{}, errors.WithStack(err)
	}

	endpointIpsResponse, err := a.client.ListResolverEndpointIpAddressesWithContext(ctx, &route53resolver.ListResolverEndpointIpAddressesInput{
		ResolverEndpointId: aws.String(inboundEndpointId),
	})
	if err != nil {
		return resolver.ResolverRule{}, errors.WithStack(err)
	}

	var targetAddress []*route53resolver.TargetAddress
	for _, ip := range endpointIpsResponse.IpAddresses {
		targetAddress = append(targetAddress, &route53resolver.TargetAddress{
			Ip:   ip.Ip,
			Port: aws.Int64(53),
		})
	}
	resolverRule, err = a.createResolverRule(ctx, logger, domainName, resolverRuleName, outboundEndpointId, targetAddress)
	if err != nil {
		return resolver.ResolverRule{}, errors.WithStack(err)
	}

	return resolverRule, nil
}

// createResolverRule will create a new Resolver Rule.
// If the Rule already exists, it will try to fetch it to return the Rule ARN and ID.
func (a *AWSResolver) createResolverRule(ctx context.Context, logger logr.Logger, domainName, resolverRuleName, endpointId string, targetIps []*route53resolver.TargetAddress) (resolver.ResolverRule, error) {
	now := time.Now()
	response, err := a.client.CreateResolverRuleWithContext(ctx, &route53resolver.CreateResolverRuleInput{
		CreatorRequestId:   aws.String(fmt.Sprintf("%d", now.UnixNano())),
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
				resolverRule, err := a.GetResolverRuleByName(ctx, resolverRuleName, "FORWARD")
				if err != nil {
					return resolver.ResolverRule{}, errors.WithStack(err)
				}
				return resolverRule, nil
			default:
				return resolver.ResolverRule{}, errors.WithStack(err)
			}
		}

		return resolver.ResolverRule{}, errors.WithStack(err)
	}

	resolverRule := resolver.ResolverRule{RuleArn: *response.ResolverRule.Arn, RuleId: *response.ResolverRule.Id}
	logger.Info("Created resolver rule", "rule", resolverRule)

	return resolverRule, nil
}

// CreateResolverEndpointWithContext creates a Resolver endpoint.
// It won't return an error if the endpoint already exists. Errors can be found here
// https://docs.aws.amazon.com/Route53/latest/APIReference/API_route53resolver_CreateResolverEndpoint.html#API_route53resolver_CreateResolverEndpoint_Errors
func (a *AWSResolver) createResolverEndpointWithContext(ctx context.Context, logger logr.Logger, direction, name string, securityGroupIds, subnetIds []string) (string, error) {
	resolverEndpoint, err := a.getResolverEndpoint(ctx, name)
	if err != nil {
		if !errors.Is(err, &ResolverEndpointNotFoundError{}) {
			return "", errors.WithStack(err)
		}
	}

	// If we find it, we just return it.
	if err == nil {
		logger.Info("Resolver endpoint was already there, no need to create it", "resolverEndpointName", name, "resolverEndpointDirection", direction)
		return *resolverEndpoint.Id, nil
	}

	// Otherwise we create it.
	now := time.Now()
	var ipAddresses []*route53resolver.IpAddressRequest
	for _, id := range subnetIds {
		ipAddresses = append(ipAddresses, &route53resolver.IpAddressRequest{
			SubnetId: aws.String(id),
		})
	}
	logger.Info("Creating resolver endpoint", "direction", direction, "endpointName", name, "securityGroupId", securityGroupIds, "subnetIds", subnetIds)
	response, err := a.client.CreateResolverEndpointWithContext(ctx, &route53resolver.CreateResolverEndpointInput{
		CreatorRequestId: aws.String(fmt.Sprintf("%d", now.UnixNano())),
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

func (a *AWSResolver) getResolverEndpoint(ctx context.Context, resolverEndpointName string) (*route53resolver.ResolverEndpoint, error) {
	listRulesResponse, err := a.client.ListResolverEndpointsWithContext(ctx, &route53resolver.ListResolverEndpointsInput{
		Filters: []*route53resolver.Filter{
			{
				Name:   aws.String("Name"),
				Values: aws.StringSlice([]string{resolverEndpointName}),
			},
		},
	})
	if err != nil {
		return &route53resolver.ResolverEndpoint{}, errors.WithStack(err)
	}

	if len(listRulesResponse.ResolverEndpoints) > 0 {
		return listRulesResponse.ResolverEndpoints[0], nil
	}

	return nil, &ResolverEndpointNotFoundError{}
}

func (a *AWSResolver) DeleteResolverRule(ctx context.Context, logger logr.Logger, cluster resolver.Cluster, resolverRuleId string) error {
	logger.Info("Deleting resolver rule", "resolverRuleId", resolverRuleId)
	err := a.deleteResolverRule(ctx, resolverRuleId)
	if err != nil {
		return errors.WithStack(err)
	}

	logger.Info("Deleting inbound resolver endpoint", "resolverEndpointName", getInboundEndpointName(cluster.Name))
	err = a.deleteResolverEndpoint(ctx, getInboundEndpointName(cluster.Name))
	if err != nil {
		return errors.WithStack(err)
	}

	logger.Info("Deleting outbound resolver endpoint", "resolverEndpointName", getOutboundEndpointName(cluster.Name))
	err = a.deleteResolverEndpoint(ctx, getOutboundEndpointName(cluster.Name))
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (a *AWSResolver) deleteResolverRule(ctx context.Context, resolverRuleId string) error {
	_, err := a.client.DeleteResolverRuleWithContext(ctx, &route53resolver.DeleteResolverRuleInput{ResolverRuleId: aws.String(resolverRuleId)})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case route53resolver.ErrCodeResourceNotFoundException:
				return nil
			default:
				return errors.WithStack(err)
			}
		}

		return errors.WithStack(err)
	}

	return nil
}

func (a *AWSResolver) deleteResolverEndpoint(ctx context.Context, resolverEndpointName string) error {
	resolverEndpoint, err := a.getResolverEndpoint(ctx, resolverEndpointName)
	if errors.Is(err, &ResolverEndpointNotFoundError{}) {
		return nil
	}
	if err != nil {
		return errors.WithStack(err)
	}

	_, err = a.client.DeleteResolverEndpointWithContext(ctx, &route53resolver.DeleteResolverEndpointInput{ResolverEndpointId: resolverEndpoint.Id})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case route53resolver.ErrCodeResourceNotFoundException:
				return nil
			default:
				return errors.WithStack(err)
			}
		}

		return errors.WithStack(err)
	}

	return nil
}

func (a *AWSResolver) GetResolverRuleByName(ctx context.Context, resolverRuleName, resolverRuleType string) (resolver.ResolverRule, error) {
	listRulesResponse, err := a.client.ListResolverRulesWithContext(ctx, &route53resolver.ListResolverRulesInput{
		Filters: []*route53resolver.Filter{
			{
				Name:   aws.String("Name"),
				Values: aws.StringSlice([]string{resolverRuleName}),
			},
			{
				Name:   aws.String("TYPE"),
				Values: aws.StringSlice([]string{resolverRuleType}),
			},
		},
	})
	if err != nil {
		return resolver.ResolverRule{}, errors.WithStack(err)
	}

	if len(listRulesResponse.ResolverRules) > 0 {
		return resolver.ResolverRule{
			RuleId:  *listRulesResponse.ResolverRules[0].Id,
			RuleArn: *listRulesResponse.ResolverRules[0].Arn,
		}, nil
	}

	return resolver.ResolverRule{}, &resolver.ResolverRuleNotFoundError{}
}

func (a *AWSResolver) AssociateResolverRuleWithContext(ctx context.Context, logger logr.Logger, associationName, vpcId, resolverRuleId string) error {
	logger = logger.WithValues("resolverRuleId", resolverRuleId, "vpcId", vpcId, "associationName", associationName)
	logger.Info("Checking if Resolver Rule is already associated to VPC")
	listResolverRuleAssociationsResponse, err := a.client.ListResolverRuleAssociationsWithContext(ctx, &route53resolver.ListResolverRuleAssociationsInput{
		Filters: []*route53resolver.Filter{
			{
				Name:   aws.String("Name"),
				Values: aws.StringSlice([]string{associationName}),
			},
			{
				Name:   aws.String("ResolverRuleId"),
				Values: aws.StringSlice([]string{resolverRuleId}),
			},
			{
				Name:   aws.String("VPCId"),
				Values: aws.StringSlice([]string{vpcId}),
			},
		},
	})
	if err != nil {
		return errors.WithStack(err)
	}

	// If the rule is already associated we just return.
	if len(listResolverRuleAssociationsResponse.ResolverRuleAssociations) > 0 {
		logger.Info("The Resolver Rule was already associated with the VPC")
		return nil
	}

	logger.Info("Associating Resolver Rule to VPC")
	_, err = a.client.AssociateResolverRuleWithContext(ctx, &route53resolver.AssociateResolverRuleInput{
		Name:           aws.String(associationName),
		ResolverRuleId: aws.String(resolverRuleId),
		VPCId:          aws.String(vpcId),
	})
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (a *AWSResolver) DisassociateResolverRuleWithContext(ctx context.Context, logger logr.Logger, vpcID, resolverRuleId string) error {
	logger = logger.WithValues("resolverRuleId", resolverRuleId, "resolverRuleType", "FORWARD", "vpcId", vpcID)

	logger.Info("Disassociating Resolver Rule from VPC")
	_, err := a.client.DisassociateResolverRuleWithContext(ctx, &route53resolver.DisassociateResolverRuleInput{
		ResolverRuleId: aws.String(resolverRuleId),
		VPCId:          aws.String(vpcID),
	})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case route53resolver.ErrCodeResourceNotFoundException:
				return nil
			default:
				return errors.WithStack(err)
			}
		}

		return errors.WithStack(err)
	}

	return nil
}

func getInboundEndpointName(clusterName string) string {
	return fmt.Sprintf("%s-inbound", clusterName)
}

func getOutboundEndpointName(clusterName string) string {
	return fmt.Sprintf("%s-outbound", clusterName)
}
