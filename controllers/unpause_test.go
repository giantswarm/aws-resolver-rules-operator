package controllers_test

import (
	"context"
	"errors"

	gsannotations "github.com/giantswarm/k8smetadata/pkg/annotation"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	capa "sigs.k8s.io/cluster-api-provider-aws/api/v1beta1"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/aws-resolver-rules-operator/controllers"
	"github.com/aws-resolver-rules-operator/controllers/controllersfakes"
)

var _ = Describe("Unpause reconciler", func() {
	var (
		awsClusterClient *controllersfakes.FakeAWSClusterClient
		ctx              context.Context
		reconciler       *controllers.UnpauseReconciler
		cluster          *capi.Cluster
		awsCluster       *capa.AWSCluster
		result           ctrl.Result
		reconcileErr     error
	)

	const (
		ClusterName = "foo"
	)

	BeforeEach(func() {
		ctx = context.Background()
		awsClusterClient = new(controllersfakes.FakeAWSClusterClient)

		reconciler = controllers.NewUnpauseReconciler(awsClusterClient)
		awsCluster = &capa.AWSCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ClusterName,
				Namespace: "bar",
			},
			Spec: capa.AWSClusterSpec{},
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

	When("the cluster has an owner", func() {
		BeforeEach(func() {
			awsClusterClient.GetReturns(awsCluster, nil)
			awsClusterClient.GetOwnerReturns(cluster, nil)
		})

		When("is not using private VPC mode", func() {
			BeforeEach(func() {
				awsClusterClient.GetReturns(awsCluster, nil)
				if awsCluster.Annotations == nil {
					awsCluster.Annotations = map[string]string{}
				}
				awsCluster.Annotations[gsannotations.AWSVPCMode] = "non-private"
			})
			It("doesn't really reconcile", func() {
				Expect(reconcileErr).NotTo(HaveOccurred())
			})
		})

		When("the cluster is being deleted", func() {
			BeforeEach(func() {
				deletionTime := metav1.Now()
				awsCluster.DeletionTimestamp = &deletionTime
			})

			It("nothing really happens", func() {
				Expect(awsClusterClient.UnpauseCallCount()).To(BeZero())
				Expect(reconcileErr).NotTo(HaveOccurred())
			})
		})

		When("the cluster is not being deleted", func() {
			When("the cluster is using private VPC mode", func() {
				BeforeEach(func() {
					awsClusterClient.GetReturns(awsCluster, nil)
					if awsCluster.Annotations == nil {
						awsCluster.Annotations = map[string]string{}
					}
					awsCluster.Annotations[gsannotations.AWSVPCMode] = gsannotations.AWSVPCModePrivate
				})

				When("VPC and Subnets conditions are Ready", func() {
					BeforeEach(func() {
						awsCluster.Status.Conditions = []capi.Condition{
							{
								Type:   capa.VpcReadyCondition,
								Status: v1.ConditionTrue,
							},
							{
								Type:   capa.SubnetsReadyCondition,
								Status: v1.ConditionTrue,
							},
						}
					})

					It("unpauses the cluster", func() {
						Expect(awsClusterClient.UnpauseCallCount()).To(Equal(1))
					})

					When("it fails trying to unpause the cluster", func() {
						BeforeEach(func() {
							awsClusterClient.UnpauseReturns(errors.New("failed unpausing the AWSCluster"))
						})

						It("returns the error", func() {
							Expect(reconcileErr).To(HaveOccurred())
						})
					})
				})

				When("VPC Ready condition is not Ready yet", func() {
					BeforeEach(func() {
						awsCluster.Status.Conditions = []capi.Condition{
							{
								Type:   capa.SubnetsReadyCondition,
								Status: v1.ConditionTrue,
							},
						}
					})

					It("does not unpauses the cluster", func() {
						Expect(awsClusterClient.UnpauseCallCount()).To(BeZero())
					})
				})

				When("Subnet Ready condition is not Ready yet", func() {
					BeforeEach(func() {
						awsCluster.Status.Conditions = []capi.Condition{
							{
								Type:   capa.VpcReadyCondition,
								Status: v1.ConditionTrue,
							},
						}
					})

					It("does not unpauses the cluster", func() {
						Expect(awsClusterClient.UnpauseCallCount()).To(BeZero())
					})
				})

				When("the cluster is using private DNS mode", func() {
					BeforeEach(func() {
						awsClusterClient.GetReturns(awsCluster, nil)
						awsCluster.Annotations[gsannotations.AWSDNSMode] = gsannotations.DNSModePrivate
					})

					When("VPC, Subnets and ResolverRules conditions are Ready", func() {
						BeforeEach(func() {
							awsCluster.Status.Conditions = []capi.Condition{
								{
									Type:   capa.VpcReadyCondition,
									Status: v1.ConditionTrue,
								},
								{
									Type:   capa.SubnetsReadyCondition,
									Status: v1.ConditionTrue,
								},
								{
									Type:   controllers.ResolverRulesAssociatedCondition,
									Status: v1.ConditionTrue,
								},
							}
						})

						It("unpauses the cluster", func() {
							Expect(awsClusterClient.UnpauseCallCount()).To(Equal(1))
						})
					})

					When("ResolverRules Ready condition is not Ready yet", func() {
						BeforeEach(func() {
							awsCluster.Status.Conditions = []capi.Condition{
								{
									Type:   capa.VpcReadyCondition,
									Status: v1.ConditionTrue,
								},
								{
									Type:   capa.SubnetsReadyCondition,
									Status: v1.ConditionTrue,
								},
							}
						})

						It("does not unpauses the cluster", func() {
							Expect(awsClusterClient.UnpauseCallCount()).To(BeZero())
						})
					})
				})
			})

			// When("ResolverRules Ready condition is not Ready yet", func() {
			// 	BeforeEach(func() {
			// 		awsCluster.Status.Conditions = []capi.Condition{
			// 			{
			// 				Type:   capa.VpcReadyCondition,
			// 				Status: v1.ConditionTrue,
			// 			},
			// 			{
			// 				Type:   capa.SubnetsReadyCondition,
			// 				Status: v1.ConditionTrue,
			// 			},
			// 		}
			// 	})
			//
			// 	It("does not unpauses the cluster", func() {
			// 		Expect(awsClusterClient.UnpauseCallCount()).To(BeZero())
			// 	})
			// })
		})
	})
})
