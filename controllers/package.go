package controllers

import (
	"strings"

	gsannotations "github.com/giantswarm/k8smetadata/pkg/annotation"
	capa "sigs.k8s.io/cluster-api-provider-aws/api/v1beta1"
	eks "sigs.k8s.io/cluster-api-provider-aws/controlplane/eks/api/v1beta1"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"

	"github.com/aws-resolver-rules-operator/pkg/resolver"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

func buildClusterFromAWSCluster(awsCluster *capa.AWSCluster, identity *capa.AWSClusterRoleIdentity) resolver.Cluster {
	cluster := resolver.Cluster{
		Name:                 awsCluster.Name,
		ControlPlaneEndpoint: awsCluster.Spec.ControlPlaneEndpoint.Host,
		Region:               awsCluster.Spec.Region,
		IsDnsModePrivate:     awsCluster.Annotations[gsannotations.AWSDNSMode] == gsannotations.DNSModePrivate,
		IsVpcModePrivate:     awsCluster.Annotations[gsannotations.AWSVPCMode] == gsannotations.AWSVPCModePrivate,
		VPCCidr:              awsCluster.Spec.NetworkSpec.VPC.CidrBlock,
		VPCId:                awsCluster.Spec.NetworkSpec.VPC.ID,
		IAMRoleARN:           identity.Spec.RoleArn,
		Subnets:              getSubnetIds(awsCluster.Spec.NetworkSpec.Subnets),
	}

	additionalVpcsAnnotation, ok := awsCluster.Annotations[gsannotations.AWSDNSAdditionalVPC]
	if ok {
		cluster.VPCsToAssociateToHostedZone = strings.Split(additionalVpcsAnnotation, ",")
	}

	return cluster
}

func buildClusterFromAWSManagedControlPlane(awsManagedControlPlane *eks.AWSManagedControlPlane, identity *capa.AWSClusterRoleIdentity) resolver.Cluster {
	cluster := resolver.Cluster{
		Name:                 awsManagedControlPlane.Name,
		ControlPlaneEndpoint: awsManagedControlPlane.Spec.ControlPlaneEndpoint.Host,
		Region:               awsManagedControlPlane.Spec.Region,
		IsDnsModePrivate:     awsManagedControlPlane.Annotations[gsannotations.AWSDNSMode] == gsannotations.DNSModePrivate,
		IsVpcModePrivate:     awsManagedControlPlane.Annotations[gsannotations.AWSVPCMode] == gsannotations.AWSVPCModePrivate,
		VPCCidr:              awsManagedControlPlane.Spec.NetworkSpec.VPC.CidrBlock,
		VPCId:                awsManagedControlPlane.Spec.NetworkSpec.VPC.ID,
		IAMRoleARN:           identity.Spec.RoleArn,
		Subnets:              getSubnetIds(awsManagedControlPlane.Spec.NetworkSpec.Subnets),
	}

	additionalVpcsAnnotation, ok := awsManagedControlPlane.Annotations[gsannotations.AWSDNSAdditionalVPC]
	if ok {
		cluster.VPCsToAssociateToHostedZone = strings.Split(additionalVpcsAnnotation, ",")
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

func isCAPA(capiCluster *capi.Cluster) bool {
	return capiCluster.Spec.InfrastructureRef.Kind == "AWSCluster"
}

func isEKS(capiCluster *capi.Cluster) bool {
	return capiCluster.Spec.InfrastructureRef.Kind == "AWSManagedCluster"
}
