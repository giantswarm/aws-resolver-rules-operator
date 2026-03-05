package controllers

import (
	"context"

	"github.com/pkg/errors"
	capa "sigs.k8s.io/cluster-api-provider-aws/v2/api/v1beta2"
	eks "sigs.k8s.io/cluster-api-provider-aws/v2/controlplane/eks/api/v1beta2"
	"sigs.k8s.io/cluster-api/util/annotations"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/aws-resolver-rules-operator/pkg/resolver"
)

// EKSDnsReconciler reconciles AWSManagedControlPlane objects
type EKSDnsReconciler struct {
	clusterClient ClusterClient
	dnsZone       resolver.Zoner
	// managementClusterName is the name of the CR of the management cluster
	managementClusterName string
	// managementClusterNamespace is the namespace of the CR of the management cluster
	managementClusterNamespace string
}

func NewEKSDnsReconciler(clusterClient ClusterClient, dns resolver.Zoner, managementClusterName string, managementClusterNamespace string) *EKSDnsReconciler {
	return &EKSDnsReconciler{
		clusterClient:              clusterClient,
		dnsZone:                    dns,
		managementClusterName:      managementClusterName,
		managementClusterNamespace: managementClusterNamespace,
	}
}

func (r *EKSDnsReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling")
	defer logger.Info("Done reconciling")

	mcAWSCluster, err := r.clusterClient.GetAWSCluster(ctx, client.ObjectKey{Name: r.managementClusterName, Namespace: r.managementClusterNamespace})
	if err != nil {
		logger.Error(err, "Cant find management AWSCluster CR")
		return ctrl.Result{}, errors.WithStack(err)
	}

	mcIdentity, err := r.clusterClient.GetIdentity(ctx, mcAWSCluster.Spec.IdentityRef)
	if err != nil {
		logger.Error(err, "Cant find management AWSClusterRoleIdentity CR")
		return ctrl.Result{}, errors.WithStack(err)
	}

	awsManagedControlPlane, err := r.clusterClient.GetAWSManagedControlPlane(ctx, req.NamespacedName)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(client.IgnoreNotFound(err))
	}

	capiCluster, err := r.clusterClient.GetCluster(ctx, req.NamespacedName)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}

	if annotations.IsPaused(capiCluster, awsManagedControlPlane) {
		logger.Info("Infrastructure or core cluster is marked as paused, skipping")
		return ctrl.Result{}, nil
	}

	identity, err := r.clusterClient.GetIdentity(ctx, awsManagedControlPlane.Spec.IdentityRef)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}

	if identity == nil {
		logger.Info("AWSManagedControlPlane has no identityRef set, skipping")
		return ctrl.Result{}, nil
	}

	var delegationIdentity *capa.AWSClusterRoleIdentity
	if delegationIdentityName, ok := awsManagedControlPlane.Annotations[AWSDNSDelegationIdentityAnnotation]; ok && delegationIdentityName != "" {
		delegationIdentity, err = r.clusterClient.GetIdentity(ctx, &capa.AWSIdentityReference{Name: delegationIdentityName})
		if err != nil {
			logger.Error(err, "Failed to get delegation AWSClusterRoleIdentity", "identityName", delegationIdentityName)
			return ctrl.Result{}, errors.WithStack(err)
		}
	}

	cluster := buildClusterFromAWSManagedControlPlane(awsManagedControlPlane, identity, mcIdentity, delegationIdentity)

	if !capiCluster.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, awsManagedControlPlane, cluster)
	}

	return r.reconcileNormal(ctx, awsManagedControlPlane, cluster)
}

// reconcileNormal creates the hosted zone and the DNS records for the workload cluster.
// It will take care of dns delegation in the parent hosted zone when using public dns mode.
func (r *EKSDnsReconciler) reconcileNormal(ctx context.Context, awsManagedControlPlane *eks.AWSManagedControlPlane, cluster resolver.Cluster) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	err := r.clusterClient.AddAWSManagedControlPlaneFinalizer(ctx, awsManagedControlPlane, DnsFinalizer)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}

	err = r.dnsZone.CreateHostedZone(ctx, logger, cluster)
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// reconcileDelete deletes the hosted zone and the DNS records for the workload cluster.
// It will delete the delegation records in the parent hosted zone when using public dns mode.
func (r *EKSDnsReconciler) reconcileDelete(ctx context.Context, awsManagedControlPlane *eks.AWSManagedControlPlane, cluster resolver.Cluster) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(awsManagedControlPlane, DnsFinalizer) {
		return ctrl.Result{}, nil
	}

	logger := log.FromContext(ctx)

	err := r.dnsZone.DeleteHostedZone(ctx, logger, cluster)
	if err != nil {
		return ctrl.Result{}, err
	}

	err = r.clusterClient.RemoveAWSManagedControlPlaneFinalizer(ctx, awsManagedControlPlane, DnsFinalizer)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *EKSDnsReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named("eks_dnszone").
		For(&eks.AWSManagedControlPlane{}).
		WithEventFilter(
			predicate.Funcs{
				UpdateFunc: predicateToFilterAWSManagedControlPlaneResourceVersionChanges,
			},
		).
		Complete(r)
}
