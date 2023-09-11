package aws_test

import (
	"context"
	"fmt"

	awssdk "github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	capa "sigs.k8s.io/cluster-api-provider-aws/api/v1beta1"

	"github.com/aws-resolver-rules-operator/pkg/aws"
	"github.com/aws-resolver-rules-operator/pkg/resolver"
)

var _ = Describe("Transitgateway", func() {
	var (
		ctx context.Context

		cluster *capa.AWSCluster

		transitGateways resolver.TransitGatewayClient
	)

	createTransitGateway := func() string {
		input := &ec2.CreateTransitGatewayInput{
			TagSpecifications: []*ec2.TagSpecification{
				{
					ResourceType: awssdk.String(ec2.ResourceTypeTransitGateway),
					Tags: []*ec2.Tag{
						{
							Key:   awssdk.String(fmt.Sprintf("kubernetes.io/cluster/%s", cluster.Name)),
							Value: awssdk.String("owned"),
						},
					},
				},
			},
		}
		out, err := rawEC2Client.CreateTransitGateway(input)
		Expect(err).NotTo(HaveOccurred())

		return *out.TransitGateway.TransitGatewayId
	}

	BeforeEach(func() {
		ctx = context.Background()
		var err error
		cluster = &capa.AWSCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name: uuid.NewString(),
			},
		}

		transitGateways, err = awsClients.NewTransitGateways(Region, AwsIamArn)
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("Apply", func() {
		It("creates a transit gateway", func() {
			arn, err := transitGateways.Apply(ctx, cluster)
			Expect(err).NotTo(HaveOccurred())

			transitGatewayID, err := aws.GetARNResourceID(arn)
			Expect(err).NotTo(HaveOccurred())

			out, err := rawEC2Client.DescribeTransitGateways(&ec2.DescribeTransitGatewaysInput{
				TransitGatewayIds: awssdk.StringSlice([]string{transitGatewayID}),
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(out.TransitGateways).To(HaveLen(1))
		})

		When("the transit gateway already exists", func() {
			var originalID string

			BeforeEach(func() {
				arn, err := transitGateways.Apply(ctx, cluster)
				Expect(err).NotTo(HaveOccurred())

				originalID, err = aws.GetARNResourceID(arn)
				Expect(err).NotTo(HaveOccurred())
			})

			It("does not return an error", func() {
				arn, err := transitGateways.Apply(ctx, cluster)
				Expect(err).NotTo(HaveOccurred())

				actualID, err := aws.GetARNResourceID(arn)
				Expect(err).NotTo(HaveOccurred())

				Expect(actualID).To(Equal(originalID))

				out, err := rawEC2Client.DescribeTransitGateways(&ec2.DescribeTransitGatewaysInput{
					TransitGatewayIds: awssdk.StringSlice([]string{actualID}),
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(out.TransitGateways).To(HaveLen(1))
			})

			When("multiple transit gateways exist for the same cluster", func() {
				BeforeEach(func() {
					createTransitGateway()
				})

				It("returns an error", func() {
					_, err := transitGateways.Apply(ctx, cluster)
					Expect(err).To(MatchError(ContainSubstring(
						"found unexpected number: 2 of transit gatways for cluster",
					)))
				})
			})
		})
	})

	Describe("Delete", func() {
		var gatewayID string

		BeforeEach(func() {
			gatewayID = createTransitGateway()
		})

		It("deletes the transit gateway", func() {
			err := transitGateways.Delete(ctx, cluster)
			Expect(err).NotTo(HaveOccurred())

			out, err := rawEC2Client.DescribeTransitGateways(&ec2.DescribeTransitGatewaysInput{
				TransitGatewayIds: awssdk.StringSlice([]string{gatewayID}),
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(out.TransitGateways).To(HaveLen(0))
		})

		When("the gateway has already been deleted", func() {
			BeforeEach(func() {
				_, err := rawEC2Client.DeleteTransitGateway(&ec2.DeleteTransitGatewayInput{
					TransitGatewayId: awssdk.String(gatewayID),
				})
				Expect(err).NotTo(HaveOccurred())
			})

			It("does not return an error", func() {
				err := transitGateways.Delete(ctx, cluster)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		When("there are multiple gateways for the same cluster", func() {
			BeforeEach(func() {
				createTransitGateway()
			})

			It("returns an error", func() {
				err := transitGateways.Delete(ctx, cluster)
				Expect(err).To(MatchError(ContainSubstring(
					"found unexpected number: 2 of transit gatways for cluster",
				)))
			})
		})
	})
})
