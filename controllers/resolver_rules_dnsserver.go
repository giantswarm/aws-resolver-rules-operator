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
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/aws-resolver-rules-operator/pkg/resolver"
)

const (
	DNSServerFinalizer = "aws-resolver-rules-operator.dnsserver.finalizers.giantswarm.io"
)

//counterfeiter:generate . AWSClusterClient
type AWSClusterClient interface {
	Get(context.Context, types.NamespacedName) (*capa.AWSCluster, error)
	GetOwner(context.Context, *capa.AWSCluster) (*capi.Cluster, error)
	AddFinalizer(context.Context, *capa.AWSCluster, string) error
	RemoveFinalizer(context.Context, *capa.AWSCluster, string) error
	GetIdentity(context.Context, *capa.AWSCluster) (*capa.AWSClusterRoleIdentity, error)
}

// ResolverRulesDNSServerReconciler reconciles AwsClusters by creating a Resolver Rule for the workload cluster API
// endpoint and associating it with the DNS Server VPC. That way, other clients of the DNS Server will be able to
// resolve the workload cluster endpoint.
type ResolverRulesDNSServerReconciler struct {
	awsClusterClient AWSClusterClient
	resolver         resolver.Resolver
}

func NewResolverRulesDNSServerReconciler(awsClusterClient AWSClusterClient, resolver resolver.Resolver) *ResolverRulesDNSServerReconciler {
	return &ResolverRulesDNSServerReconciler{
		awsClusterClient: awsClusterClient,
		resolver:         resolver,
	}
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=awsclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=awsclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=awsclusters/finalizers,verbs=update
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status,verbs=get;list;watch

func (r *ResolverRulesDNSServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
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

	dnsModeAnnotation, ok := awsCluster.Annotations[gsannotations.AWSDNSMode]
	if !ok || dnsModeAnnotation != gsannotations.DNSModePrivate {
		logger.Info("AWSCluster is not using private DNS mode, skipping")
		return ctrl.Result{}, nil
	}

	if !awsCluster.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, awsCluster, identity)
	}

	return r.reconcileNormal(ctx, awsCluster, identity)
}

// reconcileNormal takes care of creating a resolver rule for the workload clusters.
func (r *ResolverRulesDNSServerReconciler) reconcileNormal(ctx context.Context, awsCluster *capa.AWSCluster, identity *capa.AWSClusterRoleIdentity) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	err := r.awsClusterClient.AddFinalizer(ctx, awsCluster, DNSServerFinalizer)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}

	cluster := resolver.Cluster{Name: awsCluster.Name, Region: awsCluster.Spec.Region, VPCCidr: awsCluster.Spec.NetworkSpec.VPC.CidrBlock, VPCId: awsCluster.Spec.NetworkSpec.VPC.ID, IAMRoleARN: identity.Spec.RoleArn, Subnets: getSubnetIds(awsCluster)}
	associatedResolverRule, err := r.resolver.CreateRule(ctx, logger, cluster)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}

	logger.Info("Created resolver rule", "ruleId", associatedResolverRule.Id, "ruleArn", associatedResolverRule.Arn)

	return reconcile.Result{}, nil
}

// reconcileDelete deletes the Resolver Rule created for the workload cluster.
func (r *ResolverRulesDNSServerReconciler) reconcileDelete(ctx context.Context, awsCluster *capa.AWSCluster, identity *capa.AWSClusterRoleIdentity) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	cluster := resolver.Cluster{Name: awsCluster.Name, Region: awsCluster.Spec.Region, VPCCidr: awsCluster.Spec.NetworkSpec.VPC.CidrBlock, VPCId: awsCluster.Spec.NetworkSpec.VPC.ID, IAMRoleARN: identity.Spec.RoleArn, Subnets: getSubnetIds(awsCluster)}
	err := r.resolver.DeleteRule(ctx, logger, cluster)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}

	logger.Info("Deleted resolver rule")

	err = r.awsClusterClient.RemoveFinalizer(ctx, awsCluster, DNSServerFinalizer)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}

	return reconcile.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ResolverRulesDNSServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named("capa_resolverrules_dnsserver").
		For(&capa.AWSCluster{}).
		WithEventFilter(predicates.ResourceNotPaused(mgr.GetLogger())).
		Complete(r)
}

func getSubnetIds(awsCluster *capa.AWSCluster) []string {
	var subnetIds []string
	for _, subnet := range awsCluster.Spec.NetworkSpec.Subnets {
		subnetIds = append(subnetIds, subnet.ID)
	}

	return subnetIds
}
