package k8sclient_test

import (
	"context"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	capa "sigs.k8s.io/cluster-api-provider-aws/api/v1beta1"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"

	"github.com/aws-resolver-rules-operator/controllers"
	"github.com/aws-resolver-rules-operator/pkg/k8sclient"
)

var _ = Describe("AWSCluster", func() {
	var (
		ctx context.Context

		client k8sclient.AWSCluster
	)

	BeforeEach(func() {
		ctx = context.Background()
		client = *k8sclient.NewAWSCluster(k8sClient)
	})

	Describe("Get", func() {
		BeforeEach(func() {
			awscluster := &capa.AWSCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: namespace,
				},
			}
			Expect(k8sClient.Create(ctx, awscluster)).To(Succeed())
		})

		It("gets the desired cluster", func() {
			actualCluster, err := client.Get(ctx, types.NamespacedName{
				Namespace: namespace,
				Name:      "test-cluster",
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(actualCluster.Name).To(Equal("test-cluster"))
			Expect(actualCluster.Namespace).To(Equal(namespace))
		})

		When("the cluster does not exist", func() {
			It("returns an error", func() {
				_, err := client.Get(ctx, types.NamespacedName{
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
				actualCluster, err := client.Get(ctx, types.NamespacedName{
					Namespace: namespace,
					Name:      "test-cluster",
				})
				Expect(err).To(MatchError(ContainSubstring("context canceled")))
				Expect(actualCluster).To(BeNil())
			})
		})
	})

	Describe("GetOwner", func() {
		var awscluster *capa.AWSCluster

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

			awscluster = &capa.AWSCluster{
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
			Expect(k8sClient.Create(ctx, awscluster)).To(Succeed())
		})

		It("returns the owner cluster", func() {
			actualCluster, err := client.GetOwner(ctx, awscluster)
			Expect(err).NotTo(HaveOccurred())
			Expect(actualCluster).ToNot(BeNil())
			Expect(actualCluster.Name).To(Equal("test-cluster"))
			Expect(actualCluster.Namespace).To(Equal(namespace))
		})

		When("the AWSCluster does not have an owner reference", func() {
			BeforeEach(func() {
				awscluster = &capa.AWSCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "no-owner",
						Namespace: namespace,
					},
				}
				Expect(k8sClient.Create(ctx, awscluster)).To(Succeed())
			})

			It("returns a nil owner and no error", func() {
				actualCluster, err := client.GetOwner(ctx, awscluster)
				Expect(err).NotTo(HaveOccurred())
				Expect(actualCluster).To(BeNil())
			})
		})

		When("the owner cluster does not exist", func() {
			BeforeEach(func() {
				awscluster = &capa.AWSCluster{
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
				Expect(k8sClient.Create(ctx, awscluster)).To(Succeed())
			})

			It("returns a nil owner and no error", func() {
				actualCluster, err := client.GetOwner(ctx, awscluster)
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
				actualCluster, err := client.GetOwner(ctx, awscluster)
				Expect(err).To(MatchError(ContainSubstring("context canceled")))
				Expect(actualCluster).To(BeNil())
			})
		})
	})

	Describe("AddFinalizer", func() {
		var awscluster *capa.AWSCluster

		BeforeEach(func() {
			awscluster = &capa.AWSCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: namespace,
				},
			}
			Expect(k8sClient.Create(ctx, awscluster)).To(Succeed())
		})

		It("adds the finalizer to the cluster", func() {
			err := client.AddFinalizer(ctx, awscluster, controllers.Finalizer)
			Expect(err).NotTo(HaveOccurred())

			actualCluster := &capa.AWSCluster{}
			err = k8sClient.Get(ctx, types.NamespacedName{Name: awscluster.Name, Namespace: awscluster.Namespace}, actualCluster)
			Expect(err).NotTo(HaveOccurred())

			Expect(actualCluster.Finalizers).To(ContainElement(controllers.Finalizer))
		})

		When("the finalizer already exists", func() {
			BeforeEach(func() {
				err := client.AddFinalizer(ctx, awscluster, controllers.Finalizer)
				Expect(err).NotTo(HaveOccurred())
			})

			It("does not return an error", func() {
				err := client.AddFinalizer(ctx, awscluster, controllers.Finalizer)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		When("the cluster does not exist", func() {
			It("returns an error", func() {
				awscluster = &capa.AWSCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "does-not-exist",
						Namespace: namespace,
					},
				}
				err := client.AddFinalizer(ctx, awscluster, controllers.Finalizer)
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
				err := client.AddFinalizer(ctx, awscluster, controllers.Finalizer)
				Expect(err).To(MatchError(ContainSubstring("context canceled")))
			})
		})
	})

	Describe("RemoveFinalizer", func() {
		var awscluster *capa.AWSCluster

		BeforeEach(func() {
			awscluster = &capa.AWSCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: namespace,
					Finalizers: []string{
						controllers.Finalizer,
					},
				},
			}
			Expect(k8sClient.Create(ctx, awscluster)).To(Succeed())
		})

		It("removes the finalizer to the cluster", func() {
			err := client.RemoveFinalizer(ctx, awscluster, controllers.Finalizer)
			Expect(err).NotTo(HaveOccurred())

			actualCluster := &capa.AWSCluster{}
			err = k8sClient.Get(ctx, types.NamespacedName{Name: awscluster.Name, Namespace: awscluster.Namespace}, actualCluster)
			Expect(err).NotTo(HaveOccurred())

			Expect(actualCluster.Finalizers).NotTo(ContainElement(controllers.Finalizer))
		})

		When("the finalizer doesn't exists", func() {
			BeforeEach(func() {
				err := client.RemoveFinalizer(ctx, awscluster, controllers.Finalizer)
				Expect(err).NotTo(HaveOccurred())
			})

			It("does not return an error", func() {
				err := client.RemoveFinalizer(ctx, awscluster, controllers.Finalizer)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		When("the cluster does not exist", func() {
			It("returns an error", func() {
				awscluster = &capa.AWSCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "does-not-exist",
						Namespace: namespace,
					},
				}
				err := client.RemoveFinalizer(ctx, awscluster, controllers.Finalizer)
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
				err := client.RemoveFinalizer(ctx, awscluster, controllers.Finalizer)
				Expect(err).To(MatchError(ContainSubstring("context canceled")))
			})
		})
	})
})
