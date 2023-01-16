package controllers_test

import (
	"context"
	"errors"
	"fmt"

	gsannotations "github.com/giantswarm/k8smetadata/pkg/annotation"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	capa "sigs.k8s.io/cluster-api-provider-aws/api/v1beta1"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/aws-resolver-rules-operator/controllers"
	"github.com/aws-resolver-rules-operator/controllers/controllersfakes"
	"github.com/aws-resolver-rules-operator/pkg/resolver"
	"github.com/aws-resolver-rules-operator/pkg/resolver/resolverfakes"
)

var _ = Describe("AWSCluster", func() {
	var (
		awsClusterClient        *controllersfakes.FakeAWSClusterClient
		ctx                     context.Context
		reconciler              *controllers.AwsClusterReconciler
		cluster                 *capi.Cluster
		awsCluster              *capa.AWSCluster
		awsClusterRoleIdentity  *capa.AWSClusterRoleIdentity
		result                  ctrl.Result
		reconcileErr            error
		resolverClient          *resolverfakes.FakeResolverClient
		dnsServerResolverClient *resolverfakes.FakeResolverClient
		ec2Client               *resolverfakes.FakeEC2Client
		ramClient               *resolverfakes.FakeRAMClient
	)

	const (
		ClusterName               = "foo"
		DnsServerAWSAccountId     = "dns-server-aws-account-id"
		DnsServerVPCId            = "dns-server-vpc-id"
		WorkloadClusterBaseDomain = "eu-central-1.aws.some.domain.com"
	)

	BeforeEach(func() {
		ctx = context.Background()
		awsClusterClient = new(controllersfakes.FakeAWSClusterClient)
		resolverClient = new(resolverfakes.FakeResolverClient)
		dnsServerResolverClient = new(resolverfakes.FakeResolverClient)
		ramClient = new(resolverfakes.FakeRAMClient)
		ec2Client = new(resolverfakes.FakeEC2Client)
		fakeAWSClients := &resolver.FakeClients{
			ResolverClient:         resolverClient,
			EC2Client:              ec2Client,
			RAMClient:              ramClient,
			ExternalResolverClient: dnsServerResolverClient,
		}
		dnsServer, err := resolver.NewDNSServer(DnsServerAWSAccountId, "1234567890", "eu-central-1", "external-iam-role-to-assume", DnsServerVPCId)
		Expect(err).NotTo(HaveOccurred())

		resolver, err := resolver.NewResolver(fakeAWSClients, dnsServer, WorkloadClusterBaseDomain)
		Expect(err).NotTo(HaveOccurred())

		reconciler = controllers.NewAwsClusterReconciler(awsClusterClient, resolver)
		awsClusterRoleIdentity = &capa.AWSClusterRoleIdentity{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "bar",
				Name:      "default",
			},
			Spec: capa.AWSClusterRoleIdentitySpec{},
		}
		awsCluster = &capa.AWSCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ClusterName,
				Namespace: "bar",
				Annotations: map[string]string{
					gsannotations.AWSDNSMode: gsannotations.DNSModePrivate,
				},
			},
			Spec: capa.AWSClusterSpec{
				NetworkSpec: capa.NetworkSpec{
					VPC: capa.VPCSpec{
						ID: "myvpc-1a2b3c4d",
					},
					Subnets: []capa.SubnetSpec{
						{
							ID: "subnet-1",
						},
						{
							ID: "subnet-2",
						},
					},
				},
				Region: "gs-south-1",
				IdentityRef: &capa.AWSIdentityReference{
					Name: "default",
					Kind: capa.ClusterRoleIdentityKind,
				},
			},
		}
		cluster = &capi.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ClusterName,
				Namespace: "bar",
			},
		}
	})

	JustBeforeEach(func() {
		request := ctrl.Request{
			NamespacedName: types.NamespacedName{
				Name:      ClusterName,
				Namespace: "bar",
			},
		}
		_, reconcileErr = reconciler.Reconcile(ctx, request)
	})

	When("the cluster does not have an owner yet", func() {
		BeforeEach(func() {
			awsClusterClient.GetReturns(awsCluster, nil)
			awsClusterClient.GetOwnerReturns(nil, nil)
		})

		It("does not requeue the event", func() {
			Expect(result.Requeue).To(BeFalse())
			Expect(result.RequeueAfter).To(BeZero())
			Expect(reconcileErr).NotTo(HaveOccurred())
		})
	})

	When("the cluster is paused", func() {
		BeforeEach(func() {
			awsClusterClient.GetReturns(awsCluster, nil)
			cluster.Spec.Paused = true
			awsClusterClient.GetOwnerReturns(cluster, nil)
		})

		It("does not reconcile", func() {
			Expect(result.Requeue).To(BeFalse())
			Expect(result.RequeueAfter).To(BeZero())
			Expect(reconcileErr).NotTo(HaveOccurred())
		})
	})

	When("the infrastructure cluster is paused", func() {
		BeforeEach(func() {
			awsClusterClient.GetReturns(awsCluster, nil)
			awsCluster.Annotations = map[string]string{
				capi.PausedAnnotation: "true",
			}
		})

		It("does not reconcile", func() {
			Expect(result.Requeue).To(BeFalse())
			Expect(result.RequeueAfter).To(BeZero())
			Expect(reconcileErr).NotTo(HaveOccurred())
		})
	})

	When("the cluster has an owner, it's not paused, it's not marked for deletion but has no identity set", func() {
		BeforeEach(func() {
			awsClusterClient.GetReturns(awsCluster, nil)
			awsClusterClient.GetOwnerReturns(cluster, nil)
			awsClusterClient.GetIdentityReturns(nil, nil)
		})

		It("gets the cluster and owner cluster", func() {
			Expect(awsClusterClient.GetCallCount()).To(Equal(1))
			Expect(awsClusterClient.GetOwnerCallCount()).To(Equal(1))

			_, actualCluster := awsClusterClient.GetOwnerArgsForCall(0)
			Expect(actualCluster).To(Equal(awsCluster))
			Expect(reconcileErr).NotTo(HaveOccurred())
		})

		It("doesn't reconcile if no Identity is set", func() {
			Expect(result.Requeue).To(BeFalse())
			Expect(result.RequeueAfter).To(BeZero())
			Expect(reconcileErr).NotTo(HaveOccurred())
		})
	})

	When("the cluster has an owner and an identity set", func() {
		BeforeEach(func() {
			awsClusterClient.GetReturns(awsCluster, nil)
			awsClusterClient.GetOwnerReturns(cluster, nil)
			awsClusterClient.GetIdentityReturns(awsClusterRoleIdentity, nil)
		})

		When("is not using private DNS mode", func() {
			BeforeEach(func() {
				awsClusterClient.GetReturns(awsCluster, nil)
				awsCluster.Annotations = map[string]string{
					gsannotations.AWSDNSMode: "non-private",
				}
			})
			It("returns early", func() {
				Expect(reconcileErr).NotTo(HaveOccurred())
				Expect(ec2Client.CreateSecurityGroupForResolverEndpointsCallCount()).To(BeZero())
			})
		})

		When("is using private DNS mode", func() {
			It("creates security group", func() {
				Expect(ec2Client.CreateSecurityGroupForResolverEndpointsCallCount()).To(Equal(1))
				_, vpcId, groupName := ec2Client.CreateSecurityGroupForResolverEndpointsArgsForCall(0)
				Expect(vpcId).To(Equal(awsCluster.Spec.NetworkSpec.VPC.ID))
				Expect(groupName).To(Equal(fmt.Sprintf("%s-resolverrules-endpoints", ClusterName)))
				Expect(reconcileErr).NotTo(HaveOccurred())
			})

			When("creating security group fails", func() {
				BeforeEach(func() {
					ec2Client.CreateSecurityGroupForResolverEndpointsReturns("", errors.New("error creating security group"))
				})
				It("returns the error", func() {
					Expect(reconcileErr).To(HaveOccurred())
				})
			})

			When("creating security group succeeds", func() {
				BeforeEach(func() {
					ec2Client.CreateSecurityGroupForResolverEndpointsReturns("my-security-group", nil)
				})

				It("creates resolver rule", func() {
					_, _, cluster, securityGroupId, domainName, resolverRuleName := resolverClient.CreateResolverRuleArgsForCall(0)
					Expect(domainName).To(Equal(fmt.Sprintf("%s.%s", ClusterName, WorkloadClusterBaseDomain)))
					Expect(resolverRuleName).To(Equal(fmt.Sprintf("giantswarm-%s", ClusterName)))
					Expect(securityGroupId).To(Equal("my-security-group"))
					Expect(cluster.Name).To(Equal("foo"))
				})

				When("creating resolver rule fails", func() {
					BeforeEach(func() {
						resolverClient.CreateResolverRuleReturns("", "", errors.New("error creating resolver rule"))
					})

					It("returns the error", func() {
						Expect(reconcileErr).To(HaveOccurred())
					})
				})

				When("creating resolver rule succeeds", func() {
					BeforeEach(func() {
						resolverClient.CreateResolverRuleReturns("resolver-rule-principal-arn", "resolver-rule-id", nil)
					})

					It("creates ram share resource", func() {
						_, resourceShareName, allowPrincipals, principals, resourceArns := ramClient.CreateResourceShareWithContextArgsForCall(0)
						Expect(resourceShareName).To(Equal(fmt.Sprintf("giantswarm-%s-rr", ClusterName)))
						Expect(allowPrincipals).To(Equal(true))
						Expect(principals).To(Equal([]string{"resolver-rule-principal-arn"}))
						Expect(resourceArns).To(Equal([]string{DnsServerAWSAccountId}))
					})

					When("creating ram share resource fails", func() {
						BeforeEach(func() {
							ramClient.CreateResourceShareWithContextReturns("", errors.New("error creating ram"))
						})

						It("returns the error", func() {
							Expect(reconcileErr).To(HaveOccurred())
						})
					})

					When("creating ram share resource succeeds", func() {
						BeforeEach(func() {
							ramClient.CreateResourceShareWithContextReturns("resource-share-arn", nil)
						})

						It("associates resolver rule with VPC account", func() {
							_, associationName, vpcId, resolverRuleId := dnsServerResolverClient.AssociateResolverRuleWithContextArgsForCall(0)
							Expect(associationName).To(Equal(fmt.Sprintf("giantswarm-%s-rr-association", ClusterName)))
							Expect(vpcId).To(Equal(DnsServerVPCId))
							Expect(resolverRuleId).To(Equal("resolver-rule-id"))
						})

						When("creating associating resolver rule fails", func() {
							BeforeEach(func() {
								dnsServerResolverClient.AssociateResolverRuleWithContextReturns(errors.New("error associating resolver rule with vpc"))
							})

							It("returns the error", func() {
								Expect(reconcileErr).To(HaveOccurred())
							})
						})
					})
				})
			})
		})
	})
})
