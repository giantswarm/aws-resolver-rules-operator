package controllers_test

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	k8stypes "k8s.io/apimachinery/pkg/types"
	capa "sigs.k8s.io/cluster-api-provider-aws/api/v1beta1"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/giantswarm/k8smetadata/pkg/annotation"

	"github.com/aws-resolver-rules-operator/controllers"
	"github.com/aws-resolver-rules-operator/pkg/conditions"
	"github.com/aws-resolver-rules-operator/pkg/k8sclient"
	"github.com/aws-resolver-rules-operator/pkg/resolver/resolverfakes"
)

var _ = Describe("ManagementClusterTransitGatewayReconciler", func() {
	var (
		ctx context.Context

		transitGatewayClient *resolverfakes.FakeTransitGatewayClient
		prefixListClient     *resolverfakes.FakePrefixListClient
		reconciler           *controllers.ManagementClusterNetworkReconciler

		transitGatewayARN   string
		prefixListARN       string
		requestResourceName string
		reconcileResult     ctrl.Result
		reconcileErr        error

		cluster *capa.AWSCluster
	)

	getActualCluster := func() *capa.AWSCluster {
		actualCluster := &capa.AWSCluster{}
		err := k8sClient.Get(ctx, k8stypes.NamespacedName{Name: requestResourceName, Namespace: namespace}, actualCluster)
		Expect(err).NotTo(HaveOccurred())

		return actualCluster
	}

	BeforeEach(func() {
		ctx = context.Background()

		cluster = newRandomCluster(
			annotation.NetworkTopologyModeAnnotation,
			annotation.NetworkTopologyModeGiantSwarmManaged,
		)
		requestResourceName = cluster.Name
		transitGatewayARN = fmt.Sprintf("arn:aws:iam::123456789012:transit-gateways/%s", uuid.NewString())
		prefixListARN = fmt.Sprintf("arn:aws:iam::123456789012:managed-prefix-lists/%s", uuid.NewString())

		clusterClient := k8sclient.NewAWSClusterClient(k8sClient)
		transitGatewayClient = new(resolverfakes.FakeTransitGatewayClient)
		prefixListClient = new(resolverfakes.FakePrefixListClient)
		reconciler = controllers.NewManagementClusterTransitGateway(
			client.ObjectKeyFromObject(cluster),
			clusterClient,
			transitGatewayClient,
			prefixListClient,
		)
	})

	JustBeforeEach(func() {
		request := ctrl.Request{
			NamespacedName: k8stypes.NamespacedName{
				Name:      requestResourceName,
				Namespace: namespace,
			},
		}
		reconcileResult, reconcileErr = reconciler.Reconcile(ctx, request)
	})

	Describe("pre-reconciliation", func() {
		It("adds a finalizer to the cluster", func() {
			Expect(reconcileErr).NotTo(HaveOccurred())
			Expect(reconcileResult.Requeue).To(BeFalse())

			actualCluster := getActualCluster()
			Expect(actualCluster.Finalizers).To(ContainElement(controllers.FinalizerManagementCluster))
		})

		It("adds the TransitGatewayCreated condition to the cluster", func() {
			actualCluster := getActualCluster()
			Expect(actualCluster.Status.Conditions).To(ContainElements(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(conditions.TransitGatewayCreated),
				"Status": Equal(corev1.ConditionTrue),
			})))
		})

		When("the cluster does not exist", func() {
			BeforeEach(func() {
				requestResourceName = "does-not-exist"
			})

			It("does not reconcile", func() {
				Expect(reconcileErr).NotTo(HaveOccurred())
				Expect(reconcileResult.Requeue).To(BeFalse())
				Expect(transitGatewayClient.Invocations()).To(BeEmpty())
			})
		})

		When("the cluster is paused", func() {
			BeforeEach(func() {
				patchedCluster := cluster.DeepCopy()
				patchedCluster.Annotations = map[string]string{
					capi.PausedAnnotation: "true",
				}

				err := k8sClient.Patch(context.Background(), patchedCluster, client.MergeFrom(cluster))
				Expect(err).NotTo(HaveOccurred())
			})

			It("does not reconcile", func() {
				Expect(reconcileErr).NotTo(HaveOccurred())
				Expect(reconcileResult.Requeue).To(BeFalse())
				Expect(transitGatewayClient.Invocations()).To(BeEmpty())
			})
		})

		When("the cluster is not the management cluster", func() {
			BeforeEach(func() {
				otherCluster := newRandomCluster()
				requestResourceName = otherCluster.Name
			})

			It("does not reconcile", func() {
				Expect(reconcileErr).NotTo(HaveOccurred())
				Expect(reconcileResult.Requeue).To(BeFalse())
				Expect(transitGatewayClient.Invocations()).To(BeEmpty())
			})
		})

		When("the cluster is in the None mode", func() {
			BeforeEach(func() {
				patchedCluster := cluster.DeepCopy()
				patchedCluster.Annotations = map[string]string{
					annotation.NetworkTopologyModeAnnotation: annotation.NetworkTopologyModeNone,
				}

				err := k8sClient.Patch(context.Background(), patchedCluster, client.MergeFrom(cluster))
				Expect(err).NotTo(HaveOccurred())
			})

			It("does not reconcile", func() {
				Expect(reconcileErr).NotTo(HaveOccurred())
				Expect(reconcileResult.Requeue).To(BeFalse())
				Expect(transitGatewayClient.Invocations()).To(BeEmpty())
			})
		})

		When("the cluster is in the UserManaged mode", func() {
			BeforeEach(func() {
				patchedCluster := cluster.DeepCopy()
				patchedCluster.Annotations = map[string]string{
					annotation.NetworkTopologyModeAnnotation: annotation.NetworkTopologyModeUserManaged,
				}

				err := k8sClient.Patch(context.Background(), patchedCluster, client.MergeFrom(cluster))
				Expect(err).NotTo(HaveOccurred())
			})

			It("does not reconcile", func() {
				Expect(reconcileErr).NotTo(HaveOccurred())
				Expect(reconcileResult.Requeue).To(BeFalse())
				Expect(transitGatewayClient.Invocations()).To(BeEmpty())
			})
		})
	})

	Describe("GiantSwarm Managed Mode", func() {
		BeforeEach(func() {
			patchedCluster := cluster.DeepCopy()
			patchedCluster.Annotations = map[string]string{
				annotation.NetworkTopologyModeAnnotation: annotation.NetworkTopologyModeGiantSwarmManaged,
			}

			err := k8sClient.Patch(context.Background(), patchedCluster, client.MergeFrom(cluster))
			Expect(err).NotTo(HaveOccurred())

			transitGatewayClient.ApplyReturns(transitGatewayARN, nil)
			prefixListClient.ApplyReturns(prefixListARN, nil)
		})

		It("does not change the mode annotation value", func() {
			actualCluster := getActualCluster()
			Expect(actualCluster.Annotations).To(HaveKeyWithValue(
				annotation.NetworkTopologyModeAnnotation,
				annotation.NetworkTopologyModeGiantSwarmManaged,
			))
		})

		It("does not requeue the event", func() {
			Expect(reconcileErr).NotTo(HaveOccurred())
			Expect(reconcileResult.Requeue).To(BeFalse())
		})

		It("creates a transit gateway", func() {
			Expect(transitGatewayClient.ApplyCallCount()).To(Equal(1))
		})

		It("sets the transit gateway id annotation", func() {
			actualCluster := getActualCluster()
			Expect(actualCluster.Annotations).To(HaveKeyWithValue(
				annotation.NetworkTopologyTransitGatewayIDAnnotation,
				transitGatewayARN,
			))
		})

		It("creates a prefix list", func() {
			Expect(prefixListClient.ApplyCallCount()).To(Equal(1))
		})

		It("sets the transit gateway id annotation", func() {
			actualCluster := getActualCluster()
			Expect(actualCluster.Annotations).To(HaveKeyWithValue(
				annotation.NetworkTopologyPrefixListIDAnnotation,
				prefixListARN,
			))
		})

		When("creating the transit gateway fails", func() {
			BeforeEach(func() {
				transitGatewayClient.ApplyReturns("", errors.New("boom"))
			})

			It("returns an error", func() {
				Expect(reconcileErr).To(MatchError(ContainSubstring("boom")))
			})
		})

		When("The cluster has been deleted", func() {
			BeforeEach(func() {
				patchedCluster := cluster.DeepCopy()
				patchedCluster.Finalizers = []string{controllers.FinalizerManagementCluster}

				err := k8sClient.Patch(context.Background(), patchedCluster, client.MergeFrom(cluster))
				Expect(err).NotTo(HaveOccurred())

				err = k8sClient.Delete(context.Background(), cluster)
				Expect(err).NotTo(HaveOccurred())
			})

			It("deletes the transit gateway", func() {
				Expect(transitGatewayClient.DeleteCallCount()).To(Equal(1))
			})

			It("deletes the prefix list", func() {
				Expect(prefixListClient.DeleteCallCount()).To(Equal(1))
			})

			It("removes the finalizer", func() {
				err := k8sClient.Get(context.Background(), types.NamespacedName{Name: cluster.Name, Namespace: namespace}, cluster)
				Expect(k8serrors.IsNotFound(err)).To(BeTrue())
			})

			When("deleting the trnsit gateway fails", func() {
				BeforeEach(func() {
					transitGatewayClient.DeleteReturns(errors.New("boom"))
				})

				It("returns an error", func() {
					Expect(reconcileErr).To(MatchError(ContainSubstring("boom")))
				})
			})

			When("deleting the prefix list fails", func() {
				BeforeEach(func() {
					prefixListClient.DeleteReturns(errors.New("boom"))
				})

				It("returns an error", func() {
					Expect(reconcileErr).To(MatchError(ContainSubstring("boom")))
				})
			})
		})
	})
})
