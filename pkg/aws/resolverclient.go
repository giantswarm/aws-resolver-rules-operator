package aws

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53resolver"
	"github.com/aws/aws-sdk-go-v2/service/route53resolver/types"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	"github.com/aws-resolver-rules-operator/pkg/resolver"
)

type AWSResolver struct {
	client *route53resolver.Client
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
	inboundEndpointId, err := a.createResolverEndpoint(ctx, logger, "INBOUND", getInboundEndpointName(cluster.Name), []string{securityGroupId}, cluster.Subnets, cluster.AdditionalTags)
	if err != nil {
		return resolver.ResolverRule{}, errors.WithStack(err)
	}

	outboundEndpointId, err := a.createResolverEndpoint(ctx, logger, "OUTBOUND", getOutboundEndpointName(cluster.Name), []string{securityGroupId}, cluster.Subnets, cluster.AdditionalTags)
	if err != nil {
		return resolver.ResolverRule{}, errors.WithStack(err)
	}

	endpointIpsResponse, err := a.client.ListResolverEndpointIpAddresses(ctx, &route53resolver.ListResolverEndpointIpAddressesInput{
		ResolverEndpointId: aws.String(inboundEndpointId),
	})
	if err != nil {
		return resolver.ResolverRule{}, errors.WithStack(err)
	}

	var targetAddress []types.TargetAddress
	for _, ip := range endpointIpsResponse.IpAddresses {
		targetAddress = append(targetAddress, types.TargetAddress{
			Ip:   ip.Ip,
			Port: aws.Int32(53),
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
func (a *AWSResolver) createResolverRule(ctx context.Context, logger logr.Logger, domainName, resolverRuleName, endpointId string, targetIps []types.TargetAddress, tags map[string]string) (resolver.ResolverRule, error) {
	now := time.Now()
	response, err := a.client.CreateResolverRule(ctx, &route53resolver.CreateResolverRuleInput{
		CreatorRequestId:   aws.String(fmt.Sprintf("%d", now.UnixNano())),
		DomainName:         aws.String(domainName),
		Name:               aws.String(resolverRuleName),
		ResolverEndpointId: aws.String(endpointId),
		RuleType:           types.RuleTypeOptionForward,
		TargetIps:          targetIps,
		Tags:               getRoute53ResolverTags(tags),
	})
	if err != nil {
		var ree *types.ResourceExistsException
		if errors.As(err, &ree) {
			resolverRule, err := a.GetResolverRuleByName(ctx, resolverRuleName, string(types.RuleTypeOptionForward))
			if err != nil {
				return resolver.ResolverRule{}, errors.WithStack(err)
			}
			return resolverRule, nil
		}
		return resolver.ResolverRule{}, errors.WithStack(err)
	}

	resolverRule := a.buildResolverRule(response.ResolverRule)
	logger.Info("Created resolver rule", "rule", resolverRule)

	return resolverRule, nil
}

// createResolverEndpoint creates a Resolver endpoint.
// It won't return an error if the endpoint already exists. Errors can be found here
// https://docs.aws.amazon.com/Route53/latest/APIReference/API_route53resolver_CreateResolverEndpoint.html#API_route53resolver_CreateResolverEndpoint_Errors
func (a *AWSResolver) createResolverEndpoint(ctx context.Context, logger logr.Logger, direction, name string, securityGroupIds, subnetIds []string, tags map[string]string) (string, error) {
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
	var ipAddresses []types.IpAddressRequest
	for _, id := range subnetIds {
		ipAddresses = append(ipAddresses, types.IpAddressRequest{
			SubnetId: aws.String(id),
		})
	}
	logger.Info("Creating resolver endpoint", "direction", direction, "endpointName", name, "securityGroupId", securityGroupIds, "subnetIds", subnetIds)
	response, err := a.client.CreateResolverEndpoint(ctx, &route53resolver.CreateResolverEndpointInput{
		CreatorRequestId: aws.String(fmt.Sprintf("%d", now.UnixNano())),
		Direction:        types.ResolverEndpointDirection(direction),
		IpAddresses:      ipAddresses,
		Name:             aws.String(name),
		SecurityGroupIds: securityGroupIds,
		Tags:             getRoute53ResolverTags(tags),
	})
	if err != nil {
		var ree *types.ResourceExistsException
		if errors.As(err, &ree) {
			endpointsResponse, err := a.client.ListResolverEndpoints(ctx, &route53resolver.ListResolverEndpointsInput{
				Filters: []types.Filter{
					{
						Name:   aws.String("Name"),
						Values: []string{name},
					},
				},
			})
			if err != nil {
				return "", errors.WithStack(err)
			}

			return *endpointsResponse.ResolverEndpoints[0].Id, nil
		}
		return "", errors.WithStack(err)
	}

	return *response.ResolverEndpoint.Id, nil
}

func (a *AWSResolver) getResolverEndpoint(ctx context.Context, resolverEndpointName string) (types.ResolverEndpoint, error) {
	listRulesResponse, err := a.client.ListResolverEndpoints(ctx, &route53resolver.ListResolverEndpointsInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("Name"),
				Values: []string{resolverEndpointName},
			},
		},
	})
	if err != nil {
		return types.ResolverEndpoint{}, errors.WithStack(err)
	}

	if len(listRulesResponse.ResolverEndpoints) > 0 {
		return listRulesResponse.ResolverEndpoints[0], nil
	}

	return types.ResolverEndpoint{}, &ResolverEndpointNotFoundError{}
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
	_, err := a.client.DeleteResolverRule(ctx, &route53resolver.DeleteResolverRuleInput{ResolverRuleId: aws.String(resolverRuleId)})
	if err != nil {
		var rnfe *types.ResourceNotFoundException
		if errors.As(err, &rnfe) {
			return nil
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

	_, err = a.client.DeleteResolverEndpoint(ctx, &route53resolver.DeleteResolverEndpointInput{ResolverEndpointId: resolverEndpoint.Id})
	if err != nil {
		var rnfe *types.ResourceNotFoundException
		if errors.As(err, rnfe) {
			return nil
		}
		return errors.WithStack(err)
	}

	return nil
}

func (a *AWSResolver) GetResolverRuleByName(ctx context.Context, resolverRuleName, resolverRuleType string) (resolver.ResolverRule, error) {
	listRulesResponse, err := a.client.ListResolverRules(ctx, &route53resolver.ListResolverRulesInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("Name"),
				Values: []string{resolverRuleName},
			},
			{
				Name:   aws.String("TYPE"),
				Values: []string{resolverRuleType},
			},
		},
	})
	if err != nil {
		return resolver.ResolverRule{}, errors.WithStack(err)
	}

	if len(listRulesResponse.ResolverRules) > 0 {
		return a.buildResolverRule(&listRulesResponse.ResolverRules[0]), nil
	}

	return resolver.ResolverRule{}, &resolver.ResolverRuleNotFoundError{}
}

// AssociateResolverRule creates an association between a resolver rule and a VPC.
// You cannot associate rules with same domain name with same VPC on AWS, in which case
// `AssociateResolverRule` will log the error and ignore associating the rule with the VPC.
func (a *AWSResolver) AssociateResolverRule(ctx context.Context, logger logr.Logger, associationName, vpcId, resolverRuleId string) error {
	logger = logger.WithValues("resolverRuleId", resolverRuleId, "vpcId", vpcId, "associationName", associationName)

	logger.Info("Associating Resolver Rule with the VPC")
	_, err := a.client.AssociateResolverRule(ctx, &route53resolver.AssociateResolverRuleInput{
		Name:           aws.String(associationName),
		ResolverRuleId: aws.String(resolverRuleId),
		VPCId:          aws.String(vpcId),
	})
	if err != nil {
		var ire *types.InvalidRequestException
		if errors.As(err, &ire) {
			// We get a generic `InvalidRequestException` when we try to associate a resolver rule but there is another rule
			// with the same domain name and same VPC.
			// This controller will try to associate rules on every reconciliation loop, so to ignore that specific error
			// we need to check the contents of the error message. This is very brittle.
			if strings.Contains(err.Error(), "Cannot associate rules with same domain name with same VPC") {
				logger.Info("The Resolver Rule was already associated with the VPC (or there is another rule with same VPC and same domain name in rule), skipping associating the rule with the VPC again")
				return nil
			}
			return errors.WithStack(err)
		}
		return errors.WithStack(err)
	}

	logger.Info("The Resolver Rule has been associated with the VPC")

	return nil
}

func (a *AWSResolver) DisassociateResolverRule(ctx context.Context, logger logr.Logger, vpcID, resolverRuleId string) error {
	logger = logger.WithValues("resolverRuleId", resolverRuleId, "resolverRuleType", "FORWARD", "vpcId", vpcID)

	logger.Info("Disassociating Resolver Rule from VPC")
	_, err := a.client.DisassociateResolverRule(ctx, &route53resolver.DisassociateResolverRuleInput{
		ResolverRuleId: aws.String(resolverRuleId),
		VPCId:          aws.String(vpcID),
	})
	if err != nil {
		var rnfe *types.ResourceNotFoundException
		if errors.As(err, &rnfe) {
			return nil
		}
		return errors.WithStack(err)
	}

	return nil
}

func (a *AWSResolver) FindResolverRulesByAWSAccountId(ctx context.Context, logger logr.Logger, awsAccountId string) ([]resolver.ResolverRule, error) {
	// Fetch first page of results.
	unfilteredResolverRules := []types.ResolverRule{}
	listResolverRulesResponse, err := a.client.ListResolverRules(ctx, &route53resolver.ListResolverRulesInput{
		MaxResults: aws.Int32(100),
		NextToken:  nil,
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	unfilteredResolverRules = append(unfilteredResolverRules, listResolverRulesResponse.ResolverRules...)

	// If the response contains `NexToken` we need to keep sending requests including the token to get all results.
	for listResolverRulesResponse.NextToken != nil && *listResolverRulesResponse.NextToken != "" {
		listResolverRulesResponse, err = a.client.ListResolverRules(ctx, &route53resolver.ListResolverRulesInput{
			MaxResults: aws.Int32(100),
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
			resolverRulesInAccount = append(resolverRulesInAccount, a.buildResolverRule(&rule))
		}
	}

	return resolverRulesInAccount, nil
}

func (a *AWSResolver) FindResolverRuleIdsAssociatedWithVPCId(ctx context.Context, logger logr.Logger, vpcId string) ([]string, error) {
	// Fetch first page of results.
	associatedResolverRuleIds := []string{}
	allResolverRuleAssociations := []types.ResolverRuleAssociation{}
	listResolverRuleAssociationsResponse, err := a.client.ListResolverRuleAssociations(ctx, &route53resolver.ListResolverRuleAssociationsInput{
		MaxResults: aws.Int32(100),
		NextToken:  nil,
		Filters: []types.Filter{
			{
				Name:   aws.String("VPCId"),
				Values: []string{vpcId},
			},
		},
	})
	if err != nil {
		return associatedResolverRuleIds, errors.WithStack(err)
	}

	allResolverRuleAssociations = append(allResolverRuleAssociations, listResolverRuleAssociationsResponse.ResolverRuleAssociations...)

	// If the response contains `NexToken` we need to keep sending requests including the token to get all results.
	for listResolverRuleAssociationsResponse.NextToken != nil && *listResolverRuleAssociationsResponse.NextToken != "" {
		listResolverRuleAssociationsResponse, err = a.client.ListResolverRuleAssociations(ctx, &route53resolver.ListResolverRuleAssociationsInput{
			MaxResults: aws.Int32(100),
			NextToken:  listResolverRuleAssociationsResponse.NextToken,
			Filters: []types.Filter{
				{
					Name:   aws.String("VPCId"),
					Values: []string{vpcId},
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

func (a *AWSResolver) buildResolverRule(rule *types.ResolverRule) resolver.ResolverRule {
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

func getRoute53ResolverTags(t map[string]string) []types.Tag {
	var tags []types.Tag
	for key, value := range t {
		tags = append(tags, types.Tag{
			Key:   aws.String(key),
			Value: aws.String(value),
		})
	}
	return tags
}
