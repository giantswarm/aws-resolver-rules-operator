package key

import (
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

func IsCAPA(capiCluster *capi.Cluster) bool {
	return capiCluster.Spec.InfrastructureRef != nil && capiCluster.Spec.InfrastructureRef.Kind == "AWSCluster"
}

func IsEKS(capiCluster *capi.Cluster) bool {
	return capiCluster.Spec.InfrastructureRef != nil && capiCluster.Spec.InfrastructureRef.Kind == "AWSManagedCluster"
}
