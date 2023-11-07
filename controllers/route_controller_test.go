package controllers_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	gsannotation "github.com/giantswarm/k8smetadata/pkg/annotation"
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

var _ = Describe("RouteReconciler", func() {
	var (
		ClusterName            = "foo"
		WorkloadClusterVPCId   = "myvpc-1a2b3c4d"
		WorkloadClusterVPCCidr = "10.0.0.0/16"

		ctx context.Context

		clusterClient *controllersfakes.FakeClusterClient
		routeClient   *controllersfakes.FakeRouteClient
		reconciler    *controllers.RouteReconciler

		reconcileErr error
		result       ctrl.Result

		awsCluster             *capa.AWSCluster
		cluster                *capi.Cluster
		awsClusterRoleIdentity *capa.AWSClusterRoleIdentity

		prefixlistID      = "pl-0d08b9d90543af924"
		transitGatewayID  = "tgw-019120b363d1e81e4"
		prefixListARN     = fmt.Sprintf("arn:aws:ec2:eu-north-1:123456789012:prefix-list/%s", prefixlistID)
		transitGatewayARN = fmt.Sprintf("arn:aws:ec2:eu-north-1:123456789012:transit-gateway/%s", transitGatewayID)
		subnets           []string
	)

	BeforeEach(func() {
		ctx = context.Background()
		clusterClient = new(controllersfakes.FakeClusterClient)
		routeClient = new(controllersfakes.FakeRouteClient)

		reconciler = controllers.NewRouteReconciler(clusterClient, routeClient)

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
			Spec: capi.ClusterSpec{
				InfrastructureRef: &v1.ObjectReference{
					Kind: "AWSCluster",
				},
			},
		}

		subnets = []string{awsCluster.Spec.NetworkSpec.Subnets[0].ID, awsCluster.Spec.NetworkSpec.Subnets[1].ID}
	})

	JustBeforeEach(func() {
		request := ctrl.Request{
			NamespacedName: types.NamespacedName{
				Name:      ClusterName,
				Namespace: "bar",
			},
		}
		result, reconcileErr = reconciler.Reconcile(ctx, request)
	})

	When("there is an error trying to get the AWS Cluster being reconciled", func() {
		BeforeEach(func() {
			clusterClient.GetAWSClusterReturns(nil, errors.New("failed fetching the cluster"))
		})

		It("returns the error", func() {
			Expect(clusterClient.GetIdentityCallCount()).To(Equal(0))
			Expect(reconcileErr).To(HaveOccurred())
		})
	})

	When("reconciling an existing AWS Cluster", func() {
		BeforeEach(func() {
			clusterClient.GetClusterReturns(cluster, nil)
			clusterClient.GetAWSClusterReturns(awsCluster, nil)
			clusterClient.GetIdentityReturns(awsClusterRoleIdentity, nil)
		})
		When("Annotation is neither UserManaged nor GiantswarmManaged", func() {
			When("Annotation is empty", func() {
				BeforeEach(func() {
					cluster.Annotations = map[string]string{}
				})

				It("returns without error", func() {
					Expect(clusterClient.GetIdentityCallCount()).To(Equal(0))
					Expect(reconcileErr).ToNot(HaveOccurred())
				})
			})
			When("Annotation is set to faulty value", func() {
				BeforeEach(func() {
					cluster.Annotations = map[string]string{
						gsannotation.NetworkTopologyModeAnnotation: "faultyValue",
					}
				})

				It("returns an error", func() {
					Expect(clusterClient.GetIdentityCallCount()).To(Equal(0))
					Expect(reconcileErr).To(HaveOccurred())
				})
			})
		})

		When("Annotation is set to UserManaged or GiantswarmManaged", func() {
			BeforeEach(func() {
				cluster.Annotations = map[string]string{
					gsannotation.NetworkTopologyModeAnnotation: gsannotation.NetworkTopologyModeUserManaged,
				}

			})
			When("there is an error trying to get the AWS Cluster being reconciled", func() {
				BeforeEach(func() {
					clusterClient.GetAWSClusterReturns(nil, errors.New("failed fetching the AWSCluster"))
				})

				It("returns the error", func() {
					Expect(clusterClient.GetAWSClusterCallCount()).To(Equal(1))
					Expect(reconcileErr).To(HaveOccurred())
				})
			})
			When("there is an error trying to get the AWSClusterRoleIdentity", func() {
				BeforeEach(func() {
					clusterClient.GetAWSClusterReturns(awsCluster, nil)
					clusterClient.GetIdentityReturns(nil, errors.New("failed fetching the AWSClusterRoleIdentity"))
				})

				It("returns the error", func() {
					Expect(clusterClient.GetAWSClusterCallCount()).To(Equal(1))
					Expect(clusterClient.GetIdentityCallCount()).To(Equal(1))
					Expect(reconcileErr).To(HaveOccurred())
				})
			})

			When("transit gateway annotation is empty", func() {
				BeforeEach(func() {
					cluster.Annotations[gsannotation.NetworkTopologyTransitGatewayIDAnnotation] = ""
				})

				It("requeues the request", func() {
					Expect(clusterClient.GetAWSClusterCallCount()).To(Equal(1))
					Expect(clusterClient.GetIdentityCallCount()).To(Equal(1))
					Expect(reconcileErr).NotTo(HaveOccurred())
					Expect(result).To(Equal(ctrl.Result{RequeueAfter: 1 * time.Minute}))
				})

			})

			When("prefixlist annotation is empty", func() {
				BeforeEach(func() {
					cluster.Annotations[gsannotation.NetworkTopologyPrefixListIDAnnotation] = ""
				})

				It("requeues the request", func() {
					Expect(clusterClient.GetAWSClusterCallCount()).To(Equal(1))
					Expect(clusterClient.GetIdentityCallCount()).To(Equal(1))
					Expect(reconcileErr).NotTo(HaveOccurred())
					Expect(result).To(Equal(ctrl.Result{RequeueAfter: 1 * time.Minute}))
				})
			})
			When("annotations are set", func() {
				BeforeEach(func() {
					cluster.Annotations[gsannotation.NetworkTopologyPrefixListIDAnnotation] = prefixListARN
					cluster.Annotations[gsannotation.NetworkTopologyTransitGatewayIDAnnotation] = transitGatewayARN
				})
				When("the cluster is not being deleted", func() {

					It("adds the finalizer to the AWS Cluster", func() {
						Expect(clusterClient.AddAWSClusterFinalizerCallCount()).To(Equal(1))
						_, awsClusterArg, finalizer := clusterClient.AddAWSClusterFinalizerArgsForCall(0)
						Expect(awsClusterArg.Name).To(Equal(ClusterName))
						Expect(finalizer).To(Equal(controllers.RouteFinalizer))
					})

					It("adds the routes", func() {
						Expect(routeClient.AddRoutesCallCount()).To(Equal(1))
						_, transitGatewayIDArg, prefixListIDArg, subnetsArg, roleARN, region, _ := routeClient.AddRoutesArgsForCall(0)
						Expect(*transitGatewayIDArg).To(Equal(transitGatewayID))
						Expect(*prefixListIDArg).To(Equal(prefixlistID))
						Expect(subnetsArg).To(Equal(subnets))
						Expect(roleARN).To(Equal(awsClusterRoleIdentity.Spec.RoleArn))
						Expect(region).To(Equal(awsCluster.Spec.Region))
					})
				})

				When("the cluster is being deleted", func() {
					BeforeEach(func() {
						cluster.DeletionTimestamp = &metav1.Time{Time: time.Now()}
					})
					It("deletes the routes", func() {
						Expect(routeClient.RemoveRoutesCallCount()).To(Equal(1))
						_, transitGatewayIDArg, prefixListIDArg, subnetsArg, roleARN, region, _ := routeClient.RemoveRoutesArgsForCall(0)
						Expect(*transitGatewayIDArg).To(Equal(transitGatewayID))
						Expect(*prefixListIDArg).To(Equal(prefixlistID))
						Expect(subnetsArg).To(Equal(subnets))
						Expect(roleARN).To(Equal(awsClusterRoleIdentity.Spec.RoleArn))
						Expect(region).To(Equal(awsCluster.Spec.Region))
					})

					It("removes the finalizer from the AWS Cluster", func() {
						Expect(clusterClient.RemoveAWSClusterFinalizerCallCount()).To(Equal(1))
						_, awsClusterArg, finalizer := clusterClient.RemoveAWSClusterFinalizerArgsForCall(0)
						Expect(awsClusterArg.Name).To(Equal(ClusterName))
						Expect(finalizer).To(Equal(controllers.RouteFinalizer))
					})
				})
			})
		})
	})
})
