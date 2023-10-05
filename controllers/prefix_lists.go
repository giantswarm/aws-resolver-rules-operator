package controllers

import (
	"context"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
	capa "sigs.k8s.io/cluster-api-provider-aws/api/v1beta1"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	capiannotations "sigs.k8s.io/cluster-api/util/annotations"
	capiconditions "sigs.k8s.io/cluster-api/util/conditions"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/aws-resolver-rules-operator/pkg/conditions"
	"github.com/aws-resolver-rules-operator/pkg/resolver"
	"github.com/aws-resolver-rules-operator/pkg/util/annotations"
)

const FinalizerPrefixLists = "network-topology.finalizers.giantswarm.io/prefix-lists"

type PrefixListsReconciler struct {
	managementCluster types.NamespacedName
	clusterClient     AWSClusterClient
	prefixListClient  resolver.PrefixListClient
}

func NewPrefixListsReconciler(
	managementCluster types.NamespacedName,
	client AWSClusterClient,
	prefixListClient resolver.PrefixListClient,
) *PrefixListsReconciler {
	return &PrefixListsReconciler{
		managementCluster: managementCluster,
		clusterClient:     client,
		prefixListClient:  prefixListClient,
	}
}

func (r *PrefixListsReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&capa.AWSCluster{}).
		Named("prefix-lists").
		Complete(r)
}

func (r *PrefixListsReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	logger.Info("Reconciling")
	defer logger.Info("Done reconciling")

	cluster, err := r.clusterClient.GetAWSCluster(ctx, req.NamespacedName)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(client.IgnoreNotFound(err))
	}

	if !r.isManagementCluster(cluster) {
		logger.Info("Cluster not management cluster. Skipping...")
		return ctrl.Result{}, nil
	}

	if !annotations.IsNetworkTopologyModeGiantSwarmManaged(cluster) {
		logger.Info("Cluster not using GiantSwarmManaged network topology mode. Skipping...")
		return ctrl.Result{}, nil
	}

	if capiannotations.HasPaused(cluster) {
		logger.Info("Cluster is marked as paused. Won't reconcile")
		return ctrl.Result{}, nil
	}

	defer func() {
		_ = r.clusterClient.UpdateStatus(ctx, cluster)
	}()

	if !capiconditions.Has(cluster, conditions.NetworkTopologyCondition) {
		capiconditions.MarkFalse(
			cluster,
			conditions.NetworkTopologyCondition,
			"InProgress",
			capi.ConditionSeverityInfo, "")
	}

	if !cluster.DeletionTimestamp.IsZero() {
		logger.Info("Reconciling delete")
		return r.reconcileDelete(ctx, cluster)
	}

	return r.reconcileNormal(ctx, cluster)
}

func (r *PrefixListsReconciler) reconcileNormal(ctx context.Context, cluster *capa.AWSCluster) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	err := r.clusterClient.AddFinalizer(ctx, cluster, FinalizerPrefixLists)
	if err != nil {
		logger.Error(err, "Failed to add finalizer")
		return ctrl.Result{}, errors.WithStack(err)
	}

	id, err := r.prefixListClient.Apply(ctx, cluster.Name)
	if err != nil {
		logger.Error(err, "Failed to create prefix lists")
		return ctrl.Result{}, errors.WithStack(err)
	}

	baseCluster := cluster.DeepCopy()
	annotations.SetNetworkTopologyPrefixList(cluster, id)
	if cluster, err = r.clusterClient.PatchCluster(ctx, cluster, client.MergeFrom(baseCluster)); err != nil {
		logger.Error(err, "Failed to patch cluster resource with prefix list ID")
		return ctrl.Result{}, errors.WithStack(err)
	}

	// conditions.MarkReady(cluster, conditions.TransitGatewayCreated)
	return ctrl.Result{}, nil
}

func (r *PrefixListsReconciler) reconcileDelete(ctx context.Context, cluster *capa.AWSCluster) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	err := r.prefixListClient.Delete(ctx, cluster.Name)
	if err != nil {
		logger.Error(err, "Failed to delete prefix lists")
		return ctrl.Result{}, errors.WithStack(err)
	}

	err = r.clusterClient.RemoveFinalizer(ctx, cluster, FinalizerPrefixLists)
	if err != nil {
		logger.Error(err, "Failed to delete finalizer")
		return ctrl.Result{}, errors.WithStack(err)
	}
	return ctrl.Result{}, nil
}

func (r *PrefixListsReconciler) isManagementCluster(cluster *capa.AWSCluster) bool {
	return cluster.Name == r.managementCluster.Name &&
		cluster.Namespace == r.managementCluster.Namespace
}
