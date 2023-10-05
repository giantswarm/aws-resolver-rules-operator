package controllers_test

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	capa "sigs.k8s.io/cluster-api-provider-aws/api/v1beta1"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/giantswarm/k8smetadata/pkg/annotation"

	"github.com/aws-resolver-rules-operator/controllers"
	"github.com/aws-resolver-rules-operator/controllers/controllersfakes"
	"github.com/aws-resolver-rules-operator/pkg/conditions"
	"github.com/aws-resolver-rules-operator/pkg/resolver/resolverfakes"
)

var _ = Describe("PrefixListsReconciler", func() {
	var (
		ctx context.Context

		awsClusterClient                 *controllersfakes.FakeAWSClusterClient
		prefixListClient                 *resolverfakes.FakePrefixListClient
		reconciler                       *controllers.PrefixListsReconciler
		awsCluster, managementAWSCluster *capa.AWSCluster

		reconcileResult ctrl.Result
		reconcileErr    error
	)

	const (
		ClusterName      = "foo"
		ClusterNamespace = "default"
	)

	BeforeEach(func() {
		ctx = context.Background()

		awsClusterClient = new(controllersfakes.FakeAWSClusterClient)
		prefixListClient = new(resolverfakes.FakePrefixListClient)

		managementAWSCluster = &capa.AWSCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ClusterName,
				Namespace: ClusterNamespace,
			},
			Spec: capa.AWSClusterSpec{},
		}
		awsCluster = &capa.AWSCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ClusterName,
				Namespace: ClusterNamespace,
			},
			Spec: capa.AWSClusterSpec{},
		}

		reconciler = controllers.NewPrefixListsReconciler(
			client.ObjectKeyFromObject(managementAWSCluster),
			awsClusterClient,
			prefixListClient,
		)
	})

	JustBeforeEach(func() {
		request := ctrl.Request{
			NamespacedName: k8stypes.NamespacedName{
				Name:      ClusterName,
				Namespace: ClusterNamespace,
			},
		}
		reconcileResult, reconcileErr = reconciler.Reconcile(ctx, request)
	})

	When("there is an error trying to get the AWSCluster being reconciled", func() {
		expectedError := errors.New("failed fetching the Cluster")

		BeforeEach(func() {
			awsClusterClient.GetAWSClusterReturns(nil, expectedError)
		})

		It("returns the error", func() {
			Expect(awsClusterClient.AddFinalizerCallCount()).To(BeZero())
			Expect(reconcileErr).To(HaveOccurred())
			Expect(reconcileErr).Should(MatchError(expectedError))
		})
	})

	When("reconciling an existing cluster", func() {
		BeforeEach(func() {
			awsClusterClient.GetAWSClusterReturns(awsCluster, nil)
		})

		When("the infrastructure cluster is paused", func() {
			BeforeEach(func() {
				awsCluster.Annotations = map[string]string{
					capi.PausedAnnotation: "true",
				}
			})

			It("does not reconcile", func() {
				Expect(awsClusterClient.AddFinalizerCallCount()).To(BeZero())
				Expect(reconcileErr).NotTo(HaveOccurred())
			})
		})

		When("the cluster is not the management cluster", func() {
			BeforeEach(func() {
				managementAWSCluster = &capa.AWSCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "totally-different-cluster",
						Namespace: ClusterNamespace,
					},
					Spec: capa.AWSClusterSpec{},
				}
			})

			It("does not reconcile", func() {
				Expect(reconcileErr).NotTo(HaveOccurred())
				Expect(reconcileResult.Requeue).To(BeFalse())
				Expect(prefixListClient.Invocations()).To(BeEmpty())
			})
		})

		When("the cluster is in the None mode", func() {
			BeforeEach(func() {
				awsCluster.Annotations = map[string]string{
					annotation.NetworkTopologyModeAnnotation: annotation.NetworkTopologyModeNone,
				}
			})

			It("does not reconcile", func() {
				Expect(reconcileErr).NotTo(HaveOccurred())
				Expect(reconcileResult.Requeue).To(BeFalse())
				Expect(prefixListClient.Invocations()).To(BeEmpty())
			})
		})

		When("the cluster is in the UserManaged mode", func() {
			BeforeEach(func() {
				awsCluster.Annotations = map[string]string{
					annotation.NetworkTopologyModeAnnotation: annotation.NetworkTopologyModeUserManaged,
				}
			})

			It("does not reconcile", func() {
				Expect(reconcileErr).NotTo(HaveOccurred())
				Expect(reconcileResult.Requeue).To(BeFalse())
				Expect(prefixListClient.Invocations()).To(BeEmpty())
			})
		})

		When("the cluster is in the GiantSwarmManaged mode", func() {
			BeforeEach(func() {
				awsCluster.Annotations = map[string]string{
					annotation.NetworkTopologyModeAnnotation: annotation.NetworkTopologyModeGiantSwarmManaged,
				}
			})

			It("adds the NetworkTopologyCondition condition to the cluster", func() {
				Expect(awsCluster.Status.Conditions).To(ContainElements(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(conditions.NetworkTopologyCondition),
					"Status": Equal(corev1.ConditionFalse),
				})))
			})

			It("adds the finalizer to the AWSCluster", func() {
				Expect(awsClusterClient.AddFinalizerCallCount()).To(Equal(1))
				Expect(reconcileErr).NotTo(HaveOccurred())
			})

			It("creates the prefix list", func() {
				Expect(prefixListClient.ApplyCallCount()).To(Equal(1))
				Expect(reconcileErr).NotTo(HaveOccurred())
			})

			When("creating the prefix list fails", func() {
				err := errors.New("error creating prefix list")
				BeforeEach(func() {
					prefixListClient.ApplyReturns("", err)
				})

				It("returns an error", func() {
					Expect(reconcileErr).To(MatchError(err))
				})
			})

			When("creating the prefix list successfully", func() {
				BeforeEach(func() {
					prefixListClient.ApplyReturns("prefix-list-id", nil)
				})

				It("adds the created prefix list as annotation", func() {
					Expect(awsCluster.Annotations).To(HaveKeyWithValue(annotation.NetworkTopologyPrefixListIDAnnotation, "prefix-list-id"))
					Expect(reconcileErr).NotTo(HaveOccurred())
				})
			})
		})

		When("the cluster is being deleted", func() {
			BeforeEach(func() {
				deletionTime := metav1.Now()
				awsCluster.DeletionTimestamp = &deletionTime
			})

			It("deletes the prefix list", func() {
				_, clusterName := prefixListClient.DeleteArgsForCall(0)
				Expect(clusterName).To(Equal(ClusterName))
			})
		})
	})
})
