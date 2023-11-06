package aws_test

import (
	"context"

	awssdk "github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/aws-resolver-rules-operator/pkg/aws"
	"github.com/aws-resolver-rules-operator/pkg/resolver"
)

var _ = Describe("RouteTables", func() {
	var (
		ctx               context.Context
		routeTablesClient resolver.RouteTablesClient
		routeTableId      string
		prefixListID      string
		transitGatewayID  string
	)

	BeforeEach(func() {
		ctx = context.Background()

		routeTablesClient, err = awsClients.NewRouteTablesClient(Region, AwsIamArn)
		Expect(err).NotTo(HaveOccurred())

		ec2Output, err := rawEC2Client.CreateRouteTable(&ec2.CreateRouteTableInput{
			VpcId: &VPCId,
		})
		Expect(err).NotTo(HaveOccurred())

		routeTableId = *ec2Output.RouteTable.RouteTableId

		_, err = rawEC2Client.AssociateRouteTable(&ec2.AssociateRouteTableInput{
			RouteTableId: awssdk.String(routeTableId),
			SubnetId:     awssdk.String(subnets[0]),
		})
		Expect(err).NotTo(HaveOccurred())

		//Without transit gateway and prefix list, the route is not created
		clusterName := uuid.NewString()

		transitGateways, err := awsClients.NewTransitGatewayClient(Region, AwsIamArn)
		Expect(err).NotTo(HaveOccurred())

		arn, err := transitGateways.Apply(ctx, clusterName)
		Expect(err).NotTo(HaveOccurred())

		transitGatewayID, err = aws.GetARNResourceID(arn)
		Expect(err).NotTo(HaveOccurred())

		out, err := rawEC2Client.DescribeTransitGateways(&ec2.DescribeTransitGatewaysInput{
			TransitGatewayIds: awssdk.StringSlice([]string{transitGatewayID}),
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(out.TransitGateways).To(HaveLen(1))

		prefixLists, err := awsClients.NewPrefixListClient(Region, AwsIamArn)
		Expect(err).NotTo(HaveOccurred())

		arn, err = prefixLists.Apply(ctx, clusterName)
		Expect(err).NotTo(HaveOccurred())

		prefixListID, err = aws.GetARNResourceID(arn)
		Expect(err).NotTo(HaveOccurred())

	})

	Describe("GetRouteTables", func() {
		It("gets the route tables", func() {
			routeTables, err := routeTablesClient.GetRouteTables(ctx, subnets)
			Expect(err).NotTo(HaveOccurred())
			Expect(routeTables).To(HaveLen(1))
			Expect(*routeTables[0].RouteTableId).To(Equal(routeTableId))
		})
	})

	Describe("CreateRoute", func() {
		It("creates a route", func() {
			err := routeTablesClient.CreateRoute(ctx, &routeTableId, &prefixListID, &transitGatewayID)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("DeleteRoute", func() {
		When("the route does not exist", func() {
			It("does not return an error", func() {
				err := routeTablesClient.DeleteRoute(ctx, &routeTableId, &prefixListID)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		When("the route exists", func() {
			JustBeforeEach(func() {
				_, err := rawEC2Client.CreateRoute(&ec2.CreateRouteInput{
					RouteTableId:            &routeTableId,
					DestinationPrefixListId: &prefixListID,
					TransitGatewayId:        &transitGatewayID,
				})
				Expect(err).NotTo(HaveOccurred())
			})
			It("deletes a route", func() {
				err := routeTablesClient.DeleteRoute(ctx, &routeTableId, &prefixListID)
				Expect(err).NotTo(HaveOccurred())
			})
		})

	})
})
