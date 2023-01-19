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
		WorkloadClusterVPCId      = "myvpc-1a2b3c4d"
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
						ID: WorkloadClusterVPCId,
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

		It("does not really reconcile", func() {
			Expect(awsClusterClient.AddFinalizerCallCount()).To(Equal(0))
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

		It("does not really reconcile", func() {
			Expect(awsClusterClient.AddFinalizerCallCount()).To(Equal(0))
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

		It("does not really reconcile", func() {
			Expect(awsClusterClient.AddFinalizerCallCount()).To(Equal(0))
			Expect(result.Requeue).To(BeFalse())
			Expect(result.RequeueAfter).To(BeZero())
			Expect(reconcileErr).NotTo(HaveOccurred())
		})
	})

	When("the cluster has an owner and it's not paused", func() {
		BeforeEach(func() {
			awsClusterClient.GetReturns(awsCluster, nil)
			awsClusterClient.GetOwnerReturns(cluster, nil)
		})

		When("the cluster has no identity set", func() {
			BeforeEach(func() {
				awsClusterClient.GetIdentityReturns(nil, nil)
			})

			It("gets the cluster and owner cluster", func() {
				Expect(awsClusterClient.GetCallCount()).To(Equal(1))
				Expect(awsClusterClient.GetOwnerCallCount()).To(Equal(1))

				_, actualCluster := awsClusterClient.GetOwnerArgsForCall(0)
				Expect(actualCluster).To(Equal(awsCluster))
				Expect(reconcileErr).NotTo(HaveOccurred())
			})

			It("doesn't really reconcile", func() {
				Expect(awsClusterClient.AddFinalizerCallCount()).To(Equal(0))
				Expect(result.Requeue).To(BeFalse())
				Expect(result.RequeueAfter).To(BeZero())
				Expect(reconcileErr).NotTo(HaveOccurred())
			})
		})

		When("the cluster has an identity set", func() {
			BeforeEach(func() {
				awsClusterClient.GetIdentityReturns(awsClusterRoleIdentity, nil)
			})

			When("is not using private DNS mode", func() {
				BeforeEach(func() {
					awsClusterClient.GetReturns(awsCluster, nil)
					awsCluster.Annotations = map[string]string{
						gsannotations.AWSDNSMode: "non-private",
					}
				})
				It("doesn't really reconcile", func() {
					Expect(awsClusterClient.AddFinalizerCallCount()).To(Equal(0))
					Expect(reconcileErr).NotTo(HaveOccurred())
					Expect(ec2Client.CreateSecurityGroupForResolverEndpointsCallCount()).To(BeZero())
				})
			})

			When("is using private DNS mode", func() {
				It("adds the finalizer to the AWSCluster", func() {
					Expect(awsClusterClient.AddFinalizerCallCount()).To(Equal(1))
					Expect(reconcileErr).NotTo(HaveOccurred())
				})

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
							resolverClient.CreateResolverRuleReturns(resolver.ResolverRule{}, errors.New("error creating resolver rule"))
						})

						It("returns the error", func() {
							Expect(reconcileErr).To(HaveOccurred())
						})
					})

					When("creating resolver rule succeeds", func() {
						BeforeEach(func() {
							resolverClient.CreateResolverRuleReturns(resolver.ResolverRule{RuleId: "resolver-rule-id", RuleArn: "resolver-rule-principal-arn"}, nil)
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
								_, _, associationName, vpcId, resolverRuleId := dnsServerResolverClient.AssociateResolverRuleWithContextArgsForCall(0)
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

			When("the cluster is being deleted", func() {
				BeforeEach(func() {
					deletionTime := metav1.Now()
					awsCluster.DeletionTimestamp = &deletionTime
				})

				It("deletes the ram share resource", func() {
					_, _, resourceShareName := ramClient.DeleteResourceShareWithContextArgsForCall(0)
					Expect(resourceShareName).To(Equal("giantswarm-foo-rr"))
				})

				When("removing the ram share resource fails", func() {
					BeforeEach(func() {
						ramClient.DeleteResourceShareWithContextReturns(errors.New("failing deleting ram resource share"))
					})

					It("does not delete the finalizer", func() {
						Expect(awsClusterClient.RemoveFinalizerCallCount()).To(Equal(0))
					})
				})

				When("removing the ram share resource succeeded", func() {
					It("deletes the security group", func() {
						_, _, vpcId, groupName := ec2Client.DeleteSecurityGroupForResolverEndpointsArgsForCall(0)
						Expect(vpcId).To(Equal(WorkloadClusterVPCId))
						Expect(groupName).To(Equal("foo-resolverrules-endpoints"))
					})

					It("deletes the finalizer", func() {
						Expect(awsClusterClient.RemoveFinalizerCallCount()).To(Equal(1))
					})

					When("removing the security group fails", func() {
						BeforeEach(func() {
							ec2Client.DeleteSecurityGroupForResolverEndpointsReturns(errors.New("failed deleting security group"))
						})

						It("does not delete the finalizer", func() {
							Expect(awsClusterClient.RemoveFinalizerCallCount()).To(Equal(0))
						})
					})
				})

				When("it fails trying to fetch the Resolver Rule", func() {
					BeforeEach(func() {
						resolverClient.GetResolverRuleByNameReturns(resolver.ResolverRule{}, errors.New("failed trying to fetch resolver rule"))
					})

					It("does not tries to delete the resolver rule", func() {
						Expect(reconcileErr).To(HaveOccurred())
					})

					It("does not delete the finalizer", func() {
						Expect(awsClusterClient.RemoveFinalizerCallCount()).To(Equal(0))
					})
				})

				When("the resolver rule is already deleted", func() {
					BeforeEach(func() {
						resolverClient.GetResolverRuleByNameReturns(resolver.ResolverRule{}, &resolver.ResolverRuleNotFoundError{})
					})

					It("does not tries to delete the resolver rule", func() {
						Expect(dnsServerResolverClient.DisassociateResolverRuleWithContextCallCount()).To(Equal(0))
						Expect(resolverClient.DeleteResolverRuleCallCount()).To(Equal(0))
						Expect(reconcileErr).NotTo(HaveOccurred())
					})

					It("deletes the finalizer", func() {
						Expect(awsClusterClient.RemoveFinalizerCallCount()).To(Equal(1))
					})
				})

				When("the resolver rule still exists", func() {
					BeforeEach(func() {
						resolverClient.GetResolverRuleByNameReturns(resolver.ResolverRule{RuleId: "resolver-rule-id", RuleArn: "resolver-rule-arn"}, nil)
					})

					It("disassociates resolver rule from VPC", func() {
						_, _, vpcId, resolverRuleId := dnsServerResolverClient.DisassociateResolverRuleWithContextArgsForCall(0)
						Expect(vpcId).To(Equal(DnsServerVPCId))
						Expect(resolverRuleId).To(Equal("resolver-rule-id"))
					})

					When("disassociating resolver rule from VPC fails", func() {
						BeforeEach(func() {
							dnsServerResolverClient.DisassociateResolverRuleWithContextReturns(errors.New("failing disassociating resolver rule"))
						})

						It("does not delete the finalizer", func() {
							Expect(awsClusterClient.RemoveFinalizerCallCount()).To(Equal(0))
						})
					})

					When("disassociating resolver rule succeeded", func() {
						It("deletes the resolver rule", func() {
							_, _, cluster, resolverRuleId := resolverClient.DeleteResolverRuleArgsForCall(0)
							Expect(resolverRuleId).To(Equal("resolver-rule-id"))
							Expect(cluster.Name).To(Equal("foo"))
						})

						It("deletes the finalizer", func() {
							Expect(awsClusterClient.RemoveFinalizerCallCount()).To(Equal(1))
						})

						When("removing the resolver rule fails", func() {
							BeforeEach(func() {
								resolverClient.DeleteResolverRuleReturns(errors.New("failing removing resolver rule"))
							})

							It("does not delete the finalizer", func() {
								Expect(awsClusterClient.RemoveFinalizerCallCount()).To(Equal(0))
							})
						})
					})
				})
			})
		})
	})
})
