package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/giantswarm/k8smetadata/pkg/annotation"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
	k8stypes "k8s.io/apimachinery/pkg/types"
	capa "sigs.k8s.io/cluster-api-provider-aws/v2/api/v1beta2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/aws-resolver-rules-operator/pkg/resolver"
	"github.com/aws-resolver-rules-operator/pkg/util/annotations"
)

const (
	FinalizerResourceShare       = "network-topology.finalizers.giantswarm.io/share"
	ResourceMissingRequeDuration = time.Minute
)

type ShareReconciler struct {
	ramClient         resolver.RAMClient
	clusterClient     AWSClusterClient
	managementCluster k8stypes.NamespacedName
}

func NewShareReconciler(
	managementCluster types.NamespacedName,
	clusterClient AWSClusterClient,
	ramClient resolver.RAMClient,
) *ShareReconciler {
	return &ShareReconciler{
		ramClient:         ramClient,
		clusterClient:     clusterClient,
		managementCluster: managementCluster,
	}
}

func (r *ShareReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named("share-reconciler").
		For(&capa.AWSCluster{}).
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

func (r *ShareReconciler) reconcileDelete(ctx context.Context, cluster *capa.AWSCluster) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(cluster, FinalizerResourceShare) {
		return ctrl.Result{}, nil
	}

	logger := log.FromContext(ctx)

	if resourcesStillInUse(cluster) {
		logger.Info("Transit gateway and prefix list not yet cleaned up. Skipping...")
		return ctrl.Result{}, nil
	}

	err := r.ramClient.DeleteResourceShare(ctx, getTransitGatewayResourceShareName(cluster))
	if err != nil {
		logger.Error(err, "failed to delete resource share")
		return ctrl.Result{}, err
	}

	err = r.ramClient.DeleteResourceShare(ctx, getPrefixListResourceShareName(cluster))
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

func (r *ShareReconciler) reconcileNormal(ctx context.Context, cluster *capa.AWSCluster) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	accountID, err := r.getAccountId(ctx, cluster)
	if err != nil {
		return ctrl.Result{}, err
	}

	managementCluster, err := r.clusterClient.GetAWSCluster(ctx, r.managementCluster)
	if err != nil {
		logger.Error(err, "failed to get management cluster")
		return ctrl.Result{}, errors.WithStack(err)
	}

	// We need to share the transit gateway separately from the prefix list, as
	// the networktopology reconciler needs to attach the transit gateway
	// first, before moving on to creating the prefix list. If the transit
	// gateway isn't shared it won't be visible in the WC's account
	result := ctrl.Result{}
	requeue, err := r.shareTransitGateway(ctx, cluster, managementCluster, accountID)
	if err != nil {
		return ctrl.Result{}, err
	}
	if requeue {
		result.RequeueAfter = ResourceMissingRequeDuration
	}

	requeue, err = r.sharePrefixList(ctx, cluster, managementCluster, accountID)
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

func (r *ShareReconciler) shareTransitGateway(ctx context.Context, cluster, managementCluster *capa.AWSCluster, accountID string) (requeue bool, err error) {
	logger := log.FromContext(ctx)

	transitGatewayARN := getTransitGatewayARN(cluster, managementCluster)

	if transitGatewayARN == "" {
		logger.Info("transit gateway arn annotation not set yet")
		return true, nil
	}

	logger = logger.WithValues("transit-gateway-annotation", transitGatewayARN)

	err = r.clusterClient.AddFinalizer(ctx, cluster, FinalizerResourceShare)
	if err != nil {
		logger.Error(err, "failed to add finalizer")
		return false, err
	}

	err = r.ramClient.ApplyResourceShare(ctx, resolver.ResourceShare{
		Name: getTransitGatewayResourceShareName(cluster),
		ResourceArns: []string{
			transitGatewayARN,
		},
		ExternalAccountID: accountID,
	})
	if err != nil {
		logger.Error(err, "failed to apply resource share")
		return false, err
	}

	return false, nil
}

func (r *ShareReconciler) sharePrefixList(ctx context.Context, cluster, managementCluster *capa.AWSCluster, accountID string) (requeue bool, err error) {
	logger := log.FromContext(ctx)
	prefixListARN := getPrefixListARN(cluster, managementCluster)

	if prefixListARN == "" {
		logger.Info("prefix list arn annotation not set yet")
		return true, nil
	}

	logger = logger.WithValues("prefix-list-annotation", prefixListARN)

	err = r.ramClient.ApplyResourceShare(ctx, resolver.ResourceShare{
		Name: getPrefixListResourceShareName(cluster),
		ResourceArns: []string{
			prefixListARN,
		},
		ExternalAccountID: accountID,
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
