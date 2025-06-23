package controllers_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubectl/pkg/scheme"
	capa "sigs.k8s.io/cluster-api-provider-aws/v2/api/v1beta2"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/controllers/remote"
	capiexp "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	karpenterinfra "github.com/aws-resolver-rules-operator/api/v1alpha1"
	"github.com/aws-resolver-rules-operator/controllers"
	"github.com/aws-resolver-rules-operator/pkg/resolver"
	"github.com/aws-resolver-rules-operator/pkg/resolver/resolverfakes"
)

var _ = Describe("KarpenterMachinePool reconciler", func() {
	var (
		capiBootstrapSecretContent []byte
		capiBootstrapSecretHash    string
		dataSecretName             string
		s3Client                   *resolverfakes.FakeS3Client
		ec2Client                  *resolverfakes.FakeEC2Client
		fakeCtrlClient             client.Client
		fakeClusterClientGetter    remote.ClusterClientGetter
		ctx                        context.Context
		reconciler                 *controllers.KarpenterMachinePoolReconciler
		reconcileErr               error
		reconcileResult            reconcile.Result
	)

	const (
		ClusterName                   = "foo"
		AWSClusterBucketName          = "my-awesome-bucket"
		DataSecretName                = "foo-mp-12345"
		KarpenterMachinePoolName      = "foo"
		KarpenterMachinePoolNamespace = "org-bar"
		KubernetesVersion             = "v1.29.1"
	)

	BeforeEach(func() {
		ctx = context.Background()

		capiBootstrapSecretContent = []byte("some-bootstrap-data")
		capiBootstrapSecretHash = fmt.Sprintf("%x", sha256.Sum256(capiBootstrapSecretContent))

		err := capi.AddToScheme(scheme.Scheme)
		Expect(err).NotTo(HaveOccurred())

		err = capiexp.AddToScheme(scheme.Scheme)
		Expect(err).NotTo(HaveOccurred())

		err = capa.AddToScheme(scheme.Scheme)
		Expect(err).NotTo(HaveOccurred())

		err = karpenterinfra.AddToScheme(scheme.Scheme)
		Expect(err).NotTo(HaveOccurred())

		fakeCtrlClient = fake.NewClientBuilder().
			WithScheme(scheme.Scheme).
			WithStatusSubresource(&karpenterinfra.KarpenterMachinePool{}).
			Build()

		// Use the default fake cluster client getter
		fakeClusterClientGetter = func(ctx context.Context, _ string, _ client.Client, _ client.ObjectKey) (client.Client, error) {
			// Return the same client that we're using for the test
			return fakeCtrlClient, nil
		}

		s3Client = new(resolverfakes.FakeS3Client)
		ec2Client = new(resolverfakes.FakeEC2Client)

		clientsFactory := &resolver.FakeClients{
			S3Client:  s3Client,
			EC2Client: ec2Client,
		}

		reconciler = controllers.NewKarpenterMachinepoolReconciler(fakeCtrlClient, fakeClusterClientGetter, clientsFactory)
	})

	JustBeforeEach(func() {
		request := ctrl.Request{
			NamespacedName: types.NamespacedName{
				Namespace: KarpenterMachinePoolNamespace,
				Name:      KarpenterMachinePoolName,
			},
		}
		reconcileResult, reconcileErr = reconciler.Reconcile(ctx, request)
	})

	When("the reconciled KarpenterMachinePool is gone", func() {
		It("does nothing", func() {
			Expect(reconcileErr).NotTo(HaveOccurred())
		})
	})

	When("the KarpenterMachinePool exists but it does not contain an owner", func() {
		BeforeEach(func() {
			karpenterMachinePool := &karpenterinfra.KarpenterMachinePool{
				ObjectMeta: ctrl.ObjectMeta{
					Namespace: KarpenterMachinePoolNamespace,
					Name:      KarpenterMachinePoolName,
				},
			}
			err := fakeCtrlClient.Create(ctx, karpenterMachinePool)
			Expect(err).NotTo(HaveOccurred())
		})
		It("does nothing", func() {
			Expect(reconcileErr).NotTo(HaveOccurred())
		})
	})

	When("the KarpenterMachinePool is being deleted", func() {
		BeforeEach(func() {
			karpenterMachinePool := &karpenterinfra.KarpenterMachinePool{
				ObjectMeta: ctrl.ObjectMeta{
					Namespace: KarpenterMachinePoolNamespace,
					Name:      KarpenterMachinePoolName,
					Labels: map[string]string{
						capi.ClusterNameLabel: ClusterName,
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "cluster.x-k8s.io/v1beta1",
							Kind:       "MachinePool",
							Name:       KarpenterMachinePoolName,
						},
					},
					Finalizers: []string{controllers.KarpenterFinalizer},
				},
			}
			err := fakeCtrlClient.Create(ctx, karpenterMachinePool)
			Expect(err).NotTo(HaveOccurred())

			err = fakeCtrlClient.Delete(ctx, karpenterMachinePool)
			Expect(err).NotTo(HaveOccurred())

			dataSecretName = DataSecretName
			version := KubernetesVersion
			machinePool := &capiexp.MachinePool{
				ObjectMeta: ctrl.ObjectMeta{
					Namespace: KarpenterMachinePoolNamespace,
					Name:      KarpenterMachinePoolName,
					Labels: map[string]string{
						capi.ClusterNameLabel: ClusterName,
					},
				},
				Spec: capiexp.MachinePoolSpec{
					ClusterName: ClusterName,
					Template: capi.MachineTemplateSpec{
						ObjectMeta: capi.ObjectMeta{},
						Spec: capi.MachineSpec{
							ClusterName: ClusterName,
							Bootstrap: capi.Bootstrap{
								ConfigRef: &v1.ObjectReference{
									Kind:       "KubeadmConfig",
									Namespace:  KarpenterMachinePoolNamespace,
									Name:       fmt.Sprintf("%s-1a2b3c", KarpenterMachinePoolName),
									APIVersion: "bootstrap.cluster.x-k8s.io/v1beta1",
								},
								DataSecretName: &dataSecretName,
							},
							InfrastructureRef: v1.ObjectReference{
								Kind:       "KarpenterMachinePool",
								Namespace:  KarpenterMachinePoolNamespace,
								Name:       KarpenterMachinePoolName,
								APIVersion: "infrastructure.cluster.x-k8s.io/v1alpha1",
							},
							Version: &version,
						},
					},
				},
			}
			err = fakeCtrlClient.Create(ctx, machinePool)
			Expect(err).NotTo(HaveOccurred())

			awsCluster := &capa.AWSCluster{
				ObjectMeta: ctrl.ObjectMeta{
					Namespace: KarpenterMachinePoolNamespace,
					Name:      ClusterName,
				},
				Spec: capa.AWSClusterSpec{
					IdentityRef: &capa.AWSIdentityReference{
						Name: "default",
						Kind: capa.ClusterRoleIdentityKind,
					},
					S3Bucket: &capa.S3Bucket{Name: AWSClusterBucketName},
				},
			}
			err = fakeCtrlClient.Create(ctx, awsCluster)
			Expect(err).NotTo(HaveOccurred())

			clusterKubeconfigSecret := &v1.Secret{
				ObjectMeta: ctrl.ObjectMeta{
					Namespace: KarpenterMachinePoolNamespace,
					Name:      fmt.Sprintf("%s-kubeconfig", ClusterName),
				},
			}
			err = fakeCtrlClient.Create(ctx, clusterKubeconfigSecret)
			Expect(err).NotTo(HaveOccurred())

			awsClusterRoleIdentity := &capa.AWSClusterRoleIdentity{
				ObjectMeta: metav1.ObjectMeta{
					Name: "default",
				},
				Spec: capa.AWSClusterRoleIdentitySpec{},
			}
			err = fakeCtrlClient.Create(ctx, awsClusterRoleIdentity)
			Expect(err).NotTo(HaveOccurred())

			bootstrapSecret := &v1.Secret{
				ObjectMeta: ctrl.ObjectMeta{
					Namespace: KarpenterMachinePoolNamespace,
					Name:      DataSecretName,
				},
				Data: map[string][]byte{"value": capiBootstrapSecretContent},
			}
			err = fakeCtrlClient.Create(ctx, bootstrapSecret)
			Expect(err).NotTo(HaveOccurred())
		})

		When("the owner cluster is also being deleted", func() {
			BeforeEach(func() {
				cluster := &capi.Cluster{
					ObjectMeta: ctrl.ObjectMeta{
						Namespace: KarpenterMachinePoolNamespace,
						Name:      ClusterName,
						Labels: map[string]string{
							capi.ClusterNameLabel: ClusterName,
						},
						Finalizers: []string{"something-to-keep-it-around-when-deleting"},
					},
					Spec: capi.ClusterSpec{
						InfrastructureRef: &v1.ObjectReference{
							Kind:       "AWSCluster",
							Namespace:  KarpenterMachinePoolNamespace,
							Name:       ClusterName,
							APIVersion: "infrastructure.cluster.x-k8s.io/v1beta2",
						},
					},
				}
				err := fakeCtrlClient.Create(ctx, cluster)
				Expect(err).NotTo(HaveOccurred())

				err = fakeCtrlClient.Delete(ctx, cluster)
				Expect(err).NotTo(HaveOccurred())
			})
			// This test is a bit cumbersome because we are deleting CRs, so we can't use different `It` blocks or the CRs would be gone.
			// We first mock the call to `TerminateInstancesByTag` to return some instances so that we can test
			// the behavior when there are pending instances to remove.
			// Then we manually/explicitly call the reconciler inside the test again, to be able to test the behavior
			// when there are no instances to remove.
			When("there are ec2 instances from karpenter", func() {
				BeforeEach(func() {
					ec2Client.TerminateInstancesByTagReturnsOnCall(0, []string{"i-abc123", "i-def456"}, nil)
				})

				It("deletes KarpenterMachinePool ec2 instances and finalizer", func() {
					Expect(reconcileErr).NotTo(HaveOccurred())
					Expect(ec2Client.TerminateInstancesByTagCallCount()).To(Equal(1))
					Expect(reconcileResult.RequeueAfter).To(Equal(30 * time.Second))

					karpenterMachinePoolList := &karpenterinfra.KarpenterMachinePoolList{}
					err := fakeCtrlClient.List(ctx, karpenterMachinePoolList)
					Expect(err).NotTo(HaveOccurred())
					// Finalizer should be there blocking the deletion of the CR
					Expect(karpenterMachinePoolList.Items).To(HaveLen(1))

					ec2Client.TerminateInstancesByTagReturnsOnCall(0, nil, nil)

					reconcileResult, reconcileErr = reconciler.Reconcile(ctx, ctrl.Request{
						NamespacedName: types.NamespacedName{
							Namespace: KarpenterMachinePoolNamespace,
							Name:      KarpenterMachinePoolName,
						},
					})

					karpenterMachinePoolList = &karpenterinfra.KarpenterMachinePoolList{}
					err = fakeCtrlClient.List(ctx, karpenterMachinePoolList)
					Expect(err).NotTo(HaveOccurred())
					// Finalizer should've been removed and the CR should be gone
					Expect(karpenterMachinePoolList.Items).To(HaveLen(0))
				})
			})
		})
	})

	When("the KarpenterMachinePool exists and it has a MachinePool owner", func() {
		BeforeEach(func() {
			karpenterMachinePool := &karpenterinfra.KarpenterMachinePool{
				ObjectMeta: ctrl.ObjectMeta{
					Namespace: KarpenterMachinePoolNamespace,
					Name:      KarpenterMachinePoolName,
					Labels: map[string]string{
						capi.ClusterNameLabel: ClusterName,
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "cluster.x-k8s.io/v1beta1",
							Kind:       "MachinePool",
							Name:       KarpenterMachinePoolName,
						},
					},
				},
			}
			err := fakeCtrlClient.Create(ctx, karpenterMachinePool)
			Expect(err).NotTo(HaveOccurred())
		})
		When("the referenced MachinePool does not exist", func() {
			It("returns an error", func() {
				Expect(reconcileErr).To(MatchError(ContainSubstring("failed to get MachinePool owning the KarpenterMachinePool")))
			})
		})
		When("the referenced MachinePool exists without MachinePool.spec.template.spec.bootstrap.dataSecretName being set", func() {
			BeforeEach(func() {
				version := KubernetesVersion
				machinePool := &capiexp.MachinePool{
					ObjectMeta: ctrl.ObjectMeta{
						Namespace: KarpenterMachinePoolNamespace,
						Name:      KarpenterMachinePoolName,
						Labels: map[string]string{
							capi.ClusterNameLabel: ClusterName,
						},
					},
					Spec: capiexp.MachinePoolSpec{
						ClusterName: ClusterName,
						// Replicas:    nil,
						Template: capi.MachineTemplateSpec{
							ObjectMeta: capi.ObjectMeta{},
							Spec: capi.MachineSpec{
								ClusterName: "",
								Bootstrap: capi.Bootstrap{
									ConfigRef: &v1.ObjectReference{
										Kind:       "KubeadmConfig",
										Namespace:  KarpenterMachinePoolNamespace,
										Name:       fmt.Sprintf("%s-1a2b3c", KarpenterMachinePoolName),
										APIVersion: "bootstrap.cluster.x-k8s.io/v1beta1",
									},
								},
								InfrastructureRef: v1.ObjectReference{
									Kind:       "KarpenterMachinePool",
									Namespace:  KarpenterMachinePoolNamespace,
									Name:       KarpenterMachinePoolName,
									APIVersion: "infrastructure.cluster.x-k8s.io/v1alpha1",
								},
								Version: &version,
							},
						},
					},
				}
				err := fakeCtrlClient.Create(ctx, machinePool)
				Expect(err).NotTo(HaveOccurred())
			})
			It("returns early", func() {
				Expect(reconcileErr).NotTo(HaveOccurred())
			})
		})
		When("the referenced MachinePool exists and MachinePool.spec.template.spec.bootstrap.dataSecretName is set", func() {
			BeforeEach(func() {
				dataSecretName = DataSecretName
				version := KubernetesVersion
				machinePool := &capiexp.MachinePool{
					ObjectMeta: ctrl.ObjectMeta{
						Namespace: KarpenterMachinePoolNamespace,
						Name:      KarpenterMachinePoolName,
						Labels: map[string]string{
							capi.ClusterNameLabel: ClusterName,
						},
					},
					Spec: capiexp.MachinePoolSpec{
						ClusterName: ClusterName,
						// Replicas:    nil,
						Template: capi.MachineTemplateSpec{
							ObjectMeta: capi.ObjectMeta{},
							Spec: capi.MachineSpec{
								ClusterName: ClusterName,
								Bootstrap: capi.Bootstrap{
									ConfigRef: &v1.ObjectReference{
										Kind:       "KubeadmConfig",
										Namespace:  KarpenterMachinePoolNamespace,
										Name:       fmt.Sprintf("%s-1a2b3c", KarpenterMachinePoolName),
										APIVersion: "bootstrap.cluster.x-k8s.io/v1beta1",
									},
									DataSecretName: &dataSecretName,
								},
								InfrastructureRef: v1.ObjectReference{
									Kind:       "KarpenterMachinePool",
									Namespace:  KarpenterMachinePoolNamespace,
									Name:       KarpenterMachinePoolName,
									APIVersion: "infrastructure.cluster.x-k8s.io/v1alpha1",
								},
								Version: &version,
							},
						},
					},
				}
				err := fakeCtrlClient.Create(ctx, machinePool)
				Expect(err).NotTo(HaveOccurred())
			})
			When("there is no Cluster owning the MachinePool", func() {
				It("returns an error", func() {
					Expect(reconcileErr).To(MatchError(ContainSubstring("failed to get Cluster owning the MachinePool that owns the KarpenterMachinePool")))
				})
			})
			When("there is a Cluster that owns the MachinePool but it's paused", func() {
				BeforeEach(func() {
					cluster := &capi.Cluster{
						ObjectMeta: ctrl.ObjectMeta{
							Namespace: KarpenterMachinePoolNamespace,
							Name:      ClusterName,
							Labels: map[string]string{
								capi.ClusterNameLabel: ClusterName,
							},
						},
						Spec: capi.ClusterSpec{
							Paused: true,
							InfrastructureRef: &v1.ObjectReference{
								Kind:       "AWSCluster",
								Namespace:  KarpenterMachinePoolNamespace,
								Name:       ClusterName,
								APIVersion: "infrastructure.cluster.x-k8s.io/v1beta2",
							},
						},
					}
					err := fakeCtrlClient.Create(ctx, cluster)
					Expect(err).NotTo(HaveOccurred())

					clusterKubeconfigSecret := &v1.Secret{
						ObjectMeta: ctrl.ObjectMeta{
							Namespace: KarpenterMachinePoolNamespace,
							Name:      fmt.Sprintf("%s-kubeconfig", ClusterName),
						},
					}
					err = fakeCtrlClient.Create(ctx, clusterKubeconfigSecret)
					Expect(err).NotTo(HaveOccurred())
				})
				It("returns early", func() {
					Expect(reconcileErr).NotTo(HaveOccurred())
					Expect(s3Client.PutCallCount()).To(Equal(0))
				})
			})
			When("there is a Cluster that owns the MachinePool", func() {
				BeforeEach(func() {
					cluster := &capi.Cluster{
						ObjectMeta: ctrl.ObjectMeta{
							Namespace: KarpenterMachinePoolNamespace,
							Name:      ClusterName,
							Labels: map[string]string{
								capi.ClusterNameLabel: ClusterName,
							},
						},
						Spec: capi.ClusterSpec{
							InfrastructureRef: &v1.ObjectReference{
								Kind:       "AWSCluster",
								Namespace:  KarpenterMachinePoolNamespace,
								Name:       ClusterName,
								APIVersion: "infrastructure.cluster.x-k8s.io/v1beta2",
							},
						},
					}
					err := fakeCtrlClient.Create(ctx, cluster)
					Expect(err).NotTo(HaveOccurred())
				})
				When("there is no AWSCluster", func() {
					It("returns an error", func() {
						Expect(reconcileErr).To(MatchError(ContainSubstring("failed to get AWSCluster referenced in Cluster.spec.infrastructureRef")))
					})
				})
				When("the AWSCluster exists but there is no S3 bucket defined on it", func() {
					BeforeEach(func() {
						awsCluster := &capa.AWSCluster{
							ObjectMeta: ctrl.ObjectMeta{
								Namespace: KarpenterMachinePoolNamespace,
								Name:      ClusterName,
								Labels: map[string]string{
									capi.ClusterNameLabel: ClusterName,
								},
							},
							Spec: capa.AWSClusterSpec{},
						}
						err := fakeCtrlClient.Create(ctx, awsCluster)
						Expect(err).NotTo(HaveOccurred())
					})
					It("returns an error", func() {
						Expect(reconcileErr).To(MatchError(errors.New("a cluster wide object storage configured at `AWSCluster.spec.s3Bucket` is required")))
					})
				})
				When("the AWSCluster exists and there is a S3 bucket defined on it", func() {
					BeforeEach(func() {
						awsCluster := &capa.AWSCluster{
							ObjectMeta: ctrl.ObjectMeta{
								Namespace: KarpenterMachinePoolNamespace,
								Name:      ClusterName,
							},
							Spec: capa.AWSClusterSpec{
								IdentityRef: &capa.AWSIdentityReference{
									Name: "default",
									Kind: capa.ClusterRoleIdentityKind,
								},
								S3Bucket: &capa.S3Bucket{Name: AWSClusterBucketName},
							},
						}
						err := fakeCtrlClient.Create(ctx, awsCluster)
						Expect(err).NotTo(HaveOccurred())
					})
					When("it can't find the identity used by the AWSCluster", func() {
						It("returns an error", func() {
							Expect(reconcileErr).To(MatchError(ContainSubstring("failed to get AWSClusterRoleIdentity referenced in AWSCluster")))
						})
					})
					When("it finds the identity used by the AWSCluster", func() {
						BeforeEach(func() {
							awsClusterRoleIdentity := &capa.AWSClusterRoleIdentity{
								ObjectMeta: metav1.ObjectMeta{
									Name: "default",
								},
								Spec: capa.AWSClusterRoleIdentitySpec{},
							}
							err := fakeCtrlClient.Create(ctx, awsClusterRoleIdentity)
							Expect(err).NotTo(HaveOccurred())
						})

						When("the bootstrap secret referenced in the dataSecretName field does not exist", func() {
							It("returns an error", func() {
								Expect(reconcileErr).To(MatchError(ContainSubstring("bootstrap secret in MachinePool.spec.template.spec.bootstrap.dataSecretName is not found")))
							})
						})
						When("the bootstrap secret exists but it does not contain the 'value' key", func() {
							BeforeEach(func() {
								bootstrapSecret := &v1.Secret{
									ObjectMeta: ctrl.ObjectMeta{
										Namespace: KarpenterMachinePoolNamespace,
										Name:      DataSecretName,
									},
									Data: map[string][]byte{"not-what-we-expect": capiBootstrapSecretContent},
								}
								err := fakeCtrlClient.Create(ctx, bootstrapSecret)
								Expect(err).NotTo(HaveOccurred())
							})
							It("returns an error", func() {
								Expect(reconcileErr).To(MatchError(errors.New("error retrieving bootstrap data: secret value key is missing")))
							})
						})
						When("the bootstrap secret does exist with the right format", func() {
							BeforeEach(func() {
								bootstrapSecret := &v1.Secret{
									ObjectMeta: ctrl.ObjectMeta{
										Namespace: KarpenterMachinePoolNamespace,
										Name:      DataSecretName,
									},
									Data: map[string][]byte{"value": capiBootstrapSecretContent},
								}
								err := fakeCtrlClient.Create(ctx, bootstrapSecret)
								Expect(err).NotTo(HaveOccurred())
							})
							It("adds the finalizer to the KarpenterMachinePool", func() {
								Expect(reconcileErr).NotTo(HaveOccurred())
								updatedKarpenterMachinePool := &karpenterinfra.KarpenterMachinePool{}
								err := fakeCtrlClient.Get(ctx, types.NamespacedName{Namespace: KarpenterMachinePoolNamespace, Name: KarpenterMachinePoolName}, updatedKarpenterMachinePool)
								Expect(err).NotTo(HaveOccurred())
								Expect(updatedKarpenterMachinePool.GetFinalizers()).To(ContainElement(controllers.KarpenterFinalizer))
							})
							It("writes the user data to S3", func() {
								Expect(s3Client.PutCallCount()).To(Equal(1))
								Expect(reconcileErr).NotTo(HaveOccurred())
								_, bucket, path, data := s3Client.PutArgsForCall(0)
								Expect(bucket).To(Equal(AWSClusterBucketName))
								Expect(path).To(Equal(fmt.Sprintf("%s/%s", controllers.S3ObjectPrefix, KarpenterMachinePoolName)))
								Expect(data).To(Equal(capiBootstrapSecretContent))
							})
							It("writes annotation containing bootstrap data hash", func() {
								Expect(reconcileErr).NotTo(HaveOccurred())
								updatedKarpenterMachinePool := &karpenterinfra.KarpenterMachinePool{}
								err := fakeCtrlClient.Get(ctx, types.NamespacedName{Namespace: KarpenterMachinePoolNamespace, Name: KarpenterMachinePoolName}, updatedKarpenterMachinePool)
								Expect(err).NotTo(HaveOccurred())
								Expect(updatedKarpenterMachinePool.Annotations).To(HaveKeyWithValue(controllers.BootstrapDataHashAnnotation, Equal(capiBootstrapSecretHash)))
							})
							When("there are no NodeClaim in the workload cluster yet", func() {
								It("requeues to try again soon", func() {
									Expect(reconcileErr).NotTo(HaveOccurred())
									Expect(reconcileResult.RequeueAfter).To(Equal(1 * time.Minute))
									updatedKarpenterMachinePool := &karpenterinfra.KarpenterMachinePool{}
									err := fakeCtrlClient.Get(ctx, types.NamespacedName{Namespace: KarpenterMachinePoolNamespace, Name: KarpenterMachinePoolName}, updatedKarpenterMachinePool)
									Expect(err).NotTo(HaveOccurred())
									Expect(updatedKarpenterMachinePool.Status.Ready).To(BeFalse())
									Expect(updatedKarpenterMachinePool.Status.Replicas).To(BeZero())
									Expect(updatedKarpenterMachinePool.Spec.ProviderIDList).To(BeEmpty())
								})
							})
							When("there are NodeClaim resources in the workload cluster", func() {
								BeforeEach(func() {
									nodeClaim1 := &unstructured.Unstructured{}
									nodeClaim1.Object = map[string]interface{}{
										"metadata": map[string]interface{}{
											"name": fmt.Sprintf("%s-z9y8x", KarpenterMachinePoolName),
										},
										"spec": map[string]interface{}{},
										"status": map[string]interface{}{
											"providerID": "aws:///us-west-2a/i-1234567890abcdef0",
										},
									}
									nodeClaim1.SetGroupVersionKind(schema.GroupVersionKind{
										Group:   "karpenter.sh",
										Kind:    "NodeClaim",
										Version: "v1",
									})
									err := fakeCtrlClient.Create(ctx, nodeClaim1)
									Expect(err).NotTo(HaveOccurred())

									nodeClaim2 := &unstructured.Unstructured{}
									nodeClaim2.Object = map[string]interface{}{
										"metadata": map[string]interface{}{
											"name": fmt.Sprintf("%s-m0n1o", KarpenterMachinePoolName),
										},
										"spec": map[string]interface{}{},
										"status": map[string]interface{}{
											"providerID": "aws:///us-west-2a/i-09876543219fedcba",
										},
									}
									nodeClaim2.SetGroupVersionKind(schema.GroupVersionKind{
										Group:   "karpenter.sh",
										Kind:    "NodeClaim",
										Version: "v1",
									})
									err = fakeCtrlClient.Create(ctx, nodeClaim2)
									Expect(err).NotTo(HaveOccurred())
								})
								It("updates the KarpenterMachinePool spec and status accordingly", func() {
									Expect(reconcileErr).NotTo(HaveOccurred())
									updatedKarpenterMachinePool := &karpenterinfra.KarpenterMachinePool{}
									err := fakeCtrlClient.Get(ctx, types.NamespacedName{Namespace: KarpenterMachinePoolNamespace, Name: KarpenterMachinePoolName}, updatedKarpenterMachinePool)
									Expect(err).NotTo(HaveOccurred())
									Expect(updatedKarpenterMachinePool.Status.Ready).To(BeTrue())
									Expect(updatedKarpenterMachinePool.Status.Replicas).To(Equal(int32(2)))
									Expect(updatedKarpenterMachinePool.Spec.ProviderIDList).To(ContainElements("aws:///us-west-2a/i-1234567890abcdef0", "aws:///us-west-2a/i-09876543219fedcba"))
								})
							})
							When("the S3 API returns an error", func() {
								s3apiError := errors.New("some-error")
								BeforeEach(func() {
									s3Client.PutReturns(s3apiError)
								})
								It("returns the error", func() {
									Expect(reconcileErr).To(MatchError(s3apiError))
								})
							})
						})
					})
				})
			})
		})
	})
	When("the KarpenterMachinePool exists with a hash annotation signaling unchanged bootstrap data", func() {
		BeforeEach(func() {
			karpenterMachinePool := &karpenterinfra.KarpenterMachinePool{
				ObjectMeta: ctrl.ObjectMeta{
					Namespace: KarpenterMachinePoolNamespace,
					Name:      KarpenterMachinePoolName,
					Annotations: map[string]string{
						controllers.BootstrapDataHashAnnotation: capiBootstrapSecretHash,
					},
					Labels: map[string]string{
						capi.ClusterNameLabel: ClusterName,
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "cluster.x-k8s.io/v1beta1",
							Kind:       "MachinePool",
							Name:       KarpenterMachinePoolName,
						},
					},
				},
			}
			err := fakeCtrlClient.Create(ctx, karpenterMachinePool)
			Expect(err).NotTo(HaveOccurred())

			dataSecretName := DataSecretName
			version := KubernetesVersion
			machinePool := &capiexp.MachinePool{
				ObjectMeta: ctrl.ObjectMeta{
					Namespace: KarpenterMachinePoolNamespace,
					Name:      KarpenterMachinePoolName,
					Labels: map[string]string{
						capi.ClusterNameLabel: ClusterName,
					},
				},
				Spec: capiexp.MachinePoolSpec{
					ClusterName: ClusterName,
					// Replicas:    nil,
					Template: capi.MachineTemplateSpec{
						ObjectMeta: capi.ObjectMeta{},
						Spec: capi.MachineSpec{
							ClusterName: ClusterName,
							Bootstrap: capi.Bootstrap{
								ConfigRef: &v1.ObjectReference{
									Kind:       "KubeadmConfig",
									Namespace:  KarpenterMachinePoolNamespace,
									Name:       fmt.Sprintf("%s-1a2b3c", KarpenterMachinePoolName),
									APIVersion: "bootstrap.cluster.x-k8s.io/v1beta1",
								},
								DataSecretName: &dataSecretName,
							},
							InfrastructureRef: v1.ObjectReference{
								Kind:       "KarpenterMachinePool",
								Namespace:  KarpenterMachinePoolNamespace,
								Name:       KarpenterMachinePoolName,
								APIVersion: "infrastructure.cluster.x-k8s.io/v1alpha1",
							},
							Version: &version,
						},
					},
				},
			}
			err = fakeCtrlClient.Create(ctx, machinePool)
			Expect(err).NotTo(HaveOccurred())

			cluster := &capi.Cluster{
				ObjectMeta: ctrl.ObjectMeta{
					Namespace: KarpenterMachinePoolNamespace,
					Name:      ClusterName,
					Labels: map[string]string{
						capi.ClusterNameLabel: ClusterName,
					},
				},
				Spec: capi.ClusterSpec{
					InfrastructureRef: &v1.ObjectReference{
						Kind:       "AWSCluster",
						Namespace:  KarpenterMachinePoolNamespace,
						Name:       ClusterName,
						APIVersion: "infrastructure.cluster.x-k8s.io/v1beta2",
					},
				},
			}
			err = fakeCtrlClient.Create(ctx, cluster)
			Expect(err).NotTo(HaveOccurred())

			clusterKubeconfigSecret := &v1.Secret{
				ObjectMeta: ctrl.ObjectMeta{
					Namespace: KarpenterMachinePoolNamespace,
					Name:      fmt.Sprintf("%s-kubeconfig", ClusterName),
				},
			}
			err = fakeCtrlClient.Create(ctx, clusterKubeconfigSecret)
			Expect(err).NotTo(HaveOccurred())

			awsCluster := &capa.AWSCluster{
				ObjectMeta: ctrl.ObjectMeta{
					Namespace: KarpenterMachinePoolNamespace,
					Name:      ClusterName,
				},
				Spec: capa.AWSClusterSpec{
					IdentityRef: &capa.AWSIdentityReference{
						Name: "default",
						Kind: capa.ClusterRoleIdentityKind,
					},
					S3Bucket: &capa.S3Bucket{Name: AWSClusterBucketName},
				},
			}
			err = fakeCtrlClient.Create(ctx, awsCluster)
			Expect(err).NotTo(HaveOccurred())

			awsClusterRoleIdentity := &capa.AWSClusterRoleIdentity{
				ObjectMeta: metav1.ObjectMeta{
					Name: "default",
				},
				Spec: capa.AWSClusterRoleIdentitySpec{},
			}
			err = fakeCtrlClient.Create(ctx, awsClusterRoleIdentity)
			Expect(err).NotTo(HaveOccurred())

			bootstrapSecret := &v1.Secret{
				ObjectMeta: ctrl.ObjectMeta{
					Namespace: KarpenterMachinePoolNamespace,
					Name:      DataSecretName,
				},
				Data: map[string][]byte{"value": capiBootstrapSecretContent},
			}
			err = fakeCtrlClient.Create(ctx, bootstrapSecret)
			Expect(err).NotTo(HaveOccurred())
		})
		It("doesn't write the user data to S3 again", func() {
			Expect(reconcileErr).NotTo(HaveOccurred())
			Expect(s3Client.PutCallCount()).To(Equal(0))
		})
	})
})
