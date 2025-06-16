package aws

import (
	"context"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
)

type AWSEC2 struct {
	client *ec2.EC2
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
	_, err = a.client.DeleteSecurityGroupWithContext(ctx, &ec2.DeleteSecurityGroupInput{GroupId: securityGroup.GroupId})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case "InvalidGroup.NotFound":
				return nil
			default:
				return errors.WithStack(err)
			}
		}

		return errors.WithStack(err)
	}

	return nil
}

func (a *AWSEC2) getSecurityGroupByName(ctx context.Context, vpcId, groupName string) (*ec2.SecurityGroup, error) {
	securityGroupResponse, err := a.client.DescribeSecurityGroupsWithContext(ctx, &ec2.DescribeSecurityGroupsInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: aws.StringSlice([]string{vpcId}),
			},
			{
				Name:   aws.String("group-name"),
				Values: aws.StringSlice([]string{groupName}),
			},
		},
	})
	if err != nil {
		return &ec2.SecurityGroup{}, errors.WithStack(err)
	}

	if len(securityGroupResponse.SecurityGroups) < 1 {
		return &ec2.SecurityGroup{}, &SecurityGroupNotFoundError{}
	}

	return securityGroupResponse.SecurityGroups[0], nil
}

func (a *AWSEC2) createSecurityGroup(ctx context.Context, vpcId, groupName string, tags map[string]string) (string, error) {
	response, err := a.client.CreateSecurityGroupWithContext(ctx, &ec2.CreateSecurityGroupInput{
		Description: aws.String(SecurityGroupDescription),
		GroupName:   aws.String(groupName),
		VpcId:       aws.String(vpcId),
		TagSpecifications: []*ec2.TagSpecification{
			{
				ResourceType: aws.String(ec2.ResourceTypeSecurityGroup),
				Tags:         getEc2Tags(tags),
			},
		},
	})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case "InvalidGroup.Duplicate":
				securityGroup, err := a.getSecurityGroupByName(ctx, vpcId, groupName)
				if err != nil {
					return "", errors.WithStack(err)
				}

				return *securityGroup.GroupId, nil
			default:
				return "", errors.WithStack(err)
			}
		}

		return "", errors.WithStack(err)
	}

	return *response.GroupId, nil
}

// authorizeSecurityGroupIngressWithContext adds the specified inbound (ingress) rules to a security group.
// It won't return an error if the rule already exists for the security group. Errors can be found here
// https://docs.aws.amazon.com/AWSEC2/latest/APIReference/errors-overview.html#CommonErrors
func (a *AWSEC2) authorizeSecurityGroupIngressWithContext(ctx context.Context, securityGroupId, protocol, cidr string, port int, tags map[string]string) error {
	_, err := a.client.AuthorizeSecurityGroupIngressWithContext(ctx, &ec2.AuthorizeSecurityGroupIngressInput{
		FromPort:   aws.Int64(int64(port)),
		GroupId:    aws.String(securityGroupId),
		IpProtocol: aws.String(protocol),
		ToPort:     aws.Int64(int64(port)),
		CidrIp:     aws.String(cidr),
		TagSpecifications: []*ec2.TagSpecification{
			{
				ResourceType: aws.String(ec2.ResourceTypeSecurityGroupRule),
				Tags:         getEc2Tags(tags),
			},
		},
	})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case "InvalidPermission.Duplicate":
				return nil
			default:
				return errors.WithStack(err)
			}
		}

		return errors.WithStack(err)
	}

	return nil
}

func getEc2Tags(t map[string]string) []*ec2.Tag {
	var tags []*ec2.Tag
	for k, v := range t {
		tags = append(tags, &ec2.Tag{
			Key:   aws.String(k),
			Value: aws.String(v),
		})
	}
	return tags
}

// TerminateInstancesByTag terminates all EC2 instances that have the specified tag key and value.
func (a *AWSEC2) TerminateInstancesByTag(ctx context.Context, logger logr.Logger, tagKey, tagValue string) error {
	logger.Info("Finding EC2 instances with tag", "tagKey", tagKey, "tagValue", tagValue)

	// Create filter for the tag
	filter := []*ec2.Filter{
		{
			Name:   aws.String("tag:" + tagKey),
			Values: []*string{aws.String(tagValue)},
		},
	}

	// Describe instances with the tag
	resp, err := a.client.DescribeInstancesWithContext(ctx, &ec2.DescribeInstancesInput{
		Filters: filter,
	})
	if err != nil {
		return errors.WithStack(err)
	}

	// Collect instance IDs
	var instanceIds []*string
	for _, reservation := range resp.Reservations {
		for _, instance := range reservation.Instances {
			// Only include running or pending instances
			if *instance.State.Name == ec2.InstanceStateNameRunning || *instance.State.Name == ec2.InstanceStateNamePending {
				logger.Info("Found instance to terminate", "instanceId", *instance.InstanceId)
				instanceIds = append(instanceIds, instance.InstanceId)
			}
		}
	}

	// If no instances found, return
	if len(instanceIds) == 0 {
		logger.Info("No instances found with the specified tag")
		return nil
	}

	// Terminate the instances
	logger.Info("Terminating instances", "count", len(instanceIds))
	_, err = a.client.TerminateInstancesWithContext(ctx, &ec2.TerminateInstancesInput{
		InstanceIds: instanceIds,
	})
	if err != nil {
		return errors.WithStack(err)
	}

	logger.Info("Successfully requested termination of instances", "count", len(instanceIds), "tagKey", tagKey, "tagValue", tagValue)
	return nil
}
