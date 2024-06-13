package fixture

import (
	"context"
	"fmt"

	awssdk "github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"

	"github.com/aws-resolver-rules-operator/pkg/aws"
)

const (
	ManagementClusterCIDR = "172.64.0.0"
	WorkloadClusterCIDR   = "172.96.0.0"
)

func DetachTransitGateway(ec2Client *ec2.EC2, gatewayID, vpcID string) error {
	if gatewayID == "" || vpcID == "" {
		return nil
	}

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
	attachments, err := ec2Client.DescribeTransitGatewayVpcAttachments(describeTGWattachmentInput)
	if err != nil {
		return err
	}

	for _, attachment := range attachments.TransitGatewayVpcAttachments {
		_, err = ec2Client.DeleteTransitGatewayVpcAttachmentWithContext(context.Background(), &ec2.DeleteTransitGatewayVpcAttachmentInput{
			TransitGatewayAttachmentId: attachment.TransitGatewayAttachmentId,
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func DeleteTransitGateway(ec2Client *ec2.EC2, gatewayID string) error {
	if gatewayID == "" {
		return nil
	}

	_, err := ec2Client.DeleteTransitGateway(&ec2.DeleteTransitGatewayInput{
		TransitGatewayId: awssdk.String(gatewayID),
	})
	return err
}

func DeletePrefixList(ec2Client *ec2.EC2, prefixListID string) error {
	if prefixListID == "" {
		return nil
	}

	_, err := ec2Client.DeleteManagedPrefixList(&ec2.DeleteManagedPrefixListInput{
		PrefixListId: awssdk.String(prefixListID),
	})
	return err
}

func DisassociateRouteTable(ec2Client *ec2.EC2, associationID string) error {
	if associationID == "" {
		return nil
	}

	_, err := ec2Client.DisassociateRouteTable(&ec2.DisassociateRouteTableInput{
		AssociationId: awssdk.String(associationID),
	})

	if aws.HasErrorCode(err, aws.ErrAssociationNotFound) {
		return nil
	}

	return err
}

func DeleteRouteTable(ec2Client *ec2.EC2, routeTableID string) error {
	if routeTableID == "" {
		return nil
	}

	_, err := ec2Client.DeleteRouteTable(&ec2.DeleteRouteTableInput{
		RouteTableId: awssdk.String(routeTableID),
	})
	if aws.HasErrorCode(err, aws.ErrRouteTableNotFound) {
		return nil
	}

	return err
}

func DeleteSubnet(ec2Client *ec2.EC2, subnetID string) error {
	if subnetID == "" {
		return nil
	}

	_, err := ec2Client.DeleteSubnet(&ec2.DeleteSubnetInput{
		SubnetId: awssdk.String(subnetID),
	})
	if aws.HasErrorCode(err, aws.ErrSubnetNotFound) {
		return nil
	}

	return err
}

func CreateVPC(ec2Client *ec2.EC2, cidr string) (string, error) {
	vpcCIDR := fmt.Sprintf("%s/%d", cidr, 16)

	output, err := ec2Client.CreateVpc(&ec2.CreateVpcInput{
		CidrBlock: awssdk.String(vpcCIDR),
	})
	if err != nil {
		return "", fmt.Errorf("error while creating vpc: %w", err)
	}

	return *output.Vpc.VpcId, nil
}

func DeleteVPC(ec2Client *ec2.EC2, vpcID string) error {
	if vpcID == "" {
		return nil
	}

	_, err := ec2Client.DeleteVpc(&ec2.DeleteVpcInput{
		VpcId: awssdk.String(vpcID),
	})
	if aws.HasErrorCode(err, aws.ErrVPCNotFound) {
		return nil
	}

	return err
}
