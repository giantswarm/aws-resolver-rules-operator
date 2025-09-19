package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/giantswarm/k8smetadata/pkg/annotation"
	"github.com/pkg/errors"
	k8stypes "k8s.io/apimachinery/pkg/types"
	capa "sigs.k8s.io/cluster-api-provider-aws/v2/api/v1beta2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/aws-resolver-rules-operator/pkg/resolver"
	"github.com/aws-resolver-rules-operator/pkg/util/annotations"
)

const (
	FinalizerResourceShare       = "network-topology.finalizers.giantswarm.io/share"
	ResourceMissingRequeDuration = time.Minute
)

type ShareReconciler struct {
	awsClients        resolver.AWSClients
	clusterClient     AWSClusterClient
	managementCluster k8stypes.NamespacedName
}

func NewShareReconciler(
	managementCluster k8stypes.NamespacedName,
	clusterClient AWSClusterClient,
	awsClients resolver.AWSClients,
) *ShareReconciler {
	return &ShareReconciler{
		awsClients:        awsClients,
		clusterClient:     clusterClient,
		managementCluster: managementCluster,
	}
}

func (r *ShareReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named("share-reconciler").
		For(&capa.AWSCluster{}).
		WithEventFilter(
			predicate.Funcs{
				UpdateFunc: predicateToFilterAWSClusterResourceVersionChanges,
			},
		).
		Complete(r)
}

func (r *ShareReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	logger.Info("Reconciling")
	defer logger.Info("Done reconciling")

	cluster, err := r.clusterClient.GetAWSCluster(ctx, req.NamespacedName)
	if err != nil {
		logger.Error(err, "failed to get cluster")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if annotations.GetAnnotation(cluster, annotation.NetworkTopologyModeAnnotation) != annotation.NetworkTopologyModeGiantSwarmManaged {
		logger.Info("Network topology mode is not set to GiantSwarmManaged, skipping sharing operation")
		return ctrl.Result{}, nil
	}

	if !cluster.DeletionTimestamp.IsZero() {
		logger.Info("Reconciling delete")
		return r.reconcileDelete(ctx, cluster)
	}

	return r.reconcileNormal(ctx, cluster)
}

func (r *ShareReconciler) getRamClient(ctx context.Context) (resolver.RAMClient, error) {
	logger := log.FromContext(ctx)

	managementCluster, err := r.clusterClient.GetAWSCluster(ctx, r.managementCluster)
	if err != nil {
		logger.Error(err, "failed to get management cluster")
		return nil, errors.WithStack(err)
	}

	return r.getRamClientFromCluster(ctx, managementCluster)
}

func (r *ShareReconciler) getRamClientFromCluster(ctx context.Context, cluster *capa.AWSCluster) (resolver.RAMClient, error) {
	logger := log.FromContext(ctx)

	identity, err := r.clusterClient.GetIdentity(ctx, cluster)
	if err != nil {
		logger.Error(err, "Failed to get cluster identity")
		return nil, errors.WithStack(err)
	}

	ramClient, err := r.awsClients.NewRAMClient(cluster.Spec.Region, identity.Spec.RoleArn)
	if err != nil {
		logger.Error(err, "Failed to create ram client")
		return nil, errors.WithStack(err)
	}

	return ramClient, err
}

func (r *ShareReconciler) reconcileDelete(ctx context.Context, cluster *capa.AWSCluster) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(cluster, FinalizerResourceShare) {
		return ctrl.Result{}, nil
	}

	logger := log.FromContext(ctx)
	ramClient, err := r.getRamClient(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}

	if resourcesStillInUse(cluster) {
		logger.Info("Transit gateway and prefix list not yet cleaned up. Skipping...")
		return ctrl.Result{}, nil
	}

	err = ramClient.DeleteResourceShare(ctx, getTransitGatewayResourceShareName(cluster))
	if err != nil {
		logger.Error(err, "failed to delete resource share")
		return ctrl.Result{}, err
	}

	err = ramClient.DeleteResourceShare(ctx, getPrefixListResourceShareName(cluster))
	if err != nil {
		logger.Error(err, "failed to delete resource share")
		return ctrl.Result{}, err
	}

	err = r.clusterClient.RemoveFinalizer(ctx, cluster, FinalizerResourceShare)
	if err != nil {
		logger.Error(err, "failed to remove finalizer")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

type shareScope struct {
	cluster           *capa.AWSCluster
	managementCluster *capa.AWSCluster
	accountID         string
	ramClient         resolver.RAMClient
}

func (r *ShareReconciler) reconcileNormal(ctx context.Context, cluster *capa.AWSCluster) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	accountID, err := r.getAccountId(ctx, cluster)
	if err != nil {
		return ctrl.Result{}, err
	}

	managementCluster, err := r.clusterClient.GetAWSCluster(ctx, r.managementCluster)
	if err != nil {
		logger.Error(err, "failed to get management cluster")
		return ctrl.Result{}, err
	}

	ramClient, err := r.getRamClientFromCluster(ctx, managementCluster)
	if err != nil {
		return ctrl.Result{}, err
	}

	scope := shareScope{
		cluster:           cluster,
		managementCluster: managementCluster,
		accountID:         accountID,
		ramClient:         ramClient,
	}
	// We need to share the transit gateway separately from the prefix list, as
	// the networktopology reconciler needs to attach the transit gateway
	// first, before moving on to creating the prefix list. If the transit
	// gateway isn't shared it won't be visible in the WC's account
	result := ctrl.Result{}
	requeue, err := r.shareTransitGateway(ctx, scope)
	if err != nil {
		return ctrl.Result{}, err
	}
	if requeue {
		result.RequeueAfter = ResourceMissingRequeDuration
	}

	requeue, err = r.sharePrefixList(ctx, scope)
	if err != nil {
		return ctrl.Result{}, err
	}
	if requeue {
		result.RequeueAfter = ResourceMissingRequeDuration
	}

	return result, nil
}

func (r *ShareReconciler) getAccountId(ctx context.Context, cluster *capa.AWSCluster) (string, error) {
	logger := log.FromContext(ctx)

	identity, err := r.clusterClient.GetIdentity(ctx, cluster)
	if err != nil {
		logger.Error(err, "failed to get AWSCluster role identity")
		return "", err
	}

	roleArn, err := arn.Parse(identity.Spec.RoleArn)
	if err != nil {
		logger.Error(err, "failed to parse aws cluster role identity arn")
		return "", err
	}

	return roleArn.AccountID, nil
}

func getTransitGatewayResourceShareName(cluster *capa.AWSCluster) string {
	return fmt.Sprintf("%s-%s", cluster.Name, "transit-gateway")
}

func getPrefixListResourceShareName(cluster *capa.AWSCluster) string {
	return fmt.Sprintf("%s-%s", cluster.Name, "prefix-list")
}

func (r *ShareReconciler) shareTransitGateway(ctx context.Context, scope shareScope) (requeue bool, err error) {
	logger := log.FromContext(ctx)

	transitGatewayARN := getTransitGatewayARN(scope.cluster, scope.managementCluster)

	if transitGatewayARN == "" {
		logger.Info("transit gateway arn annotation not set yet")
		return true, nil
	}

	logger = logger.WithValues("transit-gateway-annotation", transitGatewayARN)

	err = r.clusterClient.AddFinalizer(ctx, scope.cluster, FinalizerResourceShare)
	if err != nil {
		logger.Error(err, "failed to add finalizer")
		return false, err
	}

	err = scope.ramClient.ApplyResourceShare(ctx, resolver.ResourceShare{
		Name: getTransitGatewayResourceShareName(scope.cluster),
		ResourceArns: []string{
			transitGatewayARN,
		},
		ExternalAccountID: scope.accountID,
	})
	if err != nil {
		logger.Error(err, "failed to apply resource share")
		return false, err
	}

	return false, nil
}

func (r *ShareReconciler) sharePrefixList(ctx context.Context, scope shareScope) (requeue bool, err error) {
	logger := log.FromContext(ctx)
	prefixListARN := getPrefixListARN(scope.cluster, scope.managementCluster)

	if prefixListARN == "" {
		logger.Info("prefix list arn annotation not set yet")
		return true, nil
	}

	logger = logger.WithValues("prefix-list-annotation", prefixListARN)

	err = scope.ramClient.ApplyResourceShare(ctx, resolver.ResourceShare{
		Name: getPrefixListResourceShareName(scope.cluster),
		ResourceArns: []string{
			prefixListARN,
		},
		ExternalAccountID: scope.accountID,
	})
	if err != nil {
		logger.Error(err, "failed to apply resource share")
		return false, err
	}

	return false, err
}

func resourcesStillInUse(cluster *capa.AWSCluster) bool {
	return controllerutil.ContainsFinalizer(cluster, FinalizerTransitGatewayAttachment) ||
		controllerutil.ContainsFinalizer(cluster, FinalizerPrefixListEntry)
}
