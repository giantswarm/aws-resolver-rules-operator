package controllers

import (
	capa "sigs.k8s.io/cluster-api-provider-aws/api/v1beta1"

	"github.com/aws-resolver-rules-operator/pkg/resolver"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

func buildCluster(awsCluster *capa.AWSCluster, identity *capa.AWSClusterRoleIdentity) resolver.Cluster {
	return resolver.Cluster{
		Name:       awsCluster.Name,
		Region:     awsCluster.Spec.Region,
		VPCCidr:    awsCluster.Spec.NetworkSpec.VPC.CidrBlock,
		VPCId:      awsCluster.Spec.NetworkSpec.VPC.ID,
		IAMRoleARN: identity.Spec.RoleArn,
		Subnets:    getSubnetIds(awsCluster),
	}
}
