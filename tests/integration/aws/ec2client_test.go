package aws_test

import (
	"context"

	awssdk "github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("EC2 client", func() {

	var (
		resolverEndpointsSecurityGroup string
	)

	BeforeEach(func() {
		ctx = context.Background()

		ec2Client, err = awsClients.NewEC2Client(Region, AwsIamArn)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		_, err = rawEC2Client.DeleteSecurityGroup(&ec2.DeleteSecurityGroupInput{GroupId: awssdk.String(resolverEndpointsSecurityGroup)})
		Expect(err).NotTo(HaveOccurred())
	})

	It("creates the security group successfully", func() {
		resolverEndpointsSecurityGroup, err = ec2Client.CreateSecurityGroupForResolverEndpoints(ctx, VPCId, "my-security-group")
		Expect(err).NotTo(HaveOccurred())

		securityGroupsResponse, err := rawEC2Client.DescribeSecurityGroupsWithContext(ctx, &ec2.DescribeSecurityGroupsInput{
			Filters: []*ec2.Filter{
				{
					Name:   awssdk.String("vpc-id"),
					Values: awssdk.StringSlice([]string{VPCId}),
				},
				{
					Name:   awssdk.String("group-name"),
					Values: awssdk.StringSlice([]string{"my-security-group"}),
				},
			},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(len(securityGroupsResponse.SecurityGroups)).To(Equal(1))
		Expect(*securityGroupsResponse.SecurityGroups[0].GroupName).To(Equal("my-security-group"))
		Expect(*securityGroupsResponse.SecurityGroups[0].IpPermissions[0].FromPort).To(Equal(int64(53)))
		Expect(*securityGroupsResponse.SecurityGroups[0].IpPermissions[0].IpProtocol).To(Equal("udp"))
		Expect(*securityGroupsResponse.SecurityGroups[0].IpPermissions[1].FromPort).To(Equal(int64(53)))
		Expect(*securityGroupsResponse.SecurityGroups[0].IpPermissions[1].IpProtocol).To(Equal("tcp"))

		By("creating the security group again it doesn't fail", func() {
			_, err = ec2Client.CreateSecurityGroupForResolverEndpoints(ctx, VPCId, "my-security-group")
			Expect(err).NotTo(HaveOccurred())

			securityGroupsResponse, err = rawEC2Client.DescribeSecurityGroupsWithContext(ctx, &ec2.DescribeSecurityGroupsInput{
				Filters: []*ec2.Filter{
					{
						Name:   awssdk.String("vpc-id"),
						Values: awssdk.StringSlice([]string{VPCId}),
					},
					{
						Name:   awssdk.String("group-name"),
						Values: awssdk.StringSlice([]string{"my-security-group"}),
					},
				},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(len(securityGroupsResponse.SecurityGroups)).To(Equal(1))
		})
	})
})
