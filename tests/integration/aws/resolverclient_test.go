package aws_test

import (
	"context"

	awssdk "github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/route53resolver"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/aws-resolver-rules-operator/pkg/resolver"
)

var _ = Describe("Route53 Resolver client", func() {

	var (
		cluster                            resolver.Cluster
		createdResolverRule                resolver.ResolverRule
		resolverRulesSecurityGroupForTests string
	)

	BeforeEach(func() {
		ctx = context.Background()

		resolverClient, err = awsClients.NewResolverClient(Region, AWS_IAM_ARN)
		Expect(err).NotTo(HaveOccurred())

		cluster = resolver.Cluster{
			Name:       "my-cluster",
			Region:     Region,
			VPCId:      VPCId,
			IAMRoleARN: AWS_IAM_ARN,
			Subnets:    subnets,
		}

		createSecurityGroupResponse, err := rawEC2Client.CreateSecurityGroup(&ec2.CreateSecurityGroupInput{
			Description: awssdk.String("Some security group that we create for resolver rules during testing"),
			GroupName:   awssdk.String("resolver-rules-security-group"),
			VpcId:       awssdk.String(VPCId),
		})
		Expect(err).NotTo(HaveOccurred())
		resolverRulesSecurityGroupForTests = *createSecurityGroupResponse.GroupId
	})

	AfterEach(func() {
		listEndpointsResponse, err := rawResolverClient.ListResolverEndpointsWithContext(ctx, &route53resolver.ListResolverEndpointsInput{
			Filters: []*route53resolver.Filter{
				{
					Name:   awssdk.String("SecurityGroupIds"),
					Values: awssdk.StringSlice([]string{resolverRulesSecurityGroupForTests}),
				},
			},
		})
		Expect(err).NotTo(HaveOccurred())

		for _, endpoint := range listEndpointsResponse.ResolverEndpoints {
			_, err = rawResolverClient.DeleteResolverEndpointWithContext(ctx, &route53resolver.DeleteResolverEndpointInput{ResolverEndpointId: endpoint.Id})
			Expect(err).NotTo(HaveOccurred())
		}

		_, err = rawEC2Client.DeleteSecurityGroup(&ec2.DeleteSecurityGroupInput{GroupId: awssdk.String(resolverRulesSecurityGroupForTests)})
		Expect(err).NotTo(HaveOccurred())
	})

	It("creates the resolver rule successfully", func() {
		createdResolverRule, err = resolverClient.CreateResolverRule(ctx, logger, cluster, resolverRulesSecurityGroupForTests, "example.com", "my-resolver-rule")
		Expect(err).NotTo(HaveOccurred())

		rulesResponse, err := rawResolverClient.ListResolverRulesWithContext(ctx, &route53resolver.ListResolverRulesInput{
			Filters: []*route53resolver.Filter{
				{
					Name:   awssdk.String("Name"),
					Values: awssdk.StringSlice([]string{"my-resolver-rule"}),
				},
			},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(len(rulesResponse.ResolverRules)).To(Equal(1))

		By("creating the resolver rule again it doesn't fail", func() {
			createdResolverRule, err = resolverClient.CreateResolverRule(ctx, logger, cluster, resolverRulesSecurityGroupForTests, "example.com", "my-resolver-rule")
			Expect(err).NotTo(HaveOccurred())

			rulesResponse, err = rawResolverClient.ListResolverRulesWithContext(ctx, &route53resolver.ListResolverRulesInput{
				Filters: []*route53resolver.Filter{
					{
						Name:   awssdk.String("Name"),
						Values: awssdk.StringSlice([]string{"my-resolver-rule"}),
					},
				},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(len(rulesResponse.ResolverRules)).To(Equal(1))
		})

		By("deleting the resolver rule works", func() {
			err = resolverClient.DeleteResolverRule(ctx, logger, cluster, createdResolverRule.RuleId)
			Expect(err).NotTo(HaveOccurred())

			rulesResponse, err = rawResolverClient.ListResolverRulesWithContext(ctx, &route53resolver.ListResolverRulesInput{
				Filters: []*route53resolver.Filter{
					{
						Name:   awssdk.String("Name"),
						Values: awssdk.StringSlice([]string{"my-resolver-rule"}),
					},
				},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(len(rulesResponse.ResolverRules)).To(Equal(0))
		})

		By("deleting the resolver rule again doesn't fail", func() {
			err = resolverClient.DeleteResolverRule(ctx, logger, cluster, createdResolverRule.RuleId)
			Expect(err).NotTo(HaveOccurred())

			rulesResponse, err = rawResolverClient.ListResolverRulesWithContext(ctx, &route53resolver.ListResolverRulesInput{
				Filters: []*route53resolver.Filter{
					{
						Name:   awssdk.String("Name"),
						Values: awssdk.StringSlice([]string{"my-resolver-rule"}),
					},
				},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(len(rulesResponse.ResolverRules)).To(Equal(0))
		})
	})
})
