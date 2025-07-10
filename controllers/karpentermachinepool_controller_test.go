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
	capiexp "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest/komega"
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
		ctx                        context.Context
		reconciler                 *controllers.KarpenterMachinePoolReconciler
		reconcileErr               error
		reconcileResult            reconcile.Result
	)

	const (
		AMIName                       = "flatcar-stable-4152.2.3-kube-1.29.1-tooling-1.26.0-gs"
		AMIOwner                      = "1234567890"
		AWSRegion                     = "eu-west-1"
		ClusterName                   = "foo"
		AWSClusterBucketName          = "my-awesome-bucket"
		DataSecretName                = "foo-mp-12345"
		KarpenterMachinePoolName      = "foo"
		KarpenterNodesInstanceProfile = "karpenter-iam-role"
		KarpenterNodesSecurityGroup   = "sg-12345678"
		KarpenterNodesSubnets         = "subnet-12345678"
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

		// fakeCtrlClient := fake.NewClientBuilder().
		// 	WithScheme(scheme.Scheme).
		// 	// WithStatusSubresource(&karpenterinfra.KarpenterMachinePool{}).
		// 	Build()

		workloadClusterClientGetter := func(ctx context.Context, _ string, _ client.Client, _ client.ObjectKey) (client.Client, error) {
			// Return the same client that we're using for the test
			return k8sClient, nil
		}

		s3Client = new(resolverfakes.FakeS3Client)
		ec2Client = new(resolverfakes.FakeEC2Client)

		clientsFactory := &resolver.FakeClients{
			S3Client:  s3Client,
			EC2Client: ec2Client,
		}

		reconciler = controllers.NewKarpenterMachinepoolReconciler(k8sClient, workloadClusterClientGetter, clientsFactory)
	})

	JustBeforeEach(func() {
		request := ctrl.Request{
			NamespacedName: types.NamespacedName{
				Namespace: namespace,
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
					Namespace: namespace,
					Name:      KarpenterMachinePoolName,
				},
			}
			err := k8sClient.Create(ctx, karpenterMachinePool)
			Expect(err).NotTo(HaveOccurred())
		})
		It("does nothing", func() {
			Expect(reconcileErr).NotTo(HaveOccurred())
		})
	})

	When("the KarpenterMachinePool is being deleted", func() {
		BeforeEach(func() {
			dataSecretName = DataSecretName
			version := KubernetesVersion
			machinePool := &capiexp.MachinePool{
				ObjectMeta: ctrl.ObjectMeta{
					Namespace: namespace,
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
									Namespace:  namespace,
									Name:       fmt.Sprintf("%s-1a2b3c", KarpenterMachinePoolName),
									APIVersion: "bootstrap.cluster.x-k8s.io/v1beta1",
								},
								DataSecretName: &dataSecretName,
							},
							InfrastructureRef: v1.ObjectReference{
								Kind:       "KarpenterMachinePool",
								Namespace:  namespace,
								Name:       KarpenterMachinePoolName,
								APIVersion: "infrastructure.cluster.x-k8s.io/v1alpha1",
							},
							Version: &version,
						},
					},
				},
			}
			err := k8sClient.Create(ctx, machinePool)
			Expect(err).NotTo(HaveOccurred())

			Eventually(komega.Get(machinePool), time.Second*10, time.Millisecond*250).Should(Succeed())

			karpenterMachinePool := &karpenterinfra.KarpenterMachinePool{
				ObjectMeta: ctrl.ObjectMeta{
					Namespace: namespace,
					Name:      KarpenterMachinePoolName,
					Labels: map[string]string{
						capi.ClusterNameLabel: ClusterName,
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "cluster.x-k8s.io/v1beta1",
							Kind:       "MachinePool",
							Name:       KarpenterMachinePoolName,
							UID:        machinePool.GetUID(),
						},
					},
					Finalizers: []string{controllers.KarpenterFinalizer},
				},
			}
			err = k8sClient.Create(ctx, karpenterMachinePool)
			Expect(err).NotTo(HaveOccurred())

			err = k8sClient.Delete(ctx, karpenterMachinePool)
			Expect(err).NotTo(HaveOccurred())

			awsCluster := &capa.AWSCluster{
				ObjectMeta: ctrl.ObjectMeta{
					Namespace: namespace,
					Name:      ClusterName,
				},
				Spec: capa.AWSClusterSpec{
					IdentityRef: &capa.AWSIdentityReference{
						Name: "default-delete-test",
						Kind: capa.ClusterRoleIdentityKind,
					},
					S3Bucket: &capa.S3Bucket{Name: AWSClusterBucketName},
				},
			}
			err = k8sClient.Create(ctx, awsCluster)
			Expect(err).NotTo(HaveOccurred())

			clusterKubeconfigSecret := &v1.Secret{
				ObjectMeta: ctrl.ObjectMeta{
					Namespace: namespace,
					Name:      fmt.Sprintf("%s-kubeconfig", ClusterName),
				},
			}
			err = k8sClient.Create(ctx, clusterKubeconfigSecret)
			Expect(err).NotTo(HaveOccurred())

			awsClusterRoleIdentity := &capa.AWSClusterRoleIdentity{
				ObjectMeta: metav1.ObjectMeta{
					Name: "default-delete-test",
				},
				Spec: capa.AWSClusterRoleIdentitySpec{
					AWSRoleSpec: capa.AWSRoleSpec{
						RoleArn: "arn:aws:iam::123456789012:role/test-role",
					},
				},
			}
			err = k8sClient.Create(ctx, awsClusterRoleIdentity)
			Expect(err).NotTo(HaveOccurred())

			bootstrapSecret := &v1.Secret{
				ObjectMeta: ctrl.ObjectMeta{
					Namespace: namespace,
					Name:      DataSecretName,
				},
				Data: map[string][]byte{"value": capiBootstrapSecretContent},
			}
			err = k8sClient.Create(ctx, bootstrapSecret)
			Expect(err).NotTo(HaveOccurred())
		})

		When("the owner cluster is also being deleted", func() {
			BeforeEach(func() {
				kubeadmControlPlane := &unstructured.Unstructured{}
				kubeadmControlPlane.Object = map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":      ClusterName,
						"namespace": namespace,
					},
					"spec": map[string]interface{}{
						"kubeadmConfigSpec": map[string]interface{}{},
						"machineTemplate": map[string]interface{}{
							"infrastructureRef": map[string]interface{}{},
						},
						"version": "v1.21.2",
					},
				}
				kubeadmControlPlane.SetGroupVersionKind(schema.GroupVersionKind{
					Group:   "controlplane.cluster.x-k8s.io",
					Kind:    "KubeadmControlPlane",
					Version: "v1beta1",
				})
				err := k8sClient.Create(ctx, kubeadmControlPlane)
				Expect(err).NotTo(HaveOccurred())

				cluster := &capi.Cluster{
					ObjectMeta: ctrl.ObjectMeta{
						Namespace: namespace,
						Name:      ClusterName,
						Labels: map[string]string{
							capi.ClusterNameLabel: ClusterName,
						},
						Finalizers: []string{"giantswarm.io/something-to-keep-it-around-when-deleting"},
					},
					Spec: capi.ClusterSpec{
						ControlPlaneRef: &v1.ObjectReference{
							Kind:       "KubeadmControlPlane",
							Namespace:  namespace,
							Name:       ClusterName,
							APIVersion: "controlplane.cluster.x-k8s.io/v1beta1",
						},
						InfrastructureRef: &v1.ObjectReference{
							Kind:       "AWSCluster",
							Namespace:  namespace,
							Name:       ClusterName,
							APIVersion: "infrastructure.cluster.x-k8s.io/v1beta2",
						},
						Topology: nil,
					},
				}
				err = k8sClient.Create(ctx, cluster)
				Expect(err).NotTo(HaveOccurred())

				err = k8sClient.Delete(ctx, cluster)
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
					err := k8sClient.List(ctx, karpenterMachinePoolList, client.InNamespace(namespace))
					Expect(err).NotTo(HaveOccurred())
					// Finalizer should be there blocking the deletion of the CR
					Expect(karpenterMachinePoolList.Items).To(HaveLen(1))

					ec2Client.TerminateInstancesByTagReturnsOnCall(0, nil, nil)

					reconcileResult, reconcileErr = reconciler.Reconcile(ctx, ctrl.Request{
						NamespacedName: types.NamespacedName{
							Namespace: namespace,
							Name:      KarpenterMachinePoolName,
						},
					})

					karpenterMachinePoolList = &karpenterinfra.KarpenterMachinePoolList{}
					err = k8sClient.List(ctx, karpenterMachinePoolList, client.InNamespace(namespace))
					Expect(err).NotTo(HaveOccurred())
					// Finalizer should've been removed and the CR should be gone
					Expect(karpenterMachinePoolList.Items).To(HaveLen(0))
				})
			})
		})
	})

	When("the KarpenterMachinePool exists and it has a MachinePool owner", func() {
		When("the referenced MachinePool does not exist", func() {
			BeforeEach(func() {
				karpenterMachinePool := &karpenterinfra.KarpenterMachinePool{
					ObjectMeta: ctrl.ObjectMeta{
						Namespace: namespace,
						Name:      KarpenterMachinePoolName,
						Labels: map[string]string{
							capi.ClusterNameLabel: ClusterName,
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "cluster.x-k8s.io/v1beta1",
								Kind:       "MachinePool",
								Name:       KarpenterMachinePoolName,
								UID:        "12345678-1234-1234-1234-123456789012",
							},
						},
					},
					Spec: karpenterinfra.KarpenterMachinePoolSpec{
						EC2NodeClass: &karpenterinfra.EC2NodeClassSpec{},
						NodePool: &karpenterinfra.NodePoolSpec{
							Template: karpenterinfra.NodeClaimTemplate{
								Spec: karpenterinfra.NodeClaimTemplateSpec{
									Requirements: []karpenterinfra.NodeSelectorRequirementWithMinValues{
										{
											NodeSelectorRequirement: v1.NodeSelectorRequirement{
												Key:      "kubernetes.io/os",
												Operator: v1.NodeSelectorOpIn,
												Values:   []string{"linux"},
											},
										},
									},
								},
							},
							Disruption: karpenterinfra.Disruption{
								ConsolidateAfter:    karpenterinfra.MustParseNillableDuration("30s"),
								ConsolidationPolicy: karpenterinfra.ConsolidationPolicyWhenEmptyOrUnderutilized,
							},
						},
					},
				}
				err := k8sClient.Create(ctx, karpenterMachinePool)
				Expect(err).NotTo(HaveOccurred())
			})
			It("returns an error", func() {
				Expect(reconcileErr).To(MatchError(ContainSubstring("failed to get MachinePool owning the KarpenterMachinePool")))
			})
		})
		When("the referenced MachinePool exists without MachinePool.spec.template.spec.bootstrap.dataSecretName being set", func() {
			BeforeEach(func() {
				version := KubernetesVersion
				machinePool := &capiexp.MachinePool{
					ObjectMeta: ctrl.ObjectMeta{
						Namespace: namespace,
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
										Namespace:  namespace,
										Name:       fmt.Sprintf("%s-1a2b3c", KarpenterMachinePoolName),
										APIVersion: "bootstrap.cluster.x-k8s.io/v1beta1",
									},
								},
								InfrastructureRef: v1.ObjectReference{
									Kind:       "KarpenterMachinePool",
									Namespace:  namespace,
									Name:       KarpenterMachinePoolName,
									APIVersion: "infrastructure.cluster.x-k8s.io/v1alpha1",
								},
								Version: &version,
							},
						},
					},
				}
				err := k8sClient.Create(ctx, machinePool)
				Expect(err).NotTo(HaveOccurred())

				Eventually(komega.Get(machinePool), time.Second*10, time.Millisecond*250).Should(Succeed())

				karpenterMachinePool := &karpenterinfra.KarpenterMachinePool{
					ObjectMeta: ctrl.ObjectMeta{
						Namespace: namespace,
						Name:      KarpenterMachinePoolName,
						Labels: map[string]string{
							capi.ClusterNameLabel: ClusterName,
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "cluster.x-k8s.io/v1beta1",
								Kind:       "MachinePool",
								Name:       KarpenterMachinePoolName,
								UID:        machinePool.GetUID(),
							},
						},
					},
					Spec: karpenterinfra.KarpenterMachinePoolSpec{
						NodePool: &karpenterinfra.NodePoolSpec{
							Template: karpenterinfra.NodeClaimTemplate{
								Spec: karpenterinfra.NodeClaimTemplateSpec{
									Requirements: []karpenterinfra.NodeSelectorRequirementWithMinValues{
										{
											NodeSelectorRequirement: v1.NodeSelectorRequirement{
												Key:      "kubernetes.io/os",
												Operator: v1.NodeSelectorOpIn,
												Values:   []string{"linux"},
											},
										},
									},
								},
							},
							Disruption: karpenterinfra.Disruption{
								ConsolidateAfter:    karpenterinfra.MustParseNillableDuration("30s"),
								ConsolidationPolicy: karpenterinfra.ConsolidationPolicyWhenEmptyOrUnderutilized,
							},
						},
					},
				}
				err = k8sClient.Create(ctx, karpenterMachinePool)
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
						Namespace: namespace,
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
										Namespace:  namespace,
										Name:       fmt.Sprintf("%s-1a2b3c", KarpenterMachinePoolName),
										APIVersion: "bootstrap.cluster.x-k8s.io/v1beta1",
									},
									DataSecretName: &dataSecretName,
								},
								InfrastructureRef: v1.ObjectReference{
									Kind:       "KarpenterMachinePool",
									Namespace:  namespace,
									Name:       KarpenterMachinePoolName,
									APIVersion: "infrastructure.cluster.x-k8s.io/v1alpha1",
								},
								Version: &version,
							},
						},
					},
				}
				err := k8sClient.Create(ctx, machinePool)
				Expect(err).NotTo(HaveOccurred())

				Eventually(komega.Get(machinePool), time.Second*10, time.Millisecond*250).Should(Succeed())

				karpenterMachinePool := &karpenterinfra.KarpenterMachinePool{
					ObjectMeta: ctrl.ObjectMeta{
						Namespace: namespace,
						Name:      KarpenterMachinePoolName,
						Labels: map[string]string{
							capi.ClusterNameLabel: ClusterName,
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "cluster.x-k8s.io/v1beta1",
								Kind:       "MachinePool",
								Name:       KarpenterMachinePoolName,
								UID:        machinePool.GetUID(),
							},
						},
					},
					Spec: karpenterinfra.KarpenterMachinePoolSpec{
						EC2NodeClass: &karpenterinfra.EC2NodeClassSpec{
							AMIName:        AMIName,
							AMIOwner:       AMIOwner,
							SecurityGroups: map[string]string{"my-target-sg": "is-this"},
							Subnets:        map[string]string{"my-target-subnet": "is-that"},
						},
						IamInstanceProfile: KarpenterNodesInstanceProfile,
						NodePool: &karpenterinfra.NodePoolSpec{
							Template: karpenterinfra.NodeClaimTemplate{
								Spec: karpenterinfra.NodeClaimTemplateSpec{
									Requirements: []karpenterinfra.NodeSelectorRequirementWithMinValues{
										{
											NodeSelectorRequirement: v1.NodeSelectorRequirement{
												Key:      "kubernetes.io/os",
												Operator: v1.NodeSelectorOpIn,
												Values:   []string{"linux"},
											},
										},
									},
								},
							},
							Disruption: karpenterinfra.Disruption{
								ConsolidateAfter:    karpenterinfra.MustParseNillableDuration("30s"),
								ConsolidationPolicy: karpenterinfra.ConsolidationPolicyWhenEmptyOrUnderutilized,
							},
						},
					},
				}
				err = k8sClient.Create(ctx, karpenterMachinePool)
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
							Namespace: namespace,
							Name:      ClusterName,
							Labels: map[string]string{
								capi.ClusterNameLabel: ClusterName,
							},
						},
						Spec: capi.ClusterSpec{
							Paused: true,
							ControlPlaneRef: &v1.ObjectReference{
								Kind:       "KubeadmControlPlane",
								Namespace:  namespace,
								Name:       ClusterName,
								APIVersion: "controlplane.cluster.x-k8s.io/v1beta1",
							},
							InfrastructureRef: &v1.ObjectReference{
								Kind:       "AWSCluster",
								Namespace:  namespace,
								Name:       ClusterName,
								APIVersion: "infrastructure.cluster.x-k8s.io/v1beta2",
							},
						},
					}
					err := k8sClient.Create(ctx, cluster)
					Expect(err).NotTo(HaveOccurred())

					kubeadmControlPlane := &unstructured.Unstructured{}
					kubeadmControlPlane.Object = map[string]interface{}{
						"metadata": map[string]interface{}{
							"name":      ClusterName,
							"namespace": namespace,
						},
						"spec": map[string]interface{}{
							"kubeadmConfigSpec": map[string]interface{}{},
							"machineTemplate": map[string]interface{}{
								"infrastructureRef": map[string]interface{}{},
							},
							"version": "v1.21.2",
						},
					}
					kubeadmControlPlane.SetGroupVersionKind(schema.GroupVersionKind{
						Group:   "controlplane.cluster.x-k8s.io",
						Kind:    "KubeadmControlPlane",
						Version: "v1beta1",
					})
					err = k8sClient.Create(ctx, kubeadmControlPlane)
					Expect(err).NotTo(HaveOccurred())

					clusterKubeconfigSecret := &v1.Secret{
						ObjectMeta: ctrl.ObjectMeta{
							Namespace: namespace,
							Name:      fmt.Sprintf("%s-kubeconfig", ClusterName),
						},
					}
					err = k8sClient.Create(ctx, clusterKubeconfigSecret)
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
							Namespace: namespace,
							Name:      ClusterName,
							Labels: map[string]string{
								capi.ClusterNameLabel: ClusterName,
							},
						},
						Spec: capi.ClusterSpec{
							ControlPlaneRef: &v1.ObjectReference{
								Kind:       "KubeadmControlPlane",
								Namespace:  namespace,
								Name:       ClusterName,
								APIVersion: "controlplane.cluster.x-k8s.io/v1beta1",
							},
							InfrastructureRef: &v1.ObjectReference{
								Kind:       "AWSCluster",
								Namespace:  namespace,
								Name:       ClusterName,
								APIVersion: "infrastructure.cluster.x-k8s.io/v1beta2",
							},
						},
					}
					err := k8sClient.Create(ctx, cluster)
					Expect(err).NotTo(HaveOccurred())

					kubeadmControlPlane := &unstructured.Unstructured{}
					kubeadmControlPlane.Object = map[string]interface{}{
						"metadata": map[string]interface{}{
							"name":      ClusterName,
							"namespace": namespace,
						},
						"spec": map[string]interface{}{
							"kubeadmConfigSpec": map[string]interface{}{},
							"machineTemplate": map[string]interface{}{
								"infrastructureRef": map[string]interface{}{},
							},
							"version": "v1.21.2",
						},
					}
					kubeadmControlPlane.SetGroupVersionKind(schema.GroupVersionKind{
						Group:   "controlplane.cluster.x-k8s.io",
						Kind:    "KubeadmControlPlane",
						Version: "v1beta1",
					})
					err = k8sClient.Create(ctx, kubeadmControlPlane)
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
								Namespace: namespace,
								Name:      ClusterName,
								Labels: map[string]string{
									capi.ClusterNameLabel: ClusterName,
								},
							},
							Spec: capa.AWSClusterSpec{},
						}
						err := k8sClient.Create(ctx, awsCluster)
						Expect(err).NotTo(HaveOccurred())
					})
					It("returns an error", func() {
						Expect(reconcileErr).To(MatchError(errors.New("a cluster wide object storage configured at `AWSCluster.spec.s3Bucket` is required")))
					})
				})
				When("the AWSCluster exists and there is a S3 bucket defined on it", func() {
					When("it can't find the identity used by the AWSCluster", func() {
						BeforeEach(func() {
							awsCluster := &capa.AWSCluster{
								ObjectMeta: ctrl.ObjectMeta{
									Namespace: namespace,
									Name:      ClusterName,
								},
								Spec: capa.AWSClusterSpec{
									IdentityRef: &capa.AWSIdentityReference{
										Name: "not-referenced-by-test",
										Kind: capa.ClusterRoleIdentityKind,
									},
									S3Bucket: &capa.S3Bucket{Name: AWSClusterBucketName},
								},
							}
							err := k8sClient.Create(ctx, awsCluster)
							Expect(err).NotTo(HaveOccurred())
						})
						It("returns an error", func() {
							Expect(reconcileErr).To(MatchError(ContainSubstring("failed to get AWSClusterRoleIdentity referenced in AWSCluster")))
						})
					})
					When("it finds the identity used by the AWSCluster", func() {
						BeforeEach(func() {
							awsCluster := &capa.AWSCluster{
								ObjectMeta: ctrl.ObjectMeta{
									Namespace: namespace,
									Name:      ClusterName,
								},
								Spec: capa.AWSClusterSpec{
									IdentityRef: &capa.AWSIdentityReference{
										Name: "default",
										Kind: capa.ClusterRoleIdentityKind,
									},
									Region:   AWSRegion,
									S3Bucket: &capa.S3Bucket{Name: AWSClusterBucketName},
								},
							}
							err := k8sClient.Create(ctx, awsCluster)
							Expect(err).NotTo(HaveOccurred())

							awsClusterRoleIdentity := &capa.AWSClusterRoleIdentity{
								ObjectMeta: metav1.ObjectMeta{
									Name: "default",
								},
								Spec: capa.AWSClusterRoleIdentitySpec{
									AWSRoleSpec: capa.AWSRoleSpec{
										RoleArn: "arn:aws:iam::123456789012:role/test-role",
									},
								},
							}
							err = k8sClient.Create(ctx, awsClusterRoleIdentity)
							Expect(err).To(SatisfyAny(
								BeNil(),
								MatchError(ContainSubstring("already exists")),
							))
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
										Namespace: namespace,
										Name:      DataSecretName,
									},
									Data: map[string][]byte{"not-what-we-expect": capiBootstrapSecretContent},
								}
								err := k8sClient.Create(ctx, bootstrapSecret)
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
										Namespace: namespace,
										Name:      DataSecretName,
									},
									Data: map[string][]byte{"value": capiBootstrapSecretContent},
								}
								err := k8sClient.Create(ctx, bootstrapSecret)
								Expect(err).NotTo(HaveOccurred())
							})
							It("creates karpenter resources in the wc", func() {
								Expect(reconcileErr).NotTo(HaveOccurred())

								nodepoolList := &unstructured.UnstructuredList{}
								nodepoolList.SetGroupVersionKind(schema.GroupVersionKind{
									Group:   "karpenter.sh",
									Kind:    "NodePoolList",
									Version: "v1",
								})

								err := k8sClient.List(ctx, nodepoolList)
								Expect(err).NotTo(HaveOccurred())
								Expect(nodepoolList.Items).To(HaveLen(1))
								Expect(nodepoolList.Items[0].GetName()).To(Equal(KarpenterMachinePoolName))

								ec2nodeclassList := &unstructured.UnstructuredList{}
								ec2nodeclassList.SetGroupVersionKind(schema.GroupVersionKind{
									Group:   "karpenter.k8s.aws",
									Kind:    "EC2NodeClassList",
									Version: "v1",
								})

								err = k8sClient.List(ctx, ec2nodeclassList)
								Expect(err).NotTo(HaveOccurred())
								Expect(ec2nodeclassList.Items).To(HaveLen(1))
								Expect(ec2nodeclassList.Items[0].GetName()).To(Equal(KarpenterMachinePoolName))
								amiSelectorTerms, found, err := unstructured.NestedSlice(ec2nodeclassList.Items[0].Object, "spec", "amiSelectorTerms")
								Expect(err).NotTo(HaveOccurred())
								Expect(found).To(BeTrue())
								Expect(amiSelectorTerms).To(HaveLen(1))

								// Let's make sure the amiSelectorTerms field is what we expect
								term0, ok := amiSelectorTerms[0].(map[string]interface{})
								Expect(ok).To(BeTrue(), "expected amiSelectorTerms[0] to be a map")
								// Assert the name field
								nameVal, ok := term0["name"].(string)
								Expect(ok).To(BeTrue(), "expected name to be a string")
								Expect(nameVal).To(Equal(AMIName))
								// Assert the owner field
								ownerF, ok := term0["owner"].(string)
								Expect(ok).To(BeTrue(), "expected owner to be a number")
								Expect(ownerF).To(Equal(AMIOwner))

								// 	Assert security groups are the expected ones
								securityGroupSelectorTerms, found, err := unstructured.NestedSlice(ec2nodeclassList.Items[0].Object, "spec", "securityGroupSelectorTerms")
								Expect(err).NotTo(HaveOccurred())
								Expect(found).To(BeTrue())
								Expect(securityGroupSelectorTerms).To(HaveLen(1))
								// Let's make sure the securityGroupSelectorTerms field is what we expect
								securityGroupSelectorTerm0, ok := securityGroupSelectorTerms[0].(map[string]interface{})
								Expect(ok).To(BeTrue(), "expected securityGroupSelectorTerms[0] to be a map")
								// Assert the security group name field
								securityGroupTags, ok := securityGroupSelectorTerm0["tags"].(map[string]interface{})
								Expect(ok).To(BeTrue(), "expected tags to be a map[string]string")
								Expect(securityGroupTags["my-target-sg"]).To(Equal("is-this"))

								// 	Assert subnets are the expected ones
								subnetSelectorTerms, found, err := unstructured.NestedSlice(ec2nodeclassList.Items[0].Object, "spec", "subnetSelectorTerms")
								Expect(err).NotTo(HaveOccurred())
								Expect(found).To(BeTrue())
								Expect(subnetSelectorTerms).To(HaveLen(1))
								// Let's make sure the subnetSelectorTerms field is what we expect
								subnetSelectorTerm0, ok := subnetSelectorTerms[0].(map[string]interface{})
								Expect(ok).To(BeTrue(), "expected subnetSelectorTerms[0] to be a map")
								// Assert the security group name field
								subnetTags, ok := subnetSelectorTerm0["tags"].(map[string]interface{})
								Expect(ok).To(BeTrue(), "expected tags to be a map[string]string")
								Expect(subnetTags["my-target-subnet"]).To(Equal("is-that"))

								// 	Assert userdata is the expected one
								userData, found, err := unstructured.NestedString(ec2nodeclassList.Items[0].Object, "spec", "userData")
								Expect(err).NotTo(HaveOccurred())
								Expect(found).To(BeTrue())
								Expect(userData).To(Equal(fmt.Sprintf("{\"ignition\":{\"config\":{\"merge\":[{\"source\":\"s3://%s/karpenter-machine-pool/%s-%s\",\"verification\":{}}],\"replace\":{\"verification\":{}}},\"proxy\":{},\"security\":{\"tls\":{}},\"timeouts\":{},\"version\":\"3.4.0\"},\"kernelArguments\":{},\"passwd\":{},\"storage\":{},\"systemd\":{}}", AWSClusterBucketName, ClusterName, KarpenterMachinePoolName)))

								// 	Assert instance profile is the expected one
								iamInstanceProfile, found, err := unstructured.NestedString(ec2nodeclassList.Items[0].Object, "spec", "instanceProfile")
								Expect(err).NotTo(HaveOccurred())
								Expect(found).To(BeTrue())
								Expect(iamInstanceProfile).To(Equal(KarpenterNodesInstanceProfile))
							})
							It("adds the finalizer to the KarpenterMachinePool", func() {
								Expect(reconcileErr).NotTo(HaveOccurred())
								updatedKarpenterMachinePool := &karpenterinfra.KarpenterMachinePool{}
								err := k8sClient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: KarpenterMachinePoolName}, updatedKarpenterMachinePool)
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
								err := k8sClient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: KarpenterMachinePoolName}, updatedKarpenterMachinePool)
								Expect(err).NotTo(HaveOccurred())
								Expect(updatedKarpenterMachinePool.Annotations).To(HaveKeyWithValue(controllers.BootstrapDataHashAnnotation, Equal(capiBootstrapSecretHash)))
							})
							When("there are no NodeClaim in the workload cluster yet", func() {
								It("requeues to try again soon", func() {
									Expect(reconcileErr).NotTo(HaveOccurred())
									Expect(reconcileResult.RequeueAfter).To(Equal(1 * time.Minute))
									updatedKarpenterMachinePool := &karpenterinfra.KarpenterMachinePool{}
									err := k8sClient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: KarpenterMachinePoolName}, updatedKarpenterMachinePool)
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
										"spec": map[string]interface{}{
											"nodeClassRef": map[string]interface{}{
												"group": "karpenter.k8s.aws",
												"kind":  "EC2NodeClass",
												"name":  "default",
											},
											"requirements": []interface{}{
												map[string]interface{}{
													"key":      "kubernetes.io/arch",
													"operator": "In",
													"values":   []string{"amd64"},
												},
											},
										},
									}
									nodeClaim1.SetGroupVersionKind(schema.GroupVersionKind{
										Group:   "karpenter.sh",
										Kind:    "NodeClaim",
										Version: "v1",
									})
									err := k8sClient.Create(ctx, nodeClaim1)
									Expect(err).NotTo(HaveOccurred())
									err = unstructured.SetNestedField(nodeClaim1.Object, map[string]interface{}{"providerID": "aws:///us-west-2a/i-1234567890abcdef0"}, "status")
									Expect(err).NotTo(HaveOccurred())
									err = k8sClient.Status().Update(ctx, nodeClaim1)

									nodeClaim2 := &unstructured.Unstructured{}
									nodeClaim2.Object = map[string]interface{}{
										"metadata": map[string]interface{}{
											"name": fmt.Sprintf("%s-m0n1o", KarpenterMachinePoolName),
										},
										"spec": map[string]interface{}{
											"nodeClassRef": map[string]interface{}{
												"group": "karpenter.k8s.aws",
												"kind":  "EC2NodeClass",
												"name":  "default",
											},
											"requirements": []interface{}{
												map[string]interface{}{
													"key":      "kubernetes.io/arch",
													"operator": "In",
													"values":   []string{"amd64"},
												},
											},
										},
									}
									nodeClaim2.SetGroupVersionKind(schema.GroupVersionKind{
										Group:   "karpenter.sh",
										Kind:    "NodeClaim",
										Version: "v1",
									})
									err = k8sClient.Create(ctx, nodeClaim2)
									Expect(err).NotTo(HaveOccurred())
									err = unstructured.SetNestedField(nodeClaim2.Object, map[string]interface{}{"providerID": "aws:///us-west-2a/i-09876543219fedcba"}, "status")
									Expect(err).NotTo(HaveOccurred())
									err = k8sClient.Status().Update(ctx, nodeClaim2)
								})
								It("updates the KarpenterMachinePool spec and status accordingly", func() {
									Expect(reconcileErr).NotTo(HaveOccurred())
									updatedKarpenterMachinePool := &karpenterinfra.KarpenterMachinePool{}
									err := k8sClient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: KarpenterMachinePoolName}, updatedKarpenterMachinePool)
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
			dataSecretName := DataSecretName
			version := KubernetesVersion
			machinePool := &capiexp.MachinePool{
				ObjectMeta: ctrl.ObjectMeta{
					Namespace: namespace,
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
									Namespace:  namespace,
									Name:       fmt.Sprintf("%s-1a2b3c", KarpenterMachinePoolName),
									APIVersion: "bootstrap.cluster.x-k8s.io/v1beta1",
								},
								DataSecretName: &dataSecretName,
							},
							InfrastructureRef: v1.ObjectReference{
								Kind:       "KarpenterMachinePool",
								Namespace:  namespace,
								Name:       KarpenterMachinePoolName,
								APIVersion: "infrastructure.cluster.x-k8s.io/v1alpha1",
							},
							Version: &version,
						},
					},
				},
			}
			err := k8sClient.Create(ctx, machinePool)
			Expect(err).NotTo(HaveOccurred())

			Eventually(komega.Get(machinePool), time.Second*10, time.Millisecond*250).Should(Succeed())

			karpenterMachinePool := &karpenterinfra.KarpenterMachinePool{
				ObjectMeta: ctrl.ObjectMeta{
					Namespace: namespace,
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
							UID:        machinePool.GetUID(),
						},
					},
				},
				Spec: karpenterinfra.KarpenterMachinePoolSpec{
					EC2NodeClass: &karpenterinfra.EC2NodeClassSpec{
						SecurityGroups: map[string]string{"my-target-sg": "is-this"},
						Subnets:        map[string]string{"my-target-subnet": "is-that"},
					},
					IamInstanceProfile: KarpenterNodesInstanceProfile,
					NodePool: &karpenterinfra.NodePoolSpec{
						Template: karpenterinfra.NodeClaimTemplate{
							Spec: karpenterinfra.NodeClaimTemplateSpec{
								Requirements: []karpenterinfra.NodeSelectorRequirementWithMinValues{
									{
										NodeSelectorRequirement: v1.NodeSelectorRequirement{
											Key:      "kubernetes.io/os",
											Operator: v1.NodeSelectorOpIn,
											Values:   []string{"linux"},
										},
									},
								},
							},
						},
						Disruption: karpenterinfra.Disruption{
							ConsolidateAfter:    karpenterinfra.MustParseNillableDuration("30s"),
							ConsolidationPolicy: karpenterinfra.ConsolidationPolicyWhenEmptyOrUnderutilized,
						},
					},
				},
			}
			err = k8sClient.Create(ctx, karpenterMachinePool)
			Expect(err).NotTo(HaveOccurred())

			cluster := &capi.Cluster{
				ObjectMeta: ctrl.ObjectMeta{
					Namespace: namespace,
					Name:      ClusterName,
					Labels: map[string]string{
						capi.ClusterNameLabel: ClusterName,
					},
				},
				Spec: capi.ClusterSpec{
					InfrastructureRef: &v1.ObjectReference{
						Kind:       "AWSCluster",
						Namespace:  namespace,
						Name:       ClusterName,
						APIVersion: "infrastructure.cluster.x-k8s.io/v1beta2",
					},
				},
			}
			err = k8sClient.Create(ctx, cluster)
			Expect(err).NotTo(HaveOccurred())

			clusterKubeconfigSecret := &v1.Secret{
				ObjectMeta: ctrl.ObjectMeta{
					Namespace: namespace,
					Name:      fmt.Sprintf("%s-kubeconfig", ClusterName),
				},
			}
			err = k8sClient.Create(ctx, clusterKubeconfigSecret)
			Expect(err).NotTo(HaveOccurred())

			awsCluster := &capa.AWSCluster{
				ObjectMeta: ctrl.ObjectMeta{
					Namespace: namespace,
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
			err = k8sClient.Create(ctx, awsCluster)
			Expect(err).NotTo(HaveOccurred())

			awsClusterRoleIdentity := &capa.AWSClusterRoleIdentity{
				ObjectMeta: metav1.ObjectMeta{
					Name: "default",
				},
				Spec: capa.AWSClusterRoleIdentitySpec{
					AWSRoleSpec: capa.AWSRoleSpec{
						RoleArn: "arn:aws:iam::123456789012:role/test-role",
					},
				},
			}
			err = k8sClient.Create(ctx, awsClusterRoleIdentity)
			Expect(err).To(SatisfyAny(
				BeNil(),
				MatchError(ContainSubstring("already exists")),
			))

			bootstrapSecret := &v1.Secret{
				ObjectMeta: ctrl.ObjectMeta{
					Namespace: namespace,
					Name:      DataSecretName,
				},
				Data: map[string][]byte{"value": capiBootstrapSecretContent},
			}
			err = k8sClient.Create(ctx, bootstrapSecret)
			Expect(err).NotTo(HaveOccurred())
		})
		It("doesn't write the user data to S3 again", func() {
			Expect(reconcileErr).NotTo(HaveOccurred())
			Expect(s3Client.PutCallCount()).To(Equal(0))
		})
	})

	Describe("Version comparison functions", func() {
		Describe("CompareKubernetesVersions", func() {
			It("should correctly compare versions", func() {
				// Test cases: (version1, version2, expected_result)
				testCases := []struct {
					v1, v2 string
					want   int
				}{
					{"v1.20.0", "v1.20.0", 0},
					{"v1.20.0", "v1.21.0", -1},
					{"v1.21.0", "v1.20.0", 1},
					{"v1.20.1", "v1.20.0", 1},
					{"v1.20.0", "v1.20.1", -1},
					{"1.20.0", "v1.20.0", 0},
					{"v1.20.0", "1.20.0", 0},
				}

				for _, tc := range testCases {
					result, err := controllers.CompareKubernetesVersions(tc.v1, tc.v2)
					Expect(err).NotTo(HaveOccurred())
					Expect(result).To(Equal(tc.want), "comparing %s with %s", tc.v1, tc.v2)
				}
			})

			It("should handle invalid version formats", func() {
				_, err := controllers.CompareKubernetesVersions("invalid", "v1.20.0")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid version format"))
			})
		})

		Describe("IsVersionSkewAllowed", func() {
			It("should allow updates when control plane is older or equal", func() {
				testCases := []struct {
					controlPlane, worker string
					allowed              bool
				}{
					{"v1.20.0", "v1.20.0", true}, // Same version
					{"v1.20.0", "v1.21.0", true}, // Worker newer
					{"v1.20.0", "v1.22.0", true}, // Worker newer
				}

				for _, tc := range testCases {
					allowed, err := controllers.IsVersionSkewAllowed(tc.controlPlane, tc.worker)
					Expect(err).NotTo(HaveOccurred())
					Expect(allowed).To(Equal(tc.allowed), "control plane %s, worker %s", tc.controlPlane, tc.worker)
				}
			})

			It("should allow updates within 2 minor versions", func() {
				testCases := []struct {
					controlPlane, worker string
					allowed              bool
				}{
					{"v1.22.0", "v1.20.0", true},  // 2 versions behind
					{"v1.22.0", "v1.21.0", true},  // 1 version behind
					{"v1.22.0", "v1.19.0", false}, // 3 versions behind
					{"v1.23.0", "v1.20.0", false}, // 3 versions behind
				}

				for _, tc := range testCases {
					allowed, err := controllers.IsVersionSkewAllowed(tc.controlPlane, tc.worker)
					Expect(err).NotTo(HaveOccurred())
					Expect(allowed).To(Equal(tc.allowed), "control plane %s, worker %s", tc.controlPlane, tc.worker)
				}
			})

			It("should handle invalid version formats", func() {
				_, err := controllers.IsVersionSkewAllowed("invalid", "v1.20.0")
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
