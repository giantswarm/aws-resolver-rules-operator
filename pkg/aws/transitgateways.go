package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	awssdk "github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"

	"github.com/aws-resolver-rules-operator/pkg/resolver"
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

func (t *TransitGateways) ApplyAttachment(ctx context.Context, attachment resolver.TransitGatewayAttachment) error {
	gatewayID, err := GetARNResourceID(attachment.TransitGatewayARN)
	if err != nil {
		return err
	}

	vpcAttachment, err := t.getAttachment(ctx, gatewayID, attachment.VPCID)
	if err != nil {
		return err
	}

	if vpcAttachment != nil {
		return nil
	}

	return t.attach(ctx, gatewayID, attachment)
}

func (t *TransitGateways) Detach(ctx context.Context, attachment resolver.TransitGatewayAttachment) error {
	gatewayID, err := GetARNResourceID(attachment.TransitGatewayARN)
	if err != nil {
		return err
	}

	vpcAttachment, err := t.getAttachment(ctx, gatewayID, attachment.VPCID)
	if err != nil {
		return err
	}

	if vpcAttachment == nil {
		return nil
	}

	_, err = t.ec2.DeleteTransitGatewayVpcAttachmentWithContext(ctx, &ec2.DeleteTransitGatewayVpcAttachmentInput{
		TransitGatewayAttachmentId: vpcAttachment.TransitGatewayAttachmentId,
	})
	return err
}

func (t *TransitGateways) Delete(ctx context.Context, name string) error {
	gateway, err := t.get(ctx, name)
	if err != nil {
		return err
	}

	if gateway == nil {
		return nil
	}

	return t.delete(ctx, gateway)
}

func (t *TransitGateways) delete(ctx context.Context, gateway *ec2.TransitGateway) error {
	id := gateway.TransitGatewayId
	_, err := t.ec2.DeleteTransitGateway(&ec2.DeleteTransitGatewayInput{
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
	// Add cluster tag if not already present
	if _, ok := tags[clusterTag(name)]; !ok {
		ec2tags = append(ec2tags, &ec2.Tag{
			Key:   awssdk.String(clusterTag(name)),
			Value: awssdk.String(clusterTagValue),
		})
	}

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

func (t *TransitGateways) attach(ctx context.Context, transitGatewayID string, attachment resolver.TransitGatewayAttachment) error {
	_, err := t.ec2.CreateTransitGatewayVpcAttachmentWithContext(ctx, &ec2.CreateTransitGatewayVpcAttachmentInput{
		TransitGatewayId: awssdk.String(transitGatewayID),
		VpcId:            awssdk.String(attachment.VPCID),
		SubnetIds:        awssdk.StringSlice(attachment.SubnetIDs),
		TagSpecifications: []*ec2.TagSpecification{
			{
				ResourceType: awssdk.String(ec2.ResourceTypeTransitGatewayAttachment),
				Tags: []*ec2.Tag{
					{
						Key:   awssdk.String("Name"),
						Value: awssdk.String(attachment.Name),
					},
					{
						Key:   awssdk.String(fmt.Sprintf("kubernetes.io/cluster/%s", attachment.Name)),
						Value: awssdk.String("owned"),
					},
				},
			},
		},
	})

	return err
}

func (t *TransitGateways) getAttachment(ctx context.Context, gatewayID, vpcID string) (*ec2.TransitGatewayVpcAttachment, error) {
	describeTGWattachmentInput := &ec2.DescribeTransitGatewayVpcAttachmentsInput{
		Filters: []*ec2.Filter{
			{
				Name:   awssdk.String("transit-gateway-id"),
				Values: awssdk.StringSlice([]string{gatewayID}),
			},
			{
				Name:   awssdk.String("vpc-id"),
				Values: awssdk.StringSlice([]string{vpcID}),
			},
		},
	}
	attachments, err := t.ec2.DescribeTransitGatewayVpcAttachments(describeTGWattachmentInput)
	if err != nil {
		return nil, err
	}

	if len(attachments.TransitGatewayVpcAttachments) == 0 {
		return nil, nil
	}

	if len(attachments.TransitGatewayVpcAttachments) > 1 {
		return nil, fmt.Errorf(
			"wrong number of transit gateway attachments found. Expected 1, found %d",
			len(attachments.TransitGatewayVpcAttachments),
		)
	}

	return attachments.TransitGatewayVpcAttachments[0], nil
}
