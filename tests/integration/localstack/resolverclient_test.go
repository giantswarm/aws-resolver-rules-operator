package aws_test

import (
	"context"
	"fmt"
	"time"

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

		resolverClient, err = awsClients.NewResolverClient(Region, AwsIamArn)
		Expect(err).NotTo(HaveOccurred())

		cluster = resolver.Cluster{
			Name:       "my-cluster",
			Region:     Region,
			VPCId:      VPCId,
			IAMRoleARN: AwsIamArn,
			Subnets:    subnets,
		}
	})

	When("there are no resolver rules", func() {
		BeforeEach(func() {
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
				err = resolverClient.DeleteResolverRule(ctx, logger, cluster, createdResolverRule.Id)
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
				err = resolverClient.DeleteResolverRule(ctx, logger, cluster, createdResolverRule.Id)
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

	When("there are resolver rules in AWS account", func() {
		var (
			createdResolverRules             []*route53resolver.ResolverRule
			createdResolverRulesAssociations []*route53resolver.ResolverRuleAssociation
			totalNumberOfResolverRules       = 110
			resolverEndpointId               string
		)

		BeforeEach(func() {
			createSecurityGroupResponse, err := rawEC2Client.CreateSecurityGroup(&ec2.CreateSecurityGroupInput{
				Description: awssdk.String("Some security group that we create for resolver rules during testing"),
				GroupName:   awssdk.String("resolver-rules-security-group"),
				VpcId:       awssdk.String(VPCId),
			})
			Expect(err).NotTo(HaveOccurred())
			resolverRulesSecurityGroupForTests = *createSecurityGroupResponse.GroupId

			now := time.Now()
			ipAddresses := []*route53resolver.IpAddressRequest{}
			for _, subnet := range subnets {
				ipAddresses = append(ipAddresses, &route53resolver.IpAddressRequest{SubnetId: awssdk.String(subnet)})
			}
			createResolverEndpointResponse, err := rawResolverClient.CreateResolverEndpointWithContext(ctx, &route53resolver.CreateResolverEndpointInput{
				CreatorRequestId: awssdk.String(fmt.Sprintf("1%d", now.UnixNano())),
				Direction:        awssdk.String("OUTBOUND"),
				IpAddresses:      ipAddresses,
				Name:             awssdk.String("test-resolver-endpoint"),
				SecurityGroupIds: awssdk.StringSlice([]string{resolverRulesSecurityGroupForTests}),
			})
			Expect(err).NotTo(HaveOccurred())
			resolverEndpointId = *createResolverEndpointResponse.ResolverEndpoint.Id

			for i := 1; i <= totalNumberOfResolverRules; i++ {
				createResolverRuleResponse, err := rawResolverClient.CreateResolverRuleWithContext(ctx, &route53resolver.CreateResolverRuleInput{
					CreatorRequestId:   awssdk.String(fmt.Sprintf("%d%d", i, now.UnixNano())),
					DomainName:         awssdk.String(fmt.Sprintf("a%d.example.com", i)),
					Name:               awssdk.String(fmt.Sprintf("resolver-rule-%d", i)),
					ResolverEndpointId: awssdk.String(resolverEndpointId),
					RuleType:           awssdk.String("FORWARD"),
					TargetIps:          nil,
				})
				Expect(err).NotTo(HaveOccurred())

				associateResolverRuleResponse, err := rawResolverClient.AssociateResolverRuleWithContext(ctx, &route53resolver.AssociateResolverRuleInput{
					Name:           createResolverRuleResponse.ResolverRule.Name,
					ResolverRuleId: createResolverRuleResponse.ResolverRule.Id,
					VPCId:          awssdk.String(VPCId),
				})
				Expect(err).NotTo(HaveOccurred())

				createdResolverRules = append(createdResolverRules, createResolverRuleResponse.ResolverRule)
				createdResolverRulesAssociations = append(createdResolverRulesAssociations, associateResolverRuleResponse.ResolverRuleAssociation)
			}
		})
		AfterEach(func() {
			for _, ruleAssociation := range createdResolverRulesAssociations {
				_, err = rawResolverClient.DisassociateResolverRuleWithContext(ctx, &route53resolver.DisassociateResolverRuleInput{ResolverRuleId: ruleAssociation.ResolverRuleId, VPCId: ruleAssociation.VPCId})
				Expect(err).NotTo(HaveOccurred())
			}
			for _, rule := range createdResolverRules {
				_, err = rawResolverClient.DeleteResolverRuleWithContext(ctx, &route53resolver.DeleteResolverRuleInput{ResolverRuleId: rule.Id})
				Expect(err).NotTo(HaveOccurred())
			}

			Eventually(func() error {
				_, err = rawResolverClient.DeleteResolverEndpointWithContext(ctx, &route53resolver.DeleteResolverEndpointInput{ResolverEndpointId: awssdk.String(resolverEndpointId)})
				return err
			}, "3s", "500ms").Should(Succeed())

			_, err = rawEC2Client.DeleteSecurityGroup(&ec2.DeleteSecurityGroupInput{GroupId: awssdk.String(resolverRulesSecurityGroupForTests)})
			Expect(err).NotTo(HaveOccurred())
		})

		It("finds resolver rules", func() {
			By("filtering by AWS account", func() {
				rules, err := resolverClient.FindResolverRulesByAWSAccountId(ctx, logger, LocalstackAWSAccountId)
				Expect(err).NotTo(HaveOccurred())
				Expect(len(rules)).To(Equal(totalNumberOfResolverRules))
			})

			By("no filtering by aws account when AWS account id is empty", func() {
				rules, err := resolverClient.FindResolverRulesByAWSAccountId(ctx, logger, "")
				Expect(err).NotTo(HaveOccurred())
				Expect(len(rules)).To(Equal(totalNumberOfResolverRules))
			})

			By("passing a different AWS Account id it won't find any resolver rules", func() {
				rules, err := resolverClient.FindResolverRulesByAWSAccountId(ctx, logger, "1234567890")
				Expect(err).NotTo(HaveOccurred())
				Expect(len(rules)).To(BeZero())
			})

			By("it finds resolver rule associations", func() {
				associations, err := resolverClient.FindResolverRuleIdsAssociatedWithVPCId(ctx, logger, VPCId)
				Expect(err).NotTo(HaveOccurred())
				Expect(len(associations)).To(Equal(totalNumberOfResolverRules))
			})

			By("associating a resolver rule that is already associated with the same vpc don't return error", func() {
				now := time.Now()
				createResolverRuleResponse, err := rawResolverClient.CreateResolverRuleWithContext(ctx, &route53resolver.CreateResolverRuleInput{
					CreatorRequestId:   awssdk.String(fmt.Sprintf("%d", now.UnixNano())),
					DomainName:         awssdk.String("test.example.com"),
					Name:               awssdk.String("resolver-rule-associate-twice"),
					ResolverEndpointId: awssdk.String(resolverEndpointId),
					RuleType:           awssdk.String("FORWARD"),
					TargetIps:          nil,
				})
				Expect(err).NotTo(HaveOccurred())

				// Associating once with the VPC should be fine
				err = resolverClient.AssociateResolverRuleWithContext(ctx, logger, "associationName", VPCId, *createResolverRuleResponse.ResolverRule.Id)
				Expect(err).NotTo(HaveOccurred())
				// Associating again should yield no errors
				err = resolverClient.AssociateResolverRuleWithContext(ctx, logger, "associationName", VPCId, *createResolverRuleResponse.ResolverRule.Id)
				Expect(err).NotTo(HaveOccurred())
				// Disassociating should return no errors
				err = resolverClient.DisassociateResolverRuleWithContext(ctx, logger, VPCId, *createResolverRuleResponse.ResolverRule.Id)
				Expect(err).NotTo(HaveOccurred())
				// Disassociating again should return no errors
				err = resolverClient.DisassociateResolverRuleWithContext(ctx, logger, VPCId, *createResolverRuleResponse.ResolverRule.Id)
				Expect(err).NotTo(HaveOccurred())
				// Deleting the resolver rule should work after disassociating it from the VPC
				err = resolverClient.DeleteResolverRule(ctx, logger, cluster, *createResolverRuleResponse.ResolverRule.Id)
				Expect(err).NotTo(HaveOccurred())
				// Deleting the resolver rule again shouldn't return any errors
				err = resolverClient.DeleteResolverRule(ctx, logger, cluster, *createResolverRuleResponse.ResolverRule.Id)
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
})
