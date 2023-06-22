package controllers

import (
	"fmt"

	capa "sigs.k8s.io/cluster-api-provider-aws/api/v1beta1"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"

	"github.com/aws-resolver-rules-operator/pkg/resolver"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

func buildCluster(awsCluster *capa.AWSCluster, cluster *capi.Cluster, identity *capa.AWSClusterRoleIdentity) resolver.Cluster {
	controlPlaneEndpoint := ""
	if cluster != nil && cluster.Spec.ControlPlaneEndpoint.Host != "" && cluster.Spec.ControlPlaneEndpoint.Port != 0 {
		controlPlaneEndpoint = fmt.Sprintf("%s:%d", cluster.Spec.ControlPlaneEndpoint.Host, cluster.Spec.ControlPlaneEndpoint.Port)
	}

	return resolver.Cluster{
		Name:                 awsCluster.Name,
		ControlPlaneEndpoint: controlPlaneEndpoint,
		Region:               awsCluster.Spec.Region,
		VPCCidr:              awsCluster.Spec.NetworkSpec.VPC.CidrBlock,
		VPCId:                awsCluster.Spec.NetworkSpec.VPC.ID,
		IAMRoleARN:           identity.Spec.RoleArn,
		Subnets:              getSubnetIds(awsCluster),
	}
}
