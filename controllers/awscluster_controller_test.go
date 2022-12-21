package controllers_test

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	capa "sigs.k8s.io/cluster-api-provider-aws/api/v1beta1"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/aws-resolver-rules-operator/controllers"
	"github.com/aws-resolver-rules-operator/controllers/controllersfakes"
	"github.com/aws-resolver-rules-operator/pkg/awsclient"
)

var _ = Describe("AWSCluster", func() {
	var (
		awsClusterClient      *controllersfakes.FakeAWSClusterClient
		ctx                   context.Context
		reconciler            controllers.AwsClusterReconciler
		cluster               *capi.Cluster
		awsCluster            *capa.AWSCluster
		result                ctrl.Result
		reconcileErr          error
		route53resolverClient *controllersfakes.FakeRoute53ResolverClient
		ec2Client             *controllersfakes.FakeEC2Client
	)

	BeforeEach(func() {
		ctx = context.Background()
		awsClusterClient = new(controllersfakes.FakeAWSClusterClient)
		route53resolverClient = new(controllersfakes.FakeRoute53ResolverClient)
		ec2Client = new(controllersfakes.FakeEC2Client)
		reconciler = controllers.AwsClusterReconciler{
			AWSClusterClient: awsClusterClient,
			AWSClients: &awsclient.FakeClientsFactory{
				Route53ResolverClient: route53resolverClient,
				EC2Client:             ec2Client,
			},
		}

		awsCluster = &capa.AWSCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "bar",
			},
			Spec: capa.AWSClusterSpec{
				NetworkSpec: capa.NetworkSpec{
					VPC: capa.VPCSpec{
						ID: "myvpc-1a2b3c4d",
					},
					Subnets: []capa.SubnetSpec{},
				},
				Region:      "gs-south-1",
				IdentityRef: nil,
			},
		}
		cluster = &capi.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "bar",
			},
		}
	})

	JustBeforeEach(func() {
		request := ctrl.Request{
			NamespacedName: types.NamespacedName{
				Name:      "foo",
				Namespace: "bar",
			},
		}
		_, reconcileErr = reconciler.Reconcile(ctx, request)
	})

	When("the cluster has an owner, it's not paused and it's not marked for deletion", func() {
		BeforeEach(func() {
			awsClusterClient.GetReturns(awsCluster, nil)
			awsClusterClient.GetOwnerReturns(cluster, nil)
		})

		It("gets the cluster and owner cluster", func() {
			Expect(awsClusterClient.GetCallCount()).To(Equal(1))
			Expect(awsClusterClient.GetOwnerCallCount()).To(Equal(1))

			_, actualCluster := awsClusterClient.GetOwnerArgsForCall(0)
			Expect(actualCluster).To(Equal(awsCluster))
			Expect(reconcileErr).NotTo(HaveOccurred())
		})

		When("when creating security group fails", func() {
			BeforeEach(func() {
				ec2Client.CreateSecurityGroupWithContextReturns("", errors.New("boom"))
			})
			It("returns ec2 error", func() {
				Expect(reconcileErr).To(HaveOccurred())
			})
		})

		When("when creating security group succeeds", func() {
			BeforeEach(func() {
				ec2Client.CreateSecurityGroupWithContextReturns("my-security-group", nil)
				route53resolverClient.CreateResolverEndpointWithContextReturnsOnCall(0, "inbound-endpoint", nil)
				route53resolverClient.CreateResolverEndpointWithContextReturnsOnCall(1, "outbound-endpoint", nil)
			})

			It("creates security group", func() {
				Expect(ec2Client.CreateSecurityGroupWithContextCallCount()).To(Equal(1))
				_, vpcId, description, groupName := ec2Client.CreateSecurityGroupWithContextArgsForCall(0)
				Expect(vpcId).To(Equal(awsCluster.Spec.NetworkSpec.VPC.ID))
				Expect(description).To(Equal("Security group for resolver rule endpoints"))
				Expect(groupName).To(Equal("foo-resolverrules-endpoints"))
				Expect(reconcileErr).NotTo(HaveOccurred())
			})

			It("creates ingress rules", func() {
				_, securityGroupId, protocol, port := ec2Client.AuthorizeSecurityGroupIngressWithContextArgsForCall(0)
				Expect(securityGroupId).To(Equal("my-security-group"))
				Expect(protocol).To(Equal("udp"))
				Expect(port).To(Equal(53))
				Expect(reconcileErr).NotTo(HaveOccurred())

				_, securityGroupId, protocol, port = ec2Client.AuthorizeSecurityGroupIngressWithContextArgsForCall(1)
				Expect(securityGroupId).To(Equal("my-security-group"))
				Expect(protocol).To(Equal("tcp"))
				Expect(port).To(Equal(53))
			})

			It("creates resolver endpoints", func() {
				_, direction, endpointName, securityGroupIds, subnetIds := route53resolverClient.CreateResolverEndpointWithContextArgsForCall(0)
				Expect(direction).To(Equal("INBOUND"))
				Expect(endpointName).To(Equal("foo-inbound"))
				Expect(securityGroupIds).To(Equal([]string{"my-security-group"}))
				Expect(subnetIds).To(Equal([]string{""}))

				_, direction, endpointName, securityGroupIds, subnetIds = route53resolverClient.CreateResolverEndpointWithContextArgsForCall(1)
				Expect(direction).To(Equal("OUTBOUND"))
				Expect(endpointName).To(Equal("foo-outbound"))
				Expect(securityGroupIds).To(Equal([]string{"my-security-group"}))
				Expect(subnetIds).To(Equal([]string{""}))
			})

			It("creates resolver rule", func() {
				_, domainName, resolverRuleName, endpointId, kind, subnetIds := route53resolverClient.CreateResolverRuleWithContextArgsForCall(0)
				Expect(domainName).To(Equal("api.asdas"))
				Expect(resolverRuleName).To(Equal("giantswarm-foo"))
				Expect(endpointId).To(Equal("outbound-endpoint"))
				Expect(kind).To(Equal("FORWARD"))
				Expect(subnetIds).To(Equal([]string{""}))
			})
		})
	})

	When("the cluster does not have an owner yet", func() {
		BeforeEach(func() {
			awsClusterClient.GetReturns(awsCluster, nil)
			awsClusterClient.GetOwnerReturns(nil, nil)
		})

		It("does not requeue the event", func() {
			Expect(result.Requeue).To(BeFalse())
			Expect(result.RequeueAfter).To(BeZero())
			Expect(reconcileErr).NotTo(HaveOccurred())
		})
	})

	When("the cluster is paused", func() {
		BeforeEach(func() {
			awsClusterClient.GetReturns(awsCluster, nil)
			cluster.Spec.Paused = true
			awsClusterClient.GetOwnerReturns(cluster, nil)
		})

		It("does not reconcile", func() {
			Expect(result.Requeue).To(BeFalse())
			Expect(result.RequeueAfter).To(BeZero())
			Expect(reconcileErr).NotTo(HaveOccurred())
		})
	})

	When("the infrastructure cluster is paused", func() {
		BeforeEach(func() {
			awsClusterClient.GetReturns(awsCluster, nil)
			awsCluster.Annotations = map[string]string{
				capi.PausedAnnotation: "true",
			}
		})

		It("does not reconcile", func() {
			Expect(result.Requeue).To(BeFalse())
			Expect(result.RequeueAfter).To(BeZero())
			Expect(reconcileErr).NotTo(HaveOccurred())
		})
	})
})
