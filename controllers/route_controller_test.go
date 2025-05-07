package controllers_test

import (
	"context"
	"fmt"
	"time"

	"github.com/giantswarm/k8smetadata/pkg/annotation"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	k8stypes "k8s.io/apimachinery/pkg/types"
	capa "sigs.k8s.io/cluster-api-provider-aws/v2/api/v1beta2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws-resolver-rules-operator/controllers"
	"github.com/aws-resolver-rules-operator/pkg/k8sclient"
	"github.com/aws-resolver-rules-operator/pkg/resolver"
	"github.com/aws-resolver-rules-operator/pkg/resolver/resolverfakes"
)

var _ = Describe("RouteReconciler", func() {
	var (
		requestResourceName string

		ctx context.Context

		clientsFactory   *resolver.FakeClients
		routeTableClient *resolverfakes.FakeRouteTableClient
		reconciler       *controllers.RouteReconciler

		reconcileErr error
		result       ctrl.Result

		awsCluster             *capa.AWSCluster
		awsClusterRoleIdentity *capa.AWSClusterRoleIdentity

		prefixlistID      = "pl-0d08b9d90543af924"
		transitGatewayID  = "tgw-019120b363d1e81e4"
		prefixListARN     = fmt.Sprintf("arn:aws:ec2:eu-north-1:123456789012:prefix-list/%s", prefixlistID)
		transitGatewayARN = fmt.Sprintf("arn:aws:ec2:eu-north-1:123456789012:transit-gateway/%s", transitGatewayID)
		//subnets           []string
	)

	getActualCluster := func() *capa.AWSCluster {
		actualCluster := &capa.AWSCluster{}
		err := k8sClient.Get(ctx, k8stypes.NamespacedName{Name: requestResourceName, Namespace: namespace}, actualCluster)
		Expect(err).NotTo(HaveOccurred())

		return actualCluster
	}

	BeforeEach(func() {
		ctx = context.Background()
		clusterClient := k8sclient.NewClusterClient(k8sClient)
		routeTableClient = new(resolverfakes.FakeRouteTableClient)
		clientsFactory = &resolver.FakeClients{
			RouteTableClient: routeTableClient,
		}

		managementCluster := createRandomCluster(
			annotation.NetworkTopologyModeAnnotation,
			annotation.NetworkTopologyModeGiantSwarmManaged,
		)

		reconciler = controllers.NewRouteReconciler(k8stypes.NamespacedName{Namespace: managementCluster.Namespace, Name: managementCluster.Name}, clusterClient, clientsFactory)

		awsClusterRoleIdentity, awsCluster = createRandomClusterWithIdentity(
			annotation.NetworkTopologyModeAnnotation,
			annotation.NetworkTopologyModeGiantSwarmManaged,
			annotation.NetworkTopologyTransitGatewayIDAnnotation,
			transitGatewayARN,
			annotation.NetworkTopologyPrefixListIDAnnotation,
			prefixListARN,
		)
		requestResourceName = awsCluster.Name
		//subnets = []string{awsCluster.Spec.NetworkSpec.Subnets[0].ID}
	})

	JustBeforeEach(func() {
		request := ctrl.Request{
			NamespacedName: k8stypes.NamespacedName{
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
			Expect(actualCluster.Finalizers).To(ContainElement(controllers.RouteFinalizer))
		})

		When("the cluster does not exist", func() {
			BeforeEach(func() {
				requestResourceName = notExistResource
			})

			It("does not reconcile", func() {
				Expect(reconcileErr).NotTo(HaveOccurred())
				Expect(result.Requeue).To(BeFalse())
				Expect(routeTableClient.Invocations()).To(BeEmpty())
			})
		})

		When("the cluster does not have the transit gateway annotation set", func() {
			BeforeEach(func() {
				patchedCluster := awsCluster.DeepCopy()
				delete(patchedCluster.Annotations, annotation.NetworkTopologyTransitGatewayIDAnnotation)
				err := k8sClient.Patch(context.Background(), patchedCluster, client.MergeFrom(awsCluster))
				Expect(err).NotTo(HaveOccurred())
			})

			It("requeues the event", func() {
				Expect(reconcileErr).NotTo(HaveOccurred())
				Expect(result.RequeueAfter).To(Equal(time.Minute))
				Expect(routeTableClient.Invocations()).To(BeEmpty())
			})
		})

		When("the cluster role identity doesn't exist", func() {
			BeforeEach(func() {
				Expect(k8sClient.Delete(ctx, awsClusterRoleIdentity)).To(Succeed())
			})

			It("returns an error", func() {
				Expect(k8serrors.IsNotFound(reconcileErr)).To(BeTrue())
				Expect(routeTableClient.Invocations()).To(BeEmpty())
			})
		})
	})

	//There is no difference between GiantswarmManaged and UserManaged mode
	Describe("GiantswarmManaged Mode", func() {
		When("There is an error while adding routes", func() {
			BeforeEach(func() {
				routeTableClient.AddRoutesReturns(fmt.Errorf("error"))
			})
			It("returns the error", func() {
				Expect(reconcileErr).To(HaveOccurred())
			})
		})

		When("Routes are added successfully", func() {
			BeforeEach(func() {
				routeTableClient.AddRoutesReturns(nil)
			})
			It("does not return an error", func() {
				Expect(reconcileErr).NotTo(HaveOccurred())
			})
		})

		When("Cluster was deleted", func() {
			BeforeEach(func() {
				patchedCluster := awsCluster.DeepCopy()
				patchedCluster.Finalizers = []string{controllers.RouteFinalizer}

				err := k8sClient.Patch(context.Background(), patchedCluster, client.MergeFrom(awsCluster))
				Expect(err).NotTo(HaveOccurred())

				err = k8sClient.Delete(context.Background(), awsCluster)
				Expect(err).NotTo(HaveOccurred())
			})

			When("There is an error while removing routes", func() {
				BeforeEach(func() {
					routeTableClient.RemoveRoutesReturns(fmt.Errorf("error"))
				})
				It("returns the error", func() {
					Expect(reconcileErr).To(HaveOccurred())
				})
			})

			When("Routes are deleted successfully", func() {
				BeforeEach(func() {
					routeTableClient.RemoveRoutesReturns(nil)
				})
				It("It removes the finalizer", func() {
					err := k8sClient.Get(context.Background(), k8stypes.NamespacedName{Name: awsCluster.Name, Namespace: namespace}, awsCluster)
					Expect(k8serrors.IsNotFound(err)).To(BeTrue())
				})
			})
		})
	})

})
