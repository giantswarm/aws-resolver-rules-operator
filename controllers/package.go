package controllers

import (
	"context"
	"strings"

	gsannotations "github.com/giantswarm/k8smetadata/pkg/annotation"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
	capa "sigs.k8s.io/cluster-api-provider-aws/api/v1beta1"
	eks "sigs.k8s.io/cluster-api-provider-aws/controlplane/eks/api/v1beta1"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"

	"github.com/aws-resolver-rules-operator/pkg/resolver"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

const (
	DnsFinalizer = "capa-operator.finalizers.giantswarm.io/dns-controller"
)

func buildClusterFromAWSCluster(awsCluster *capa.AWSCluster, identity *capa.AWSClusterRoleIdentity, mcIdentity *capa.AWSClusterRoleIdentity) resolver.Cluster {
	cluster := resolver.Cluster{
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

	return cluster
}

func buildClusterFromAWSManagedControlPlane(awsManagedControlPlane *eks.AWSManagedControlPlane, identity *capa.AWSClusterRoleIdentity, mcIdentity *capa.AWSClusterRoleIdentity) resolver.Cluster {
	cluster := resolver.Cluster{
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

	return cluster
}

// getSubnetIds will fetch the subnet ids for the subnets in the spec that contain certain tag.
// These are the subnets in your VPC that you forward DNS queries to.
func getSubnetIds(subnets capa.Subnets) []string {
	var subnetIds []string
	for _, subnet := range subnets {
		if _, ok := subnet.Tags["subnet.giantswarm.io/endpoints"]; ok {
			subnetIds = append(subnetIds, subnet.ID)
		}
	}

	return subnetIds
}

// getBastionIp tries to find a bastion machine in this cluster and fetch its IP address from the status field.
// It will return the internal IP address when using private VPC mode, or an external IP address otherwise.
func getBastionIp(ctx context.Context, clusterClient ClusterClient, cluster resolver.Cluster) (string, error) {
	bastionMachine, err := clusterClient.GetBastionMachine(ctx, cluster.Name)
	if err != nil {
		return "", errors.WithStack(err)
	}

	addressType := capi.MachineExternalIP
	if cluster.IsVpcModePrivate {
		addressType = capi.MachineInternalIP
	}

	for _, addr := range bastionMachine.Status.Addresses {
		if addr.Type == addressType {
			return addr.Address, nil
		}
	}

	return "", nil
}

//counterfeiter:generate . ClusterClient
type ClusterClient interface {
	GetAWSCluster(context.Context, types.NamespacedName) (*capa.AWSCluster, error)
	GetAWSManagedControlPlane(context.Context, types.NamespacedName) (*eks.AWSManagedControlPlane, error)
	GetCluster(context.Context, types.NamespacedName) (*capi.Cluster, error)
	AddAWSClusterFinalizer(ctx context.Context, cluster *capa.AWSCluster, finalizer string) error
	AddAWSManagedControlPlaneFinalizer(ctx context.Context, awsManagedControlPlane *eks.AWSManagedControlPlane, finalizer string) error
	Unpause(context.Context, *capa.AWSCluster, *capi.Cluster) error
	RemoveAWSClusterFinalizer(ctx context.Context, awsCluster *capa.AWSCluster, finalizer string) error
	RemoveAWSManagedControlPlaneFinalizer(ctx context.Context, awsManagedControlPlane *eks.AWSManagedControlPlane, finalizer string) error
	GetIdentity(context.Context, *capa.AWSIdentityReference) (*capa.AWSClusterRoleIdentity, error)
	MarkConditionTrue(context.Context, *capi.Cluster, capi.ConditionType) error
	GetBastionMachine(ctx context.Context, clusterName string) (*capi.Machine, error)
}
