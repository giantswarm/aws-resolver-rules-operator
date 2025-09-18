package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/smithy-go"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
)

type AWSEC2 struct {
	client *ec2.Client
}

const (
	DNSPort                  = 53
	SecurityGroupDescription = "Security group for resolver rule endpoints"
)

// CreateSecurityGroupForResolverEndpoints creates a security group on EC2. It will NOT return error if the security group already exists.
// The error returned by the AWS SDK when a security group already exists can be found here
// https://docs.aws.amazon.com/AWSEC2/latest/APIReference/errors-overview.html#api-error-codes-table-client
func (a *AWSEC2) CreateSecurityGroupForResolverEndpoints(ctx context.Context, vpcId, groupName string, tags map[string]string) (string, error) {
	securityGroupId, err := a.createSecurityGroup(ctx, vpcId, groupName, tags)
	if err != nil {
		return "", errors.WithStack(err)
	}

	err = a.authorizeSecurityGroupIngressWithContext(ctx, securityGroupId, "udp", "0.0.0.0/0", DNSPort, tags)
	if err != nil {
		return "", errors.WithStack(err)
	}

	err = a.authorizeSecurityGroupIngressWithContext(ctx, securityGroupId, "tcp", "0.0.0.0/0", DNSPort, tags)
	if err != nil {
		return "", errors.WithStack(err)
	}

	return securityGroupId, nil
}

func (a *AWSEC2) DeleteSecurityGroupForResolverEndpoints(ctx context.Context, logger logr.Logger, vpcId, groupName string) error {
	logger.Info("Trying to find Resolver Rule security group", "securityGroupName", groupName, "vpcId", vpcId)
	securityGroup, err := a.getSecurityGroupByName(ctx, vpcId, groupName)
	if errors.Is(err, &SecurityGroupNotFoundError{}) {
		logger.Info("Security Group was not found, skipping deletion")
		return nil
	}
	if err != nil {
		return errors.WithStack(err)
	}

	logger.Info("Deleting security group", "securityGroupName", groupName)
	_, err = a.client.DeleteSecurityGroup(ctx, &ec2.DeleteSecurityGroupInput{GroupId: securityGroup.GroupId})
	if err != nil {
		var gae *smithy.GenericAPIError
		if errors.As(err, &gae) {
			if gae.Code == "InvalidGroup.NotFound" {
				return nil
			}
		}
		return errors.WithStack(err)
	}

	return nil
}

func (a *AWSEC2) getSecurityGroupByName(ctx context.Context, vpcId, groupName string) (ec2types.SecurityGroup, error) {
	securityGroupResponse, err := a.client.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{
		Filters: []ec2types.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []string{vpcId},
			},
			{
				Name:   aws.String("group-name"),
				Values: []string{groupName},
			},
		},
	})
	if err != nil {
		return ec2types.SecurityGroup{}, errors.WithStack(err)
	}

	if len(securityGroupResponse.SecurityGroups) < 1 {
		return ec2types.SecurityGroup{}, &SecurityGroupNotFoundError{}
	}

	return securityGroupResponse.SecurityGroups[0], nil
}

func (a *AWSEC2) createSecurityGroup(ctx context.Context, vpcId, groupName string, tags map[string]string) (string, error) {
	response, err := a.client.CreateSecurityGroup(ctx, &ec2.CreateSecurityGroupInput{
		Description: aws.String(SecurityGroupDescription),
		GroupName:   aws.String(groupName),
		VpcId:       aws.String(vpcId),
		TagSpecifications: []ec2types.TagSpecification{
			{
				ResourceType: ec2types.ResourceTypeSecurityGroup,
				Tags:         getEc2Tags(tags),
			},
		},
	})
	if err != nil {
		var gae *smithy.GenericAPIError
		if errors.As(err, &gae) {
			if gae.Code == "InvalidGroup.Duplicate" {
				securityGroup, err := a.getSecurityGroupByName(ctx, vpcId, groupName)
				if err != nil {
					return "", errors.WithStack(err)
				}
				return *securityGroup.GroupId, nil
			}
		}
		return "", errors.WithStack(err)
	}

	return *response.GroupId, nil
}

// authorizeSecurityGroupIngressWithContext adds the specified inbound (ingress) rules to a security group.
// It won't return an error if the rule already exists for the security group. Errors can be found here
// https://docs.aws.amazon.com/AWSEC2/latest/APIReference/errors-overview.html#CommonErrors
func (a *AWSEC2) authorizeSecurityGroupIngressWithContext(ctx context.Context, securityGroupId, protocol, cidr string, port int32, tags map[string]string) error {
	_, err := a.client.AuthorizeSecurityGroupIngress(ctx, &ec2.AuthorizeSecurityGroupIngressInput{
		FromPort:   aws.Int32(port),
		GroupId:    aws.String(securityGroupId),
		IpProtocol: aws.String(protocol),
		ToPort:     aws.Int32(port),
		CidrIp:     aws.String(cidr),
		TagSpecifications: []ec2types.TagSpecification{
			{
				ResourceType: ec2types.ResourceTypeSecurityGroupRule,
				Tags:         getEc2Tags(tags),
			},
		},
	})
	if err != nil {
		var gae *smithy.GenericAPIError
		if errors.As(err, &gae) {
			if gae.Code == "InvalidPermission.Duplicate" {
				return nil
			}
		}
		return errors.WithStack(err)
	}

	return nil
}

func getEc2Tags(t map[string]string) []ec2types.Tag {
	var tags []ec2types.Tag
	for k, v := range t {
		tags = append(tags, ec2types.Tag{
			Key:   aws.String(k),
			Value: aws.String(v),
		})
	}
	return tags
}

// TerminateInstancesByTag terminates all EC2 instances that have the specified tag key and value.
func (a *AWSEC2) TerminateInstancesByTag(ctx context.Context, logger logr.Logger, tagKey, tagValue string) ([]string, error) {
	ids := []string{}

	logger.Info("Finding EC2 instances with tag", "tagKey", tagKey, "tagValue", tagValue)

	// Create filter for the tag
	filter := []ec2types.Filter{
		{
			Name:   aws.String("tag:" + tagKey),
			Values: []string{tagValue},
		},
	}

	// Describe instances with the tag
	resp, err := a.client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		Filters: filter,
	})
	if err != nil {
		return ids, errors.WithStack(err)
	}

	// Collect instance IDs
	var instanceIds []string
	for _, reservation := range resp.Reservations {
		for _, instance := range reservation.Instances {
			// Only include running or pending instances
			if instance.State.Name == ec2types.InstanceStateNameRunning || instance.State.Name == ec2types.InstanceStateNamePending {
				logger.Info("Found instance to terminate", "instanceId", *instance.InstanceId)
				instanceIds = append(instanceIds, *instance.InstanceId)
			}
		}
	}

	// If no instances found, return
	if len(instanceIds) == 0 {
		logger.Info("No instances found with the specified tag")
		return ids, nil
	}

	// Terminate the instances
	logger.Info("Terminating instances", "count", len(instanceIds))
	_, err = a.client.TerminateInstances(ctx, &ec2.TerminateInstancesInput{
		InstanceIds: instanceIds,
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	ids = append(ids, instanceIds...)

	logger.Info("Successfully requested termination of instances", "count", len(ids), "instances", ids, "tagKey", tagKey, "tagValue", tagValue)
	return ids, nil
}
