package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
	capa "sigs.k8s.io/cluster-api-provider-aws/v2/api/v1beta2"
	capiannotations "sigs.k8s.io/cluster-api/util/annotations"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/aws-resolver-rules-operator/pkg/conditions"
	"github.com/aws-resolver-rules-operator/pkg/resolver"
	"github.com/aws-resolver-rules-operator/pkg/util/annotations"
)

const FinalizerPrefixListEntry = "network-topology.finalizers.giantswarm.io/prefix-list-entries"

type PrefixListEntryReconciler struct {
	managementCluster types.NamespacedName
	clusterClient     AWSClusterClient
	clients           resolver.AWSClients
}

func NewPrefixListEntryReconciler(
	managementCluster types.NamespacedName,
	clusterClient AWSClusterClient,
	clients resolver.AWSClients,
) *PrefixListEntryReconciler {
	return &PrefixListEntryReconciler{
		managementCluster: managementCluster,
		clusterClient:     clusterClient,
		clients:           clients,
	}
}

func (r *PrefixListEntryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&capa.AWSCluster{}).
		WithEventFilter(
			predicate.Funcs{
				UpdateFunc: predicateToFilterAWSClusterResourceVersionChanges,
			},
		).
		Named("prefix-list-entry").
		Complete(r)
}

type entryScope struct {
	cluster          *capa.AWSCluster
	prefixListClient resolver.PrefixListClient
	prefixListARN    string
}

func (r *PrefixListEntryReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	logger.Info("Reconciling")
	defer logger.Info("Done reconciling")

	cluster, err := r.clusterClient.GetAWSCluster(ctx, req.NamespacedName)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(client.IgnoreNotFound(err))
	}

	managementCluster, err := r.clusterClient.GetAWSCluster(ctx, r.managementCluster)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}

	if !annotations.IsNetworkTopologyModeGiantSwarmManaged(cluster) {
		logger.Info("Cluster not using GiantSwarmManaged network topology mode. Skipping...")
		return ctrl.Result{}, nil
	}

	if capiannotations.HasPaused(cluster) {
		logger.Info("Cluster is marked as paused. Won't reconcile")
		return ctrl.Result{}, nil
	}

	prefixListARN := getPrefixListARN(cluster, managementCluster)

	if prefixListARN == "" {
		logger.Info("Prefix List not created yet. Skipping...")
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}

	logger = logger.WithValues("prefix-list-arn", prefixListARN)
	log.IntoContext(ctx, logger)

	identity, err := r.clusterClient.GetIdentity(ctx, managementCluster)
	if err != nil {
		logger.Error(err, "failed to get cluster identity")
		return ctrl.Result{}, errors.WithStack(err)
	}

	prefixListClient, err := r.clients.NewPrefixListClient(cluster.Spec.Region, identity.Spec.RoleArn)
	if err != nil {
		logger.Error(err, "failed to create wc prefix list client")
		return ctrl.Result{}, errors.WithStack(err)
	}

	defer func() {
		_ = r.clusterClient.UpdateStatus(ctx, cluster)
	}()

	scope := entryScope{
		cluster:          cluster,
		prefixListClient: prefixListClient,
		prefixListARN:    prefixListARN,
	}

	if !cluster.DeletionTimestamp.IsZero() {
		logger.Info("Reconciling delete")
		return r.reconcileDelete(ctx, scope)
	}

	return r.reconcileNormal(ctx, scope)
}

func (r *PrefixListEntryReconciler) reconcileNormal(ctx context.Context, scope entryScope) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if scope.cluster.Spec.NetworkSpec.VPC.ID == "" {
		logger.Info("VPC not created yet. Skipping...")
		return ctrl.Result{}, nil
	}

	err := r.clusterClient.AddFinalizer(ctx, scope.cluster, FinalizerPrefixListEntry)
	if err != nil {
		logger.Error(err, "Failed to add finalizer")
		return ctrl.Result{}, errors.WithStack(err)
	}

	logger.Info("Applying prefix list entry")
	err = scope.prefixListClient.ApplyEntry(ctx, resolver.PrefixListEntry{
		PrefixListARN: scope.prefixListARN,
		CIDR:          scope.cluster.Spec.NetworkSpec.VPC.CidrBlock,
		Description:   getPrefixListEntryDescription(scope.cluster.Name),
	})
	if err != nil {
		logger.Error(err, "Failed to apply prefix list entry")
		return ctrl.Result{}, errors.WithStack(err)
	}

	conditions.MarkReady(scope.cluster, conditions.PrefixListEntriesReady)
	return ctrl.Result{}, nil
}

func (r *PrefixListEntryReconciler) reconcileDelete(ctx context.Context, scope entryScope) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(scope.cluster, FinalizerPrefixListEntry) {
		return ctrl.Result{}, nil
	}

	logger := log.FromContext(ctx)

	logger.Info("Deleting prefix list entry")
	err := scope.prefixListClient.DeleteEntry(ctx, resolver.PrefixListEntry{
		PrefixListARN: scope.prefixListARN,
		CIDR:          scope.cluster.Spec.NetworkSpec.VPC.CidrBlock,
		Description:   getPrefixListEntryDescription(scope.cluster.Name),
	})
	if err != nil {
		logger.Error(err, "Failed to delete prefix list entry")
		return ctrl.Result{}, errors.WithStack(err)
	}

	err = r.clusterClient.RemoveFinalizer(ctx, scope.cluster, FinalizerPrefixListEntry)
	if err != nil {
		logger.Error(err, "Failed to delete finalizer")
		return ctrl.Result{}, errors.WithStack(err)
	}

	return ctrl.Result{}, nil
}

func getPrefixListEntryDescription(clusterName string) string {
	return fmt.Sprintf("CIDR block for cluster %s", clusterName)
}
