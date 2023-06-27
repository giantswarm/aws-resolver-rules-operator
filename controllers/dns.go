package controllers

import (
	"context"
	"strings"

	gsannotations "github.com/giantswarm/k8smetadata/pkg/annotation"
	"github.com/pkg/errors"
	capa "sigs.k8s.io/cluster-api-provider-aws/api/v1beta1"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/aws-resolver-rules-operator/pkg/resolver"
)

const (
	DnsFinalizer = "capa-operator.finalizers.giantswarm.io/dns-controller"
)

// DnsReconciler reconciles AWSClusters.
// It creates a hosted zone that could be a private or a public one depending on the DNS mode of the workload cluster.
// The mode is selected using the `aws.giantswarm.io/dns-mode` annotation on the `AWSCluster` CR.
// It also creates three DNS records in the hosted zone
// - `api`: a dns record of type `A` pointing to the control plane Load Balancer
// - `bastion1`: a dns record of type `A` pointing to the bastion `Machine` IP
// - `*`: a CNAME pointing to the `ingress.$basedomain` record
//
// When the mode is public, it creates a record set in the parent's hosted zone so that dns delegation works.
//
// When the mode is private, the hosted zone is associated with a list of VPCs that can be specified using
// the `aws.giantswarm.io/dns-assign-additional-vpc` annotation on the `AWSCluster` CR.
//
// When a workload cluster is deleted, the hosted zone is deleted, together with the delegation on the parent zone.
type DnsReconciler struct {
	awsClusterClient AWSClusterClient
	dnsZone          resolver.Zoner
}

func NewDnsReconciler(awsClusterClient AWSClusterClient, dns resolver.Zoner) *DnsReconciler {
	return &DnsReconciler{
		awsClusterClient: awsClusterClient,
		dnsZone:          dns,
	}
}

func (r *DnsReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
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

	cluster, err := r.awsClusterClient.GetCluster(ctx, req.NamespacedName)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(client.IgnoreNotFound(err))
	}

	if identity == nil {
		logger.Info("AWSCluster has no identityRef set, skipping")
		return ctrl.Result{}, nil
	}

	if !awsCluster.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, awsCluster, cluster, identity)
	}

	return r.reconcileNormal(ctx, awsCluster, cluster, identity)
}

// reconcileNormal creates the hosted zone and the DNS records for the workload cluster.
// It will take care of dns delegation in the parent hosted zone when using public dns mode.
func (r *DnsReconciler) reconcileNormal(ctx context.Context, awsCluster *capa.AWSCluster, capiCluster *capi.Cluster, identity *capa.AWSClusterRoleIdentity) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	err := r.awsClusterClient.AddFinalizer(ctx, awsCluster, DnsFinalizer)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}

	addrType := capi.MachineExternalIP
	if awsCluster.Annotations[gsannotations.AWSVPCMode] == gsannotations.AWSVPCModePrivate {
		addrType = capi.MachineInternalIP
	}

	bastionIp, err := r.awsClusterClient.GetBastionIp(ctx, awsCluster, addrType)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}

	cluster := buildCluster(awsCluster, identity)
	cluster.BastionIp = bastionIp
	cluster.ControlPlaneEndpoint = capiCluster.Spec.ControlPlaneEndpoint.Host

	annotation, ok := awsCluster.Annotations[gsannotations.AWSDNSMode]
	if ok && annotation == gsannotations.DNSModePrivate {
		additionalVPCToAssociate := strings.Split(awsCluster.Annotations[gsannotations.AWSDNSAdditionalVPC], ",")
		err = r.dnsZone.CreatePrivateHostedZone(ctx, logger, cluster, additionalVPCToAssociate)
		if err != nil {
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	}

	err = r.dnsZone.CreatePublicHostedZone(ctx, logger, cluster)
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// reconcileDelete deletes the hosted zone and the DNS records for the workload cluster.
// It will delete the delegation records in the parent hosted zone when using public dns mode.
func (r *DnsReconciler) reconcileDelete(ctx context.Context, awsCluster *capa.AWSCluster, capiCluster *capi.Cluster, identity *capa.AWSClusterRoleIdentity) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	cluster := buildCluster(awsCluster, identity)

	err := r.dnsZone.DeleteHostedZone(ctx, logger, cluster)
	if err != nil {
		return ctrl.Result{}, err
	}

	err = r.awsClusterClient.RemoveFinalizer(ctx, awsCluster, DnsFinalizer)
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
		Complete(r)
}
