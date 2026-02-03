package controllers

import (
	"context"
	"strings"

	gsannotations "github.com/giantswarm/k8smetadata/pkg/annotation"
	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/types"
	capa "sigs.k8s.io/cluster-api-provider-aws/v2/api/v1beta2"
	eks "sigs.k8s.io/cluster-api-provider-aws/v2/controlplane/eks/api/v1beta2"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/event"

	"github.com/aws-resolver-rules-operator/pkg/resolver"
	"github.com/aws-resolver-rules-operator/pkg/util/annotations"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

const (
	DnsFinalizer = "capa-operator.finalizers.giantswarm.io/dns-controller"

	// DNSHostedZoneNameAnnotation is the annotation key for specifying a custom DNS hosted zone name.
	// The value should be the full hosted zone name (e.g., "my-cluster.other.domain.com").
	DNSHostedZoneNameAnnotation = "giantswarm.io/dns-hosted-zone-name"

	// AWSDNSDelegationIdentityAnnotation is the annotation key for specifying the name of an
	// AWSClusterRoleIdentity to use for DNS delegation to the parent zone.
	AWSDNSDelegationIdentityAnnotation = "aws.giantswarm.io/dns-delegation-identity"
)

func buildClusterFromAWSCluster(awsCluster *capa.AWSCluster, identity *capa.AWSClusterRoleIdentity, mcIdentity *capa.AWSClusterRoleIdentity, delegationIdentity *capa.AWSClusterRoleIdentity) resolver.Cluster {
	cluster := resolver.Cluster{
		AdditionalTags:       awsCluster.Spec.AdditionalTags,
		Name:                 awsCluster.Name,
		ControlPlaneEndpoint: awsCluster.Spec.ControlPlaneEndpoint.Host,
		Region:               awsCluster.Spec.Region,
		IsDnsModePrivate:     awsCluster.Annotations[gsannotations.AWSDNSMode] == gsannotations.DNSModePrivate,
		IsVpcModePrivate:     awsCluster.Annotations[gsannotations.AWSVPCMode] == gsannotations.AWSVPCModePrivate,
		IsEKS:                false,
		VPCCidr:              awsCluster.Spec.NetworkSpec.VPC.CidrBlock,
		VPCId:                awsCluster.Spec.NetworkSpec.VPC.ID,
		IAMRoleARN:           identity.Spec.RoleArn,
		Subnets:              getSubnetIds(awsCluster.Spec.NetworkSpec.Subnets),
	}

	additionalVpcsAnnotation, ok := awsCluster.Annotations[gsannotations.AWSDNSAdditionalVPC]
	if ok {
		cluster.VPCsToAssociateToHostedZone = strings.Split(additionalVpcsAnnotation, ",")
	}
	if mcIdentity != nil {
		cluster.MCIAMRoleARN = mcIdentity.Spec.RoleArn
	}

	cluster.CustomHostedZoneName = awsCluster.Annotations[DNSHostedZoneNameAnnotation]
	if delegationIdentity != nil {
		cluster.DelegationIAMRoleARN = delegationIdentity.Spec.RoleArn
	}

	return cluster
}

func buildClusterFromAWSManagedControlPlane(awsManagedControlPlane *eks.AWSManagedControlPlane, identity *capa.AWSClusterRoleIdentity, mcIdentity *capa.AWSClusterRoleIdentity, delegationIdentity *capa.AWSClusterRoleIdentity) resolver.Cluster {
	cluster := resolver.Cluster{
		AdditionalTags:       awsManagedControlPlane.Spec.AdditionalTags,
		Name:                 awsManagedControlPlane.Name,
		ControlPlaneEndpoint: awsManagedControlPlane.Spec.ControlPlaneEndpoint.Host,
		Region:               awsManagedControlPlane.Spec.Region,
		IsDnsModePrivate:     awsManagedControlPlane.Annotations[gsannotations.AWSDNSMode] == gsannotations.DNSModePrivate,
		IsVpcModePrivate:     awsManagedControlPlane.Annotations[gsannotations.AWSVPCMode] == gsannotations.AWSVPCModePrivate,
		IsEKS:                true,
		VPCCidr:              awsManagedControlPlane.Spec.NetworkSpec.VPC.CidrBlock,
		VPCId:                awsManagedControlPlane.Spec.NetworkSpec.VPC.ID,
		IAMRoleARN:           identity.Spec.RoleArn,
		Subnets:              getSubnetIds(awsManagedControlPlane.Spec.NetworkSpec.Subnets),
	}

	additionalVpcsAnnotation, ok := awsManagedControlPlane.Annotations[gsannotations.AWSDNSAdditionalVPC]
	if ok {
		cluster.VPCsToAssociateToHostedZone = strings.Split(additionalVpcsAnnotation, ",")
	}
	if mcIdentity != nil {
		cluster.MCIAMRoleARN = mcIdentity.Spec.RoleArn
	}

	cluster.CustomHostedZoneName = awsManagedControlPlane.Annotations[DNSHostedZoneNameAnnotation]
	if delegationIdentity != nil {
		cluster.DelegationIAMRoleARN = delegationIdentity.Spec.RoleArn
	}

	return cluster
}

// getSubnetIds will fetch the subnet ids for the subnets in the spec that contain certain tag.
// These are the subnets in your VPC that you forward DNS queries to.
func getSubnetIds(subnets capa.Subnets) []string {
	var subnetIds []string
	for _, subnet := range subnets {
		if _, ok := subnet.Tags["subnet.giantswarm.io/endpoints"]; ok {
			subnetIds = append(subnetIds, subnet.ResourceID)
		}
	}

	return subnetIds
}

//counterfeiter:generate . ClusterClient
type ClusterClient interface {
	GetAWSCluster(context.Context, types.NamespacedName) (*capa.AWSCluster, error)
	GetAWSManagedControlPlane(context.Context, types.NamespacedName) (*eks.AWSManagedControlPlane, error)
	GetCluster(context.Context, types.NamespacedName) (*capi.Cluster, error)
	AddAWSClusterFinalizer(ctx context.Context, cluster *capa.AWSCluster, finalizer string) error
	AddAWSManagedControlPlaneFinalizer(ctx context.Context, awsManagedControlPlane *eks.AWSManagedControlPlane, finalizer string) error
	AddClusterFinalizer(context.Context, *capi.Cluster, string) error
	Unpause(context.Context, *capa.AWSCluster, *capi.Cluster) error
	RemoveAWSClusterFinalizer(ctx context.Context, awsCluster *capa.AWSCluster, finalizer string) error
	RemoveAWSManagedControlPlaneFinalizer(ctx context.Context, awsManagedControlPlane *eks.AWSManagedControlPlane, finalizer string) error
	RemoveClusterFinalizer(context.Context, *capi.Cluster, string) error
	GetIdentity(context.Context, *capa.AWSIdentityReference) (*capa.AWSClusterRoleIdentity, error)
	MarkConditionTrue(context.Context, *capi.Cluster, capi.ConditionType) error
}

// predicateToFilterAWSClusterResourceVersionChanges is a function to avoid reconciling if the event triggering the reconciliation
// is related to incremental status updates for AWSCluster resources only
func predicateToFilterAWSClusterResourceVersionChanges(e event.UpdateEvent) bool {
	if e.ObjectOld.GetObjectKind().GroupVersionKind().Kind != "AWSCluster" {
		return true
	}

	oldCluster := e.ObjectOld.(*capa.AWSCluster).DeepCopy()
	newCluster := e.ObjectNew.(*capa.AWSCluster).DeepCopy()

	oldCluster.Status = capa.AWSClusterStatus{}
	newCluster.Status = capa.AWSClusterStatus{}

	oldCluster.ResourceVersion = ""
	newCluster.ResourceVersion = ""

	return !cmp.Equal(oldCluster, newCluster)
}

// predicateToFilterAWSManagedControlPlaneResourceVersionChanges is a function to avoid reconciling if the event triggering the reconciliation
// is related to incremental status updates for AWSManagedControlPlane resources only
func predicateToFilterAWSManagedControlPlaneResourceVersionChanges(e event.UpdateEvent) bool {
	if e.ObjectOld.GetObjectKind().GroupVersionKind().Kind != "AWSManagedControlPlane" {
		return true
	}

	oldCluster := e.ObjectOld.(*eks.AWSManagedControlPlane).DeepCopy()
	newCluster := e.ObjectNew.(*eks.AWSManagedControlPlane).DeepCopy()

	oldCluster.Status = eks.AWSManagedControlPlaneStatus{}
	newCluster.Status = eks.AWSManagedControlPlaneStatus{}

	oldCluster.ResourceVersion = ""
	newCluster.ResourceVersion = ""

	return !cmp.Equal(oldCluster, newCluster)
}

func getTransitGatewayARN(cluster, managementCluster *capa.AWSCluster) string {
	transitGatewayARN := annotations.GetNetworkTopologyTransitGateway(cluster)
	if transitGatewayARN != "" {
		return transitGatewayARN
	}

	return annotations.GetNetworkTopologyTransitGateway(managementCluster)
}

func getPrefixListARN(cluster, managementCluster *capa.AWSCluster) string {
	prefixListARN := annotations.GetNetworkTopologyPrefixList(cluster)
	if prefixListARN != "" {
		return prefixListARN
	}

	return annotations.GetNetworkTopologyPrefixList(managementCluster)
}
