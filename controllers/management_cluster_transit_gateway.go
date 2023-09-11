package controllers

import (
	"context"

	"github.com/giantswarm/microerror"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	capa "sigs.k8s.io/cluster-api-provider-aws/api/v1beta1"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	capiannotations "sigs.k8s.io/cluster-api/util/annotations"
	capiconditions "sigs.k8s.io/cluster-api/util/conditions"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/aws-resolver-rules-operator/pkg/conditions"
	"github.com/aws-resolver-rules-operator/pkg/resolver"
	"github.com/aws-resolver-rules-operator/pkg/util/annotations"
)

const FinalizerManagementCluster = "network-topology.finalizers.giantswarm.io/management-cluster"

type ManagementClusterTransitGatewayReconciler struct {
	clusterClient   AWSClusterClient
	transitGateways resolver.TransitGatewayClient
}

func NewManagementClusterTransitGateway(client AWSClusterClient, awsClient resolver.TransitGatewayClient) *ManagementClusterTransitGatewayReconciler {
	return &ManagementClusterTransitGatewayReconciler{
		clusterClient:   client,
		transitGateways: awsClient,
	}
}

// ManagementClusterPredicate constructs a Predicate that will filter out only
// the management cluster
func ManagementClusterPredicate() predicate.Predicate {
	return predicate.NewPredicateFuncs(func(o client.Object) bool {
		return annotations.IsNetworkTopologyModeGiantSwarmManaged(o)
	})
}

// GiantSwarmManagedModePredicate constructs a Predicate for objects that
// contain the GiantSwarmManaged network topology mode annotation
func GiantSwarmManagedModePredicate() predicate.Predicate {
	return predicate.NewPredicateFuncs(func(o client.Object) bool {
		return annotations.IsNetworkTopologyModeGiantSwarmManaged(o)
	})
}

func (r *ManagementClusterTransitGatewayReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&capi.Cluster{}).
		WithEventFilter(GiantSwarmManagedModePredicate()).
		WithEventFilter(ManagementClusterPredicate()).
		Complete(r)
}

func (r *ManagementClusterTransitGatewayReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	logger.Info("Reconciling")
	defer logger.Info("Done reconciling")

	cluster, err := r.clusterClient.GetAWSCluster(ctx, req.NamespacedName)
	if k8sErrors.IsNotFound(err) {
		logger.Info("Cluster no longer exists")
		return ctrl.Result{}, nil
	}
	if err != nil {
		return ctrl.Result{}, microerror.Mask(err)
	}

	defer func() {
		_ = r.clusterClient.UpdateStatus(ctx, cluster)
	}()

	if capiannotations.HasPaused(cluster) {
		logger.Info("Cluster is marked as paused. Won't reconcile")
		return ctrl.Result{}, nil
	}

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

func (r *ManagementClusterTransitGatewayReconciler) reconcileNormal(ctx context.Context, cluster *capa.AWSCluster) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	err := r.clusterClient.AddFinalizer(ctx, cluster, FinalizerManagementCluster)
	if err != nil {
		logger.Error(err, "Failed to add finalizer")
		return ctrl.Result{}, microerror.Mask(err)
	}

	id, err := r.transitGateways.Apply(ctx, cluster)
	if err != nil {
		logger.Error(err, "Failed to create transit gateway")
		return ctrl.Result{}, microerror.Mask(err)
	}

	baseCluster := cluster.DeepCopy()
	annotations.SetNetworkTopologyTransitGateway(cluster, id)
	if cluster, err = r.clusterClient.PatchCluster(ctx, cluster, client.MergeFrom(baseCluster)); err != nil {
		logger.Error(err, "Failed to patch cluster resource with TGW ID")
		return ctrl.Result{}, microerror.Mask(err)
	}

	conditions.MarkReady(cluster, conditions.TransitGatewayCreated)
	return ctrl.Result{}, nil
}

func (r *ManagementClusterTransitGatewayReconciler) reconcileDelete(ctx context.Context, cluster *capa.AWSCluster) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	err := r.transitGateways.Delete(ctx, cluster)
	if err != nil {
		logger.Error(err, "Failed to delete transit gateway")
		return ctrl.Result{}, microerror.Mask(err)
	}

	err = r.clusterClient.RemoveFinalizer(ctx, cluster, FinalizerManagementCluster)
	if err != nil {
		logger.Error(err, "Failed to add finalizer")
		return ctrl.Result{}, microerror.Mask(err)
	}
	return ctrl.Result{}, nil
}