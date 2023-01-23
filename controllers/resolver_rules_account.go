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
	capa "sigs.k8s.io/cluster-api-provider-aws/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/aws-resolver-rules-operator/pkg/resolver"
)

const (
	Finalizer = "aws-resolver-rules-operator.account.finalizers.giantswarm.io"
)

// ResolverRulesReconciler reconciles AWSClusters by associating resolver rules with the workload cluster VPC.
// It only reconciles AWSCluster using the private DNS mode, set by the `aws.giantswarm.io/dns-mode` annotation.
//
// It will loop through all the resolver rules that belong to a specific AWS account and associate them with the WC VPC.
// The AWS account is specified using the `aws.giantswarm.io/resolver-rules-owner-account` annotation.
// When a workload cluster is deleted, the resolver rules will be disassociated.
type ResolverRulesReconciler struct {
	awsClusterClient AWSClusterClient
	resolver         resolver.Resolver
}

func NewResolverRulesReconciler(awsClusterClient AWSClusterClient, resolver resolver.Resolver) *ResolverRulesReconciler {
	return &ResolverRulesReconciler{
		awsClusterClient: awsClusterClient,
		resolver:         resolver,
	}
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=awsclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=awsclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=awsclusters/finalizers,verbs=update
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status,verbs=get;list;watch

func (r *ResolverRulesReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
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

	awsAccountOwnerOfRulesToAssociate, ok := awsCluster.Annotations[gsannotations.ResolverRulesOwnerAWSAccountId]
	if !ok {
		logger.Info("AWSCluster is missing the annotation to specify the AWS Account id that owns the resolver rules to associate with the workload cluster VPC, skipping")
		return ctrl.Result{}, nil
	}

	if !awsCluster.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, awsCluster, identity, awsAccountOwnerOfRulesToAssociate)
	}

	return r.reconcileNormal(ctx, awsCluster, identity, awsAccountOwnerOfRulesToAssociate)
}

// reconcileNormal takes care of associating resolver rules in the desired AWS account with the workload cluster's VPC.
func (r *ResolverRulesReconciler) reconcileNormal(ctx context.Context, awsCluster *capa.AWSCluster, identity *capa.AWSClusterRoleIdentity, awsAccountOwnerOfRulesToAssociate string) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	err := r.awsClusterClient.AddFinalizer(ctx, awsCluster, Finalizer)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}

	cluster := resolver.Cluster{Name: awsCluster.Name, Region: awsCluster.Spec.Region, VPCCidr: awsCluster.Spec.NetworkSpec.VPC.CidrBlock, VPCId: awsCluster.Spec.NetworkSpec.VPC.ID, IAMRoleARN: identity.Spec.RoleArn, Subnets: getSubnetIds(awsCluster)}
	err = r.resolver.AssociateResolverRulesInAccountWithClusterVPC(ctx, logger, cluster, awsAccountOwnerOfRulesToAssociate)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}

	return reconcile.Result{}, nil
}

func (r *ResolverRulesReconciler) reconcileDelete(ctx context.Context, awsCluster *capa.AWSCluster, identity *capa.AWSClusterRoleIdentity, awsAccountOwnerOfRulesToAssociate string) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	cluster := resolver.Cluster{Name: awsCluster.Name, Region: awsCluster.Spec.Region, VPCCidr: awsCluster.Spec.NetworkSpec.VPC.CidrBlock, VPCId: awsCluster.Spec.NetworkSpec.VPC.ID, IAMRoleARN: identity.Spec.RoleArn, Subnets: getSubnetIds(awsCluster)}
	err := r.resolver.DisassociateResolverRulesInAccountWithClusterVPC(ctx, logger, cluster, awsAccountOwnerOfRulesToAssociate)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}

	err = r.awsClusterClient.RemoveFinalizer(ctx, awsCluster, Finalizer)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}

	return reconcile.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ResolverRulesReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named("capa_resolverrules_aws_account").
		For(&capa.AWSCluster{}).
		WithEventFilter(predicates.ResourceNotPaused(mgr.GetLogger())).
		Complete(r)
}
