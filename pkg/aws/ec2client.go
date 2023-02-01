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
func (a *AWSEC2) CreateSecurityGroupForResolverEndpoints(ctx context.Context, vpcId, groupName string) (string, error) {
	securityGroupId, err := a.createSecurityGroup(ctx, vpcId, groupName)
	if err != nil {
		return "", errors.WithStack(err)
	}

	err = a.authorizeSecurityGroupIngressWithContext(ctx, securityGroupId, "udp", "0.0.0.0/0", DNSPort)
	if err != nil {
		return "", errors.WithStack(err)
	}

	err = a.authorizeSecurityGroupIngressWithContext(ctx, securityGroupId, "tcp", "0.0.0.0/0", DNSPort)
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

func (a *AWSEC2) createSecurityGroup(ctx context.Context, vpcId, groupName string) (string, error) {
	response, err := a.client.CreateSecurityGroupWithContext(ctx, &ec2.CreateSecurityGroupInput{
		Description: aws.String(SecurityGroupDescription),
		GroupName:   aws.String(groupName),
		VpcId:       aws.String(vpcId),
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
func (a *AWSEC2) authorizeSecurityGroupIngressWithContext(ctx context.Context, securityGroupId, protocol, cidr string, port int) error {
	_, err := a.client.AuthorizeSecurityGroupIngressWithContext(ctx, &ec2.AuthorizeSecurityGroupIngressInput{
		FromPort:   aws.Int64(int64(port)),
		GroupId:    aws.String(securityGroupId),
		IpProtocol: aws.String(protocol),
		ToPort:     aws.Int64(int64(port)),
		CidrIp:     aws.String(cidr),
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
