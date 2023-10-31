package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	awssdk "github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
)

const clusterTagValue = "owned"

func clusterTag(name string) string {
	return fmt.Sprintf("kubernetes.io/cluster/%s", name)
}

type TransitGateways struct {
	ec2 *ec2.EC2
}

func (t *TransitGateways) Apply(ctx context.Context, name string, tags map[string]string) (string, error) {
	gateway, err := t.get(ctx, name)
	if err != nil {
		return "", err
	}

	if gateway != nil {
		return *gateway.TransitGatewayArn, nil
	}

	return t.create(ctx, name, tags)
}

func (t *TransitGateways) Delete(ctx context.Context, name string) error {
	gateway, err := t.get(ctx, name)
	if err != nil {
		return err
	}

	if gateway == nil {
		return nil
	}

	id := gateway.TransitGatewayId
	_, err = t.ec2.DeleteTransitGateway(&ec2.DeleteTransitGatewayInput{
		TransitGatewayId: id,
	})

	return err
}

func (t *TransitGateways) get(ctx context.Context, name string) (*ec2.TransitGateway, error) {
	nameTag := "tag:" + clusterTag(name)
	input := &ec2.DescribeTransitGatewaysInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String(nameTag),
				Values: aws.StringSlice([]string{clusterTagValue}),
			},
		},
	}

	out, err := t.ec2.DescribeTransitGatewaysWithContext(ctx, input)
	if err != nil {
		return nil, err
	}
	gateways := out.TransitGateways

	if len(gateways) == 0 {
		return nil, nil
	}

	if len(gateways) > 1 {
		return nil, fmt.Errorf(
			"found unexpected number: %d of transit gatways for cluster %s",
			len(gateways),
			name,
		)
	}

	return gateways[0], nil
}

func (t *TransitGateways) create(ctx context.Context, name string, tags map[string]string) (string, error) {
	ec2tags := getEc2Tags(tags)
	ec2tags = append(ec2tags, &ec2.Tag{

		Key:   awssdk.String(clusterTag(name)),
		Value: awssdk.String(clusterTagValue),
	})

	input := &ec2.CreateTransitGatewayInput{
		Description: awssdk.String(fmt.Sprintf("Transit Gateway for cluster %s", name)),
		Options: &ec2.TransitGatewayRequestOptions{
			AutoAcceptSharedAttachments: aws.String(ec2.AutoAcceptSharedAttachmentsValueEnable),
		},
		TagSpecifications: []*ec2.TagSpecification{
			{
				ResourceType: aws.String(ec2.ResourceTypeTransitGateway),
				Tags:         ec2tags,
			},
		},
	}
	out, err := t.ec2.CreateTransitGatewayWithContext(ctx, input)
	if err != nil {
		return "", err
	}

	return *out.TransitGateway.TransitGatewayArn, nil
}
