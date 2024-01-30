package controllers

import (
	"context"

	"github.com/pkg/errors"
	capa "sigs.k8s.io/cluster-api-provider-aws/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/annotations"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/aws-resolver-rules-operator/pkg/resolver"
)

// DnsReconciler reconciles AWSClusters.
// It creates a hosted zone that could be a private or a public one depending on the DNS mode of the workload cluster.
// The mode is selected using the `aws.giantswarm.io/dns-mode` annotation on the `AWSCluster` CR.
// It also creates three DNS records in the hosted zone
// - `api`: a dns record of type `A` pointing to the control plane Load Balancer
// - `*`: a CNAME pointing to the `ingress.$basedomain` record
//
// When the mode is public, it creates a record set in the parent's hosted zone so that dns delegation works.
//
// When the mode is private, the hosted zone is associated with a list of VPCs that can be specified using
// the `aws.giantswarm.io/dns-assign-additional-vpc` annotation on the `AWSCluster` CR.
//
// When a workload cluster is deleted, the hosted zone is deleted, together with the delegation on the parent zone.
type DnsReconciler struct {
	clusterClient ClusterClient
	dnsZone       resolver.Zoner
	// managementClusterName is the name of the CR of the management cluster
	managementClusterName string
	// managementClusterNamespace is the namespace of the CR of the management cluster
	managementClusterNamespace string
}

func NewDnsReconciler(clusterClient ClusterClient, dns resolver.Zoner, managementClusterName string, managementClusterNamespace string) *DnsReconciler {
	return &DnsReconciler{
		clusterClient:              clusterClient,
		dnsZone:                    dns,
		managementClusterName:      managementClusterName,
		managementClusterNamespace: managementClusterNamespace,
	}
}

func (r *DnsReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
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

	awsCluster, err := r.clusterClient.GetAWSCluster(ctx, req.NamespacedName)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(client.IgnoreNotFound(err))
	}

	capiCluster, err := r.clusterClient.GetCluster(ctx, req.NamespacedName)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}

	if annotations.IsPaused(capiCluster, awsCluster) {
		logger.Info("Infrastructure or core cluster is marked as paused, skipping")
		return ctrl.Result{}, nil
	}

	identity, err := r.clusterClient.GetIdentity(ctx, awsCluster.Spec.IdentityRef)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}

	if identity == nil {
		logger.Info("AWSCluster has no identityRef set, skipping")
		return ctrl.Result{}, nil
	}

	cluster := buildClusterFromAWSCluster(awsCluster, identity, mcIdentity)

	if !capiCluster.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, awsCluster, cluster)
	}

	return r.reconcileNormal(ctx, awsCluster, cluster)
}

// reconcileNormal creates the hosted zone and the DNS records for the workload cluster.
// It will take care of dns delegation in the parent hosted zone when using public dns mode.
func (r *DnsReconciler) reconcileNormal(ctx context.Context, awsCluster *capa.AWSCluster, cluster resolver.Cluster) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	err := r.clusterClient.AddAWSClusterFinalizer(ctx, awsCluster, DnsFinalizer)
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
func (r *DnsReconciler) reconcileDelete(ctx context.Context, awsCluster *capa.AWSCluster, cluster resolver.Cluster) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(awsCluster, DnsFinalizer) {
		return ctrl.Result{}, nil
	}

	logger := log.FromContext(ctx)

	err := r.dnsZone.DeleteHostedZone(ctx, logger, cluster)
	if err != nil {
		return ctrl.Result{}, err
	}

	err = r.clusterClient.RemoveAWSClusterFinalizer(ctx, awsCluster, DnsFinalizer)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DnsReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named("dnszone").
		For(&capa.AWSCluster{}).
		WithEventFilter(
			predicate.Funcs{
				UpdateFunc: predicateToFilterAWSClusterResourceVersionChanges,
			},
		).
		Complete(r)
}
