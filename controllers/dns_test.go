package controllers_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	gsannotations "github.com/giantswarm/k8smetadata/pkg/annotation"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gstruct"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	capa "sigs.k8s.io/cluster-api-provider-aws/api/v1beta1"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/aws-resolver-rules-operator/controllers"
	"github.com/aws-resolver-rules-operator/controllers/controllersfakes"
	"github.com/aws-resolver-rules-operator/pkg/k8sclient"
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
		cluster                 *capi.Cluster
		result                  ctrl.Result
		reconcileErr            error
		resolverClient          *resolverfakes.FakeResolverClient
		dnsServerResolverClient *resolverfakes.FakeResolverClient
		ec2Client               *resolverfakes.FakeEC2Client
		ramClient               *resolverfakes.FakeRAMClient
		route53Client           *resolverfakes.FakeRoute53Client
	)

	const (
		ClusterName               = "foo"
		ClusterNamespace          = "bar"
		WorkloadClusterBaseDomain = "test.gigantic.io"
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

		dns, err := resolver.NewDnsZone(fakeAWSClients, WorkloadClusterBaseDomain)
		Expect(err).NotTo(HaveOccurred())

		reconciler = controllers.NewDnsReconciler(awsClusterClient, dns)

		awsClusterRoleIdentity = &capa.AWSClusterRoleIdentity{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ClusterNamespace,
				Name:      "default",
			},
			Spec: capa.AWSClusterRoleIdentitySpec{},
		}
		awsCluster = &capa.AWSCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ClusterName,
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
						ID: "vpc-12345678",
					},
				},
			},
		}
		cluster = &capi.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ClusterName,
				Namespace: ClusterNamespace,
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
			awsClusterClient.GetBastionMachineReturns(nil, &k8sclient.BastionNotFoundError{})
		})

		When("the aws cluster doesn't have an owner yet", func() {
			BeforeEach(func() {
				awsClusterClient.GetOwnerReturns(nil, nil)
			})

			It("doesn't really reconcile", func() {
				Expect(awsClusterClient.GetIdentityCallCount()).To(BeZero())
				Expect(reconcileErr).NotTo(HaveOccurred())
			})
		})

		When("there is an error trying to find the owner cluster", func() {
			expectedError := errors.New("failed fetching the AWSCluster")

			BeforeEach(func() {
				awsClusterClient.GetOwnerReturns(nil, expectedError)
			})
			It("returns the error", func() {
				Expect(awsClusterClient.GetIdentityCallCount()).To(BeZero())
				Expect(reconcileErr).To(HaveOccurred())
				Expect(reconcileErr).Should(MatchError(expectedError))
			})
		})

		When("the aws cluster already has an owner", func() {
			BeforeEach(func() {
				awsClusterClient.GetOwnerReturns(cluster, nil)
			})

			When("the cluster is paused", func() {
				BeforeEach(func() {
					cluster.Spec.Paused = true
					awsClusterClient.GetOwnerReturns(cluster, nil)
				})

				It("does not reconcile", func() {
					Expect(awsClusterClient.GetIdentityCallCount()).To(BeZero())
					Expect(reconcileErr).NotTo(HaveOccurred())
				})
			})

			When("the infrastructure cluster is paused", func() {
				BeforeEach(func() {
					awsCluster.Annotations = map[string]string{
						capi.PausedAnnotation: "true",
					}
					awsClusterClient.GetAWSClusterReturns(awsCluster, nil)
				})

				It("does not reconcile", func() {
					Expect(awsClusterClient.GetIdentityCallCount()).To(BeZero())
					Expect(reconcileErr).NotTo(HaveOccurred())
				})
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
							_, _, parentHostedZoneId, hostedZoneId := route53Client.DeleteDelegationFromParentZoneArgsForCall(0)
							Expect(parentHostedZoneId).To(Equal("parent-hosted-zone-id"))
							Expect(hostedZoneId).To(Equal("hosted-zone-id"))
						})

						It("deletes the workload cluster hosted zone", func() {
							_, _, hostedZoneId := route53Client.DeleteHostedZoneArgsForCall(0)
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
					It("adds the finalizer to the AWSCluster", func() {
						Expect(awsClusterClient.AddFinalizerCallCount()).To(Equal(1))
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
							Expect(dnsZone.DnsName).To(Equal(fmt.Sprintf("%s.%s", awsCluster.Name, "test.gigantic.io")))
							expectedTags := map[string]string{
								"Name": awsCluster.Name,
								fmt.Sprintf("sigs.k8s.io/cluster-api-provider-aws/cluster/%s", awsCluster.Name): "owned",
								"sigs.k8s.io/cluster-api-provider-aws/role":                                     "common",
							}
							Expect(dnsZone.Tags).To(Equal(expectedTags))
							Expect(reconcileErr).NotTo(HaveOccurred())
						})

						It("adds delegation of ns records to parent hosted zone", func() {
							Expect(route53Client.AddDelegationToParentZoneCallCount()).To(Equal(1))
							_, _, parentHostedZoneId, hostedZoneId := route53Client.AddDelegationToParentZoneArgsForCall(0)
							Expect(parentHostedZoneId).To(Equal("parent-hosted-zone-id"))
							Expect(hostedZoneId).To(Equal("hosted-zone-id"))
						})

						It("creates DNS records for workload cluster", func() {
							Expect(route53Client.AddDnsRecordsToHostedZoneCallCount()).To(Equal(1))
							_, _, hostedZoneId, dnsRecords := route53Client.AddDnsRecordsToHostedZoneArgsForCall(0)
							Expect(hostedZoneId).To(Equal("hosted-zone-id"))
							Expect(dnsRecords).To(ContainElements(resolver.DNSRecord{
								Kind:  resolver.DnsRecordTypeCname,
								Name:  fmt.Sprintf("*.%s.%s", ClusterName, WorkloadClusterBaseDomain),
								Value: fmt.Sprintf("ingress.%s.%s", ClusterName, WorkloadClusterBaseDomain),
							}))
						})

						When("the k8s API endpoint of the workload cluster is set", func() {
							BeforeEach(func() {
								awsCluster.Spec.ControlPlaneEndpoint.Host = "control-plane-load-balancer-hostname"
							})

							It("creates DNS records for workload cluster", func() {
								_, _, _, dnsRecords := route53Client.AddDnsRecordsToHostedZoneArgsForCall(0)
								Expect(dnsRecords).To(ContainElements(resolver.DNSRecord{
									Kind:   resolver.DnsRecordTypeAlias,
									Name:   fmt.Sprintf("api.%s.%s", ClusterName, WorkloadClusterBaseDomain),
									Value:  "control-plane-load-balancer-hostname",
									Region: awsCluster.Spec.Region,
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

						When("there is a bastion server deployed", func() {
							bastionMachine := &capi.Machine{
								ObjectMeta: metav1.ObjectMeta{
									Name: "",
									Labels: map[string]string{
										capi.ClusterLabelName:   ClusterName,
										"cluster.x-k8s.io/role": "bastion",
									},
								},
								Status: capi.MachineStatus{
									Addresses: []capi.MachineAddress{
										{
											Type:    capi.MachineExternalIP,
											Address: "192.168.0.1",
										},
									},
								},
							}
							BeforeEach(func() {
								awsClusterClient.GetBastionMachineReturns(bastionMachine, nil)
							})

							It("it creates DNS record for the bastion", func() {
								_, _, _, dnsRecords := route53Client.AddDnsRecordsToHostedZoneArgsForCall(0)
								Expect(dnsRecords).To(ContainElements(resolver.DNSRecord{
									Kind:  resolver.DnsRecordTypeA,
									Name:  fmt.Sprintf("bastion1.%s.%s", ClusterName, WorkloadClusterBaseDomain),
									Value: "192.168.0.1",
								}))
							})
						})

						When("there is bastion server deployed but it has no IP set yet", func() {
							bastionMachine := &capi.Machine{
								ObjectMeta: metav1.ObjectMeta{
									Name: "",
									Labels: map[string]string{
										capi.ClusterLabelName:   ClusterName,
										"cluster.x-k8s.io/role": "bastion",
									},
								},
							}
							BeforeEach(func() {
								awsClusterClient.GetBastionMachineReturns(bastionMachine, nil)
							})

							It("it doesn't create DNS record for the bastion but requeues", func() {
								_, _, _, dnsRecords := route53Client.AddDnsRecordsToHostedZoneArgsForCall(0)
								Expect(dnsRecords).ToNot(ContainElement(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
									"Kind": Equal("A"),
									"Name": Equal(fmt.Sprintf("bastion1.%s.%s", ClusterName, WorkloadClusterBaseDomain)),
								})))
								Expect(result.RequeueAfter).Should(Equal(1 * time.Minute))
							})
						})

						When("there is no bastion server deployed", func() {
							BeforeEach(func() {
								awsClusterClient.GetBastionMachineReturns(nil, &k8sclient.BastionNotFoundError{})
							})

							It("it doesn't create DNS record for the bastion", func() {
								_, _, _, dnsRecords := route53Client.AddDnsRecordsToHostedZoneArgsForCall(0)
								Expect(dnsRecords).ToNot(ContainElement(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
									"Kind": Equal("A"),
									"Name": Equal(fmt.Sprintf("bastion1.%s.%s", ClusterName, WorkloadClusterBaseDomain)),
								})))
								Expect(result.RequeueAfter).Should(Equal(0 * time.Minute))
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

					When("the cluster uses private dns mode", func() {
						BeforeEach(func() {
							awsCluster.Annotations = map[string]string{
								gsannotations.AWSDNSMode:          "private",
								gsannotations.AWSDNSAdditionalVPC: "vpc-0011223344,vpc-0987654321",
							}
							route53Client.CreateHostedZoneReturns("hosted-zone-id", nil)
						})

						It("creates hosted zone", func() {
							Expect(route53Client.CreateHostedZoneCallCount()).To(Equal(1))
							_, _, dnsZone := route53Client.CreateHostedZoneArgsForCall(0)
							Expect(dnsZone.DnsName).To(Equal(fmt.Sprintf("%s.%s", awsCluster.Name, "test.gigantic.io")))
							Expect(dnsZone.VPCId).To(Equal(awsCluster.Spec.NetworkSpec.VPC.ID))
							Expect(dnsZone.Region).To(Equal(awsCluster.Spec.Region))
							expectedTags := map[string]string{
								"Name": awsCluster.Name,
								fmt.Sprintf("sigs.k8s.io/cluster-api-provider-aws/cluster/%s", awsCluster.Name): "owned",
								"sigs.k8s.io/cluster-api-provider-aws/role":                                     "common",
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
								Kind:  "CNAME",
								Name:  fmt.Sprintf("*.%s.%s", ClusterName, WorkloadClusterBaseDomain),
								Value: fmt.Sprintf("ingress.%s.%s", ClusterName, WorkloadClusterBaseDomain),
							}))
						})

						When("there are no additional VPCs to associate", func() {
							BeforeEach(func() {
								awsCluster.Annotations = map[string]string{
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
								awsCluster.Spec.ControlPlaneEndpoint.Host = "control-plane-load-balancer-hostname"
							})

							It("creates DNS records for workload cluster", func() {
								_, _, _, dnsRecords := route53Client.AddDnsRecordsToHostedZoneArgsForCall(0)
								Expect(dnsRecords).To(ContainElements(resolver.DNSRecord{
									Kind:   "ALIAS",
									Name:   fmt.Sprintf("api.%s.%s", ClusterName, WorkloadClusterBaseDomain),
									Value:  "control-plane-load-balancer-hostname",
									Region: awsCluster.Spec.Region,
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

						When("there is a bastion server deployed", func() {
							bastionMachine := &capi.Machine{
								ObjectMeta: metav1.ObjectMeta{
									Name: "",
									Labels: map[string]string{
										capi.ClusterLabelName:   ClusterName,
										"cluster.x-k8s.io/role": "bastion",
									},
								},
								Status: capi.MachineStatus{
									Addresses: []capi.MachineAddress{
										{
											Type:    capi.MachineExternalIP,
											Address: "192.168.0.1",
										},
									},
								},
							}
							BeforeEach(func() {
								awsClusterClient.GetBastionMachineReturns(bastionMachine, nil)
							})

							It("it creates DNS record for the bastion", func() {
								_, _, _, dnsRecords := route53Client.AddDnsRecordsToHostedZoneArgsForCall(0)
								Expect(dnsRecords).To(ContainElements(resolver.DNSRecord{
									Kind:  "A",
									Name:  fmt.Sprintf("bastion1.%s.%s", ClusterName, WorkloadClusterBaseDomain),
									Value: "192.168.0.1",
								}))
							})
						})

						When("there is a bastion server deployed but it has no IP set yet", func() {
							bastionMachine := &capi.Machine{
								ObjectMeta: metav1.ObjectMeta{
									Name: "",
									Labels: map[string]string{
										capi.ClusterLabelName:   ClusterName,
										"cluster.x-k8s.io/role": "bastion",
									},
								},
							}
							BeforeEach(func() {
								awsClusterClient.GetBastionMachineReturns(bastionMachine, nil)
							})

							It("it creates DNS record for the bastion", func() {
								_, _, _, dnsRecords := route53Client.AddDnsRecordsToHostedZoneArgsForCall(0)
								Expect(dnsRecords).ToNot(ContainElement(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
									"Kind": Equal("A"),
									"Name": Equal(fmt.Sprintf("bastion1.%s.%s", ClusterName, WorkloadClusterBaseDomain)),
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
