package aws

import (
	"context"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ec2"
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

	err = a.authorizeSecurityGroupIngressWithContext(ctx, securityGroupId, "udp", DNSPort)
	if err != nil {
		return "", errors.WithStack(err)
	}

	err = a.authorizeSecurityGroupIngressWithContext(ctx, securityGroupId, "tcp", DNSPort)
	if err != nil {
		return "", errors.WithStack(err)
	}

	return securityGroupId, nil
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
					return "", errors.WithStack(err)
				}

				return *securityGroupResponse.SecurityGroups[0].GroupId, nil
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
func (a *AWSEC2) authorizeSecurityGroupIngressWithContext(ctx context.Context, securityGroupId, protocol string, port int) error {
	_, err := a.client.AuthorizeSecurityGroupIngressWithContext(ctx, &ec2.AuthorizeSecurityGroupIngressInput{
		FromPort:   aws.Int64(int64(port)),
		GroupId:    aws.String(securityGroupId),
		IpProtocol: aws.String(protocol),
		ToPort:     aws.Int64(int64(port)),
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
