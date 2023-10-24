package aws

import (
	"context"
	"fmt"
	"strings"
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
	inboundEndpointId, err := a.createResolverEndpointWithContext(ctx, logger, "INBOUND", getInboundEndpointName(cluster.Name), []string{securityGroupId}, cluster.Subnets, cluster.AdditionalTags)
	if err != nil {
		return resolver.ResolverRule{}, errors.WithStack(err)
	}

	outboundEndpointId, err := a.createResolverEndpointWithContext(ctx, logger, "OUTBOUND", getOutboundEndpointName(cluster.Name), []string{securityGroupId}, cluster.Subnets, cluster.AdditionalTags)
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
	resolverRule, err = a.createResolverRule(ctx, logger, domainName, resolverRuleName, outboundEndpointId, targetAddress, cluster.AdditionalTags)
	if err != nil {
		return resolver.ResolverRule{}, errors.WithStack(err)
	}

	return resolverRule, nil
}

// createResolverRule will create a new Resolver Rule.
// If the Rule already exists, it will try to fetch it to return the Rule ARN and ID.
func (a *AWSResolver) createResolverRule(ctx context.Context, logger logr.Logger, domainName, resolverRuleName, endpointId string, targetIps []*route53resolver.TargetAddress, tags map[string]string) (resolver.ResolverRule, error) {
	now := time.Now()
	response, err := a.client.CreateResolverRuleWithContext(ctx, &route53resolver.CreateResolverRuleInput{
		CreatorRequestId:   aws.String(fmt.Sprintf("%d", now.UnixNano())),
		DomainName:         aws.String(domainName),
		Name:               aws.String(resolverRuleName),
		ResolverEndpointId: aws.String(endpointId),
		RuleType:           aws.String("FORWARD"),
		TargetIps:          targetIps,
		Tags:               getRoute53ResolverTags(tags),
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

	resolverRule := a.buildResolverRule(response.ResolverRule)
	logger.Info("Created resolver rule", "rule", resolverRule)

	return resolverRule, nil
}

// CreateResolverEndpointWithContext creates a Resolver endpoint.
// It won't return an error if the endpoint already exists. Errors can be found here
// https://docs.aws.amazon.com/Route53/latest/APIReference/API_route53resolver_CreateResolverEndpoint.html#API_route53resolver_CreateResolverEndpoint_Errors
func (a *AWSResolver) createResolverEndpointWithContext(ctx context.Context, logger logr.Logger, direction, name string, securityGroupIds, subnetIds []string, tags map[string]string) (string, error) {
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
		Tags:             getRoute53ResolverTags(tags),
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
		return a.buildResolverRule(listRulesResponse.ResolverRules[0]), nil
	}

	return resolver.ResolverRule{}, &resolver.ResolverRuleNotFoundError{}
}

// AssociateResolverRuleWithContext creates an association between a resolver rule and a VPC.
// You cannot associate rules with same domain name with same VPC on AWS, in which case
// `AssociateResolverRuleWithContext` will log the error and ignore associating the rule with the VPC.
func (a *AWSResolver) AssociateResolverRuleWithContext(ctx context.Context, logger logr.Logger, associationName, vpcId, resolverRuleId string) error {
	logger = logger.WithValues("resolverRuleId", resolverRuleId, "vpcId", vpcId, "associationName", associationName)

	logger.Info("Associating Resolver Rule with the VPC")
	_, err := a.client.AssociateResolverRuleWithContext(ctx, &route53resolver.AssociateResolverRuleInput{
		Name:           aws.String(associationName),
		ResolverRuleId: aws.String(resolverRuleId),
		VPCId:          aws.String(vpcId),
	})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case route53resolver.ErrCodeInvalidRequestException:
				// We get a generic `InvalidRequestException` when we try to associate a resolver rule but there is another rule
				// with the same domain name and same VPC.
				// This controller will try to associate rules on every reconciliation loop, so to ignore that specific error
				// we need to check the contents of the error message. This is very brittle.
				if strings.Contains(aerr.Message(), "Cannot associate rules with same domain name with same VPC") {
					logger.Info("The Resolver Rule was already associated with the VPC (or there is another rule with same VPC and same domain name in rule), skipping associating the rule with the VPC again")
					return nil
				}
				return errors.WithStack(err)
			default:
				return errors.WithStack(err)
			}
		}

		return errors.WithStack(err)
	}

	logger.Info("The Resolver Rule has been associated with the VPC")

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

func (a *AWSResolver) FindResolverRulesByAWSAccountId(ctx context.Context, logger logr.Logger, awsAccountId string) ([]resolver.ResolverRule, error) {
	// Fetch first page of results.
	unfilteredResolverRules := []*route53resolver.ResolverRule{}
	listResolverRulesResponse, err := a.client.ListResolverRulesWithContext(ctx, &route53resolver.ListResolverRulesInput{
		MaxResults: aws.Int64(100),
		NextToken:  nil,
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	unfilteredResolverRules = append(unfilteredResolverRules, listResolverRulesResponse.ResolverRules...)

	// If the response contains `NexToken` we need to keep sending requests including the token to get all results.
	for listResolverRulesResponse.NextToken != nil && *listResolverRulesResponse.NextToken != "" {
		listResolverRulesResponse, err = a.client.ListResolverRulesWithContext(ctx, &route53resolver.ListResolverRulesInput{
			MaxResults: aws.Int64(100),
			NextToken:  listResolverRulesResponse.NextToken,
		})
		if err != nil {
			return nil, errors.WithStack(err)
		}
		unfilteredResolverRules = append(unfilteredResolverRules, listResolverRulesResponse.ResolverRules...)
	}

	resolverRulesInAccount := []resolver.ResolverRule{}
	for _, rule := range unfilteredResolverRules {
		if awsAccountId == "" || *rule.OwnerId == awsAccountId {
			resolverRulesInAccount = append(resolverRulesInAccount, a.buildResolverRule(rule))
		}
	}

	return resolverRulesInAccount, nil
}

func (a *AWSResolver) FindResolverRuleIdsAssociatedWithVPCId(ctx context.Context, logger logr.Logger, vpcId string) ([]string, error) {
	// Fetch first page of results.
	associatedResolverRuleIds := []string{}
	allResolverRuleAssociations := []*route53resolver.ResolverRuleAssociation{}
	listResolverRuleAssociationsResponse, err := a.client.ListResolverRuleAssociationsWithContext(ctx, &route53resolver.ListResolverRuleAssociationsInput{
		MaxResults: aws.Int64(100),
		NextToken:  nil,
		Filters: []*route53resolver.Filter{
			{
				Name:   aws.String("VPCId"),
				Values: aws.StringSlice([]string{vpcId}),
			},
		},
	})
	if err != nil {
		return associatedResolverRuleIds, errors.WithStack(err)
	}

	allResolverRuleAssociations = append(allResolverRuleAssociations, listResolverRuleAssociationsResponse.ResolverRuleAssociations...)

	// If the response contains `NexToken` we need to keep sending requests including the token to get all results.
	for listResolverRuleAssociationsResponse.NextToken != nil && *listResolverRuleAssociationsResponse.NextToken != "" {
		listResolverRuleAssociationsResponse, err = a.client.ListResolverRuleAssociationsWithContext(ctx, &route53resolver.ListResolverRuleAssociationsInput{
			MaxResults: aws.Int64(100),
			NextToken:  listResolverRuleAssociationsResponse.NextToken,
			Filters: []*route53resolver.Filter{
				{
					Name:   aws.String("VPCId"),
					Values: aws.StringSlice([]string{vpcId}),
				},
			},
		})
		if err != nil {
			return nil, errors.WithStack(err)
		}
		allResolverRuleAssociations = append(allResolverRuleAssociations, listResolverRuleAssociationsResponse.ResolverRuleAssociations...)
	}

	for _, association := range allResolverRuleAssociations {
		associatedResolverRuleIds = append(associatedResolverRuleIds, *association.ResolverRuleId)
	}

	return associatedResolverRuleIds, nil
}

func (a *AWSResolver) buildResolverRule(rule *route53resolver.ResolverRule) resolver.ResolverRule {
	ruleIPs := []string{}
	for _, ip := range rule.TargetIps {
		ruleIPs = append(ruleIPs, *ip.Ip)
	}

	return resolver.ResolverRule{
		Arn:  *rule.Arn,
		Id:   *rule.Id,
		IPs:  ruleIPs,
		Name: *rule.Name,
	}
}

func getInboundEndpointName(clusterName string) string {
	return fmt.Sprintf("%s-inbound", clusterName)
}

func getOutboundEndpointName(clusterName string) string {
	return fmt.Sprintf("%s-outbound", clusterName)
}

func getRoute53ResolverTags(t map[string]string) []*route53resolver.Tag {
	var tags []*route53resolver.Tag
	for key, value := range t {
		tags = append(tags, &route53resolver.Tag{
			Key:   aws.String(key),
			Value: aws.String(value),
		})
	}
	return tags
}
