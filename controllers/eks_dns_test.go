package controllers_test

import (
	"context"
	"errors"
	"fmt"

	gsannotations "github.com/giantswarm/k8smetadata/pkg/annotation"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gstruct"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	capa "sigs.k8s.io/cluster-api-provider-aws/v2/api/v1beta2"
	eks "sigs.k8s.io/cluster-api-provider-aws/v2/controlplane/eks/api/v1beta2"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/aws-resolver-rules-operator/controllers"
	"github.com/aws-resolver-rules-operator/controllers/controllersfakes"
	"github.com/aws-resolver-rules-operator/pkg/resolver"
	"github.com/aws-resolver-rules-operator/pkg/resolver/resolverfakes"
)

var _ = Describe("Dns Zone reconciler", func() {
	var (
		clusterClient               *controllersfakes.FakeClusterClient
		ctx                         context.Context
		reconciler                  *controllers.EKSDnsReconciler
		managementClusterAWSCluster *capa.AWSCluster
		awsManagedControlPlane      *eks.AWSManagedControlPlane
		awsClusterRoleIdentity      *capa.AWSClusterRoleIdentity
		eksCluster                  *capi.Cluster
		result                      ctrl.Result
		reconcileErr                error
		resolverClient              *resolverfakes.FakeResolverClient
		dnsServerResolverClient     *resolverfakes.FakeResolverClient
		ec2Client                   *resolverfakes.FakeEC2Client
		ramClient                   *resolverfakes.FakeRAMClient
		route53Client               *resolverfakes.FakeRoute53Client
	)

	const (
		ManagementClusterName     = "management"
		ClusterName               = "foo"
		ClusterNamespace          = "bar"
		WorkloadClusterBaseDomain = "test.gigantic.io"
	)

	BeforeEach(func() {
		ctx = context.Background()
		clusterClient = new(controllersfakes.FakeClusterClient)
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

		dns, err := resolver.NewDnsZone(fakeAWSClients, WorkloadClusterBaseDomain)
		Expect(err).NotTo(HaveOccurred())

		reconciler = controllers.NewEKSDnsReconciler(clusterClient, dns, ClusterName, ClusterNamespace)

		awsClusterRoleIdentity = &capa.AWSClusterRoleIdentity{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ClusterNamespace,
				Name:      "default",
			},
			Spec: capa.AWSClusterRoleIdentitySpec{},
		}
		managementClusterAWSCluster = &capa.AWSCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ManagementClusterName,
				Namespace: ClusterNamespace,
			},
			Spec: capa.AWSClusterSpec{
				IdentityRef: &capa.AWSIdentityReference{
					Name: "default",
					Kind: capa.ClusterRoleIdentityKind,
				},
				Region: "eu-central-1",
				NetworkSpec: capa.NetworkSpec{
					VPC: capa.VPCSpec{
						ID: "vpc-0101010101",
					},
				},
			},
		}

		awsManagedControlPlane = &eks.AWSManagedControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ClusterName,
				Namespace: ClusterNamespace,
				Finalizers: []string{
					controllers.DnsFinalizer,
				},
			},
			Spec: eks.AWSManagedControlPlaneSpec{
				IdentityRef: &capa.AWSIdentityReference{
					Name: "default",
					Kind: capa.ClusterRoleIdentityKind,
				},
				Region: "eu-central-1",
				NetworkSpec: capa.NetworkSpec{
					VPC: capa.VPCSpec{
						ID: "vpc-12345678",
					},
				},
			},
		}
		eksCluster = &capi.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ClusterName,
				Namespace: ClusterNamespace,
			},
			Spec: capi.ClusterSpec{
				InfrastructureRef: &v1.ObjectReference{
					Kind: "AWSManagedCluster",
				},
			},
		}
	})

	JustBeforeEach(func() {
		request := ctrl.Request{
			NamespacedName: types.NamespacedName{
				Name:      ClusterName,
				Namespace: ClusterNamespace,
			},
		}
		result, reconcileErr = reconciler.Reconcile(ctx, request)
	})

	When("there is an error trying to get the management cluster AWSCluster", func() {
		expectedError := errors.New("failed fetching the AWSCluster")

		BeforeEach(func() {
			clusterClient.GetAWSClusterReturns(nil, expectedError)
		})

		It("returns the error", func() {
			Expect(clusterClient.AddAWSManagedControlPlaneFinalizerCallCount()).To(BeZero())
			Expect(reconcileErr).To(HaveOccurred())
			Expect(reconcileErr).Should(MatchError(expectedError))
		})
	})

	When("there is an error trying to get the management cluster identity", func() {
		expectedError := errors.New("failed fetching the AWSClusterRoleIdentity")

		BeforeEach(func() {
			clusterClient.GetAWSClusterReturns(managementClusterAWSCluster, nil)
			clusterClient.GetIdentityReturns(nil, expectedError)
		})

		It("returns the error", func() {
			Expect(clusterClient.AddAWSManagedControlPlaneFinalizerCallCount()).To(BeZero())
			Expect(reconcileErr).To(HaveOccurred())
			Expect(reconcileErr).Should(MatchError(expectedError))
		})
	})

	When("there is an error trying to get the AWSManagedControlPlane being reconciled", func() {
		expectedError := errors.New("failed fetching the AWSManagedControlPlane")

		BeforeEach(func() {
			clusterClient.GetAWSClusterReturns(managementClusterAWSCluster, nil)
			clusterClient.GetIdentityReturns(awsClusterRoleIdentity, nil)
			clusterClient.GetAWSManagedControlPlaneReturns(nil, expectedError)
		})

		It("returns the error", func() {
			Expect(clusterClient.AddAWSManagedControlPlaneFinalizerCallCount()).To(BeZero())
			Expect(reconcileErr).To(HaveOccurred())
			Expect(reconcileErr).Should(MatchError(expectedError))
		})
	})

	When("reconciling an existing cluster", func() {
		BeforeEach(func() {
			clusterClient.GetAWSClusterReturns(managementClusterAWSCluster, nil)
			clusterClient.GetIdentityReturns(awsClusterRoleIdentity, nil)
			clusterClient.GetAWSManagedControlPlaneReturns(awsManagedControlPlane, nil)
		})

		When("the aws cluster already has an owner", func() {
			BeforeEach(func() {
				clusterClient.GetClusterReturns(eksCluster, nil)
			})

			When("the cluster is paused", func() {
				BeforeEach(func() {
					eksCluster.Spec.Paused = true
					clusterClient.GetClusterReturns(eksCluster, nil)
				})

				It("does not reconcile", func() {
					Expect(clusterClient.AddAWSManagedControlPlaneFinalizerCallCount()).To(BeZero())
					Expect(reconcileErr).NotTo(HaveOccurred())
				})
			})

			When("the infrastructure cluster is paused", func() {
				BeforeEach(func() {
					awsManagedControlPlane.Annotations = map[string]string{
						capi.PausedAnnotation: "true",
					}
					clusterClient.GetAWSManagedControlPlaneReturns(awsManagedControlPlane, nil)
				})

				It("does not reconcile", func() {
					Expect(clusterClient.AddAWSManagedControlPlaneFinalizerCallCount()).To(BeZero())
					Expect(reconcileErr).NotTo(HaveOccurred())
				})
			})

			When("we get an error trying to get the cluster identity", func() {
				expectedError := errors.New("failed fetching the AWSCluster")

				BeforeEach(func() {
					// For the management cluster
					clusterClient.GetIdentityReturnsOnCall(0, awsClusterRoleIdentity, nil)
					// For the reconciled cluster
					clusterClient.GetIdentityReturnsOnCall(1, nil, expectedError)
				})

				It("doesn't really reconcile", func() {
					Expect(clusterClient.AddAWSManagedControlPlaneFinalizerCallCount()).To(BeZero())
					Expect(reconcileErr).To(HaveOccurred())
					Expect(reconcileErr).Should(MatchError(expectedError))
				})
			})

			When("the cluster has no identity set", func() {
				BeforeEach(func() {
					// For the management cluster
					clusterClient.GetIdentityReturnsOnCall(0, awsClusterRoleIdentity, nil)
					// For the reconciled cluster
					clusterClient.GetIdentityReturnsOnCall(1, nil, nil)
				})

				It("doesn't really reconcile", func() {
					Expect(clusterClient.AddAWSManagedControlPlaneFinalizerCallCount()).To(BeZero())
					Expect(result.Requeue).To(BeFalse())
					Expect(result.RequeueAfter).To(BeZero())
					Expect(reconcileErr).NotTo(HaveOccurred())
				})
			})

			When("the cluster has an identity set", func() {
				BeforeEach(func() {
					clusterClient.GetIdentityReturns(awsClusterRoleIdentity, nil)
				})

				When("the cluster is being deleted", func() {
					BeforeEach(func() {
						deletionTime := metav1.Now()
						eksCluster.DeletionTimestamp = &deletionTime
						route53Client.GetHostedZoneIdByNameReturnsOnCall(0, "hosted-zone-id", nil)
						route53Client.GetHostedZoneIdByNameReturnsOnCall(1, "parent-hosted-zone-id", nil)
					})

					When("the cluster uses public dns mode", func() {
						It("deletes the workload cluster dns records", func() {
							_, _, hostedZoneId := route53Client.DeleteDnsRecordsFromHostedZoneArgsForCall(0)
							Expect(hostedZoneId).To(Equal("hosted-zone-id"))
						})

						It("deletes delegation of ns records from parent hosted zone", func() {
							Expect(route53Client.DeleteDelegationFromParentZoneCallCount()).To(Equal(1))
							_, _, parentHostedZoneId, nsRecord := route53Client.DeleteDelegationFromParentZoneArgsForCall(0)
							Expect(parentHostedZoneId).To(Equal("parent-hosted-zone-id"))
							Expect(nsRecord).To(BeNil())
						})

						It("deletes the workload cluster hosted zone", func() {
							_, _, hostedZoneId := route53Client.DeleteHostedZoneArgsForCall(0)
							Expect(hostedZoneId).To(Equal("hosted-zone-id"))
						})

						It("deletes the finalizer", func() {
							Expect(clusterClient.RemoveAWSManagedControlPlaneFinalizerCallCount()).To(Equal(1))
						})

						When("it fails to delete the hosted zone", func() {
							BeforeEach(func() {
								route53Client.DeleteHostedZoneReturns(errors.New("failed to remove hosted zone"))
							})

							It("does not delete the finalizer", func() {
								Expect(clusterClient.RemoveAWSManagedControlPlaneFinalizerCallCount()).To(Equal(0))
							})
						})

						When("the hosted zone was already deleted", func() {
							BeforeEach(func() {
								route53Client.GetHostedZoneIdByNameReturnsOnCall(0, "", &resolver.HostedZoneNotFoundError{})
							})

							It("doesn't return an error", func() {
								Expect(route53Client.DeleteDnsRecordsFromHostedZoneCallCount()).To(Equal(0))
								Expect(route53Client.DeleteDelegationFromParentZoneCallCount()).To(Equal(0))
								Expect(route53Client.DeleteHostedZoneCallCount()).To(Equal(0))
								Expect(reconcileErr).NotTo(HaveOccurred())
							})
						})
					})

					When("the cluster uses private dns mode", func() {
						It("deletes the workload cluster dns records", func() {
							_, _, hostedZoneId := route53Client.DeleteDnsRecordsFromHostedZoneArgsForCall(0)
							Expect(hostedZoneId).To(Equal("hosted-zone-id"))
						})

						It("deletes the workload cluster hosted zone", func() {
							_, _, hostedZoneId := route53Client.DeleteHostedZoneArgsForCall(0)
							Expect(hostedZoneId).To(Equal("hosted-zone-id"))
						})

						It("deletes the finalizer", func() {
							Expect(clusterClient.RemoveAWSManagedControlPlaneFinalizerCallCount()).To(Equal(1))
						})

						When("it fails to delete the hosted zone", func() {
							BeforeEach(func() {
								route53Client.DeleteHostedZoneReturns(errors.New("failed to remove hosted zone"))
							})

							It("does not delete the finalizer", func() {
								Expect(clusterClient.RemoveAWSManagedControlPlaneFinalizerCallCount()).To(Equal(0))
							})
						})

						When("the hosted zone was already deleted", func() {
							BeforeEach(func() {
								route53Client.GetHostedZoneIdByNameReturnsOnCall(0, "", &resolver.HostedZoneNotFoundError{})
							})

							It("doesn't return an error", func() {
								Expect(route53Client.DeleteDnsRecordsFromHostedZoneCallCount()).To(Equal(0))
								Expect(route53Client.DeleteHostedZoneCallCount()).To(Equal(0))
								Expect(reconcileErr).NotTo(HaveOccurred())
							})
						})
					})
				})

				When("the cluster is not being deleted", func() {
					It("adds the finalizer to the Cluster", func() {
						Expect(clusterClient.AddAWSManagedControlPlaneFinalizerCallCount()).To(Equal(1))
						Expect(reconcileErr).NotTo(HaveOccurred())
					})

					When("the cluster uses public dns mode", func() {
						BeforeEach(func() {
							route53Client.GetHostedZoneIdByNameReturns("parent-hosted-zone-id", nil)
							route53Client.CreateHostedZoneReturns("hosted-zone-id", nil)
						})

						It("creates hosted zone", func() {
							Expect(route53Client.CreateHostedZoneCallCount()).To(Equal(1))
							_, _, dnsZone := route53Client.CreateHostedZoneArgsForCall(0)
							Expect(dnsZone.DnsName).To(Equal(fmt.Sprintf("%s.%s", awsManagedControlPlane.Name, "test.gigantic.io")))
							expectedTags := map[string]string{
								"Name": awsManagedControlPlane.Name,
								fmt.Sprintf("sigs.k8s.io/cluster-api-provider-aws/cluster/%s", awsManagedControlPlane.Name): "owned",
								"sigs.k8s.io/cluster-api-provider-aws/role":                                                 "common",
							}
							Expect(dnsZone.Tags).To(Equal(expectedTags))
							Expect(reconcileErr).NotTo(HaveOccurred())
						})

						It("adds delegation of ns records to parent hosted zone", func() {
							Expect(route53Client.AddDelegationToParentZoneCallCount()).To(Equal(1))
							_, _, parentHostedZoneId, nsRecord := route53Client.AddDelegationToParentZoneArgsForCall(0)
							Expect(parentHostedZoneId).To(Equal("parent-hosted-zone-id"))
							Expect(nsRecord).To(BeNil())
						})

						It("creates DNS records for workload cluster", func() {
							Expect(route53Client.AddDnsRecordsToHostedZoneCallCount()).To(Equal(1))
							_, _, hostedZoneId, dnsRecords := route53Client.AddDnsRecordsToHostedZoneArgsForCall(0)
							Expect(hostedZoneId).To(Equal("hosted-zone-id"))
							Expect(dnsRecords).To(ContainElements(resolver.DNSRecord{
								Kind:   resolver.DnsRecordTypeCname,
								Name:   fmt.Sprintf("*.%s.%s", ClusterName, WorkloadClusterBaseDomain),
								Values: []string{fmt.Sprintf("ingress.%s.%s", ClusterName, WorkloadClusterBaseDomain)},
							}))
						})

						When("the k8s API endpoint of the EKS workload cluster is set", func() {
							BeforeEach(func() {
								awsManagedControlPlane.Spec.ControlPlaneEndpoint.Host = "control-plane-eks-load-balancer-hostname"
								clusterClient.GetAWSManagedControlPlaneReturns(awsManagedControlPlane, nil)
							})

							It("creates DNS records for workload cluster", func() {
								_, _, _, dnsRecords := route53Client.AddDnsRecordsToHostedZoneArgsForCall(0)
								Expect(dnsRecords).To(ContainElements(resolver.DNSRecord{
									Kind:   resolver.DnsRecordTypeCname,
									Name:   fmt.Sprintf("api.%s.%s", ClusterName, WorkloadClusterBaseDomain),
									Values: []string{"control-plane-eks-load-balancer-hostname"},
									Region: awsManagedControlPlane.Spec.Region,
								}))
							})
						})

						When("the k8s API endpoint of the CAPA workload cluster is set", func() {
							BeforeEach(func() {
								awsManagedControlPlane.Spec.ControlPlaneEndpoint.Host = "control-plane-load-balancer-hostname"
								clusterClient.GetAWSManagedControlPlaneReturns(awsManagedControlPlane, nil)
							})

							It("creates DNS records for workload cluster", func() {
								_, _, _, dnsRecords := route53Client.AddDnsRecordsToHostedZoneArgsForCall(0)
								Expect(dnsRecords).To(ContainElements(resolver.DNSRecord{
									Kind:   resolver.DnsRecordTypeCname,
									Name:   fmt.Sprintf("api.%s.%s", ClusterName, WorkloadClusterBaseDomain),
									Values: []string{"control-plane-load-balancer-hostname"},
									Region: awsManagedControlPlane.Spec.Region,
								}))
							})
						})

						When("the k8s API endpoint of the workload cluster is not set", func() {
							It("it does not create DNS record for the API yet", func() {
								_, _, _, dnsRecords := route53Client.AddDnsRecordsToHostedZoneArgsForCall(0)
								Expect(dnsRecords).ToNot(ContainElement(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
									"Kind": Equal("ALIAS"),
									"Name": Equal(fmt.Sprintf("api.%s.%s", ClusterName, WorkloadClusterBaseDomain)),
								})))
							})
						})

						When("creating hosted zone fails", func() {
							expectedError := errors.New("failed to find parent hosted zone")
							BeforeEach(func() {
								route53Client.CreateHostedZoneReturns("", expectedError)
							})
							It("doesn't really reconcile", func() {
								Expect(route53Client.AddDelegationToParentZoneCallCount()).To(Equal(0))
								Expect(reconcileErr).To(HaveOccurred())
								Expect(reconcileErr).Should(MatchError(expectedError))
							})
						})

						When("creating hosted zone succeeds", func() {
							BeforeEach(func() {
								route53Client.CreateHostedZoneReturns("hosted-zone-id", nil)
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

					When("a custom hosted zone name is specified via annotation", func() {
						const customHostedZoneName = "my-cluster.custom.domain.com"

						BeforeEach(func() {
							awsManagedControlPlane.Annotations = map[string]string{
								controllers.DNSHostedZoneNameAnnotation: customHostedZoneName,
							}
							route53Client.GetHostedZoneIdByNameReturns("custom-parent-hosted-zone-id", nil)
							route53Client.CreateHostedZoneReturns("hosted-zone-id", nil)
						})

						It("creates hosted zone with custom name", func() {
							Expect(route53Client.CreateHostedZoneCallCount()).To(Equal(1))
							_, _, dnsZone := route53Client.CreateHostedZoneArgsForCall(0)
							Expect(dnsZone.DnsName).To(Equal(customHostedZoneName))
							Expect(reconcileErr).NotTo(HaveOccurred())
						})

						It("derives parent zone from custom hosted zone name", func() {
							Expect(route53Client.GetHostedZoneIdByNameCallCount()).To(Equal(1))
							_, _, parentZoneName := route53Client.GetHostedZoneIdByNameArgsForCall(0)
							Expect(parentZoneName).To(Equal("custom.domain.com"))
						})

						It("creates DNS records using custom zone name", func() {
							_, _, _, dnsRecords := route53Client.AddDnsRecordsToHostedZoneArgsForCall(0)
							Expect(dnsRecords).To(ContainElements(resolver.DNSRecord{
								Kind:   resolver.DnsRecordTypeCname,
								Name:   fmt.Sprintf("*.%s", customHostedZoneName),
								Values: []string{fmt.Sprintf("ingress.%s", customHostedZoneName)},
							}))
						})
					})

					When("a custom delegation identity is specified via annotation", func() {
						var delegationIdentity *capa.AWSClusterRoleIdentity

						BeforeEach(func() {
							delegationIdentity = &capa.AWSClusterRoleIdentity{
								ObjectMeta: metav1.ObjectMeta{
									Name: "delegation-identity",
								},
								Spec: capa.AWSClusterRoleIdentitySpec{
									AWSRoleSpec: capa.AWSRoleSpec{
										RoleArn: "arn:aws:iam::123456789012:role/CustomDelegationRole",
									},
								},
							}
							awsManagedControlPlane.Annotations = map[string]string{
								controllers.AWSDNSDelegationIdentityAnnotation: "delegation-identity",
							}
							// GetIdentity is called 3 times: MC identity (0), cluster identity (1), delegation identity (2)
							clusterClient.GetIdentityReturnsOnCall(2, delegationIdentity, nil)
							route53Client.GetHostedZoneIdByNameReturns("parent-hosted-zone-id", nil)
							route53Client.CreateHostedZoneReturns("hosted-zone-id", nil)
						})

						It("fetches the delegation identity", func() {
							Expect(clusterClient.GetIdentityCallCount()).To(Equal(3))
							_, identityRef := clusterClient.GetIdentityArgsForCall(2)
							Expect(identityRef.Name).To(Equal("delegation-identity"))
						})

						It("uses custom role ARN for delegation operations", func() {
							Expect(route53Client.AddDelegationToParentZoneCallCount()).To(Equal(1))
							Expect(reconcileErr).NotTo(HaveOccurred())
						})
					})

					When("a custom delegation identity is specified but does not exist", func() {
						expectedError := errors.New("AWSClusterRoleIdentity not found")

						BeforeEach(func() {
							awsManagedControlPlane.Annotations = map[string]string{
								controllers.AWSDNSDelegationIdentityAnnotation: "non-existent-identity",
							}
							// GetIdentity is called 3 times: MC identity (0), cluster identity (1), delegation identity (2)
							clusterClient.GetIdentityReturnsOnCall(2, nil, expectedError)
						})

						It("returns an error", func() {
							Expect(clusterClient.GetIdentityCallCount()).To(Equal(3))
							Expect(reconcileErr).To(HaveOccurred())
							Expect(reconcileErr).Should(MatchError(expectedError))
						})
					})

					When("a custom hosted zone name without valid parent is specified (single label)", func() {
						BeforeEach(func() {
							awsManagedControlPlane.Annotations = map[string]string{
								controllers.DNSHostedZoneNameAnnotation: "invalid",
							}
							route53Client.CreateHostedZoneReturns("hosted-zone-id", nil)
						})

						It("skips delegation but creates the hosted zone", func() {
							Expect(route53Client.CreateHostedZoneCallCount()).To(Equal(1))
							Expect(route53Client.AddDelegationToParentZoneCallCount()).To(Equal(0))
							Expect(reconcileErr).NotTo(HaveOccurred())
						})
					})

					When("a custom hosted zone name with only two labels is specified", func() {
						BeforeEach(func() {
							awsManagedControlPlane.Annotations = map[string]string{
								controllers.DNSHostedZoneNameAnnotation: "foo.com",
							}
							route53Client.CreateHostedZoneReturns("hosted-zone-id", nil)
						})

						It("skips delegation because parent would be TLD only", func() {
							Expect(route53Client.CreateHostedZoneCallCount()).To(Equal(1))
							_, _, dnsZone := route53Client.CreateHostedZoneArgsForCall(0)
							Expect(dnsZone.DnsName).To(Equal("foo.com"))
							Expect(route53Client.AddDelegationToParentZoneCallCount()).To(Equal(0))
							Expect(reconcileErr).NotTo(HaveOccurred())
						})
					})

					When("a deeply nested custom hosted zone name is specified", func() {
						const customHostedZoneName = "deep.nested.sub.domain.com"

						BeforeEach(func() {
							awsManagedControlPlane.Annotations = map[string]string{
								controllers.DNSHostedZoneNameAnnotation: customHostedZoneName,
							}
							route53Client.GetHostedZoneIdByNameReturns("parent-hosted-zone-id", nil)
							route53Client.CreateHostedZoneReturns("hosted-zone-id", nil)
						})

						It("derives the correct parent zone", func() {
							Expect(route53Client.GetHostedZoneIdByNameCallCount()).To(Equal(1))
							_, _, parentZoneName := route53Client.GetHostedZoneIdByNameArgsForCall(0)
							Expect(parentZoneName).To(Equal("nested.sub.domain.com"))
						})
					})

					When("the cluster uses private dns mode", func() {
						BeforeEach(func() {
							awsManagedControlPlane.Annotations = map[string]string{
								gsannotations.AWSDNSMode:          "private",
								gsannotations.AWSDNSAdditionalVPC: "vpc-0011223344,vpc-0987654321",
							}
							route53Client.CreateHostedZoneReturns("hosted-zone-id", nil)
						})

						It("creates hosted zone", func() {
							Expect(route53Client.CreateHostedZoneCallCount()).To(Equal(1))
							_, _, dnsZone := route53Client.CreateHostedZoneArgsForCall(0)
							Expect(dnsZone.DnsName).To(Equal(fmt.Sprintf("%s.%s", awsManagedControlPlane.Name, "test.gigantic.io")))
							Expect(dnsZone.VPCId).To(Equal(awsManagedControlPlane.Spec.NetworkSpec.VPC.ID))
							Expect(dnsZone.Region).To(Equal(awsManagedControlPlane.Spec.Region))
							expectedTags := map[string]string{
								"Name": awsManagedControlPlane.Name,
								fmt.Sprintf("sigs.k8s.io/cluster-api-provider-aws/cluster/%s", awsManagedControlPlane.Name): "owned",
								"sigs.k8s.io/cluster-api-provider-aws/role":                                                 "common",
							}
							Expect(dnsZone.Tags).To(Equal(expectedTags))
							expectedVPCIdsToAttach := []string{"vpc-0011223344", "vpc-0987654321"}
							Expect(dnsZone.VPCsToAssociate).To(Equal(expectedVPCIdsToAttach))
							Expect(reconcileErr).NotTo(HaveOccurred())
						})

						It("creates DNS records for workload cluster", func() {
							Expect(route53Client.AddDnsRecordsToHostedZoneCallCount()).To(Equal(1))
							_, _, hostedZoneId, dnsRecords := route53Client.AddDnsRecordsToHostedZoneArgsForCall(0)
							Expect(hostedZoneId).To(Equal("hosted-zone-id"))
							Expect(dnsRecords).To(ContainElements(resolver.DNSRecord{
								Kind:   "CNAME",
								Name:   fmt.Sprintf("*.%s.%s", ClusterName, WorkloadClusterBaseDomain),
								Values: []string{fmt.Sprintf("ingress.%s.%s", ClusterName, WorkloadClusterBaseDomain)},
							}))
						})

						When("there are no additional VPCs to associate", func() {
							BeforeEach(func() {
								awsManagedControlPlane.Annotations = map[string]string{
									gsannotations.AWSDNSMode: "private",
								}
							})

							It("creates hosted zone", func() {
								Expect(route53Client.CreateHostedZoneCallCount()).To(Equal(1))
								_, _, dnsZone := route53Client.CreateHostedZoneArgsForCall(0)
								Expect(dnsZone.VPCsToAssociate).To(HaveLen(0))
								Expect(reconcileErr).NotTo(HaveOccurred())
							})
						})

						When("the k8s API endpoint of the workload cluster is set", func() {
							BeforeEach(func() {
								awsManagedControlPlane.Spec.ControlPlaneEndpoint.Host = "control-plane-load-balancer-hostname"
							})

							It("creates DNS records for workload cluster", func() {
								_, _, _, dnsRecords := route53Client.AddDnsRecordsToHostedZoneArgsForCall(0)
								Expect(dnsRecords).To(ContainElements(resolver.DNSRecord{
									Kind:   resolver.DnsRecordTypeCname,
									Name:   fmt.Sprintf("api.%s.%s", ClusterName, WorkloadClusterBaseDomain),
									Values: []string{"control-plane-load-balancer-hostname"},
									Region: awsManagedControlPlane.Spec.Region,
								}))
							})
						})

						When("the k8s API endpoint of the workload cluster is not set", func() {
							It("it does not create DNS record for the API yet", func() {
								_, _, _, dnsRecords := route53Client.AddDnsRecordsToHostedZoneArgsForCall(0)
								Expect(dnsRecords).ToNot(ContainElement(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
									"Kind": Equal("ALIAS"),
									"Name": Equal(fmt.Sprintf("api.%s.%s", ClusterName, WorkloadClusterBaseDomain)),
								})))
							})
						})

						When("creating hosted zone fails", func() {
							expectedError := errors.New("failed to create hosted zone")
							BeforeEach(func() {
								route53Client.CreateHostedZoneReturns("", expectedError)
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
})
