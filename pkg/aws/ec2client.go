package aws

import (
	"context"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/pkg/errors"
)

type AWSEC2 struct {
	client *ec2.EC2
}

func (a *AWSEC2) CreateSecurityGroupWithContext(ctx context.Context, vpcId, description, groupName string) (string, error) {
	_, err := a.client.CreateSecurityGroupWithContext(ctx, &ec2.CreateSecurityGroupInput{
		Description: aws.String(description),
		GroupName:   aws.String(groupName),
		VpcId:       aws.String(vpcId),
	})
	if err != nil {
		return "", errors.WithStack(err)
	}

	return "", nil
}

func (a *AWSEC2) AuthorizeSecurityGroupIngressWithContext(ctx context.Context, securityGroupId, protocol string, port int) error {
	_, err := a.client.AuthorizeSecurityGroupIngressWithContext(ctx, &ec2.AuthorizeSecurityGroupIngressInput{
		FromPort:   aws.Int64(int64(port)),
		GroupId:    aws.String(securityGroupId),
		IpProtocol: aws.String(protocol),
		ToPort:     aws.Int64(int64(port)),
	})
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}
