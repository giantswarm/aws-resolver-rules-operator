package controllers_test

import (
	"context"
	"errors"
	"fmt"

	"github.com/giantswarm/k8smetadata/pkg/annotation"
	gsannotation "github.com/giantswarm/k8smetadata/pkg/annotation"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	capa "sigs.k8s.io/cluster-api-provider-aws/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/aws-resolver-rules-operator/controllers"
	"github.com/aws-resolver-rules-operator/controllers/controllersfakes"
	"github.com/aws-resolver-rules-operator/pkg/k8sclient"
	"github.com/aws-resolver-rules-operator/pkg/resolver/resolverfakes"
)

var _ = Describe("Share", func() {
	var (
		ctx context.Context

		sourceAccountID   = "123456789012"
		transitGatewayARN = fmt.Sprintf("arn:aws:ec2:eu-west-2:%s:transit-gateway/tgw-01234567890abcdef", sourceAccountID)
		prefixListARN     = fmt.Sprintf("arn:aws:ec2:eu-west-2:%s:prefix-list/pl-01234567890abcdef", sourceAccountID)
		externalAccountID = "987654321098"
		notValidArn       = "not:a:valid/arn"

		identity          *capa.AWSClusterRoleIdentity
		cluster           *capa.AWSCluster
		managementCluster *capa.AWSCluster
		request           ctrl.Request

		ramClient  *resolverfakes.FakeRAMClient
		reconciler *controllers.ShareReconciler
	)

	BeforeEach(func() {
		ctx = context.Background()

		identity, cluster = createRandomClusterWithIdentity(
			gsannotation.NetworkTopologyModeAnnotation,
			gsannotation.NetworkTopologyModeGiantSwarmManaged,
		)
		patchedIdentity := identity.DeepCopy()
		patchedIdentity.Spec.RoleArn = fmt.Sprintf("arn:aws:iam::%s:role/the-role-name", externalAccountID)
		err := k8sClient.Patch(context.Background(), patchedIdentity, client.MergeFrom(identity))
		Expect(err).NotTo(HaveOccurred())

		managementCluster = createRandomCluster(
			annotation.NetworkTopologyModeAnnotation,
			annotation.NetworkTopologyModeGiantSwarmManaged,
			annotation.NetworkTopologyTransitGatewayIDAnnotation,
			transitGatewayARN,
			annotation.NetworkTopologyPrefixListIDAnnotation,
			prefixListARN,
		)

		request = ctrl.Request{
			NamespacedName: types.NamespacedName{
				Name:      cluster.Name,
				Namespace: cluster.Namespace,
			},
		}

		ramClient = new(resolverfakes.FakeRAMClient)
		clusterClient := k8sclient.NewAWSClusterClient(k8sClient)
		reconciler = controllers.NewShareReconciler(
			client.ObjectKeyFromObject(managementCluster),
			clusterClient,
			ramClient,
		)
	})

	It("shares the transit gateway", func() {
		result, err := reconciler.Reconcile(ctx, request)

		Expect(result.Requeue).To(BeFalse())
		Expect(err).NotTo(HaveOccurred())

		Expect(ramClient.ApplyResourceShareCallCount()).To(Equal(2))
		_, resourceShare := ramClient.ApplyResourceShareArgsForCall(0)
		Expect(resourceShare.Name).To(Equal(fmt.Sprintf("%s-transit-gateway", cluster.Name)))
		Expect(resourceShare.ResourceArns).To(ConsistOf(transitGatewayARN))
		Expect(resourceShare.ExternalAccountID).To(Equal(externalAccountID))
		_, resourceShare = ramClient.ApplyResourceShareArgsForCall(1)
		Expect(resourceShare.Name).To(Equal(fmt.Sprintf("%s-prefix-list", cluster.Name)))
		Expect(resourceShare.ResourceArns).To(ConsistOf(prefixListARN))
		Expect(resourceShare.ExternalAccountID).To(Equal(externalAccountID))
	})

	It("adds a finalizer", func() {
		result, err := reconciler.Reconcile(ctx, request)
		Expect(result.Requeue).To(BeFalse())
		Expect(err).NotTo(HaveOccurred())

		actualCluster := &capa.AWSCluster{}
		err = k8sClient.Get(ctx, types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}, actualCluster)
		Expect(err).NotTo(HaveOccurred())

		Expect(actualCluster.Finalizers).To(ContainElement(controllers.FinalizerResourceShare))
	})

	When("the cluster has been deleted", func() {
		BeforeEach(func() {
			patchedCluster := cluster.DeepCopy()
			controllerutil.AddFinalizer(patchedCluster, controllers.FinalizerResourceShare)
			err := k8sClient.Patch(context.Background(), patchedCluster, client.MergeFrom(cluster))
			Expect(err).NotTo(HaveOccurred())
		})

		JustBeforeEach(func() {
			err := k8sClient.Delete(ctx, cluster)
			Expect(err).NotTo(HaveOccurred())
		})

		It("deletes the resource share", func() {
			result, err := reconciler.Reconcile(ctx, request)
			Expect(result.Requeue).To(BeFalse())
			Expect(err).NotTo(HaveOccurred())

			Expect(ramClient.DeleteResourceShareCallCount()).To(Equal(2))
			_, actualNameTransitGateway := ramClient.DeleteResourceShareArgsForCall(0)
			Expect(actualNameTransitGateway).To(Equal(fmt.Sprintf("%s-transit-gateway", cluster.Name)))
			_, actualNamePrefixList := ramClient.DeleteResourceShareArgsForCall(1)
			Expect(actualNamePrefixList).To(Equal(fmt.Sprintf("%s-prefix-list", cluster.Name)))
		})

		It("removes the finalizer", func() {
			result, err := reconciler.Reconcile(ctx, request)
			Expect(result.Requeue).To(BeFalse())
			Expect(err).NotTo(HaveOccurred())

			err = k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &capa.AWSCluster{})
			Expect(k8serrors.IsNotFound(err)).To(BeTrue())
		})

		When("deleting the resource share fails", func() {
			BeforeEach(func() {
				ramClient.DeleteResourceShareReturns(errors.New("boom"))
			})

			It("retuns an error", func() {
				_, err := reconciler.Reconcile(ctx, request)
				Expect(err).To(MatchError(ContainSubstring("boom")))

				actualCluster := &capa.AWSCluster{}
				err = k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), actualCluster)
				Expect(err).NotTo(HaveOccurred())

				Expect(actualCluster.Finalizers).To(ContainElement(controllers.FinalizerResourceShare))
			})
		})

		When("the transit gateway hasn't been detached", func() {
			BeforeEach(func() {
				patchedCluster := cluster.DeepCopy()
				controllerutil.AddFinalizer(patchedCluster, controllers.FinalizerTransitGatewayAttachment)
				err := k8sClient.Patch(context.Background(), patchedCluster, client.MergeFrom(cluster))
				Expect(err).NotTo(HaveOccurred())
			})

			It("does not reconcile", func() {
				result, err := reconciler.Reconcile(ctx, request)

				Expect(result.Requeue).To(BeFalse())
				Expect(err).NotTo(HaveOccurred())

				Expect(ramClient.DeleteResourceShareCallCount()).To(Equal(0))
			})
		})

		When("the prefix list entries haven't been removed", func() {
			BeforeEach(func() {
				patchedCluster := cluster.DeepCopy()
				controllerutil.AddFinalizer(patchedCluster, controllers.FinalizerPrefixListEntry)
				err := k8sClient.Patch(context.Background(), patchedCluster, client.MergeFrom(cluster))
				Expect(err).NotTo(HaveOccurred())
			})

			It("does not reconcile", func() {
				result, err := reconciler.Reconcile(ctx, request)

				Expect(result.Requeue).To(BeFalse())
				Expect(err).NotTo(HaveOccurred())

				Expect(ramClient.DeleteResourceShareCallCount()).To(Equal(0))
			})
		})
	})

	When("the transit gateway hasn't been created yet", func() {
		BeforeEach(func() {
			patchedCluster := managementCluster.DeepCopy()
			patchedCluster.Annotations[gsannotation.NetworkTopologyTransitGatewayIDAnnotation] = ""
			err := k8sClient.Patch(context.Background(), patchedCluster, client.MergeFrom(cluster))
			Expect(err).NotTo(HaveOccurred())
		})

		It("does not reconcile", func() {
			result, err := reconciler.Reconcile(ctx, request)

			Expect(result.Requeue).To(BeFalse())
			Expect(err).NotTo(HaveOccurred())

			Expect(ramClient.ApplyResourceShareCallCount()).To(Equal(1))
		})
	})

	When("the transit gateway hasn't been created yet", func() {
		BeforeEach(func() {
			patchedCluster := managementCluster.DeepCopy()
			patchedCluster.Annotations[gsannotation.NetworkTopologyTransitGatewayIDAnnotation] = ""
			err := k8sClient.Patch(context.Background(), patchedCluster, client.MergeFrom(cluster))
			Expect(err).NotTo(HaveOccurred())
		})

		It("still shares the prefix list", func() {
			result, err := reconciler.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Requeue).To(BeFalse())

			Expect(ramClient.ApplyResourceShareCallCount()).To(Equal(1))
			_, resourceShare := ramClient.ApplyResourceShareArgsForCall(0)
			Expect(resourceShare.ResourceArns).To(ConsistOf(prefixListARN))
		})
	})

	When("the transit gateway and prefix list are set on the workload cluster", func() {
		var (
			wcTransitGatewayARN string
			wcPrefixListARN     string
		)

		BeforeEach(func() {
			wcTransitGatewayARN = fmt.Sprintf("arn:aws:iam::123456789012:transit-gateways/%s", uuid.NewString())
			wcPrefixListARN = fmt.Sprintf("arn:aws:iam::123456789012:prefix-lists/%s", uuid.NewString())
			patchedCluster := cluster.DeepCopy()
			patchedCluster.Annotations[annotation.NetworkTopologyTransitGatewayIDAnnotation] = wcTransitGatewayARN
			patchedCluster.Annotations[annotation.NetworkTopologyPrefixListIDAnnotation] = wcPrefixListARN
			err := k8sClient.Patch(context.Background(), patchedCluster, client.MergeFrom(cluster))
			Expect(err).NotTo(HaveOccurred())
		})

		It("overrides the management cluster transit gateway", func() {
			result, err := reconciler.Reconcile(ctx, request)

			Expect(result.Requeue).To(BeFalse())
			Expect(err).NotTo(HaveOccurred())

			Expect(ramClient.ApplyResourceShareCallCount()).To(Equal(2))
			_, resourceShare := ramClient.ApplyResourceShareArgsForCall(0)
			Expect(resourceShare.Name).To(Equal(fmt.Sprintf("%s-transit-gateway", cluster.Name)))
			Expect(resourceShare.ResourceArns).To(ConsistOf(wcTransitGatewayARN))
			Expect(resourceShare.ExternalAccountID).To(Equal(externalAccountID))
			_, resourceShare = ramClient.ApplyResourceShareArgsForCall(1)
			Expect(resourceShare.Name).To(Equal(fmt.Sprintf("%s-prefix-list", cluster.Name)))
			Expect(resourceShare.ResourceArns).To(ConsistOf(wcPrefixListARN))
			Expect(resourceShare.ExternalAccountID).To(Equal(externalAccountID))
		})
	})

	When("applying the resource share fails", func() {
		BeforeEach(func() {
			ramClient.ApplyResourceShareReturns(errors.New("boom"))
		})

		It("returns an error", func() {
			_, err := reconciler.Reconcile(ctx, request)
			Expect(err).To(MatchError(ContainSubstring("boom")))
		})
	})

	When("the cluster no longer exists", func() {
		BeforeEach(func() {
			request.Name = "does-not-exist"
		})

		It("does not reconcile", func() {
			result, err := reconciler.Reconcile(ctx, request)

			Expect(result.Requeue).To(BeFalse())
			Expect(err).NotTo(HaveOccurred())

			Expect(ramClient.ApplyResourceShareCallCount()).To(Equal(0))
		})
	})

	When("getting the cluster returns an error", func() {
		BeforeEach(func() {
			fakeClusterClient := new(controllersfakes.FakeAWSClusterClient)
			fakeClusterClient.GetAWSClusterReturns(nil, errors.New("boom"))
			reconciler = controllers.NewShareReconciler(
				client.ObjectKeyFromObject(managementCluster),
				fakeClusterClient,
				ramClient,
			)
		})

		It("returns an error", func() {
			_, err := reconciler.Reconcile(ctx, request)
			Expect(err).To(MatchError(ContainSubstring("boom")))
		})
	})

	When("the AWSClusterRoleIdentity does not exist", func() {
		BeforeEach(func() {
			patchedCluster := cluster.DeepCopy()
			patchedCluster.Spec.IdentityRef = &capa.AWSIdentityReference{
				Kind: "AWSClusterRoleIdentity",
				Name: "does-not-exist",
			}
			err := k8sClient.Patch(context.Background(), patchedCluster, client.MergeFrom(cluster))
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns an error", func() {
			_, err := reconciler.Reconcile(ctx, request)
			Expect(err).To(HaveOccurred())
		})
	})

	When("the AWSClusterRoleIdentity ARN is invalid", func() {
		BeforeEach(func() {
			patchedIdentity := identity.DeepCopy()
			patchedIdentity.Spec.RoleArn = notValidArn
			err := k8sClient.Patch(context.Background(), patchedIdentity, client.MergeFrom(identity))
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns an error", func() {
			_, err := reconciler.Reconcile(ctx, request)
			Expect(err).To(HaveOccurred())
		})
	})
})
