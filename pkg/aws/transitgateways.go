package aws

import (
	"context"
	"fmt"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/pkg/errors"

	gserrors "github.com/aws-resolver-rules-operator/pkg/errors"
	"github.com/aws-resolver-rules-operator/pkg/resolver"
)

const (
	clusterTagValue                     = "owned"
	TransitGatewayDetachedErrorMessage  = "transit gateway not detached yet"
	TransitGatewayNotReadyErrorMessage  = "transit gateway not ready yet"
	TransitGatewayDetachedRetryDuration = 10 * time.Second
	TransitGatewayNotReadyRetryDuration = 10 * time.Second
)

func clusterTag(name string) string {
	return fmt.Sprintf("kubernetes.io/cluster/%s", name)
}

type TransitGateways struct {
	ec2 *ec2.Client
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
		return errors.WithStack(err)
	}

	vpcAttachment, err := t.getAttachment(ctx, gatewayID, attachment.VPCID)
	if err != nil {
		return errors.WithStack(err)
	}

	if vpcAttachment != nil {
		return nil
	}

	return t.attach(ctx, gatewayID, attachment)
}

func (t *TransitGateways) Detach(ctx context.Context, attachment resolver.TransitGatewayAttachment) error {
	gatewayID, err := GetARNResourceID(attachment.TransitGatewayARN)
	if err != nil {
		return errors.WithStack(err)
	}

	vpcAttachment, err := t.getAttachment(ctx, gatewayID, attachment.VPCID)
	if err != nil {
		return errors.WithStack(err)
	}

	if vpcAttachment == nil {
		return nil
	}

	_, err = t.ec2.DeleteTransitGatewayVpcAttachment(ctx, &ec2.DeleteTransitGatewayVpcAttachmentInput{
		TransitGatewayAttachmentId: vpcAttachment.TransitGatewayAttachmentId,
	})
	return errors.WithStack(err)
}

func (t *TransitGateways) Delete(ctx context.Context, name string) error {
	gateway, err := t.get(ctx, name)
	if err != nil {
		return errors.WithStack(err)
	}

	if gateway == nil {
		return nil
	}

	return t.delete(ctx, gateway)
}

func (t *TransitGateways) delete(ctx context.Context, gateway *ec2types.TransitGateway) error {
	id := gateway.TransitGatewayId
	_, err := t.ec2.DeleteTransitGateway(ctx, &ec2.DeleteTransitGatewayInput{
		TransitGatewayId: id,
	})
	if HasErrorCode(err, ErrIncorrectState) {
		return gserrors.NewRetryableError(
			TransitGatewayDetachedErrorMessage,
			TransitGatewayDetachedRetryDuration,
		)
	}

	return errors.WithStack(err)
}

func (t *TransitGateways) get(ctx context.Context, name string) (*ec2types.TransitGateway, error) {
	nameTag := "tag:" + clusterTag(name)
	input := &ec2.DescribeTransitGatewaysInput{
		Filters: []ec2types.Filter{
			{
				Name:   awssdk.String(nameTag),
				Values: []string{clusterTagValue},
			},
		},
	}

	out, err := t.ec2.DescribeTransitGateways(ctx, input)
	if err != nil {
		return nil, err
	}
	gateways := filterDeletedTransitGateways(out.TransitGateways)

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

	return &gateways[0], nil
}

func (t *TransitGateways) create(ctx context.Context, name string, tags map[string]string) (string, error) {
	ec2tags := getEc2Tags(tags)
	// Add cluster tag if not already present
	if _, ok := tags[clusterTag(name)]; !ok {
		ec2tags = append(ec2tags, ec2types.Tag{
			Key:   awssdk.String(clusterTag(name)),
			Value: awssdk.String(clusterTagValue),
		})
	}

	input := &ec2.CreateTransitGatewayInput{
		Description: awssdk.String(fmt.Sprintf("Transit Gateway for cluster %s", name)),
		Options: &ec2types.TransitGatewayRequestOptions{
			AutoAcceptSharedAttachments: ec2types.AutoAcceptSharedAttachmentsValueEnable,
		},
		TagSpecifications: []ec2types.TagSpecification{
			{
				ResourceType: ec2types.ResourceTypeTransitGateway,
				Tags:         ec2tags,
			},
		},
	}
	out, err := t.ec2.CreateTransitGateway(ctx, input)
	if err != nil {
		return "", err
	}

	return *out.TransitGateway.TransitGatewayArn, nil
}

func (t *TransitGateways) attach(ctx context.Context, transitGatewayID string, attachment resolver.TransitGatewayAttachment) error {
	tags := []ec2types.Tag{}
	for key, value := range attachment.Tags {
		tag := ec2types.Tag{
			Key:   awssdk.String(key),
			Value: awssdk.String(value),
		}
		tags = append(tags, tag)
	}

	_, err := t.ec2.CreateTransitGatewayVpcAttachment(ctx, &ec2.CreateTransitGatewayVpcAttachmentInput{
		TransitGatewayId: awssdk.String(transitGatewayID),
		VpcId:            awssdk.String(attachment.VPCID),
		SubnetIds:        attachment.SubnetIDs,
		TagSpecifications: []ec2types.TagSpecification{
			{
				ResourceType: ec2types.ResourceTypeTransitGatewayAttachment,
				Tags:         tags,
			},
		},
	})
	if HasErrorCode(err, ErrIncorrectState) {
		return gserrors.NewRetryableError(
			TransitGatewayNotReadyErrorMessage,
			TransitGatewayNotReadyRetryDuration,
		)
	}

	return errors.WithStack(err)
}

func (t *TransitGateways) getAttachment(ctx context.Context, gatewayID, vpcID string) (*ec2types.TransitGatewayVpcAttachment, error) {
	describeTGWattachmentInput := &ec2.DescribeTransitGatewayVpcAttachmentsInput{
		Filters: []ec2types.Filter{
			{
				Name:   awssdk.String("transit-gateway-id"),
				Values: []string{gatewayID},
			},
			{
				Name:   awssdk.String("vpc-id"),
				Values: []string{vpcID},
			},
		},
	}
	attachments, err := t.ec2.DescribeTransitGatewayVpcAttachments(ctx, describeTGWattachmentInput)
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

	return &attachments.TransitGatewayVpcAttachments[0], nil
}

func filterDeletedTransitGateways(transitGateways []ec2types.TransitGateway) []ec2types.TransitGateway {
	filtered := []ec2types.TransitGateway{}
	for _, gateway := range transitGateways {
		if !isTransitGatewayDeleted(gateway) {
			filtered = append(filtered, gateway)
		}
	}

	return filtered
}

func isTransitGatewayDeleted(transitGateway ec2types.TransitGateway) bool {
	state := transitGateway.State
	return state == ec2types.TransitGatewayStateDeleted || state == ec2types.TransitGatewayStateDeleting
}
