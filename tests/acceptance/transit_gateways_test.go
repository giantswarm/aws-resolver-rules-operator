package acceptance_test

import (
	"context"
	"fmt"

	awssdk "github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	capa "sigs.k8s.io/cluster-api-provider-aws/v2/api/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gsannotation "github.com/giantswarm/k8smetadata/pkg/annotation"

	"github.com/aws-resolver-rules-operator/pkg/aws"
)

var _ = Describe("Transit Gateways", func() {
	var (
		ctx context.Context

		transitGatewayID string
		prefixListID     string
	)

	getClusterAnnotations := func(g Gomega) map[string]string {
		actualCluster := &capa.AWSCluster{}
		nsName := client.ObjectKeyFromObject(testFixture.ManagementCluster.Cluster)
		err := k8sClient.Get(ctx, nsName, actualCluster)
		g.Expect(err).NotTo(HaveOccurred())

		return actualCluster.Annotations
	}

	BeforeEach(func() {
		ctx = context.Background()

		annotations := map[string]string{}
		Eventually(func(g Gomega) map[string]string {
			annotations = getClusterAnnotations(g)
			return annotations
		}).Should(And(
			HaveKey(gsannotation.NetworkTopologyTransitGatewayIDAnnotation),
			HaveKey(gsannotation.NetworkTopologyPrefixListIDAnnotation)),
		)

		var err error
		transitGatewayID, err = aws.GetARNResourceID(annotations[gsannotation.NetworkTopologyTransitGatewayIDAnnotation])
		Expect(err).NotTo(HaveOccurred())

		prefixListID, err = aws.GetARNResourceID(annotations[gsannotation.NetworkTopologyPrefixListIDAnnotation])
		Expect(err).NotTo(HaveOccurred())
	})

	It("creates the transit gateway", func() {
		output, err := testFixture.EC2Client.DescribeTransitGateways(&ec2.DescribeTransitGatewaysInput{
			TransitGatewayIds: []*string{awssdk.String(transitGatewayID)},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(output.TransitGateways).To(HaveLen(1))
	})

	It("attaches the transit gateway", func() {
		getTGWAttachments := func() []*ec2.TransitGatewayVpcAttachment {
			describeTGWattachmentInput := &ec2.DescribeTransitGatewayVpcAttachmentsInput{
				Filters: []*ec2.Filter{
					{
						Name:   awssdk.String("transit-gateway-id"),
						Values: []*string{awssdk.String(transitGatewayID)},
					},
					{
						Name:   awssdk.String("vpc-id"),
						Values: []*string{awssdk.String(testFixture.Network.VpcID)},
					},
				},
			}
			describeTGWattachmentOutput, err := testFixture.EC2Client.DescribeTransitGatewayVpcAttachments(describeTGWattachmentInput)
			Expect(err).NotTo(HaveOccurred())
			return describeTGWattachmentOutput.TransitGatewayVpcAttachments
		}
		Eventually(getTGWAttachments).Should(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
			"State": PointTo(Equal("available")),
		}))))
	})

	It("creates the prefix list", func() {
		Eventually(getClusterAnnotations).Should(HaveKey(gsannotation.NetworkTopologyPrefixListIDAnnotation))

		managementAWSCluster := testFixture.ManagementCluster.AWSCluster
		prefixListDescription := fmt.Sprintf("CIDR block for cluster %s", managementAWSCluster.Name)
		Eventually(func(g Gomega) []*ec2.PrefixListEntry {
			result, err := testFixture.EC2Client.GetManagedPrefixListEntries(&ec2.GetManagedPrefixListEntriesInput{
				PrefixListId: awssdk.String(prefixListID),
				MaxResults:   awssdk.Int64(100),
			})
			g.Expect(err).NotTo(HaveOccurred())
			return result.Entries
		}).Should(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
			"Cidr":        PointTo(Equal(managementAWSCluster.Spec.NetworkSpec.VPC.CidrBlock)),
			"Description": PointTo(Equal(prefixListDescription)),
		}))))
	})

	It("creates a route in explicitly attached route tables", func() {
		managementAWSCluster := testFixture.ManagementCluster.AWSCluster
		getRouteTables := func() []*ec2.RouteTable {
			subnets := []*string{}
			for _, s := range managementAWSCluster.Spec.NetworkSpec.Subnets {
				subnets = append(subnets, awssdk.String(s.ResourceID))
			}

			routeTablesOutput, err := testFixture.EC2Client.DescribeRouteTables(&ec2.DescribeRouteTablesInput{
				Filters: []*ec2.Filter{
					{Name: awssdk.String("association.subnet-id"), Values: subnets},
				},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(routeTablesOutput).NotTo(BeNil())

			return routeTablesOutput.RouteTables
		}
		Eventually(getRouteTables).Should(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
			"Routes": ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
				"DestinationPrefixListId": PointTo(Equal(prefixListID)),
				"TransitGatewayId":        PointTo(Equal(transitGatewayID)),
			}))),
		}))))
	})
})
