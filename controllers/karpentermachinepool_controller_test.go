package controllers_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gstruct"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
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

const (
	AMIName                       = "flatcar-stable-4152.2.3-kube-1.29.1-tooling-1.26.0-gs"
	AMIOwner                      = "1234567890"
	AWSRegion                     = "eu-west-1"
	ClusterName                   = "foo"
	AWSClusterBucketName          = "my-awesome-bucket"
	DataSecretName                = "foo-mp-12345"
	KarpenterMachinePoolName      = "foo"
	KarpenterNodesInstanceProfile = "karpenter-iam-role"
	KubernetesVersion             = "v1.29.1"
)

// findCondition returns the condition with the given type from the list of conditions.
func findCondition(conditions capi.Conditions, conditionType string) *capi.Condition {
	for i := range conditions {
		if conditions[i].Type == capi.ConditionType(conditionType) {
			return &conditions[i]
		}
	}
	return nil
}

var _ = Describe("KarpenterMachinePool reconciler", func() {
	var (
		capiBootstrapSecretContent []byte
		capiBootstrapSecretHash    string
		dataSecretName             string
		s3Client                   *resolverfakes.FakeS3Client
		ec2Client                  *resolverfakes.FakeEC2Client
		ctx                        context.Context
		instanceProfile            = KarpenterNodesInstanceProfile
		reconciler                 *controllers.KarpenterMachinePoolReconciler
		reconcileErr               error
		reconcileResult            reconcile.Result
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
				err = unstructured.SetNestedField(kubeadmControlPlane.Object, map[string]interface{}{"version": KubernetesVersion}, "status")
				Expect(err).NotTo(HaveOccurred())
				err = k8sClient.Status().Update(ctx, kubeadmControlPlane)
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
						EC2NodeClass: &karpenterinfra.EC2NodeClassSpec{
							AMISelectorTerms: []karpenterinfra.AMISelectorTerm{
								{
									Name:  AMIName,
									Owner: AMIOwner,
								},
							},
							SecurityGroupSelectorTerms: []karpenterinfra.SecurityGroupSelectorTerm{
								{
									Tags: map[string]string{"my-target-sg": "is-this"},
								},
							},
							SubnetSelectorTerms: []karpenterinfra.SubnetSelectorTerm{
								{
									Tags: map[string]string{"my-target-subnet": "is-that"},
								},
							},
						},
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

				terminationGracePeriod := metav1.Duration{Duration: 30 * time.Second}
				weight := int32(1)
				deviceName := "/dev/xvda"
				volumeSize := resource.MustParse("8Gi")
				volumeTypeGp3 := "gp3"
				deleteOnTerminationTrue := true
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
							AMISelectorTerms: []karpenterinfra.AMISelectorTerm{
								{
									Name:  AMIName,
									Owner: AMIOwner,
								},
							},
							BlockDeviceMappings: []*karpenterinfra.BlockDeviceMapping{
								{
									DeviceName: &deviceName,
									EBS: &karpenterinfra.BlockDevice{
										DeleteOnTermination: &deleteOnTerminationTrue,
										VolumeSize:          &volumeSize,
										VolumeType:          &volumeTypeGp3,
									},
									RootVolume: true,
								},
							},
							InstanceProfile: &instanceProfile,
							SecurityGroupSelectorTerms: []karpenterinfra.SecurityGroupSelectorTerm{
								{
									Tags: map[string]string{"my-target-sg": "is-this"},
								},
							},
							SubnetSelectorTerms: []karpenterinfra.SubnetSelectorTerm{
								{
									Tags: map[string]string{"my-target-subnet": "is-that"},
								},
							},
							Tags: map[string]string{
								"one-tag": "only-for-karpenter",
							},
						},
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
									ExpireAfter:            karpenterinfra.MustParseNillableDuration("24h"),
									TerminationGracePeriod: &terminationGracePeriod,
								},
							},
							Disruption: karpenterinfra.Disruption{
								ConsolidateAfter:    karpenterinfra.MustParseNillableDuration("30s"),
								ConsolidationPolicy: karpenterinfra.ConsolidationPolicyWhenEmptyOrUnderutilized,
							},
							Limits: map[v1.ResourceName]resource.Quantity{
								v1.ResourceCPU:    resource.MustParse("1000m"),
								v1.ResourceMemory: resource.MustParse("1000Mi"),
							},
							Weight: &weight,
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
					err = unstructured.SetNestedField(kubeadmControlPlane.Object, map[string]interface{}{"version": KubernetesVersion}, "status")
					Expect(err).NotTo(HaveOccurred())
					err = k8sClient.Status().Update(ctx, kubeadmControlPlane)
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
					err = unstructured.SetNestedField(kubeadmControlPlane.Object, map[string]interface{}{"version": KubernetesVersion}, "status")
					Expect(err).NotTo(HaveOccurred())
					err = k8sClient.Status().Update(ctx, kubeadmControlPlane)
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
									AdditionalTags: map[string]string{
										"additional-tag-for-all-resources": "custom-tag",
									},
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
								Expect(reconcileErr).To(MatchError(ContainSubstring("failed to get bootstrap secret in MachinePool.spec.template.spec.bootstrap.dataSecretName")))
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
							It("creates karpenter EC2NodeClass object in workload cluster", func() {
								Expect(reconcileErr).NotTo(HaveOccurred())

								ec2nodeclassList := &unstructured.UnstructuredList{}
								ec2nodeclassList.SetGroupVersionKind(schema.GroupVersionKind{
									Group:   controllers.EC2NodeClassAPIGroup,
									Kind:    "EC2NodeClassList",
									Version: "v1",
								})

								err := k8sClient.List(ctx, ec2nodeclassList)
								Expect(err).NotTo(HaveOccurred())
								Expect(ec2nodeclassList.Items).To(HaveLen(1))
								Expect(ec2nodeclassList.Items[0].GetName()).To(Equal(KarpenterMachinePoolName))

								ExpectUnstructured(ec2nodeclassList.Items[0], "spec", "userData").To(Equal(fmt.Sprintf("{\"ignition\":{\"config\":{\"merge\":[{\"source\":\"s3://%s/karpenter-machine-pool/%s\",\"verification\":{}}],\"replace\":{\"verification\":{}}},\"proxy\":{},\"security\":{\"tls\":{}},\"timeouts\":{},\"version\":\"3.4.0\"},\"kernelArguments\":{},\"passwd\":{},\"storage\":{},\"systemd\":{}}", AWSClusterBucketName, KarpenterMachinePoolName)))
								ExpectUnstructured(ec2nodeclassList.Items[0], "spec", "instanceProfile").To(Equal(KarpenterNodesInstanceProfile))
								ExpectUnstructured(ec2nodeclassList.Items[0], "spec", "tags").
									To(HaveKeyWithValue("additional-tag-for-all-resources", "custom-tag"))
								ExpectUnstructured(ec2nodeclassList.Items[0], "spec", "tags").
									To(HaveKeyWithValue("one-tag", "only-for-karpenter"))

								ExpectUnstructured(ec2nodeclassList.Items[0], "spec", "blockDeviceMappings").To(HaveLen(1))
								ExpectUnstructured(ec2nodeclassList.Items[0], "spec", "blockDeviceMappings").To(
									ContainElement( // slice matcher: at least one element matches
										gstruct.MatchAllKeys(gstruct.Keys{ // map matcher: all these keys must match exactly
											"deviceName": Equal("/dev/xvda"),
											"rootVolume": BeTrue(),
											"ebs": gstruct.MatchAllKeys(gstruct.Keys{
												"deleteOnTermination": BeTrue(),
												"volumeSize":          Equal("8Gi"),
												"volumeType":          Equal("gp3"),
											}),
										}),
									),
								)

								ExpectUnstructured(ec2nodeclassList.Items[0], "spec", "amiSelectorTerms").To(HaveLen(1))
								ExpectUnstructured(ec2nodeclassList.Items[0], "spec", "amiSelectorTerms").To(
									ContainElement( // slice matcher: at least one element matches
										gstruct.MatchAllKeys(gstruct.Keys{ // map matcher: all these keys must match exactly
											"name":  Equal(AMIName),
											"owner": Equal(AMIOwner),
										}),
									),
								)

								ExpectUnstructured(ec2nodeclassList.Items[0], "spec", "securityGroupSelectorTerms").To(HaveLen(1))
								ExpectUnstructured(ec2nodeclassList.Items[0], "spec", "securityGroupSelectorTerms").To(
									ConsistOf(
										gstruct.MatchAllKeys(gstruct.Keys{
											// the top-level map has a single "tags" field,
											// whose value itself must be a map containing our SG name → value
											"tags": gstruct.MatchAllKeys(gstruct.Keys{
												"my-target-sg": Equal("is-this"),
											}),
										}),
									),
								)

								ExpectUnstructured(ec2nodeclassList.Items[0], "spec", "subnetSelectorTerms").To(HaveLen(1))
								ExpectUnstructured(ec2nodeclassList.Items[0], "spec", "subnetSelectorTerms").To(
									ConsistOf(
										gstruct.MatchAllKeys(gstruct.Keys{
											"tags": gstruct.MatchAllKeys(gstruct.Keys{
												"my-target-subnet": Equal("is-that"),
											}),
										}),
									),
								)
							})
							It("creates karpenter NodePool object in workload cluster", func() {
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

								ExpectUnstructured(nodepoolList.Items[0], "spec", "disruption", "consolidateAfter").To(Equal("30s"))
								ExpectUnstructured(nodepoolList.Items[0], "spec", "disruption", "consolidationPolicy").To(BeEquivalentTo(karpenterinfra.ConsolidationPolicyWhenEmptyOrUnderutilized))
								ExpectUnstructured(nodepoolList.Items[0], "spec", "limits").To(HaveKeyWithValue("cpu", "1"))
								ExpectUnstructured(nodepoolList.Items[0], "spec", "limits").To(HaveKeyWithValue("memory", "1000Mi"))
								ExpectUnstructured(nodepoolList.Items[0], "spec", "weight").To(BeEquivalentTo(int64(1)))
								ExpectUnstructured(nodepoolList.Items[0], "spec", "template", "spec", "expireAfter").To(BeEquivalentTo("24h"))
								ExpectUnstructured(nodepoolList.Items[0], "spec", "template", "spec", "terminationGracePeriod").To(BeEquivalentTo("30s"))
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
									Expect(err).NotTo(HaveOccurred())
								})
								It("updates the KarpenterMachinePool spec and status accordingly", func() {
									Expect(reconcileErr).NotTo(HaveOccurred())
									updatedKarpenterMachinePool := &karpenterinfra.KarpenterMachinePool{}
									err := k8sClient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: KarpenterMachinePoolName}, updatedKarpenterMachinePool)
									Expect(err).NotTo(HaveOccurred())

									// Check that the Ready condition is True
									readyCondition := findCondition(updatedKarpenterMachinePool.Status.Conditions, "Ready")
									Expect(readyCondition).NotTo(BeNil())
									Expect(string(readyCondition.Status)).To(Equal("True"))

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
			kubernetesVersion := KubernetesVersion
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
							Version: &kubernetesVersion,
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
						AMISelectorTerms: []karpenterinfra.AMISelectorTerm{
							{
								Name:  AMIName,
								Owner: AMIOwner,
							},
						},
						InstanceProfile: &instanceProfile,
						SecurityGroupSelectorTerms: []karpenterinfra.SecurityGroupSelectorTerm{
							{
								Tags: map[string]string{"my-target-sg": "is-this"},
							},
						},
						SubnetSelectorTerms: []karpenterinfra.SubnetSelectorTerm{
							{
								Tags: map[string]string{"my-target-subnet": "is-that"},
							},
						},
					},
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
					"version": KubernetesVersion,
				},
			}
			kubeadmControlPlane.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "controlplane.cluster.x-k8s.io",
				Kind:    "KubeadmControlPlane",
				Version: "v1beta1",
			})
			err = k8sClient.Create(ctx, kubeadmControlPlane)
			Expect(err).NotTo(HaveOccurred())
			err = unstructured.SetNestedField(kubeadmControlPlane.Object, map[string]interface{}{"version": KubernetesVersion}, "status")
			Expect(err).NotTo(HaveOccurred())
			err = k8sClient.Status().Update(ctx, kubeadmControlPlane)
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

	When("version skew validation fails (node pool version newer than control plane)", func() {
		var controlPlaneVersion = "v1.29.0"
		var nodePoolVersion = "v1.30.0" // Newer than control plane - should violate version skew policy

		BeforeEach(func() {
			dataSecretName = DataSecretName
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
							Version: &nodePoolVersion, // Newer version than control plane
						},
					},
				},
			}
			err := k8sClient.Create(ctx, machinePool)
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
			err = k8sClient.Create(ctx, cluster)
			Expect(err).NotTo(HaveOccurred())

			// Create control plane with OLDER version than node pool
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
					"version": "v1.21.2", // Irrelevant for this test
				},
			}
			kubeadmControlPlane.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "controlplane.cluster.x-k8s.io",
				Kind:    "KubeadmControlPlane",
				Version: "v1beta1",
			})
			err = k8sClient.Create(ctx, kubeadmControlPlane)
			Expect(err).NotTo(HaveOccurred())

			// Set control plane status with OLDER version than node pool
			err = unstructured.SetNestedField(kubeadmControlPlane.Object, map[string]interface{}{"version": controlPlaneVersion}, "status")
			Expect(err).NotTo(HaveOccurred())
			err = k8sClient.Status().Update(ctx, kubeadmControlPlane)
			Expect(err).NotTo(HaveOccurred())

			awsCluster := &capa.AWSCluster{
				ObjectMeta: ctrl.ObjectMeta{
					Namespace: namespace,
					Name:      ClusterName,
					Labels: map[string]string{
						capi.ClusterNameLabel: ClusterName,
					},
				},
				Spec: capa.AWSClusterSpec{
					Region: AWSRegion,
					S3Bucket: &capa.S3Bucket{
						Name: AWSClusterBucketName,
					},
					IdentityRef: &capa.AWSIdentityReference{
						Kind: "AWSClusterRoleIdentity",
						Name: "aws-cluster-role-identity",
					},
					AdditionalTags: map[string]string{
						"additional-tag-for-all-resources": "custom-tag",
					},
				},
			}
			err = k8sClient.Create(ctx, awsCluster)
			Expect(err).NotTo(HaveOccurred())

			awsClusterRoleIdentity := &capa.AWSClusterRoleIdentity{
				ObjectMeta: ctrl.ObjectMeta{
					Name: "aws-cluster-role-identity",
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

			// Create bootstrap secret
			bootstrapSecret := &v1.Secret{
				ObjectMeta: ctrl.ObjectMeta{
					Namespace: namespace,
					Name:      DataSecretName,
				},
				Data: map[string][]byte{"value": capiBootstrapSecretContent},
			}
			err = k8sClient.Create(ctx, bootstrapSecret)
			Expect(err).NotTo(HaveOccurred())

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
				Spec: karpenterinfra.KarpenterMachinePoolSpec{},
			}
			err = k8sClient.Create(ctx, karpenterMachinePool)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns a version skew error", func() {
			Expect(reconcileErr).To(MatchError(ContainSubstring("version skew policy violation")))
			Expect(reconcileErr).To(MatchError(ContainSubstring("control plane version v1.29.0 is older than node pool version v1.30.0")))
		})

		It("persists the version skew conditions to the Kubernetes API", func() {
			// This test verifies that conditions ARE saved even when errors occur
			updatedKarpenterMachinePool := &karpenterinfra.KarpenterMachinePool{}
			err := k8sClient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: KarpenterMachinePoolName}, updatedKarpenterMachinePool)
			Expect(err).NotTo(HaveOccurred())

			// Verify that version skew condition was persisted with the correct state
			versionSkewCondition := findCondition(updatedKarpenterMachinePool.Status.Conditions, "VersionSkewValid")
			Expect(versionSkewCondition).NotTo(BeNil())
			Expect(string(versionSkewCondition.Status)).To(Equal("False"))
			Expect(versionSkewCondition.Reason).To(Equal("VersionSkewBlocked"))
			Expect(versionSkewCondition.Message).To(ContainSubstring("control plane version v1.29.0 is older than node pool version v1.30.0"))

			// Verify that EC2NodeClass condition was persisted with error state
			ec2NodeClassCondition := findCondition(updatedKarpenterMachinePool.Status.Conditions, "EC2NodeClassCreated")
			Expect(ec2NodeClassCondition).NotTo(BeNil())
			Expect(string(ec2NodeClassCondition.Status)).To(Equal("False"))
			Expect(ec2NodeClassCondition.Reason).To(Equal("VersionSkewBlocked"))

			// Verify that NodePool condition was persisted with error state
			nodePoolCondition := findCondition(updatedKarpenterMachinePool.Status.Conditions, "NodePoolCreated")
			Expect(nodePoolCondition).NotTo(BeNil())
			Expect(string(nodePoolCondition.Status)).To(Equal("False"))
			Expect(nodePoolCondition.Reason).To(Equal("VersionSkewBlocked"))
		})
	})
})

// ExpectUnstructured digs into u.Object at the given path, asserts that it was found and error‐free, and returns
// a GomegaAssertion on the raw interface{} value.
func ExpectUnstructured(u unstructured.Unstructured, fields ...string) Assertion {
	v, found, err := unstructured.NestedFieldNoCopy(u.Object, fields...)
	Expect(found).To(BeTrue(), "expected to find field %v", fields)
	Expect(err).NotTo(HaveOccurred(), "error retrieving %v: %v", fields, err)
	return Expect(v)
}
