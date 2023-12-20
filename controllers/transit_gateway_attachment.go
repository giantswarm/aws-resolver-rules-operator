package controllers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"
	k8stypes "k8s.io/apimachinery/pkg/types"
	capa "sigs.k8s.io/cluster-api-provider-aws/api/v1beta1"
	capiannotations "sigs.k8s.io/cluster-api/util/annotations"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/aws-resolver-rules-operator/pkg/conditions"
	gserrors "github.com/aws-resolver-rules-operator/pkg/errors"
	"github.com/aws-resolver-rules-operator/pkg/resolver"
	"github.com/aws-resolver-rules-operator/pkg/util/annotations"
)

const (
	FinalizerTransitGatewayAttachment     = "network-topology.finalizers.giantswarm.io/transit-gateway-attachment"
	TagSubnetTGWAttachements              = "subnet.giantswarm.io/tgw"
	RequeueDurationTransitGatewayNotReady = 10 * time.Second
)

type TransitGatewayAttachmentReconciler struct {
	managementCluster k8stypes.NamespacedName
	clusterClient     AWSClusterClient
	clients           resolver.AWSClients
}

func NewTransitGatewayAttachmentReconciler(
	managementCluster k8stypes.NamespacedName,
	clusterClient AWSClusterClient,
	clients resolver.AWSClients,
) *TransitGatewayAttachmentReconciler {
	return &TransitGatewayAttachmentReconciler{
		managementCluster: managementCluster,
		clusterClient:     clusterClient,
		clients:           clients,
	}
}

func (r *TransitGatewayAttachmentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&capa.AWSCluster{}).
		WithEventFilter(
			predicate.Funcs{
				UpdateFunc: predicateToFilterAWSClusterResourceVersionChanges,
			},
		).
		Named("transit-gateway-attachment").
		Complete(r)
}

type attachmentScope struct {
	cluster              *capa.AWSCluster
	transitGatewayClient resolver.TransitGatewayClient
	transitGatewayARN    string
}

func (r *TransitGatewayAttachmentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
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

	transitGatewayARN := getTransitGatewayARN(cluster, managementCluster)

	if transitGatewayARN == "" {
		logger.Info("Transit gateway not created yet. Skipping...")
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}

	identity, err := r.clusterClient.GetIdentity(ctx, cluster)
	if err != nil {
		logger.Error(err, "failed to get cluster identity")
		return ctrl.Result{}, errors.WithStack(err)
	}

	attacher, err := r.clients.NewTransitGatewayClient(cluster.Spec.Region, identity.Spec.RoleArn)
	if err != nil {
		logger.Error(err, "failed to create transit gateway client")
		return ctrl.Result{}, errors.WithStack(err)
	}

	defer func() {
		_ = r.clusterClient.UpdateStatus(ctx, cluster)
	}()

	scope := attachmentScope{
		cluster:              cluster,
		transitGatewayClient: attacher,
		transitGatewayARN:    transitGatewayARN,
	}

	if !cluster.DeletionTimestamp.IsZero() {
		logger.Info("Reconciling delete")
		return r.reconcileDelete(ctx, scope)
	}

	return r.reconcileNormal(ctx, scope)
}

func (r *TransitGatewayAttachmentReconciler) reconcileNormal(ctx context.Context, scope attachmentScope) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if scope.cluster.Spec.NetworkSpec.VPC.ID == "" {
		logger.Info("VPC not created yet. Skipping...")
		return ctrl.Result{}, nil
	}

	subnets, err := getSubnets(scope.cluster)
	if err != nil {
		logger.Error(err, "Failed to get subnet IDs")
		return ctrl.Result{}, err
	}

	err = r.clusterClient.AddFinalizer(ctx, scope.cluster, FinalizerTransitGatewayAttachment)
	if err != nil {
		logger.Error(err, "Failed to add finalizer")
		return ctrl.Result{}, errors.WithStack(err)
	}

	attachment := resolver.TransitGatewayAttachment{
		TransitGatewayARN: scope.transitGatewayARN,
		VPCID:             scope.cluster.Spec.NetworkSpec.VPC.ID,
		SubnetIDs:         subnets,
		Tags:              getAttachmentTags(scope.cluster),
	}
	err = scope.transitGatewayClient.ApplyAttachment(ctx, attachment)
	retryableErr := &gserrors.RetryableError{}
	if errors.As(err, &retryableErr) {
		logger.Info(fmt.Sprintf("Failed to apply attachment %s", retryableErr.Error()))
		return ctrl.Result{RequeueAfter: retryableErr.RetryAfter()}, nil
	}
	if err != nil {
		logger.Error(err, "Failed to apply attachment")
		return ctrl.Result{}, err
	}

	conditions.MarkReady(scope.cluster, conditions.TransitGatewayAttached)
	return ctrl.Result{}, nil
}

func (r *TransitGatewayAttachmentReconciler) reconcileDelete(ctx context.Context, scope attachmentScope) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	attachment := resolver.TransitGatewayAttachment{
		TransitGatewayARN: scope.transitGatewayARN,
		VPCID:             scope.cluster.Spec.NetworkSpec.VPC.ID,
	}
	err := scope.transitGatewayClient.Detach(ctx, attachment)
	if err != nil {
		logger.Error(err, "Failed to detach transit gateway")
		return ctrl.Result{}, errors.WithStack(err)
	}

	err = r.clusterClient.RemoveFinalizer(ctx, scope.cluster, FinalizerTransitGatewayAttachment)
	if err != nil {
		logger.Error(err, "Failed to delete finalizer")
		return ctrl.Result{}, errors.WithStack(err)
	}
	return ctrl.Result{}, nil
}

func getSubnets(cluster *capa.AWSCluster) ([]string, error) {
	subnets := cluster.Spec.NetworkSpec.Subnets
	subnets = filterTGWSubnets(subnets)

	subnetIDs := []string{}
	availabilityZones := map[string]bool{}
	for _, s := range subnets {
		if s.ID == "" {
			return nil, fmt.Errorf("not all subnets have been created")
		}

		if !strings.HasPrefix(s.ID, "subnet-") {
			// The meaning of the ID field changed in https://github.com/kubernetes-sigs/cluster-api-provider-aws/pull/4474 and a new
			// field `ResourceID` was added to denote the AWS subnet identifier (`subnet-...`).
			// After the change in CAPA, the `ID` field is supposed to not start with `subnet-` for
			// CAPA-managed subnets.
			// TODO: Implement this breaking change of meaning.
			return nil, fmt.Errorf("support for newer CAPA versions' ResourceID field not implemented yet")
		}

		if !availabilityZones[s.AvailabilityZone] {
			subnetIDs = append(subnetIDs, s.ID)
			availabilityZones[s.AvailabilityZone] = true
		}
	}

	if len(subnetIDs) == 0 {
		return nil, fmt.Errorf("cluster has no subnets")
	}

	return subnetIDs, nil
}

func filterTGWSubnets(subnets []capa.SubnetSpec) []capa.SubnetSpec {
	filtered := []capa.SubnetSpec{}
	for _, s := range subnets {
		_, ok := s.Tags[TagSubnetTGWAttachements]
		if ok {
			filtered = append(filtered, s)
		}
	}

	if len(filtered) == 0 {
		return subnets
	}

	return filtered
}

func getAttachmentTags(cluster *capa.AWSCluster) map[string]string {
	tags := cluster.Spec.AdditionalTags
	if tags == nil {
		tags = map[string]string{}
	}
	tags["Name"] = cluster.Name
	tags[fmt.Sprintf("kubernetes.io/cluster/%s", cluster.Name)] = "owned"

	return tags
}
