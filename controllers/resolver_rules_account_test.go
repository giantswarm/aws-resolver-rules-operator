package controllers_test

import (
	"context"
	"errors"

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

var _ = Describe("Resolver Rules account reconciler", func() {
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
		resolver, err := resolver.NewResolver(fakeAWSClients, resolver.DNSServer{}, WorkloadClusterBaseDomain)
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
				Annotations: map[string]string{
					gsannotations.AWSDNSMode: gsannotations.DNSModePrivate,
				},
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
				When("the AWS Account id annotation is missing", func() {
					BeforeEach(func() {
						delete(awsCluster.Annotations, gsannotations.ResolverRulesOwnerAWSAccountId)
					})

					It("doesn't really reconcile", func() {
						Expect(awsClusterClient.AddFinalizerCallCount()).To(Equal(0))
						Expect(reconcileErr).NotTo(HaveOccurred())
					})
				})

				When("the AWS Account id annotation is set", func() {
					BeforeEach(func() {
						awsCluster.Annotations[gsannotations.ResolverRulesOwnerAWSAccountId] = "0000000000"
					})

					When("the cluster is not being deleted", func() {
						It("adds the finalizer to the AWSCluster", func() {
							Expect(awsClusterClient.AddFinalizerCallCount()).To(Equal(1))
							Expect(reconcileErr).NotTo(HaveOccurred())
						})

						When("finding resolver rules on AWS account fails", func() {
							BeforeEach(func() {
								resolverClient.FindResolverRulesByAWSAccountIdReturns([]resolver.ResolverRule{}, errors.New("failed trying to find resolver rules on AWS account"))
							})

							It("associates resolver rules in the AWS account with the workload cluster VPC", func() {
								Expect(resolverClient.AssociateResolverRuleWithContextCallCount()).To(Equal(0))
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

						When("the cluster is being deleted", func() {
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
								deletionTime := metav1.Now()
								awsCluster.DeletionTimestamp = &deletionTime
							})

							When("finding resolver rules on AWS account fails", func() {
								BeforeEach(func() {
									resolverClient.FindResolverRulesByAWSAccountIdReturns([]resolver.ResolverRule{}, errors.New("failed trying to find resolver rules on AWS account"))
								})

								It("does not try to disassociate resolver rules", func() {
									Expect(resolverClient.DisassociateResolverRuleWithContextCallCount()).To(Equal(0))
								})

								It("does not delete the finalizer", func() {
									Expect(awsClusterClient.RemoveFinalizerCallCount()).To(Equal(0))
								})
							})

							When("finding resolver rules on AWS account succeeds", func() {
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
})
