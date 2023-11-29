package controllers

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/giantswarm/k8smetadata/pkg/annotation"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	capa "sigs.k8s.io/cluster-api-provider-aws/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/aws-resolver-rules-operator/pkg/resolver"
	"github.com/aws-resolver-rules-operator/pkg/util/annotations"
)

const FinalizerResourceShare = "network-topology.finalizers.giantswarm.io/share"

type ShareReconciler struct {
	ramClient     resolver.RAMClient
	clusterClient AWSClusterClient
}

func NewShareReconciler(clusterClient AWSClusterClient, ramClient resolver.RAMClient) *ShareReconciler {
	return &ShareReconciler{
		ramClient:     ramClient,
		clusterClient: clusterClient,
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
	if k8serrors.IsNotFound(err) {
		logger.Info("cluster no longer exists")
		return ctrl.Result{}, nil
	}
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
	logger := log.FromContext(ctx)

	if resourcesStillInUse(cluster) {
		logger.Info("Transit gateway and prefix list not yet cleaned up. Skipping...")
		return ctrl.Result{}, nil
	}

	err := r.ramClient.DeleteResourceShare(ctx, getResourceShareName(cluster, "transit-gateway"))
	if err != nil {
		logger.Error(err, "failed to delete resource share")
		return ctrl.Result{}, err
	}

	err = r.ramClient.DeleteResourceShare(ctx, getResourceShareName(cluster, "prefix-list"))
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
	accountID, err := r.getAccountId(ctx, cluster)
	if err != nil {
		return ctrl.Result{}, err
	}

	// We need to share the transit gateway separately from the prefix list, as
	// the networktopology reconciler needs to attach the transit gateway
	// first, before moving on to creating the prefix list. If the transit
	// gateway isn't shared it won't be visible in the WC's account
	err = r.shareTransitGateway(ctx, cluster, accountID)
	if err != nil {
		return ctrl.Result{}, err
	}

	err = r.sharePrefixList(ctx, cluster, accountID)
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
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

func getResourceShareName(cluster *capa.AWSCluster, resourceName string) string {
	return fmt.Sprintf("%s-%s", cluster.Name, resourceName)
}

func (r *ShareReconciler) shareTransitGateway(ctx context.Context, cluster *capa.AWSCluster, accountID string) error {
	logger := log.FromContext(ctx)
	transitGatewayAnnotation := annotations.GetNetworkTopologyTransitGateway(cluster)

	if transitGatewayAnnotation == "" {
		logger.Info("transit gateway arn annotation not set yet")
		return nil
	}

	logger = logger.WithValues("Annotation", transitGatewayAnnotation)

	transitGatewayARN, err := arn.Parse(transitGatewayAnnotation)
	if err != nil {
		logger.Error(err, "failed to parse transit gateway arn")
		return err
	}

	if accountID == transitGatewayARN.AccountID {
		logger.Info("transit gateway in same account as cluster, there is no need to share it using ram. Skipping")
		return nil
	}

	err = r.clusterClient.AddFinalizer(ctx, cluster, FinalizerResourceShare)
	if err != nil {
		logger.Error(err, "failed to add finalizer")
		return err
	}

	err = r.ramClient.ApplyResourceShare(ctx, resolver.ResourceShare{
		Name: getResourceShareName(cluster, "transit-gateway"),
		ResourceArns: []string{
			transitGatewayARN.String(),
		},
		ExternalAccountID: accountID,
	})
	if err != nil {
		logger.Error(err, "failed to apply resource share")
		return err
	}

	return nil
}

func (r *ShareReconciler) sharePrefixList(ctx context.Context, cluster *capa.AWSCluster, accountID string) error {
	logger := log.FromContext(ctx)
	prefixListAnnotation := annotations.GetNetworkTopologyPrefixList(cluster)
	if prefixListAnnotation == "" {
		logger.Info("prefix list arn annotation not set yet")
		return nil
	}

	logger = logger.WithValues("Annotation", prefixListAnnotation)

	prefixListARN, err := arn.Parse(prefixListAnnotation)
	if err != nil {
		logger.Error(err, "failed to parse prefix list arn", "Annotation", prefixListAnnotation)
		return err
	}

	if accountID == prefixListARN.AccountID {
		logger.Info("prefix list in same account as cluster, there is no need to share it using ram. Skipping")
		return nil
	}

	err = r.ramClient.ApplyResourceShare(ctx, resolver.ResourceShare{
		Name: getResourceShareName(cluster, "prefix-list"),
		ResourceArns: []string{
			prefixListARN.String(),
		},
		ExternalAccountID: accountID,
	})
	if err != nil {
		logger.Error(err, "failed to apply resource share")
		return err
	}

	return nil
}

func resourcesStillInUse(cluster *capa.AWSCluster) bool {
	return controllerutil.ContainsFinalizer(cluster, FinalizerTransitGatewayAttachment)
}
