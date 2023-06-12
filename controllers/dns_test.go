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
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/aws-resolver-rules-operator/controllers"
	"github.com/aws-resolver-rules-operator/controllers/controllersfakes"
	"github.com/aws-resolver-rules-operator/pkg/resolver"
	"github.com/aws-resolver-rules-operator/pkg/resolver/resolverfakes"
)

var _ = Describe("Dns Zone reconciler", func() {
	var (
		awsClusterClient        *controllersfakes.FakeAWSClusterClient
		ctx                     context.Context
		reconciler              *controllers.DnsReconciler
		awsCluster              *capa.AWSCluster
		awsClusterRoleIdentity  *capa.AWSClusterRoleIdentity
		result                  ctrl.Result
		reconcileErr            error
		resolverClient          *resolverfakes.FakeResolverClient
		dnsServerResolverClient *resolverfakes.FakeResolverClient
		ec2Client               *resolverfakes.FakeEC2Client
		ramClient               *resolverfakes.FakeRAMClient
		route53Client           *resolverfakes.FakeRoute53Client
	)

	const (
		ClusterName = "foo"
	)

	BeforeEach(func() {
		ctx = context.Background()
		awsClusterClient = new(controllersfakes.FakeAWSClusterClient)
		dnsServerResolverClient = new(resolverfakes.FakeResolverClient)
		ramClient = new(resolverfakes.FakeRAMClient)
		ec2Client = new(resolverfakes.FakeEC2Client)
		route53Client = new(resolverfakes.FakeRoute53Client)
		fakeAWSClients := &resolver.FakeClients{
			ResolverClient:         resolverClient,
			EC2Client:              ec2Client,
			RAMClient:              ramClient,
			ExternalResolverClient: dnsServerResolverClient,
			Route53Client:          route53Client,
		}

		dns, err := resolver.NewDnsZone(fakeAWSClients, "test.gigantic.io")
		Expect(err).NotTo(HaveOccurred())

		reconciler = controllers.NewDnsReconciler(awsClusterClient, dns)

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
				IdentityRef: &capa.AWSIdentityReference{
					Name: "default",
					Kind: capa.ClusterRoleIdentityKind,
				},
				Region: "gs-south-1",
				NetworkSpec: capa.NetworkSpec{
					VPC: capa.VPCSpec{
						ID: "vpc-12345678",
					},
				},
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
		expectedError := errors.New("failed fetching the AWSCluster")

		BeforeEach(func() {
			awsClusterClient.GetAWSClusterReturns(awsCluster, expectedError)
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

		When("we get an error trying to get the cluster identity", func() {
			expectedError := errors.New("failed fetching the AWSCluster")

			BeforeEach(func() {
				awsClusterClient.GetIdentityReturns(nil, expectedError)
			})

			It("doesn't really reconcile", func() {
				Expect(awsClusterClient.AddFinalizerCallCount()).To(BeZero())
				Expect(reconcileErr).To(HaveOccurred())
				Expect(reconcileErr).Should(MatchError(expectedError))
			})
		})

		When("the cluster has no identity set", func() {
			BeforeEach(func() {
				awsClusterClient.GetIdentityReturns(nil, nil)
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

			When("the cluster is being deleted", func() {
				BeforeEach(func() {
					deletionTime := metav1.Now()
					awsCluster.DeletionTimestamp = &deletionTime
					route53Client.GetHostedZoneIdByNameReturnsOnCall(0, "parent-hosted-zone-id", nil)
					route53Client.GetHostedZoneIdByNameReturnsOnCall(1, "hosted-zone-id", nil)
				})

				When("the hosted zone hasn't been deleted yet", func() {
					It("deletes the workload cluster hosted zone", func() {
						_, _, hostedZoneId := route53Client.DeleteHostedZoneArgsForCall(0)
						Expect(hostedZoneId).To(Equal("hosted-zone-id"))
					})

					It("deletes delegation of ns records from parent hosted zone", func() {
						Expect(route53Client.DeleteDelegationFromParentZoneCallCount()).To(Equal(1))
						_, _, parentHostedZoneId, hostedZoneId := route53Client.DeleteDelegationFromParentZoneArgsForCall(0)
						Expect(parentHostedZoneId).To(Equal("parent-hosted-zone-id"))
						Expect(hostedZoneId).To(Equal("hosted-zone-id"))
					})

					It("deletes the finalizer", func() {
						Expect(awsClusterClient.RemoveFinalizerCallCount()).To(Equal(1))
					})

					When("it fails to delete the hosted zone", func() {
						BeforeEach(func() {
							route53Client.DeleteHostedZoneReturns(errors.New("failed to remove hosted zone"))
						})

						It("does not delete the finalizer", func() {
							Expect(awsClusterClient.RemoveFinalizerCallCount()).To(Equal(0))
						})
					})
				})

				When("the hosted zone was already deleted", func() {
					BeforeEach(func() {
						route53Client.GetHostedZoneIdByNameReturnsOnCall(1, "", &resolver.HostedZoneNotFoundError{})
					})

					It("doesn't return an error", func() {
						Expect(route53Client.DeleteDelegationFromParentZoneCallCount()).To(Equal(0))
						Expect(reconcileErr).NotTo(HaveOccurred())
					})
				})
			})

			When("the cluster is not being deleted", func() {
				It("adds the finalizer to the AWSCluster", func() {
					Expect(awsClusterClient.AddFinalizerCallCount()).To(Equal(1))
					Expect(reconcileErr).NotTo(HaveOccurred())
				})

				When("the cluster uses public dns mode", func() {
					BeforeEach(func() {
						route53Client.GetHostedZoneIdByNameReturns("parent-hosted-zone-id", nil)
						route53Client.CreatePublicHostedZoneReturns("hosted-zone-id", nil)
					})

					It("creates hosted zone", func() {
						Expect(route53Client.CreatePublicHostedZoneCallCount()).To(Equal(1))
						_, _, hostedZoneName, tags := route53Client.CreatePublicHostedZoneArgsForCall(0)
						Expect(hostedZoneName).To(Equal(fmt.Sprintf("%s.%s", awsCluster.Name, "test.gigantic.io")))
						expectedTags := map[string]string{
							"Name": awsCluster.Name,
							fmt.Sprintf("sigs.k8s.io/cluster-api-provider-aws/cluster/%s", awsCluster.Name): "owned",
							"sigs.k8s.io/cluster-api-provider-aws/role":                                     "common",
						}
						Expect(tags).To(Equal(expectedTags))
						Expect(reconcileErr).NotTo(HaveOccurred())
					})

					It("adds delegation of ns records to parent hosted zone", func() {
						Expect(route53Client.AddDelegationToParentZoneCallCount()).To(Equal(1))
						_, _, parentHostedZoneId, hostedZoneId := route53Client.AddDelegationToParentZoneArgsForCall(0)
						Expect(parentHostedZoneId).To(Equal("parent-hosted-zone-id"))
						Expect(hostedZoneId).To(Equal("hosted-zone-id"))
					})

					When("creating hosted zone fails", func() {
						expectedError := errors.New("failed to find parent hosted zone")
						BeforeEach(func() {
							route53Client.CreatePublicHostedZoneReturns("", expectedError)
						})
						It("doesn't really reconcile", func() {
							Expect(route53Client.AddDelegationToParentZoneCallCount()).To(Equal(0))
							Expect(reconcileErr).To(HaveOccurred())
							Expect(reconcileErr).Should(MatchError(expectedError))
						})
					})

					When("creating hosted zone succeeds", func() {
						BeforeEach(func() {
							route53Client.CreatePublicHostedZoneReturns("hosted-zone-id", nil)
						})

						When("error trying to find parent hosted zone", func() {
							expectedError := errors.New("failed to find parent hosted zone")
							BeforeEach(func() {
								route53Client.GetHostedZoneIdByNameReturns("", expectedError)
							})

							It("doesn't really reconcile", func() {
								Expect(route53Client.AddDelegationToParentZoneCallCount()).To(Equal(0))
								Expect(reconcileErr).To(HaveOccurred())
								Expect(reconcileErr).Should(MatchError(expectedError))
							})
						})

						When("successfully finds parent hosted zone", func() {
							BeforeEach(func() {
								route53Client.GetHostedZoneIdByNameReturns("parent-hosted-zone-id", nil)
							})

							When("error adding delegation to parent hosted zone", func() {
								expectedError := errors.New("failed adding delegation to parent zone")
								BeforeEach(func() {
									route53Client.AddDelegationToParentZoneReturns(expectedError)
								})

								It("doesn't really reconcile", func() {
									Expect(reconcileErr).To(HaveOccurred())
									Expect(reconcileErr).Should(MatchError(expectedError))
								})
							})
						})
					})
				})

				When("the cluster uses private dns mode", func() {
					BeforeEach(func() {
						awsCluster.Annotations = map[string]string{
							gsannotations.AWSDNSMode:          "private",
							gsannotations.AWSDNSAdditionalVPC: "vpc-0011223344,vpc-0987654321",
						}
					})

					It("creates hosted zone", func() {
						Expect(route53Client.CreatePrivateHostedZoneCallCount()).To(Equal(1))
						_, _, hostedZoneName, vpcId, region, tags, vpcIdsToAttach := route53Client.CreatePrivateHostedZoneArgsForCall(0)
						Expect(hostedZoneName).To(Equal(fmt.Sprintf("%s.%s", awsCluster.Name, "test.gigantic.io")))
						Expect(vpcId).To(Equal(awsCluster.Spec.NetworkSpec.VPC.ID))
						Expect(region).To(Equal(awsCluster.Spec.Region))
						expectedTags := map[string]string{
							"Name": awsCluster.Name,
							fmt.Sprintf("sigs.k8s.io/cluster-api-provider-aws/cluster/%s", awsCluster.Name): "owned",
							"sigs.k8s.io/cluster-api-provider-aws/role":                                     "common",
						}
						Expect(tags).To(Equal(expectedTags))
						expectedVPCIdsToAttach := []string{"vpc-0011223344", "vpc-0987654321"}
						Expect(vpcIdsToAttach).To(Equal(expectedVPCIdsToAttach))
						Expect(reconcileErr).NotTo(HaveOccurred())
					})

					When("creating hosted zone fails", func() {
						expectedError := errors.New("failed to create hosted zone")
						BeforeEach(func() {
							route53Client.CreatePrivateHostedZoneReturns("", expectedError)
						})
						It("doesn't really reconcile", func() {
							Expect(reconcileErr).To(HaveOccurred())
							Expect(reconcileErr).Should(MatchError(expectedError))
						})
					})
				})
			})
		})
	})
})
