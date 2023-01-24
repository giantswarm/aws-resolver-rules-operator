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

var _ = Describe("Resolver rules reconciler", func() {
	var (
		awsClusterClient        *controllersfakes.FakeAWSClusterClient
		ctx                     context.Context
		reconciler              *controllers.ResolverRulesReconciler
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
		WorkloadClusterVPCCidr    = "10.0.0.0/16"
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

		reconciler = controllers.NewResolverRulesReconciler(awsClusterClient, resolver)
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
			},
			Spec: capa.AWSClusterSpec{
				NetworkSpec: capa.NetworkSpec{
					VPC: capa.VPCSpec{
						ID:        WorkloadClusterVPCId,
						CidrBlock: WorkloadClusterVPCCidr,
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

	When("there is an error trying to get the AWSCluster being reconciled", func() {
		BeforeEach(func() {
			awsClusterClient.GetReturns(awsCluster, errors.New("failed fetching the AWSCluster"))
		})

		It("returns the error", func() {
			Expect(awsClusterClient.AddFinalizerCallCount()).To(BeZero())
			Expect(reconcileErr).To(HaveOccurred())
		})
	})

	When("there is an error trying to get the owner Cluster", func() {
		BeforeEach(func() {
			awsClusterClient.GetReturns(awsCluster, nil)
			awsClusterClient.GetOwnerReturns(nil, errors.New("failed fetching the owner Cluster CR"))
		})

		It("returns the error", func() {
			Expect(awsClusterClient.AddFinalizerCallCount()).To(BeZero())
			Expect(reconcileErr).To(HaveOccurred())
		})
	})

	When("the cluster does not have an owner yet", func() {
		BeforeEach(func() {
			awsClusterClient.GetReturns(awsCluster, nil)
			awsClusterClient.GetOwnerReturns(nil, nil)
		})

		It("does not really reconcile", func() {
			Expect(awsClusterClient.AddFinalizerCallCount()).To(BeZero())
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
			Expect(awsClusterClient.AddFinalizerCallCount()).To(BeZero())
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
			Expect(awsClusterClient.AddFinalizerCallCount()).To(BeZero())
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

		When("we get an error trying to get the cluster identity", func() {
			BeforeEach(func() {
				awsClusterClient.GetIdentityReturns(nil, errors.New("failed getting the cluster identity"))
			})

			It("doesn't really reconcile", func() {
				Expect(awsClusterClient.AddFinalizerCallCount()).To(BeZero())
				Expect(reconcileErr).To(HaveOccurred())
			})
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
				Expect(awsClusterClient.AddFinalizerCallCount()).To(BeZero())
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
					Expect(awsClusterClient.AddFinalizerCallCount()).To(BeZero())
					Expect(reconcileErr).NotTo(HaveOccurred())
					Expect(ec2Client.CreateSecurityGroupForResolverEndpointsCallCount()).To(BeZero())
					Expect(ramClient.DeleteResourceShareWithContextCallCount()).To(BeZero())
				})
			})

			When("is using private DNS mode", func() {
				BeforeEach(func() {
					awsClusterClient.GetReturns(awsCluster, nil)
					awsCluster.Annotations = map[string]string{
						gsannotations.AWSDNSMode: gsannotations.DNSModePrivate,
					}
				})

				When("the cluster is not being deleted", func() {

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
								Expect(ramClient.CreateResourceShareWithContextCallCount()).To(BeZero())
								Expect(reconcileErr).To(HaveOccurred())
							})
						})

						When("creating resolver rule succeeds", func() {
							BeforeEach(func() {
								resolverClient.CreateResolverRuleReturns(resolver.ResolverRule{Id: "resolver-rule-id", Arn: "resolver-rule-principal-arn"}, nil)
							})

							It("creates ram share resource", func() {
								_, _, resourceShareName, principal, resourceArn := ramClient.CreateResourceShareWithContextArgsForCall(0)
								Expect(resourceShareName).To(Equal(fmt.Sprintf("giantswarm-%s-rr", ClusterName)))
								Expect(principal).To(Equal("resolver-rule-principal-arn"))
								Expect(resourceArn).To(Equal(DnsServerAWSAccountId))
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

								When("associating resolver rule with the DNS server VPC fails", func() {
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

					When("the AWS Account id annotation is missing", func() {
						BeforeEach(func() {
							delete(awsCluster.Annotations, gsannotations.ResolverRulesOwnerAWSAccountId)
						})

						It("doesn't really reconcile", func() {
							Expect(resolverClient.FindResolverRulesByAWSAccountIdCallCount()).To(BeZero())
							Expect(reconcileErr).NotTo(HaveOccurred())
						})
					})

					When("the AWS Account id annotation is set", func() {
						When("to an non empty string", func() {
							BeforeEach(func() {
								awsCluster.Annotations[gsannotations.ResolverRulesOwnerAWSAccountId] = "0000000000"
							})

							When("finding resolver rules on AWS account fails", func() {
								BeforeEach(func() {
									resolverClient.FindResolverRulesByAWSAccountIdReturns([]resolver.ResolverRule{}, errors.New("failed trying to find resolver rules on AWS account"))
								})

								It("it doesn't associate resolver rules and returns error", func() {
									Expect(resolverClient.AssociateResolverRuleWithContextCallCount()).To(BeZero())
								})
							})

							When("finding resolver rules on AWS account succeeds", func() {
								var existingResolverRules = []resolver.ResolverRule{
									{
										Id:   "a1",
										Arn:  "a1",
										Name: "resolver-rule-a1",
									},
									{
										Id:   "b2",
										Arn:  "b2",
										Name: "resolver-rule-b2",
									},
									{
										Id:   "c3",
										Arn:  "c3",
										Name: "resolver-rule-c3",
									},
									{
										Id:   "d4",
										Arn:  "d4",
										Name: "resolver-rule-d4",
									},
								}
								BeforeEach(func() {
									resolverClient.FindResolverRulesByAWSAccountIdReturns(existingResolverRules, nil)
									resolverClient.AssociateResolverRuleWithContextReturnsOnCall(1, errors.New("failed trying to associate resolver rule"))
									resolverClient.AssociateResolverRuleWithContextReturnsOnCall(2, errors.New("failed trying to associate resolver rule"))
								})

								It("associates resolver rules even when it fails associating some of them ", func() {
									Expect(resolverClient.AssociateResolverRuleWithContextCallCount()).To(Equal(len(existingResolverRules)))
									_, _, associationName, vpcId, resolverRuleId := resolverClient.AssociateResolverRuleWithContextArgsForCall(0)
									Expect(associationName).To(Equal(existingResolverRules[0].Name))
									Expect(resolverRuleId).To(Equal(existingResolverRules[0].Id))
									Expect(vpcId).To(Equal(awsCluster.Spec.NetworkSpec.VPC.ID))
								})

								When("some rules belong to the workload cluster VPC cidr", func() {
									var existingResolverRules = []resolver.ResolverRule{
										{
											Id:   "a1",
											Arn:  "a1",
											Name: "resolver-rule-a1",
											IPs:  []string{"10.0.0.2"},
										},
										{
											Id:   "b2",
											Arn:  "b2",
											Name: "resolver-rule-b2",
											IPs:  []string{"10.0.0.3"},
										},
										{
											Id:   "c3",
											Arn:  "c3",
											Name: "resolver-rule-c3",
											IPs:  []string{"10.0.0.4"},
										},
										{
											Id:   "d4",
											Arn:  "d4",
											Name: "resolver-rule-d4",
										},
									}
									BeforeEach(func() {
										resolverClient.FindResolverRulesByAWSAccountIdReturns(existingResolverRules, nil)
									})

									It("does not associate the rules that belong to the WC VPC cidr", func() {
										Expect(resolverClient.AssociateResolverRuleWithContextCallCount()).To(Equal(1))
										_, _, associationName, vpcId, resolverRuleId := resolverClient.AssociateResolverRuleWithContextArgsForCall(0)
										Expect(associationName).To(Equal(existingResolverRules[3].Name))
										Expect(resolverRuleId).To(Equal(existingResolverRules[3].Id))
										Expect(vpcId).To(Equal(awsCluster.Spec.NetworkSpec.VPC.ID))
									})
								})
							})
						})
						When("to a empty string", func() {
							BeforeEach(func() {
								awsCluster.Annotations[gsannotations.ResolverRulesOwnerAWSAccountId] = ""
							})

							It("it still associates resolver rules", func() {
								Expect(resolverClient.FindResolverRulesByAWSAccountIdCallCount()).To(Equal(1))
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
							Expect(awsClusterClient.RemoveFinalizerCallCount()).To(BeZero())
						})
					})

					When("removing the ram share resource succeeded", func() {
						It("deletes the security group", func() {
							_, _, vpcId, groupName := ec2Client.DeleteSecurityGroupForResolverEndpointsArgsForCall(0)
							Expect(vpcId).To(Equal(WorkloadClusterVPCId))
							Expect(groupName).To(Equal("foo-resolverrules-endpoints"))
						})

						When("removing the security group fails", func() {
							BeforeEach(func() {
								ec2Client.DeleteSecurityGroupForResolverEndpointsReturns(errors.New("failed deleting security group"))
							})

							It("does not delete the finalizer", func() {
								Expect(awsClusterClient.RemoveFinalizerCallCount()).To(BeZero())
							})
						})

						When("removing the security group succeeds", func() {
							When("it fails trying to fetch the Resolver Rule", func() {
								BeforeEach(func() {
									resolverClient.GetResolverRuleByNameReturns(resolver.ResolverRule{}, errors.New("failed trying to fetch resolver rule"))
								})

								It("does not tries to delete the resolver rule", func() {
									Expect(reconcileErr).To(HaveOccurred())
								})

								It("does not delete the finalizer", func() {
									Expect(awsClusterClient.RemoveFinalizerCallCount()).To(BeZero())
								})
							})

							When("the resolver rule is already deleted", func() {
								BeforeEach(func() {
									resolverClient.GetResolverRuleByNameReturns(resolver.ResolverRule{}, &resolver.ResolverRuleNotFoundError{})
								})

								It("does not tries to delete the resolver rule", func() {
									Expect(dnsServerResolverClient.DisassociateResolverRuleWithContextCallCount()).To(BeZero())
									Expect(resolverClient.DeleteResolverRuleCallCount()).To(BeZero())
									Expect(reconcileErr).NotTo(HaveOccurred())
								})

								It("deletes the finalizer", func() {
									Expect(awsClusterClient.RemoveFinalizerCallCount()).To(Equal(1))
								})
							})

							When("the resolver rule still exists", func() {
								BeforeEach(func() {
									resolverClient.GetResolverRuleByNameReturns(resolver.ResolverRule{Id: "resolver-rule-id", Arn: "resolver-rule-arn"}, nil)
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
										Expect(awsClusterClient.RemoveFinalizerCallCount()).To(BeZero())
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
											Expect(awsClusterClient.RemoveFinalizerCallCount()).To(BeZero())
										})
									})
								})
							})
						})
					})

					When("the AWS Account id annotation is missing", func() {
						BeforeEach(func() {
							delete(awsCluster.Annotations, gsannotations.ResolverRulesOwnerAWSAccountId)
						})

						It("doesn't disassociate resolver rules but removes the finalizer", func() {
							Expect(resolverClient.FindResolverRulesByAWSAccountIdCallCount()).To(BeZero())
							Expect(reconcileErr).NotTo(HaveOccurred())
							Expect(awsClusterClient.RemoveFinalizerCallCount()).To(Equal(1))
						})
					})

					When("the AWS Account id annotation is set", func() {
						BeforeEach(func() {
							awsCluster.Annotations[gsannotations.ResolverRulesOwnerAWSAccountId] = "0000000000"
						})

						When("finding resolver rules on AWS account fails", func() {
							BeforeEach(func() {
								resolverClient.FindResolverRulesByAWSAccountIdReturns([]resolver.ResolverRule{}, errors.New("failed trying to find resolver rules on AWS account"))
							})

							It("does not try to disassociate AWS account resolver rules with workload cluster", func() {
								Expect(resolverClient.DisassociateResolverRuleWithContextCallCount()).To(BeZero())
							})

							It("does not delete the finalizer", func() {
								Expect(awsClusterClient.RemoveFinalizerCallCount()).To(BeZero())
							})
						})

						When("finding resolver rules on AWS account succeeds", func() {
							var existingResolverRules = []resolver.ResolverRule{
								{
									Id:   "a1",
									Arn:  "a1",
									Name: "resolver-rule-a1",
								},
								{
									Id:   "b2",
									Arn:  "b2",
									Name: "resolver-rule-b2",
								},
								{
									Id:   "c3",
									Arn:  "c3",
									Name: "resolver-rule-c3",
								},
								{
									Id:   "d4",
									Arn:  "d4",
									Name: "resolver-rule-d4",
								},
							}
							BeforeEach(func() {
								resolverClient.FindResolverRulesByAWSAccountIdReturns(existingResolverRules, nil)
								resolverClient.DisassociateResolverRuleWithContextReturnsOnCall(1, errors.New("failed trying to disassociate resolver rule"))
							})

							It("disassociates resolver rules from given AWS Account from workload cluster VPC, even if some of them fail", func() {
								Expect(resolverClient.DisassociateResolverRuleWithContextCallCount()).To(Equal(len(existingResolverRules)))
								_, _, vpcId, resolverRuleId := resolverClient.DisassociateResolverRuleWithContextArgsForCall(0)
								Expect(resolverRuleId).To(Equal(existingResolverRules[0].Id))
								Expect(vpcId).To(Equal(awsCluster.Spec.NetworkSpec.VPC.ID))
							})

							It("deletes the finalizer", func() {
								Expect(awsClusterClient.RemoveFinalizerCallCount()).To(Equal(1))
							})
						})
					})
				})
			})
		})
	})
})
