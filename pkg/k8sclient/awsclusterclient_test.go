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
	capi "sigs.k8s.io/cluster-api/api/v1beta1"

	"github.com/aws-resolver-rules-operator/controllers"
	"github.com/aws-resolver-rules-operator/pkg/k8sclient"
)

var _ = Describe("AWSClusterClient", func() {
	var (
		ctx context.Context

		awsClusterClient k8sclient.AWSClusterClient
	)

	BeforeEach(func() {
		ctx = context.Background()
		awsClusterClient = *k8sclient.NewAWSClusterClient(k8sClient)
	})

	Describe("Get", func() {
		BeforeEach(func() {
			awsCluster := &capa.AWSCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: namespace,
				},
			}
			Expect(k8sClient.Create(ctx, awsCluster)).To(Succeed())
		})

		It("gets the desired cluster", func() {
			actualCluster, err := awsClusterClient.Get(ctx, types.NamespacedName{
				Namespace: namespace,
				Name:      "test-cluster",
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(actualCluster.Name).To(Equal("test-cluster"))
			Expect(actualCluster.Namespace).To(Equal(namespace))
		})

		When("the cluster does not exist", func() {
			It("returns an error", func() {
				_, err := awsClusterClient.Get(ctx, types.NamespacedName{
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
				actualCluster, err := awsClusterClient.Get(ctx, types.NamespacedName{
					Namespace: namespace,
					Name:      "test-cluster",
				})
				Expect(err).To(MatchError(ContainSubstring("context canceled")))
				Expect(actualCluster).To(BeNil())
			})
		})
	})

	Describe("GetOwner", func() {
		var awsCluster *capa.AWSCluster

		BeforeEach(func() {
			clusterUUID := types.UID(uuid.NewString())
			cluster := &capi.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: namespace,
					UID:       clusterUUID,
				},
			}
			Expect(k8sClient.Create(ctx, cluster)).To(Succeed())

			awsCluster = &capa.AWSCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: namespace,
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: capi.GroupVersion.String(),
							Kind:       "Cluster",
							Name:       "test-cluster",
							UID:        clusterUUID,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, awsCluster)).To(Succeed())
		})

		It("returns the owner cluster", func() {
			actualCluster, err := awsClusterClient.GetOwner(ctx, awsCluster)
			Expect(err).NotTo(HaveOccurred())
			Expect(actualCluster).ToNot(BeNil())
			Expect(actualCluster.Name).To(Equal("test-cluster"))
			Expect(actualCluster.Namespace).To(Equal(namespace))
		})

		When("the AWSCluster does not have an owner reference", func() {
			BeforeEach(func() {
				awsCluster = &capa.AWSCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "no-owner",
						Namespace: namespace,
					},
				}
				Expect(k8sClient.Create(ctx, awsCluster)).To(Succeed())
			})

			It("returns a nil owner and no error", func() {
				actualCluster, err := awsClusterClient.GetOwner(ctx, awsCluster)
				Expect(err).NotTo(HaveOccurred())
				Expect(actualCluster).To(BeNil())
			})
		})

		When("the owner cluster does not exist", func() {
			BeforeEach(func() {
				awsCluster = &capa.AWSCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "owner-does-not-exist",
						Namespace: namespace,
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: capi.GroupVersion.String(),
								Kind:       "Cluster",
								Name:       "does-not-exist",
								UID:        types.UID(uuid.NewString()),
							},
						},
					},
				}
				Expect(k8sClient.Create(ctx, awsCluster)).To(Succeed())
			})

			It("returns a nil owner and no error", func() {
				actualCluster, err := awsClusterClient.GetOwner(ctx, awsCluster)
				Expect(k8serrors.IsNotFound(err)).To(BeTrue())
				Expect(actualCluster).To(BeNil())
			})
		})

		When("the context is cancelled", func() {
			BeforeEach(func() {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			})

			It("returns an error", func() {
				actualCluster, err := awsClusterClient.GetOwner(ctx, awsCluster)
				Expect(err).To(MatchError(ContainSubstring("context canceled")))
				Expect(actualCluster).To(BeNil())
			})
		})
	})

	Describe("AddFinalizer", func() {
		var awsCluster *capa.AWSCluster

		BeforeEach(func() {
			awsCluster = &capa.AWSCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: namespace,
				},
			}
			Expect(k8sClient.Create(ctx, awsCluster)).To(Succeed())
		})

		It("adds the finalizer to the cluster", func() {
			err := awsClusterClient.AddFinalizer(ctx, awsCluster, controllers.Finalizer)
			Expect(err).NotTo(HaveOccurred())

			actualCluster := &capa.AWSCluster{}
			err = k8sClient.Get(ctx, types.NamespacedName{Name: awsCluster.Name, Namespace: awsCluster.Namespace}, actualCluster)
			Expect(err).NotTo(HaveOccurred())

			Expect(actualCluster.Finalizers).To(ContainElement(controllers.Finalizer))
		})

		When("the finalizer already exists", func() {
			BeforeEach(func() {
				err := awsClusterClient.AddFinalizer(ctx, awsCluster, controllers.Finalizer)
				Expect(err).NotTo(HaveOccurred())
			})

			It("does not return an error", func() {
				err := awsClusterClient.AddFinalizer(ctx, awsCluster, controllers.Finalizer)
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
				err := awsClusterClient.AddFinalizer(ctx, awsCluster, controllers.Finalizer)
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
				err := awsClusterClient.AddFinalizer(ctx, awsCluster, controllers.Finalizer)
				Expect(err).To(MatchError(ContainSubstring("context canceled")))
			})
		})
	})

	Describe("RemoveFinalizer", func() {
		var awsCluster *capa.AWSCluster

		BeforeEach(func() {
			awsCluster = &capa.AWSCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: namespace,
					Finalizers: []string{
						controllers.Finalizer,
					},
				},
			}
			Expect(k8sClient.Create(ctx, awsCluster)).To(Succeed())
		})

		It("removes the finalizer to the cluster", func() {
			err := awsClusterClient.RemoveFinalizer(ctx, awsCluster, controllers.Finalizer)
			Expect(err).NotTo(HaveOccurred())

			actualCluster := &capa.AWSCluster{}
			err = k8sClient.Get(ctx, types.NamespacedName{Name: awsCluster.Name, Namespace: awsCluster.Namespace}, actualCluster)
			Expect(err).NotTo(HaveOccurred())

			Expect(actualCluster.Finalizers).NotTo(ContainElement(controllers.Finalizer))
		})

		When("the finalizer doesn't exists", func() {
			BeforeEach(func() {
				err := awsClusterClient.RemoveFinalizer(ctx, awsCluster, controllers.Finalizer)
				Expect(err).NotTo(HaveOccurred())
			})

			It("does not return an error", func() {
				err := awsClusterClient.RemoveFinalizer(ctx, awsCluster, controllers.Finalizer)
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
				err := awsClusterClient.RemoveFinalizer(ctx, awsCluster, controllers.Finalizer)
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
				err := awsClusterClient.RemoveFinalizer(ctx, awsCluster, controllers.Finalizer)
				Expect(err).To(MatchError(ContainSubstring("context canceled")))
			})
		})
	})

	Describe("GetIdentity", func() {
		var awsCluster *capa.AWSCluster
		var awsClusterRoleIdentity *capa.AWSClusterRoleIdentity

		When("the identity is set", func() {
			BeforeEach(func() {
				awsCluster = &capa.AWSCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-cluster",
						Namespace: namespace,
					},
					Spec: capa.AWSClusterSpec{
						IdentityRef: &capa.AWSIdentityReference{Name: "default", Kind: capa.ClusterRoleIdentityKind},
					},
				}
				Expect(k8sClient.Create(ctx, awsCluster)).To(Succeed())

				awsClusterRoleIdentity = &capa.AWSClusterRoleIdentity{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "default",
						Namespace: namespace,
					},
					Spec: capa.AWSClusterRoleIdentitySpec{},
				}
				Expect(k8sClient.Create(ctx, awsClusterRoleIdentity)).To(Succeed())
			})

			It("gets the identity used for this cluster", func() {
				actualIdentity, err := awsClusterClient.GetIdentity(ctx, awsCluster)
				Expect(err).NotTo(HaveOccurred())

				Expect(actualIdentity).To(Equal(awsClusterRoleIdentity))
			})
		})

		When("the identity is not set", func() {
			BeforeEach(func() {
				awsCluster = &capa.AWSCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-cluster",
						Namespace: namespace,
					},
					Spec: capa.AWSClusterSpec{},
				}
				Expect(k8sClient.Create(ctx, awsCluster)).To(Succeed())
			})

			It("returns nil", func() {
				actualIdentity, err := awsClusterClient.GetIdentity(ctx, awsCluster)
				Expect(err).NotTo(HaveOccurred())
				Expect(actualIdentity).To(BeNil())
			})
		})
	})

	Describe("MarkConditionTrue", func() {
		var awsCluster *capa.AWSCluster

		BeforeEach(func() {
			awsCluster = &capa.AWSCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: namespace,
				},
			}
			Expect(k8sClient.Create(ctx, awsCluster)).To(Succeed())
		})

		It("marks the condition as true", func() {
			err := awsClusterClient.MarkConditionTrue(ctx, awsCluster, controllers.ResolverRulesAssociatedCondition)
			Expect(err).NotTo(HaveOccurred())

			actualCluster := &capa.AWSCluster{}
			err = k8sClient.Get(ctx, types.NamespacedName{Name: awsCluster.Name, Namespace: awsCluster.Namespace}, actualCluster)
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
					Name:      "test-cluster",
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
					Name:        "test-cluster",
					Namespace:   namespace,
					Annotations: map[string]string{capi.PausedAnnotation: "true"},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: capi.GroupVersion.String(),
							Kind:       "Cluster",
							Name:       "test-cluster",
							UID:        clusterUUID,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, awsCluster)).To(Succeed())
		})

		It("removes the pause annotation", func() {
			err := awsClusterClient.Unpause(ctx, awsCluster, cluster)
			Expect(err).NotTo(HaveOccurred())

			actualCluster := &capi.Cluster{}
			err = k8sClient.Get(ctx, types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}, actualCluster)
			Expect(err).NotTo(HaveOccurred())
			Expect(actualCluster.Spec.Paused).To(BeFalse())

			actualAwsCluster := &capa.AWSCluster{}
			err = k8sClient.Get(ctx, types.NamespacedName{Name: awsCluster.Name, Namespace: awsCluster.Namespace}, actualAwsCluster)
			Expect(err).NotTo(HaveOccurred())
			Expect(capi.PausedAnnotation).ShouldNot(BeKeyOf(actualAwsCluster.Annotations))
		})
	})
})
