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
	gserrors "github.com/aws-resolver-rules-operator/pkg/errors"
	"github.com/aws-resolver-rules-operator/pkg/k8sclient"
	"github.com/aws-resolver-rules-operator/pkg/resolver"
	"github.com/aws-resolver-rules-operator/pkg/resolver/resolverfakes"
)

var _ = Describe("TransitGatewayAttachment", func() {
	var (
		ctx context.Context

		clientsFactory       *resolver.FakeClients
		transitGatewayClient *resolverfakes.FakeTransitGatewayClient
		reconciler           *controllers.TransitGatewayAttachmentReconciler

		transitGatewayARN   string
		identity            *capa.AWSClusterRoleIdentity
		cluster             *capa.AWSCluster
		managementCluster   *capa.AWSCluster
		requestResourceName string
		reconcileResult     ctrl.Result
		reconcileErr        error
	)

	getActualCluster := func() *capa.AWSCluster {
		actualCluster := &capa.AWSCluster{}
		err := k8sClient.Get(ctx, k8stypes.NamespacedName{Name: requestResourceName, Namespace: namespace}, actualCluster)
		Expect(err).NotTo(HaveOccurred())

		return actualCluster
	}

	BeforeEach(func() {
		ctx = context.Background()

		transitGatewayARN = fmt.Sprintf("arn:aws:iam::123456789012:transit-gateways/%s", uuid.NewString())
		identity, cluster = createRandomClusterWithIdentity(
			annotation.NetworkTopologyModeAnnotation,
			annotation.NetworkTopologyModeGiantSwarmManaged,
		)

		managementCluster = createRandomCluster(
			annotation.NetworkTopologyModeAnnotation,
			annotation.NetworkTopologyModeGiantSwarmManaged,
			annotation.NetworkTopologyTransitGatewayIDAnnotation,
			transitGatewayARN,
		)
		requestResourceName = cluster.Name

		patchedCluster := cluster.DeepCopy()
		patchedCluster.Spec.AdditionalTags = capa.Tags{
			"additional-tag-1": "value1",
			"additional-tag-2": "value2",
		}
		patchedCluster.Spec.NetworkSpec.Subnets = []capa.SubnetSpec{
			newSubnetSpec("subnet-1", "av-zone-1", true),
			newSubnetSpec("subnet-2", "av-zone-2", true),
			newSubnetSpec("subnet-3", "av-zone-2", true),
			newSubnetSpec("subnet-4", "av-zone-4", false),
		}

		err := k8sClient.Patch(context.Background(), patchedCluster, client.MergeFrom(cluster))
		Expect(err).NotTo(HaveOccurred())

		clusterClient := k8sclient.NewAWSClusterClient(k8sClient)
		transitGatewayClient = new(resolverfakes.FakeTransitGatewayClient)
		clientsFactory = &resolver.FakeClients{
			TransitGatewayClient: transitGatewayClient,
		}
		reconciler = controllers.NewTransitGatewayAttachmentReconciler(
			client.ObjectKeyFromObject(managementCluster),
			clusterClient,
			clientsFactory,
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
			Expect(actualCluster.Finalizers).To(ContainElement(controllers.FinalizerTransitGatewayAttachment))
		})

		It("adds the TransitGatewayAttached condition to the cluster", func() {
			actualCluster := getActualCluster()
			Expect(actualCluster.Status.Conditions).To(ContainElements(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(conditions.TransitGatewayAttached),
				"Status": Equal(corev1.ConditionTrue),
			})))
		})

		When("the cluster does not exist", func() {
			BeforeEach(func() {
				requestResourceName = notExistResource
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
				patchedCluster.Annotations[capi.PausedAnnotation] = "true"

				err := k8sClient.Patch(context.Background(), patchedCluster, client.MergeFrom(cluster))
				Expect(err).NotTo(HaveOccurred())
			})

			It("does not reconcile", func() {
				Expect(reconcileErr).NotTo(HaveOccurred())
				Expect(reconcileResult.Requeue).To(BeFalse())
				Expect(transitGatewayClient.Invocations()).To(BeEmpty())
			})
		})

		When("the management cluster does not have the transit gateway annotation set", func() {
			BeforeEach(func() {
				patchedCluster := managementCluster.DeepCopy()
				delete(patchedCluster.Annotations, annotation.NetworkTopologyTransitGatewayIDAnnotation)
				err := k8sClient.Patch(context.Background(), patchedCluster, client.MergeFrom(managementCluster))
				Expect(err).NotTo(HaveOccurred())
			})

			It("requeues the event", func() {
				Expect(reconcileErr).NotTo(HaveOccurred())
				Expect(reconcileResult.RequeueAfter).To(Equal(time.Minute))
				Expect(transitGatewayClient.Invocations()).To(BeEmpty())
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
				Expect(reconcileResult.Requeue).To(BeFalse())
				Expect(transitGatewayClient.Invocations()).To(BeEmpty())
			})
		})

		When("the cluster role identity doesn't exist", func() {
			BeforeEach(func() {
				Expect(k8sClient.Delete(ctx, identity)).To(Succeed())
			})

			It("returns an error", func() {
				Expect(k8serrors.IsNotFound(reconcileErr)).To(BeTrue())
				Expect(transitGatewayClient.Invocations()).To(BeEmpty())
			})
		})
	})

	Describe("GiantSwarm Managed Mode", func() {
		It("attaches the transit gateway", func() {
			Expect(transitGatewayClient.ApplyAttachmentCallCount()).To(Equal(1))
			_, attachment := transitGatewayClient.ApplyAttachmentArgsForCall(0)
			Expect(attachment.TransitGatewayARN).To(Equal(transitGatewayARN))
			Expect(attachment.VPCID).To(Equal(cluster.Spec.NetworkSpec.VPC.ID))

			By("tagging the transit gateway attachment")
			Expect(attachment.Tags).To(HaveLen(4))
			Expect(attachment.Tags).To(HaveKeyWithValue("Name", cluster.Name))
			Expect(attachment.Tags).To(HaveKeyWithValue(fmt.Sprintf("kubernetes.io/cluster/%s", cluster.Name), "owned"))
			Expect(attachment.Tags).To(HaveKeyWithValue("additional-tag-1", "value1"))
			Expect(attachment.Tags).To(HaveKeyWithValue("additional-tag-2", "value2"))

			By("attaching only the tgw tagged subnets in each availability zone")
			Expect(attachment.SubnetIDs).To(ConsistOf("subnet-1", "subnet-2"))
		})

		When("the transit gateway is set on the workload cluster", func() {
			var wcTransitGatewayARN string

			BeforeEach(func() {
				wcTransitGatewayARN = fmt.Sprintf("arn:aws:iam::123456789012:transit-gateways/%s", uuid.NewString())
				patchedCluster := cluster.DeepCopy()
				patchedCluster.Annotations[annotation.NetworkTopologyTransitGatewayIDAnnotation] = wcTransitGatewayARN
				err := k8sClient.Patch(context.Background(), patchedCluster, client.MergeFrom(cluster))
				Expect(err).NotTo(HaveOccurred())
			})

			It("overrides the management cluster transit gateway", func() {
				Expect(transitGatewayClient.ApplyAttachmentCallCount()).To(Equal(1))
				_, attachment := transitGatewayClient.ApplyAttachmentArgsForCall(0)
				Expect(attachment.TransitGatewayARN).To(Equal(wcTransitGatewayARN))
			})
		})

		When("there are no subnets tagged with the tgw tag", func() {
			BeforeEach(func() {
				patchedCluster := cluster.DeepCopy()
				patchedCluster.Spec.NetworkSpec.Subnets = []capa.SubnetSpec{
					newSubnetSpec("subnet-1", "av-zone-1", false),
					newSubnetSpec("subnet-2", "av-zone-2", false),
					newSubnetSpec("subnet-3", "av-zone-2", false),
					newSubnetSpec("subnet-4", "av-zone-4", false),
				}

				err := k8sClient.Patch(context.Background(), patchedCluster, client.MergeFrom(cluster))
				Expect(err).NotTo(HaveOccurred())
			})

			It("defaults to all subnets, selecting one from each availability zone", func() {
				Expect(transitGatewayClient.ApplyAttachmentCallCount()).To(Equal(1))
				_, attachment := transitGatewayClient.ApplyAttachmentArgsForCall(0)

				By("attaching only the tgw tagged subnets in each availability zone")
				Expect(attachment.SubnetIDs).To(ConsistOf("subnet-1", "subnet-2", "subnet-4"))
			})
		})

		When("the cluster has no subnets", func() {
			BeforeEach(func() {
				patchedCluster := cluster.DeepCopy()
				patchedCluster.Spec.NetworkSpec.Subnets = []capa.SubnetSpec{}
				err := k8sClient.Patch(context.Background(), patchedCluster, client.MergeFrom(cluster))
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns an error", func() {
				Expect(reconcileErr).To(MatchError(ContainSubstring("cluster has no subnets")))
				Expect(transitGatewayClient.ApplyAttachmentCallCount()).To(Equal(0))
			})
		})

		When("not all subnets are created yet", func() {
			BeforeEach(func() {
				patchedCluster := cluster.DeepCopy()
				patchedCluster.Spec.NetworkSpec.Subnets = []capa.SubnetSpec{
					newSubnetSpec("subnet-1", "av-zone-1", false),
					newSubnetSpec("subnet-2", "av-zone-2", false),
					newSubnetSpec("subnet-3", "av-zone-2", false),
					newSubnetSpec("", "av-zone-3", false),
					newSubnetSpec("subnet-4", "av-zone-4", false),
				}

				err := k8sClient.Patch(context.Background(), patchedCluster, client.MergeFrom(cluster))
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns an error", func() {
				Expect(reconcileErr).To(MatchError(ContainSubstring("not all subnets have been created")))
				Expect(transitGatewayClient.ApplyAttachmentCallCount()).To(Equal(0))
			})
		})

		When("the subnet ID does not hold the aws subnet ID", func() {
			BeforeEach(func() {
				patchedCluster := cluster.DeepCopy()
				patchedCluster.Spec.NetworkSpec.Subnets = []capa.SubnetSpec{
					newSubnetSpec("subnet-1", "av-zone-1", false),
					newSubnetSpec("subnet-2", "av-zone-2", false),
					newSubnetSpec("subnet-3", "av-zone-2", false),
					newSubnetSpec("12346", "av-zone-3", false),
					newSubnetSpec("subnet-4", "av-zone-4", false),
				}

				err := k8sClient.Patch(context.Background(), patchedCluster, client.MergeFrom(cluster))
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns an error", func() {
				Expect(reconcileErr).To(MatchError(ContainSubstring("support for newer CAPA versions' ResourceID field not implemented yet")))
				Expect(transitGatewayClient.ApplyAttachmentCallCount()).To(Equal(0))
			})
		})

		When("attaching the transit gateway fails", func() {
			BeforeEach(func() {
				transitGatewayClient.ApplyAttachmentReturns(errors.New("boom"))
			})

			It("returns an error", func() {
				Expect(reconcileErr).To(MatchError(ContainSubstring("boom")))
			})

			When("it isn't ready", func() {
				BeforeEach(func() {
					transitGatewayClient.ApplyAttachmentReturns(gserrors.NewRetryableError("boom", time.Second))
				})

				It("requeues the event", func() {
					Expect(reconcileErr).NotTo(HaveOccurred())
					Expect(reconcileResult.RequeueAfter).To(Equal(time.Second))
				})
			})
		})

		When("the cluster has been deleted", func() {
			BeforeEach(func() {
				patchedCluster := cluster.DeepCopy()
				patchedCluster.Finalizers = []string{controllers.FinalizerTransitGatewayAttachment}

				err := k8sClient.Patch(context.Background(), patchedCluster, client.MergeFrom(cluster))
				Expect(err).NotTo(HaveOccurred())

				err = k8sClient.Delete(context.Background(), cluster)
				Expect(err).NotTo(HaveOccurred())
			})

			It("deletes the transit gateway", func() {
				Expect(transitGatewayClient.DetachCallCount()).To(Equal(1))
				_, attachment := transitGatewayClient.DetachArgsForCall(0)
				Expect(attachment.TransitGatewayARN).To(Equal(transitGatewayARN))
				Expect(attachment.VPCID).To(Equal(cluster.Spec.NetworkSpec.VPC.ID))
			})

			It("removes the finalizer", func() {
				err := k8sClient.Get(context.Background(), types.NamespacedName{Name: cluster.Name, Namespace: namespace}, cluster)
				Expect(k8serrors.IsNotFound(err)).To(BeTrue())
			})

			When("deleting the cluster fails", func() {
				BeforeEach(func() {
					transitGatewayClient.DetachReturns(errors.New("boom"))
				})

				It("returns an error", func() {
					Expect(reconcileErr).To(MatchError(ContainSubstring("boom")))
				})
			})
		})
	})
})
