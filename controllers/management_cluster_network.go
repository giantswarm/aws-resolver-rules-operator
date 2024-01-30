package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
	capa "sigs.k8s.io/cluster-api-provider-aws/api/v1beta1"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	capiannotations "sigs.k8s.io/cluster-api/util/annotations"
	capiconditions "sigs.k8s.io/cluster-api/util/conditions"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/aws-resolver-rules-operator/pkg/conditions"
	gserrors "github.com/aws-resolver-rules-operator/pkg/errors"
	"github.com/aws-resolver-rules-operator/pkg/resolver"
	"github.com/aws-resolver-rules-operator/pkg/util/annotations"
)

const (
	FinalizerManagementCluster               = "network-topology.finalizers.giantswarm.io/management-cluster"
	RequeueDurationTransitGatewayNotDetached = 10 * time.Second
)

type ManagementClusterNetworkReconciler struct {
	managementCluster types.NamespacedName
	clusterClient     AWSClusterClient
	awsClients        resolver.AWSClients
}

type networkReconcileScope struct {
	cluster              *capa.AWSCluster
	transitGatewayClient resolver.TransitGatewayClient
	prefixListClient     resolver.PrefixListClient
}

func NewManagementClusterTransitGateway(
	managementCluster types.NamespacedName,
	client AWSClusterClient,
	awsClients resolver.AWSClients,
) *ManagementClusterNetworkReconciler {
	return &ManagementClusterNetworkReconciler{
		managementCluster: managementCluster,
		clusterClient:     client,
		awsClients:        awsClients,
	}
}

func (r *ManagementClusterNetworkReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&capa.AWSCluster{}).
		WithEventFilter(
			predicate.Funcs{
				UpdateFunc: predicateToFilterAWSClusterResourceVersionChanges,
			},
		).
		Named("mc-transit-gateway").
		Complete(r)
}

func (r *ManagementClusterNetworkReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
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

	identity, err := r.clusterClient.GetIdentity(ctx, cluster)
	if err != nil {
		logger.Error(err, "Failed to get cluster identity")
		return ctrl.Result{}, errors.WithStack(err)
	}
	transitGatewayClient, err := r.awsClients.NewTransitGatewayClient(cluster.Spec.Region, identity.Spec.RoleArn)
	if err != nil {
		logger.Error(err, "Failed to create transit gateway client")
		return ctrl.Result{}, errors.WithStack(err)
	}

	prefixListClient, err := r.awsClients.NewPrefixListClient(cluster.Spec.Region, identity.Spec.RoleArn)
	if err != nil {
		logger.Error(err, "Failed to create transit gateway client")
		return ctrl.Result{}, errors.WithStack(err)
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

	scope := networkReconcileScope{
		cluster:              cluster,
		transitGatewayClient: transitGatewayClient,
		prefixListClient:     prefixListClient,
	}

	if !cluster.DeletionTimestamp.IsZero() {
		logger.Info("Reconciling delete")
		return r.reconcileDelete(ctx, scope)
	}

	return r.reconcileNormal(ctx, scope)
}

func (r *ManagementClusterNetworkReconciler) reconcileNormal(ctx context.Context, scope networkReconcileScope) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	err := r.clusterClient.AddFinalizer(ctx, scope.cluster, FinalizerManagementCluster)
	if err != nil {
		logger.Error(err, "Failed to add finalizer")
		return ctrl.Result{}, errors.WithStack(err)
	}

	err = r.applyTransitGateway(ctx, scope)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}

	err = r.applyPrefixList(ctx, scope)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}

	return ctrl.Result{}, nil
}

func (r *ManagementClusterNetworkReconciler) applyTransitGateway(ctx context.Context, scope networkReconcileScope) error {
	logger := log.FromContext(ctx)

	id, err := scope.transitGatewayClient.Apply(ctx, scope.cluster.Name, scope.cluster.Spec.AdditionalTags)
	if err != nil {
		logger.Error(err, "Failed to create transit gateway")
		return errors.WithStack(err)
	}

	baseCluster := scope.cluster.DeepCopy()
	annotations.SetNetworkTopologyTransitGateway(scope.cluster, id)
	if scope.cluster, err = r.clusterClient.PatchCluster(ctx, scope.cluster, client.MergeFrom(baseCluster)); err != nil {
		logger.Error(err, "Failed to patch cluster resource with TGW ID")
		return errors.WithStack(err)
	}

	conditions.MarkReady(scope.cluster, conditions.TransitGatewayCreated)
	return nil
}

func (r *ManagementClusterNetworkReconciler) applyPrefixList(ctx context.Context, scope networkReconcileScope) error {
	logger := log.FromContext(ctx)

	id, err := scope.prefixListClient.Apply(ctx, scope.cluster.Name, scope.cluster.Spec.AdditionalTags)
	if err != nil {
		logger.Error(err, "Failed to create prefix list")
		return errors.WithStack(err)
	}

	baseCluster := scope.cluster.DeepCopy()
	annotations.SetNetworkTopologyPrefixList(scope.cluster, id)
	if scope.cluster, err = r.clusterClient.PatchCluster(ctx, scope.cluster, client.MergeFrom(baseCluster)); err != nil {
		logger.Error(err, "Failed to patch cluster resource with prefix list ID")
		return errors.WithStack(err)
	}

	conditions.MarkReady(scope.cluster, conditions.TransitGatewayCreated)
	return nil
}

func (r *ManagementClusterNetworkReconciler) reconcileDelete(ctx context.Context, scope networkReconcileScope) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(scope.cluster, FinalizerManagementCluster) {
		return ctrl.Result{}, nil
	}

	logger := log.FromContext(ctx)

	err := scope.transitGatewayClient.Delete(ctx, scope.cluster.Name)
	retryableErr := &gserrors.RetryableError{}
	if errors.As(err, &retryableErr) {
		logger.Info(fmt.Sprintf("Failed to delete transit gateway %s", retryableErr.Error()))
		return ctrl.Result{RequeueAfter: retryableErr.RetryAfter()}, nil
	}
	if err != nil {
		logger.Error(err, "Failed to delete transit gateway")
		return ctrl.Result{}, errors.WithStack(err)
	}

	err = scope.prefixListClient.Delete(ctx, scope.cluster.Name)
	if err != nil {
		logger.Error(err, "Failed to delete prefix list")
		return ctrl.Result{}, errors.WithStack(err)
	}

	err = r.clusterClient.RemoveFinalizer(ctx, scope.cluster, FinalizerManagementCluster)
	if err != nil {
		logger.Error(err, "Failed to delete finalizer")
		return ctrl.Result{}, errors.WithStack(err)
	}
	return ctrl.Result{}, nil
}

func (r *ManagementClusterNetworkReconciler) isManagementCluster(cluster *capa.AWSCluster) bool {
	return cluster.Name == r.managementCluster.Name &&
		cluster.Namespace == r.managementCluster.Namespace
}
