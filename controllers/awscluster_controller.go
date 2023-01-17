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

	gsannotations "github.com/giantswarm/k8smetadata/pkg/annotation"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
	capa "sigs.k8s.io/cluster-api-provider-aws/api/v1beta1"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/annotations"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/aws-resolver-rules-operator/pkg/resolver"
)

const (
	Finalizer = "aws-resolver-rules-operator.finalizers.giantswarm.io"
)

//counterfeiter:generate . AWSClusterClient
type AWSClusterClient interface {
	Get(context.Context, types.NamespacedName) (*capa.AWSCluster, error)
	GetOwner(context.Context, *capa.AWSCluster) (*capi.Cluster, error)
	AddFinalizer(context.Context, *capa.AWSCluster, string) error
	RemoveFinalizer(context.Context, *capa.AWSCluster, string) error
	GetIdentity(context.Context, *capa.AWSCluster) (*capa.AWSClusterRoleIdentity, error)
}

// AwsClusterReconciler reconciles a AwsCluster object
type AwsClusterReconciler struct {
	awsClusterClient AWSClusterClient
	resolver         resolver.Resolver
}

func NewAwsClusterReconciler(awsClusterClient AWSClusterClient, resolver resolver.Resolver) *AwsClusterReconciler {
	return &AwsClusterReconciler{
		awsClusterClient: awsClusterClient,
		resolver:         resolver,
	}
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=awsclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=awsclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=awsclusters/finalizers,verbs=update
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status,verbs=get;list;watch

func (r *AwsClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling")
	defer logger.Info("Done reconciling")

	awsCluster, err := r.awsClusterClient.Get(ctx, req.NamespacedName)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(client.IgnoreNotFound(err))
	}

	cluster, err := r.awsClusterClient.GetOwner(ctx, awsCluster)
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

	identity, err := r.awsClusterClient.GetIdentity(ctx, awsCluster)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}

	if identity == nil {
		logger.Info("AWSCluster has no identityRef set, skipping")
		return ctrl.Result{}, nil
	}

	if !awsCluster.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, awsCluster, identity)
	}

	return r.reconcileNormal(ctx, awsCluster, identity)
}

// reconcileNormal takes care of creating resolver rules for the workload clusters.
func (r *AwsClusterReconciler) reconcileNormal(ctx context.Context, awsCluster *capa.AWSCluster, identity *capa.AWSClusterRoleIdentity) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	dnsModeAnnotation, ok := awsCluster.Annotations[gsannotations.AWSDNSMode]
	if !ok || dnsModeAnnotation != gsannotations.DNSModePrivate {
		logger.Info("AWSCluster is not using private DNS mode, skipping")
		return ctrl.Result{}, nil
	}

	cluster := resolver.Cluster{Name: awsCluster.Name, Region: awsCluster.Spec.Region, VPCId: awsCluster.Spec.NetworkSpec.VPC.ID, IAMRoleARN: identity.Spec.RoleArn, Subnets: getSubnetIds(awsCluster)}
	associatedResolverRule, err := r.resolver.CreateRule(ctx, logger, cluster)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}

	logger.Info("Created resolver rule", "ruleId", associatedResolverRule.RuleId, "ruleArn", associatedResolverRule.RuleArn)

	return reconcile.Result{}, nil
}

func (r *AwsClusterReconciler) reconcileDelete(ctx context.Context, awsCluster *capa.AWSCluster, identity *capa.AWSClusterRoleIdentity) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	cluster := resolver.Cluster{Name: awsCluster.Name, Region: awsCluster.Spec.Region, VPCId: awsCluster.Spec.NetworkSpec.VPC.ID, IAMRoleARN: identity.Spec.RoleArn, Subnets: getSubnetIds(awsCluster)}
	err := r.resolver.DeleteRule(ctx, logger, cluster)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}

	logger.Info("Deleted resolver rule")

	return reconcile.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AwsClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&capa.AWSCluster{}).
		Complete(r)
}

func getSubnetIds(awsCluster *capa.AWSCluster) []string {
	var subnetIds []string
	for _, subnet := range awsCluster.Spec.NetworkSpec.Subnets {
		subnetIds = append(subnetIds, subnet.ID)
	}

	return subnetIds
}
