package aws_test

import (
	"context"
	"fmt"

	awssdk "github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
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

	attachTransitGateway := func(transitGatewayID, vpcID string) {
		_, err = rawEC2Client.CreateTransitGatewayVpcAttachmentWithContext(ctx, &ec2.CreateTransitGatewayVpcAttachmentInput{
			TransitGatewayId: awssdk.String(transitGatewayID),
			VpcId:            awssdk.String(vpcID),
			SubnetIds:        awssdk.StringSlice([]string{"sub-1"}),
		})
		Expect(err).NotTo(HaveOccurred())
	}

	BeforeEach(func() {
		ctx = context.Background()
		var err error
		cluster = &capa.AWSCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name: uuid.NewString(),
			},
			Spec: capa.AWSClusterSpec{
				AdditionalTags: additionalTags,
			},
		}

		transitGateways, err = awsClients.NewTransitGatewayClient(Region, AwsIamArn)
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("Apply", func() {
		It("creates a transit gateway", func() {
			arn, err := transitGateways.Apply(ctx, cluster.Name, cluster.Spec.AdditionalTags)
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
				arn, err := transitGateways.Apply(ctx, cluster.Name, cluster.Spec.AdditionalTags)
				Expect(err).NotTo(HaveOccurred())

				originalID, err = aws.GetARNResourceID(arn)
				Expect(err).NotTo(HaveOccurred())
			})

			It("does not return an error", func() {
				arn, err := transitGateways.Apply(ctx, cluster.Name, cluster.Spec.AdditionalTags)
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
					_, err := transitGateways.Apply(ctx, cluster.Name, cluster.Spec.AdditionalTags)
					Expect(err).To(MatchError(ContainSubstring(
						"found unexpected number: 2 of transit gatways for cluster",
					)))
				})
			})
		})
	})

	Describe("ApplyAttachment", func() {
		var (
			name              string
			transitGatewayID  string
			transitGatewayARN string
			vpcID             string

			applyError error
		)

		BeforeEach(func() {
			name = uuid.NewString()
			transitGatewayID = fmt.Sprintf("tw-%s", name)
			transitGatewayARN = fmt.Sprintf("arn:aws:iam::123456789012:transit-gateways/%s", transitGatewayID)
			vpcID = fmt.Sprintf("vpc-%s", name)
		})

		JustBeforeEach(func() {
			attachment := resolver.TransitGatewayAttachment{
				Name:              name,
				TransitGatewayARN: transitGatewayARN,
				SubnetIDs:         []string{"sub-1", "sub-2"},
				VPCID:             vpcID,
			}
			applyError = transitGateways.ApplyAttachment(ctx, attachment)
		})

		It("creates the attachment", func() {
			Expect(applyError).NotTo(HaveOccurred())

			describeTGWattachmentInput := &ec2.DescribeTransitGatewayVpcAttachmentsInput{
				Filters: []*ec2.Filter{
					{
						Name:   awssdk.String("transit-gateway-id"),
						Values: awssdk.StringSlice([]string{transitGatewayID}),
					},
					{
						Name:   awssdk.String("vpc-id"),
						Values: awssdk.StringSlice([]string{vpcID}),
					},
				},
			}
			attachments, err := rawEC2Client.DescribeTransitGatewayVpcAttachments(describeTGWattachmentInput)
			Expect(err).NotTo(HaveOccurred())
			Expect(attachments.TransitGatewayVpcAttachments).To(HaveLen(1))

			actualAttachment := attachments.TransitGatewayVpcAttachments[0]
			Expect(actualAttachment.SubnetIds).To(ConsistOf(PointTo(Equal("sub-1")), PointTo(Equal("sub-2"))))
		})

		When("the transit gateway ARN is not valid", func() {
			It("returns an error", func() {
				attachment := resolver.TransitGatewayAttachment{
					Name:              name,
					TransitGatewayARN: "not-a-valid-arn",
				}
				err := transitGateways.ApplyAttachment(ctx, attachment)
				Expect(err).To(HaveOccurred())
			})
		})

		When("the attachment has already been created", func() {
			BeforeEach(func() {
				attachment := resolver.TransitGatewayAttachment{
					Name:              name,
					TransitGatewayARN: transitGatewayARN,
					SubnetIDs:         []string{"sub-1", "sub-2"},
					VPCID:             vpcID,
				}
				err := transitGateways.ApplyAttachment(ctx, attachment)
				Expect(err).NotTo(HaveOccurred())
			})

			It("does not create a new one", func() {
				Expect(applyError).NotTo(HaveOccurred())

				describeTGWattachmentInput := &ec2.DescribeTransitGatewayVpcAttachmentsInput{
					Filters: []*ec2.Filter{
						{
							Name:   awssdk.String("transit-gateway-id"),
							Values: awssdk.StringSlice([]string{transitGatewayID}),
						},
						{
							Name:   awssdk.String("vpc-id"),
							Values: awssdk.StringSlice([]string{vpcID}),
						},
					},
				}
				attachments, err := rawEC2Client.DescribeTransitGatewayVpcAttachments(describeTGWattachmentInput)
				Expect(err).NotTo(HaveOccurred())
				Expect(attachments.TransitGatewayVpcAttachments).To(HaveLen(1))
			})

			When("more than one attachment exists for the transit gateway", func() {
				BeforeEach(func() {
					attachTransitGateway(transitGatewayID, vpcID)
				})

				It("returns an error", func() {
					Expect(applyError).To(MatchError(ContainSubstring(
						"wrong number of transit gateway attachments found. Expected 1, found 2",
					)))
				})
			})
		})
	})

	Describe("Detach", func() {
		var (
			transitGatewayID  string
			transitGatewayARN string
			vpcID             string
			attachment        resolver.TransitGatewayAttachment
		)

		BeforeEach(func() {
			name := uuid.NewString()
			transitGatewayID = fmt.Sprintf("tw-%s", name)
			transitGatewayARN = fmt.Sprintf("arn:aws:iam::123456789012:transit-gateways/%s", transitGatewayID)
			vpcID = fmt.Sprintf("vpc-%s", name)

			attachTransitGateway(transitGatewayID, vpcID)

			attachment = resolver.TransitGatewayAttachment{
				TransitGatewayARN: transitGatewayARN,
				VPCID:             vpcID,
			}
		})

		It("detaches the transit gateway", func() {
			err := transitGateways.Detach(ctx, attachment)
			Expect(err).NotTo(HaveOccurred())

			out, err := rawEC2Client.DescribeTransitGatewayVpcAttachments(&ec2.DescribeTransitGatewayVpcAttachmentsInput{
				Filters: []*ec2.Filter{
					{
						Name:   awssdk.String("transit-gateway-id"),
						Values: awssdk.StringSlice([]string{transitGatewayID}),
					},
					{
						Name:   awssdk.String("vpc-id"),
						Values: awssdk.StringSlice([]string{vpcID}),
					},
				},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(out.TransitGatewayVpcAttachments).To(HaveLen(1))
			Expect(out.TransitGatewayVpcAttachments[0].State).To(PointTo(Equal("deleted")))
		})

		When("the gateway has already been detached", func() {
			BeforeEach(func() {
				attachment = resolver.TransitGatewayAttachment{
					TransitGatewayARN: transitGatewayARN,
					VPCID:             vpcID,
				}
				err := transitGateways.Detach(ctx, attachment)
				Expect(err).NotTo(HaveOccurred())
			})

			It("does not return an error", func() {
				err := transitGateways.Detach(ctx, attachment)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		When("there are multiple gateways for the same cluster", func() {
			BeforeEach(func() {
				attachTransitGateway(transitGatewayID, vpcID)
			})

			It("returns an error", func() {
				err := transitGateways.Detach(ctx, attachment)
				Expect(err).To(MatchError(ContainSubstring(
					"wrong number of transit gateway attachments found. Expected 1, found 2",
				)))
			})
		})
	})

	Describe("Delete", func() {
		var gatewayID string

		BeforeEach(func() {
			gatewayID = createTransitGateway()
		})

		It("deletes the transit gateway", func() {
			err := transitGateways.Delete(ctx, cluster.Name)
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
				err := transitGateways.Delete(ctx, cluster.Name)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		When("there are multiple gateways for the same cluster", func() {
			BeforeEach(func() {
				createTransitGateway()
			})

			It("returns an error", func() {
				err := transitGateways.Delete(ctx, cluster.Name)
				Expect(err).To(MatchError(ContainSubstring(
					"found unexpected number: 2 of transit gatways for cluster",
				)))
			})
		})
	})
})
