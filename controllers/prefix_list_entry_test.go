package controllers_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/giantswarm/k8smetadata/pkg/annotation"
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

	"github.com/aws-resolver-rules-operator/controllers"
	"github.com/aws-resolver-rules-operator/pkg/conditions"
	"github.com/aws-resolver-rules-operator/pkg/k8sclient"
	"github.com/aws-resolver-rules-operator/pkg/resolver"
	"github.com/aws-resolver-rules-operator/pkg/resolver/resolverfakes"
)

var _ = Describe("PrefixListEntryReconciler", func() {
	var (
		ctx context.Context

		prefixListClient *resolverfakes.FakePrefixListClient
		reconciler       *controllers.PrefixListEntryReconciler

		requestResourceName string
		prefixListARN       string
		cluster             *capa.AWSCluster
		identity            *capa.AWSClusterRoleIdentity
		managementCluster   *capa.AWSCluster

		request      ctrl.Request
		result       ctrl.Result
		reconcileErr error
	)

	getActualCluster := func() *capa.AWSCluster {
		actualCluster := &capa.AWSCluster{}
		err := k8sClient.Get(ctx, k8stypes.NamespacedName{Name: requestResourceName, Namespace: namespace}, actualCluster)
		Expect(err).NotTo(HaveOccurred())

		return actualCluster
	}

	BeforeEach(func() {
		ctx = context.Background()
		prefixListARN = fmt.Sprintf("arn:aws:iam::123456789012:managed-prefix-lists/%s", uuid.NewString())
		identity, cluster = createRandomClusterWithIdentity(
			annotation.NetworkTopologyModeAnnotation,
			annotation.NetworkTopologyModeGiantSwarmManaged,
		)

		managementCluster = createRandomCluster(
			annotation.NetworkTopologyModeAnnotation,
			annotation.NetworkTopologyModeGiantSwarmManaged,
			annotation.NetworkTopologyPrefixListIDAnnotation,
			prefixListARN,
		)
		requestResourceName = cluster.Name

		clusterClient := k8sclient.NewAWSClusterClient(k8sClient)
		prefixListClient = new(resolverfakes.FakePrefixListClient)
		clientsFactory := &resolver.FakeClients{
			PrefixListClient: prefixListClient,
		}
		reconciler = controllers.NewPrefixListEntryReconciler(
			client.ObjectKeyFromObject(managementCluster),
			clusterClient,
			clientsFactory,
		)
	})

	JustBeforeEach(func() {
		request = ctrl.Request{
			NamespacedName: types.NamespacedName{
				Name:      requestResourceName,
				Namespace: namespace,
			},
		}
		result, reconcileErr = reconciler.Reconcile(ctx, request)
	})

	Describe("pre-reconciliation", func() {
		It("adds a finalizer to the cluster", func() {
			Expect(reconcileErr).NotTo(HaveOccurred())
			Expect(result.Requeue).To(BeFalse())

			actualCluster := getActualCluster()
			Expect(actualCluster.Finalizers).To(ContainElement(controllers.FinalizerPrefixListEntry))
		})

		It("adds the PrefixListEntriesReady condition to the cluster", func() {
			actualCluster := getActualCluster()
			Expect(actualCluster.Status.Conditions).To(ContainElements(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(conditions.PrefixListEntriesReady),
				"Status": Equal(corev1.ConditionTrue),
			})))
		})

		When("the cluster does not exist", func() {
			BeforeEach(func() {
				requestResourceName = "does-not-exist"
			})

			It("does not reconcile", func() {
				Expect(reconcileErr).NotTo(HaveOccurred())
				Expect(result.Requeue).To(BeFalse())
				Expect(prefixListClient.Invocations()).To(BeEmpty())
			})
		})

		When("the cluster is paused", func() {
			BeforeEach(func() {
				patchedCluster := cluster.DeepCopy()
				patchedCluster.Annotations[capi.PausedAnnotation] = "true"

				err := k8sClient.Patch(context.Background(), patchedCluster, client.MergeFrom(cluster))
				Expect(err).NotTo(HaveOccurred())
			})

			It("does not reconcile", func() {
				Expect(reconcileErr).NotTo(HaveOccurred())
				Expect(result.Requeue).To(BeFalse())
				Expect(prefixListClient.Invocations()).To(BeEmpty())
			})
		})

		When("the management cluster does not have the prefix list annotation set", func() {
			BeforeEach(func() {
				patchedCluster := managementCluster.DeepCopy()
				delete(patchedCluster.Annotations, annotation.NetworkTopologyPrefixListIDAnnotation)
				err := k8sClient.Patch(context.Background(), patchedCluster, client.MergeFrom(managementCluster))
				Expect(err).NotTo(HaveOccurred())
			})

			It("requeues the event", func() {
				Expect(reconcileErr).NotTo(HaveOccurred())
				Expect(result.RequeueAfter).To(Equal(time.Minute))
				Expect(prefixListClient.Invocations()).To(BeEmpty())
			})
		})

		When("the VPC isn't created yet", func() {
			BeforeEach(func() {
				patchedCluster := cluster.DeepCopy()
				patchedCluster.Spec.NetworkSpec.VPC.ID = ""
				err := k8sClient.Patch(context.Background(), patchedCluster, client.MergeFrom(cluster))
				Expect(err).NotTo(HaveOccurred())
			})

			It("does not reconcile", func() {
				Expect(reconcileErr).NotTo(HaveOccurred())
				Expect(result.Requeue).To(BeFalse())
				Expect(prefixListClient.Invocations()).To(BeEmpty())
			})
		})

		When("the cluster role identity doesn't exist", func() {
			BeforeEach(func() {
				Expect(k8sClient.Delete(ctx, identity)).To(Succeed())
			})

			It("returns an error", func() {
				Expect(k8serrors.IsNotFound(reconcileErr)).To(BeTrue())
				Expect(prefixListClient.Invocations()).To(BeEmpty())
			})
		})
	})

	Describe("GiantSwarmManaged mode", func() {
		It("adds the vpc cidr to the prefix list", func() {
			Expect(prefixListClient.ApplyEntryCallCount()).To(Equal(1))
			_, entry := prefixListClient.ApplyEntryArgsForCall(0)
			Expect(entry.PrefixListARN).To(Equal(prefixListARN))
			Expect(entry.CIDR).To(Equal(cluster.Spec.NetworkSpec.VPC.CidrBlock))
			Expect(entry.Description).To(Equal(fmt.Sprintf("CIDR block for cluster %s", cluster.Name)))
		})

		When("adding the entry fails", func() {
			BeforeEach(func() {
				prefixListClient.ApplyEntryReturns(errors.New("boom"))
			})

			It("returns an error", func() {
				Expect(reconcileErr).To(MatchError(ContainSubstring("boom")))
			})
		})

		When("the cluster has been deleted", func() {
			BeforeEach(func() {
				patchedCluster := cluster.DeepCopy()
				patchedCluster.Finalizers = []string{controllers.FinalizerPrefixListEntry}

				err := k8sClient.Patch(context.Background(), patchedCluster, client.MergeFrom(cluster))
				Expect(err).NotTo(HaveOccurred())

				err = k8sClient.Delete(context.Background(), cluster)
				Expect(err).NotTo(HaveOccurred())
			})

			It("deletes the prefix list entry", func() {
				Expect(prefixListClient.DeleteEntryCallCount()).To(Equal(1))
				_, entry := prefixListClient.DeleteEntryArgsForCall(0)
				Expect(entry.PrefixListARN).To(Equal(prefixListARN))
				Expect(entry.CIDR).To(Equal(cluster.Spec.NetworkSpec.VPC.CidrBlock))
				Expect(entry.Description).To(Equal(fmt.Sprintf("CIDR block for cluster %s", cluster.Name)))
			})

			It("removes the finalizer", func() {
				err := k8sClient.Get(context.Background(), types.NamespacedName{Name: cluster.Name, Namespace: namespace}, cluster)
				Expect(k8serrors.IsNotFound(err)).To(BeTrue())
			})

			When("deleting the entry fails", func() {
				BeforeEach(func() {
					prefixListClient.DeleteEntryReturns(errors.New("boom"))
				})

				It("returns an error", func() {
					Expect(reconcileErr).To(MatchError(ContainSubstring("boom")))
				})
			})
		})
	})
})
