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
	"sigs.k8s.io/cluster-api/util/conditions"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/aws-resolver-rules-operator/pkg/resolver"
)

const (
	ResolverRulesFinalizer                              = "capa-operator.finalizers.giantswarm.io/resolver-rules-controller"
	ResolverRulesAssociatedCondition capi.ConditionType = "ResolverRulesAssociated"
)

//counterfeiter:generate . AWSClusterClient
type AWSClusterClient interface {
	GetAWSCluster(context.Context, types.NamespacedName) (*capa.AWSCluster, error)
	GetCluster(ctx context.Context, namespacedName types.NamespacedName) (*capi.Cluster, error)
	GetOwner(context.Context, *capa.AWSCluster) (*capi.Cluster, error)
	AddFinalizer(context.Context, *capa.AWSCluster, string) error
	Unpause(context.Context, *capa.AWSCluster, *capi.Cluster) error
	RemoveFinalizer(context.Context, *capa.AWSCluster, string) error
	GetIdentity(context.Context, *capa.AWSCluster) (*capa.AWSClusterRoleIdentity, error)
	MarkConditionTrue(context.Context, *capa.AWSCluster, capi.ConditionType) error
	PatchCluster(context.Context, *capa.AWSCluster, client.Patch) (*capa.AWSCluster, error)
	UpdateStatus(context.Context, client.Object) error
}

// ResolverRulesReconciler reconciles AWSClusters.
// It only reconciles AWSCluster using the private DNS mode, set by the `aws.giantswarm.io/dns-mode` annotation.
// It creates a Resolver Rule for the workload cluster k8s API endpoint and associates it with the DNS Server VPC.
// That way, other clients using the DNS Server will be able to resolve the workload cluster k8s API endpoint.
//
// It will also loop through all the resolver rules that belong to a specific AWS account and associate them with the AWSCluster VPC.
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

func (r *ResolverRulesReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling")
	defer logger.Info("Done reconciling")

	awsCluster, err := r.awsClusterClient.GetAWSCluster(ctx, req.NamespacedName)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(client.IgnoreNotFound(err))
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

	if !conditions.IsTrue(awsCluster, capa.VpcReadyCondition) || !conditions.IsTrue(awsCluster, capa.SubnetsReadyCondition) {
		logger.Info("The VpcReady or SubnetReady conditions are not ready yet, skipping")
		return ctrl.Result{}, nil
	}

	return r.reconcileNormal(ctx, awsCluster, identity)
}

// reconcileNormal associates the Resolver Rules in the specified AWS account with the AWSCluster VPC, and creates a
// Resolver Rule for the workload cluster k8s API endpoint.
func (r *ResolverRulesReconciler) reconcileNormal(ctx context.Context, awsCluster *capa.AWSCluster, identity *capa.AWSClusterRoleIdentity) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	err := r.awsClusterClient.AddFinalizer(ctx, awsCluster, ResolverRulesFinalizer)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}

	cluster := buildClusterFromAWSCluster(awsCluster, identity, nil)

	awsAccountOwnerOfRulesToAssociate, ok := awsCluster.Annotations[gsannotations.ResolverRulesOwnerAWSAccountId]
	if !ok {
		logger.Info("Resolver rules won't be associated with workload cluster VPC because the annotation is missing", "annotation", gsannotations.ResolverRulesOwnerAWSAccountId)
	} else {
		err := r.resolver.AssociateResolverRulesInAccountWithClusterVPC(ctx, logger, cluster, awsAccountOwnerOfRulesToAssociate)
		if err != nil {
			return ctrl.Result{}, errors.WithStack(err)
		}
	}

	// After associating resolver rules in the account we signal that resolver rules are ready.
	err = r.awsClusterClient.MarkConditionTrue(ctx, awsCluster, ResolverRulesAssociatedCondition)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}

	_, err = r.resolver.CreateRule(ctx, logger, cluster)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}

	return reconcile.Result{}, nil
}

// reconcileDelete disassociates the Resolver Rules in the specified AWS account from the AWSCluster VPC, and deletes
// the Resolver Rule created for the workload cluster k8s API endpoint.
func (r *ResolverRulesReconciler) reconcileDelete(ctx context.Context, awsCluster *capa.AWSCluster, identity *capa.AWSClusterRoleIdentity) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(awsCluster, ResolverRulesFinalizer) {
		return ctrl.Result{}, nil
	}

	logger := log.FromContext(ctx)

	cluster := buildClusterFromAWSCluster(awsCluster, identity, nil)

	err := r.resolver.DeleteRule(ctx, logger, cluster)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}

	_, ok := awsCluster.Annotations[gsannotations.ResolverRulesOwnerAWSAccountId]
	if !ok {
		logger.Info("Resolver rules won't be disassociated with workload cluster VPC because the annotation is missing", "annotation", gsannotations.ResolverRulesOwnerAWSAccountId)
	} else {
		err := r.resolver.DisassociateResolverRulesInAccountWithClusterVPC(ctx, logger, cluster)
		if err != nil {
			return ctrl.Result{}, errors.WithStack(err)
		}
	}

	logger.Info("Removing finalizer from AWSCluster")
	err = r.awsClusterClient.RemoveFinalizer(ctx, awsCluster, ResolverRulesFinalizer)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(client.IgnoreNotFound(err))
	}

	return reconcile.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ResolverRulesReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named("resolverrules").
		For(&capa.AWSCluster{}).
		WithEventFilter(
			predicate.Funcs{
				UpdateFunc: predicateToFilterAWSClusterResourceVersionChanges,
			},
		).
		Complete(r)
}
