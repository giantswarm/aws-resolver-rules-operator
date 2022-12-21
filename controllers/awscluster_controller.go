/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	"time"

	awssession "github.com/aws/aws-sdk-go/aws/session"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
	capa "sigs.k8s.io/cluster-api-provider-aws/api/v1beta1"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/annotations"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/aws-resolver-rules-operator/pkg/aws"
)

const (
	requeueAfter = time.Second * 30
	Finalizer    = "aws-resolver-rules-operator.finalizers.giantswarm.io"
)

//counterfeiter:generate . AWSClusterClient
type AWSClusterClient interface {
	Get(context.Context, types.NamespacedName) (*capa.AWSCluster, error)
	GetOwner(context.Context, *capa.AWSCluster) (*capi.Cluster, error)
	AddFinalizer(context.Context, *capa.AWSCluster, string) error
	RemoveFinalizer(context.Context, *capa.AWSCluster, string) error
}

type AWSClients interface {
	NewRoute53ResolverClient(session *awssession.Session, arn string) Route53ResolverClient
	NewEC2Client(session *awssession.Session, arn string) EC2Client
}

//counterfeiter:generate . EC2Client
type EC2Client interface {
	CreateSecurityGroupWithContext(ctx context.Context, vpcId, description, groupName string) (string, error)
	AuthorizeSecurityGroupIngressWithContext(ctx context.Context, securityGroupId, protocol string, port int) error
}

type ResolverRule struct {
	Id   string
	Name string
}

//counterfeiter:generate . Route53ResolverClient
type Route53ResolverClient interface {
	CreateResolverRuleWithContext(ctx context.Context, domainName, resolverRuleName, endpointId, kind string, targetIPs []string) (string, error)
	AssociateResolverRuleWithContext(ctx context.Context, associationName, vpcID, resolverRuleId string) (string, error)
	CreateResolverEndpointWithContext(ctx context.Context, direction, name string, securityGroupIds, subnetIds []string) (string, error)
}

// AwsClusterReconciler reconciles a AwsCluster object
type AwsClusterReconciler struct {
	AWSClusterClient AWSClusterClient
	AWSClients       AWSClients
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=awsclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=awsclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=awsclusters/finalizers,verbs=update
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status,verbs=get;list;watch

func (r *AwsClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling")
	defer logger.Info("Done reconciling")

	awsCluster, err := r.AWSClusterClient.Get(ctx, req.NamespacedName)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(client.IgnoreNotFound(err))
	}

	cluster, err := r.AWSClusterClient.GetOwner(ctx, awsCluster)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}

	if cluster == nil {
		logger.Info("Cluster controller has not yet set OwnerRef")
		return ctrl.Result{}, nil
	}

	if annotations.IsPaused(cluster, awsCluster) {
		logger.Info("Reconciliation is paused for this object")
		return ctrl.Result{}, nil
	}

	if !awsCluster.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, awsCluster)
	}

	return r.reconcileNormal(ctx, awsCluster)
}

// reconcileNormal takes care of creating resolver rules for the workload cluster endpoints/urls.
// Create rule
//   - Create SG
//   - Create ingress rules for SG
//   - Create resolver endpoints
//   - Create resolver rule
//
// Share rule
// Associate rule
func (r *AwsClusterReconciler) reconcileNormal(ctx context.Context, awsCluster *capa.AWSCluster) (ctrl.Result, error) {
	session, err := aws.SessionFromRegion(awsCluster.Spec.Region)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}

	// aws ec2 create-security-group --description "Security group for resolver rule endpoints" --group-name ${CLUSTER}-rr-endpoints2 --vpc-id $CLUSTER_VPC_ID)
	ec2Client := r.AWSClients.NewEC2Client(session, "arn")
	securityGroupId, err := ec2Client.CreateSecurityGroupWithContext(ctx, awsCluster.Spec.NetworkSpec.VPC.ID, "Security group for resolver rule endpoints", fmt.Sprintf("%s-resolverrules-endpoints", awsCluster.Name))
	if err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}

	err = ec2Client.AuthorizeSecurityGroupIngressWithContext(ctx, securityGroupId, "udp", 53)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}

	err = ec2Client.AuthorizeSecurityGroupIngressWithContext(ctx, securityGroupId, "tcp", 53)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}

	route53ResolverRulesClient := r.AWSClients.NewRoute53ResolverClient(session, "arn")
	_, err = route53ResolverRulesClient.CreateResolverEndpointWithContext(ctx, "INBOUND", fmt.Sprintf("%s-inbound", awsCluster.Name), []string{securityGroupId}, []string{""})
	if err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}

	outboundEndpointId, err := route53ResolverRulesClient.CreateResolverEndpointWithContext(ctx, "OUTBOUND", fmt.Sprintf("%s-outbound", awsCluster.Name), []string{securityGroupId}, []string{""})
	if err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}

	_, err = route53ResolverRulesClient.CreateResolverRuleWithContext(ctx, "api.asdas", fmt.Sprintf("giantswarm-%s", awsCluster.Name), outboundEndpointId, "FORWARD", []string{""})
	if err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}

	// var urls = []string{"api.asdasd", "bastion.asdfdas"}
	// for _, url := range urls {
	// }

	return reconcile.Result{}, nil
}

func (r *AwsClusterReconciler) reconcileDelete(ctx context.Context, awsCluster *capa.AWSCluster) (ctrl.Result, error) {
	return reconcile.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AwsClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		// Uncomment the following line adding a pointer to an instance of the controlled resource as an argument
		For(&capa.AWSCluster{}).
		Complete(r)
}
