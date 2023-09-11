package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	awssdk "github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	capa "sigs.k8s.io/cluster-api-provider-aws/api/v1beta1"
)

const clusterTagValue = "owned"

func clusterTag(name string) string {
	return fmt.Sprintf("kubernetes.io/cluster/%s", name)
}

type TransitGateways struct {
	ec2 *ec2.EC2
}

func (t *TransitGateways) Apply(ctx context.Context, cluster *capa.AWSCluster) (string, error) {
	gateways, err := t.get(ctx, cluster)
	if err != nil {
		return "", err
	}

	if len(gateways) == 1 {
		return *gateways[0].TransitGatewayArn, nil
	}

	if len(gateways) > 1 {
		return "", fmt.Errorf(
			"found unexpected number: %d of transit gatways for cluster %s",
			len(gateways),
			cluster.Name,
		)
	}

	return t.create(ctx, cluster)
}

func (t *TransitGateways) Delete(ctx context.Context, cluster *capa.AWSCluster) error {
	gateways, err := t.get(ctx, cluster)
	if err != nil {
		return err
	}

	if len(gateways) == 0 {
		return nil
	}

	if len(gateways) > 1 {
		return fmt.Errorf(
			"found unexpected number: %d of transit gatways for cluster %s",
			len(gateways),
			cluster.Name,
		)
	}

	id := gateways[0].TransitGatewayId
	_, err = t.ec2.DeleteTransitGateway(&ec2.DeleteTransitGatewayInput{
		TransitGatewayId: id,
	})

	return err
}

func (t *TransitGateways) get(ctx context.Context, cluster *capa.AWSCluster) ([]*ec2.TransitGateway, error) {
	nameTag := "tag:" + clusterTag(cluster.Name)
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

	return out.TransitGateways, nil
}

func (t *TransitGateways) create(ctx context.Context, cluster *capa.AWSCluster) (string, error) {
	input := &ec2.CreateTransitGatewayInput{
		Description: awssdk.String(fmt.Sprintf("Transit Gateway for cluster %s", cluster.Name)),
		Options: &ec2.TransitGatewayRequestOptions{
			AutoAcceptSharedAttachments: aws.String(ec2.AutoAcceptSharedAttachmentsValueEnable),
		},
		TagSpecifications: []*ec2.TagSpecification{
			{
				ResourceType: aws.String(ec2.ResourceTypeTransitGateway),
				Tags: []*ec2.Tag{
					{
						Key:   awssdk.String(clusterTag(cluster.Name)),
						Value: awssdk.String(clusterTagValue),
					},
				},
			},
		},
	}
	out, err := t.ec2.CreateTransitGatewayWithContext(ctx, input)
	if err != nil {
		return "", err
	}

	return *out.TransitGateway.TransitGatewayArn, nil
}
