package k8sclient_test

import (
	"context"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gstruct"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	capa "sigs.k8s.io/cluster-api-provider-aws/api/v1beta1"
	eks "sigs.k8s.io/cluster-api-provider-aws/controlplane/eks/api/v1beta1"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"

	"github.com/aws-resolver-rules-operator/controllers"
	"github.com/aws-resolver-rules-operator/pkg/k8sclient"
)

var _ = Describe("ClusterClient", func() {
	var (
		ctx context.Context

		clusterClient k8sclient.ClusterClient
	)

	BeforeEach(func() {
		ctx = context.Background()
		clusterClient = *k8sclient.NewClusterClient(k8sClient)
	})

	Describe("GetAWSCluster", func() {
		BeforeEach(func() {
			cluster := &capi.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster2",
					Namespace: namespace,
				},
			}
			Expect(k8sClient.Create(ctx, cluster)).To(Succeed())
			awsCluster := &capa.AWSCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster2",
					Namespace: namespace,
				},
			}
			Expect(k8sClient.Create(ctx, awsCluster)).To(Succeed())
		})

		It("gets the desired cluster", func() {
			actualCluster, err := clusterClient.GetAWSCluster(ctx, types.NamespacedName{
				Namespace: namespace,
				Name:      "test-cluster2",
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(actualCluster.Name).To(Equal("test-cluster2"))
			Expect(actualCluster.Namespace).To(Equal(namespace))
		})

		When("the cluster does not exist", func() {
			It("returns an error", func() {
				_, err := clusterClient.GetAWSCluster(ctx, types.NamespacedName{
					Namespace: namespace,
					Name:      "does-not-exist",
				})
				Expect(k8serrors.IsNotFound(err)).To(BeTrue())
			})
		})

		When("the context is cancelled", func() {
			BeforeEach(func() {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			})

			It("returns an error", func() {
				actualCluster, err := clusterClient.GetAWSCluster(ctx, types.NamespacedName{
					Namespace: namespace,
					Name:      "test-cluster2",
				})
				Expect(err).To(MatchError(ContainSubstring("context canceled")))
				Expect(actualCluster).To(BeNil())
			})
		})
	})

	Describe("GetAWSManagedControlPlane", func() {
		BeforeEach(func() {
			cluster := &capi.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster3",
					Namespace: namespace,
				},
			}
			Expect(k8sClient.Create(ctx, cluster)).To(Succeed())
			awsManagedControlPlane := &eks.AWSManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster3",
					Namespace: namespace,
				},
			}
			Expect(k8sClient.Create(ctx, awsManagedControlPlane)).To(Succeed())
		})

		It("gets the desired awsManagedControlPlane", func() {
			actualCluster, err := clusterClient.GetAWSManagedControlPlane(ctx, types.NamespacedName{
				Namespace: namespace,
				Name:      "test-cluster3",
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(actualCluster.Name).To(Equal("test-cluster3"))
			Expect(actualCluster.Namespace).To(Equal(namespace))
		})

		When("the awsManagedControlPlane does not exist", func() {
			It("returns an error", func() {
				_, err := clusterClient.GetAWSManagedControlPlane(ctx, types.NamespacedName{
					Namespace: namespace,
					Name:      "does-not-exist",
				})
				Expect(k8serrors.IsNotFound(err)).To(BeTrue())
			})
		})

		When("the context is cancelled", func() {
			BeforeEach(func() {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			})

			It("returns an error", func() {
				actualCluster, err := clusterClient.GetAWSManagedControlPlane(ctx, types.NamespacedName{
					Namespace: namespace,
					Name:      "test-cluster3",
				})
				Expect(err).To(MatchError(ContainSubstring("context canceled")))
				Expect(actualCluster).To(BeNil())
			})
		})
	})

	Describe("GetCluster", func() {
		BeforeEach(func() {
			cluster := &capi.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster2",
					Namespace: namespace,
				},
			}
			Expect(k8sClient.Create(ctx, cluster)).To(Succeed())
		})

		It("gets the desired cluster", func() {
			actualCluster, err := clusterClient.GetCluster(ctx, types.NamespacedName{
				Namespace: namespace,
				Name:      "test-cluster2",
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(actualCluster.Name).To(Equal("test-cluster2"))
			Expect(actualCluster.Namespace).To(Equal(namespace))
		})

		When("the cluster does not exist", func() {
			It("returns an error", func() {
				_, err := clusterClient.GetCluster(ctx, types.NamespacedName{
					Namespace: namespace,
					Name:      "does-not-exist",
				})
				Expect(k8serrors.IsNotFound(err)).To(BeTrue())
			})
		})

		When("the context is cancelled", func() {
			BeforeEach(func() {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			})

			It("returns an error", func() {
				actualCluster, err := clusterClient.GetCluster(ctx, types.NamespacedName{
					Namespace: namespace,
					Name:      "test-cluster2",
				})
				Expect(err).To(MatchError(ContainSubstring("context canceled")))
				Expect(actualCluster).To(BeNil())
			})
		})
	})

	Describe("AddAWSClusterFinalizer", func() {
		var awsCluster *capa.AWSCluster

		BeforeEach(func() {
			awsCluster = &capa.AWSCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster2",
					Namespace: namespace,
				},
			}
			Expect(k8sClient.Create(ctx, awsCluster)).To(Succeed())
		})

		It("adds the finalizer to the cluster", func() {
			err := clusterClient.AddAWSClusterFinalizer(ctx, awsCluster, controllers.ResolverRulesFinalizer)
			Expect(err).NotTo(HaveOccurred())

			actualCluster := &capa.AWSCluster{}
			err = k8sClient.Get(ctx, types.NamespacedName{Name: awsCluster.Name, Namespace: awsCluster.Namespace}, actualCluster)
			Expect(err).NotTo(HaveOccurred())

			Expect(actualCluster.Finalizers).To(ContainElement(controllers.ResolverRulesFinalizer))
		})

		When("the finalizer already exists", func() {
			BeforeEach(func() {
				err := clusterClient.AddAWSClusterFinalizer(ctx, awsCluster, controllers.ResolverRulesFinalizer)
				Expect(err).NotTo(HaveOccurred())
			})

			It("does not return an error", func() {
				err := clusterClient.AddAWSClusterFinalizer(ctx, awsCluster, controllers.ResolverRulesFinalizer)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		When("the cluster does not exist", func() {
			It("returns an error", func() {
				nonExistingAWSCluster := &capa.AWSCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "does-not-exist",
						Namespace: namespace,
					},
				}
				err := clusterClient.AddAWSClusterFinalizer(ctx, nonExistingAWSCluster, controllers.ResolverRulesFinalizer)
				Expect(k8serrors.IsNotFound(err)).To(BeTrue())
			})
		})

		When("the context is cancelled", func() {
			BeforeEach(func() {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			})

			It("returns an error", func() {
				err := clusterClient.AddAWSClusterFinalizer(ctx, awsCluster, controllers.ResolverRulesFinalizer)
				Expect(err).To(MatchError(ContainSubstring("context canceled")))
			})
		})
	})

	Describe("RemoveFinalizer", func() {
		var awsCluster *capa.AWSCluster

		BeforeEach(func() {
			awsCluster = &capa.AWSCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster2",
					Namespace: namespace,
					Finalizers: []string{
						controllers.ResolverRulesFinalizer,
					},
				},
			}
			Expect(k8sClient.Create(ctx, awsCluster)).To(Succeed())
		})

		It("removes the finalizer to the cluster", func() {
			err := clusterClient.RemoveAWSClusterFinalizer(ctx, awsCluster, controllers.ResolverRulesFinalizer)
			Expect(err).NotTo(HaveOccurred())

			actualCluster := &capa.AWSCluster{}
			err = k8sClient.Get(ctx, types.NamespacedName{Name: awsCluster.Name, Namespace: awsCluster.Namespace}, actualCluster)
			Expect(err).NotTo(HaveOccurred())

			Expect(actualCluster.Finalizers).NotTo(ContainElement(controllers.ResolverRulesFinalizer))
		})

		When("the finalizer doesn't exists", func() {
			BeforeEach(func() {
				err := clusterClient.RemoveAWSClusterFinalizer(ctx, awsCluster, controllers.ResolverRulesFinalizer)
				Expect(err).NotTo(HaveOccurred())
			})

			It("does not return an error", func() {
				err := clusterClient.RemoveAWSClusterFinalizer(ctx, awsCluster, controllers.ResolverRulesFinalizer)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		When("the cluster does not exist", func() {
			It("returns an error", func() {
				awsCluster = &capa.AWSCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "does-not-exist",
						Namespace: namespace,
					},
				}
				err := clusterClient.RemoveAWSClusterFinalizer(ctx, awsCluster, controllers.ResolverRulesFinalizer)
				Expect(k8serrors.IsNotFound(err)).To(BeTrue())
			})
		})

		When("the context is cancelled", func() {
			BeforeEach(func() {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			})

			It("returns an error", func() {
				err := clusterClient.RemoveAWSClusterFinalizer(ctx, awsCluster, controllers.ResolverRulesFinalizer)
				Expect(err).To(MatchError(ContainSubstring("context canceled")))
			})
		})
	})

	Describe("GetIdentity", func() {
		var identityRef *capa.AWSIdentityReference
		var clusterRoleIdentity *capa.AWSClusterRoleIdentity

		When("the identity is set", func() {
			BeforeEach(func() {
				identityRef = &capa.AWSIdentityReference{Name: "default2", Kind: capa.ClusterRoleIdentityKind}
				clusterRoleIdentity = &capa.AWSClusterRoleIdentity{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "default2",
						Namespace: namespace,
					},
					Spec: capa.AWSClusterRoleIdentitySpec{},
				}
				Expect(k8sClient.Create(ctx, clusterRoleIdentity)).To(Succeed())
			})

			It("gets the identity used for this cluster", func() {
				actualIdentity, err := clusterClient.GetIdentity(ctx, identityRef)
				Expect(err).NotTo(HaveOccurred())

				Expect(actualIdentity).To(Equal(clusterRoleIdentity))
			})
		})

		When("the identity is not set", func() {
			BeforeEach(func() {
				identityRef = nil
			})

			It("returns nil", func() {
				actualIdentity, err := clusterClient.GetIdentity(ctx, identityRef)
				Expect(err).NotTo(HaveOccurred())
				Expect(actualIdentity).To(BeNil())
			})
		})
	})

	Describe("MarkConditionTrue", func() {
		var cluster *capi.Cluster

		BeforeEach(func() {
			cluster = &capi.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster2",
					Namespace: namespace,
				},
			}
			Expect(k8sClient.Create(ctx, cluster)).To(Succeed())
		})

		It("marks the condition as true", func() {
			err := clusterClient.MarkConditionTrue(ctx, cluster, controllers.ResolverRulesAssociatedCondition)
			Expect(err).NotTo(HaveOccurred())

			actualCluster := &capi.Cluster{}
			err = k8sClient.Get(ctx, types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}, actualCluster)
			Expect(err).NotTo(HaveOccurred())

			Expect(actualCluster.Status.Conditions).To(ContainElement(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
				"Type":   Equal(controllers.ResolverRulesAssociatedCondition),
				"Status": Equal(v1.ConditionTrue),
			})))
		})
	})

	Describe("Unpause", func() {
		var awsCluster *capa.AWSCluster
		var cluster *capi.Cluster

		BeforeEach(func() {
			clusterUUID := types.UID(uuid.NewString())
			cluster = &capi.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster2",
					Namespace: namespace,
					UID:       clusterUUID,
				},
				Spec: capi.ClusterSpec{
					Paused: true,
				},
			}
			Expect(k8sClient.Create(ctx, cluster)).To(Succeed())

			awsCluster = &capa.AWSCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-cluster2",
					Namespace:   namespace,
					Annotations: map[string]string{capi.PausedAnnotation: "true"},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: capi.GroupVersion.String(),
							Kind:       "Cluster",
							Name:       "test-cluster2",
							UID:        clusterUUID,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, awsCluster)).To(Succeed())
		})

		It("removes the pause annotation", func() {
			err := clusterClient.Unpause(ctx, awsCluster, cluster)
			Expect(err).NotTo(HaveOccurred())

			actualCluster := &capi.Cluster{}
			err = k8sClient.Get(ctx, types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}, actualCluster)
			Expect(err).NotTo(HaveOccurred())
			Expect(actualCluster.Spec.Paused).To(BeFalse())

			actualAwsCluster := &capa.AWSCluster{}
			err = k8sClient.Get(ctx, types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}, actualAwsCluster)
			Expect(err).NotTo(HaveOccurred())
			Expect(capi.PausedAnnotation).ShouldNot(BeKeyOf(actualAwsCluster.Annotations))
		})
	})

	Describe("GetBastionMachine", func() {
		var awsCluster *capa.AWSCluster
		var cluster *capi.Cluster

		BeforeEach(func() {
			clusterUUID := types.UID(uuid.NewString())
			cluster = &capi.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster2",
					Namespace: namespace,
					UID:       clusterUUID,
				},
				Spec: capi.ClusterSpec{
					Paused: true,
				},
			}
			Expect(k8sClient.Create(ctx, cluster)).To(Succeed())

			awsCluster = &capa.AWSCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-cluster2",
					Namespace:   namespace,
					Annotations: map[string]string{capi.PausedAnnotation: "true"},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: capi.GroupVersion.String(),
							Kind:       "Cluster",
							Name:       "test-cluster2",
							UID:        clusterUUID,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, awsCluster)).To(Succeed())
		})

		When("there is no bastion machine", func() {
			expectedErr := &k8sclient.BastionNotFoundError{}
			It("returns empty string", func() {
				_, err := clusterClient.GetBastionMachine(ctx, cluster.Name)
				Expect(err).To(HaveOccurred())
				Expect(err).Should(MatchError(expectedErr))
			})
		})

		When("there is bastion machine", func() {
			var bastionMachine *capi.Machine

			BeforeEach(func() {
				bastionMachine = &capi.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-cluster2",
						Namespace: namespace,
						Labels: map[string]string{
							capi.ClusterLabelName:   "test-cluster2",
							"cluster.x-k8s.io/role": "bastion",
						},
					},
					Spec: capi.MachineSpec{
						ClusterName: "test-cluster2",
					},
				}
				Expect(k8sClient.Create(ctx, bastionMachine)).To(Succeed())
			})

			AfterEach(func() {
				Expect(k8sClient.Delete(ctx, bastionMachine))
			})
		})
	})
})
