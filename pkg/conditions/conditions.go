package conditions

import (
	gsannotation "github.com/giantswarm/k8smetadata/pkg/annotation"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	capiconditions "sigs.k8s.io/cluster-api/util/conditions"

	"github.com/aws-resolver-rules-operator/pkg/util/annotations"
)

const (
	NetworkTopologyCondition capi.ConditionType = "NetworkTopologyReady"
	TransitGatewayCreated    capi.ConditionType = "TransitGatewayCreated"
	TransitGatewayAttached   capi.ConditionType = "TransitGatewayAttached"
	PrefixListEntriesReady   capi.ConditionType = "PrefixListEntriesReady"
	NodePoolReady            capi.ConditionType = "NodePoolReady"
	EC2NodeClassReady        capi.ConditionType = "EC2NodeClassReady"
)

func MarkReady(setter capiconditions.Setter, condition capi.ConditionType) {
	capiconditions.MarkTrue(setter, condition)
}

func MarkModeNotSupported(cluster *capi.Cluster) {
	capiconditions.MarkFalse(cluster, NetworkTopologyCondition,
		"ModeNotSupported", capi.ConditionSeverityInfo,
		"The provided mode '%s' is not supported",
		annotations.GetAnnotation(cluster, gsannotation.NetworkTopologyModeAnnotation),
	)
}

func MarkVPCNotReady(cluster *capi.Cluster) {
	capiconditions.MarkFalse(cluster, NetworkTopologyCondition,
		"VPCNotReady",
		capi.ConditionSeverityInfo,
		"The cluster's VPC is not yet ready",
	)
}

func MarkIDNotProvided(cluster *capi.Cluster, id string) {
	capiconditions.MarkFalse(cluster, NetworkTopologyCondition,
		"RequiredIDMissing",
		capi.ConditionSeverityError,
		"The %s ID is missing from the annotations", id,
	)
}

func MarkNodePoolReady(setter capiconditions.Setter) {
	capiconditions.MarkTrue(setter, NodePoolReady)
}

func MarkNodePoolNotReady(setter capiconditions.Setter, reason, message string) {
	capiconditions.MarkFalse(setter, NodePoolReady, reason, capi.ConditionSeverityError, message)
}

func MarkEC2NodeClassReady(setter capiconditions.Setter) {
	capiconditions.MarkTrue(setter, EC2NodeClassReady)
}

func MarkEC2NodeClassNotReady(setter capiconditions.Setter, reason, message string) {
	capiconditions.MarkFalse(setter, EC2NodeClassReady, reason, capi.ConditionSeverityError, message)
}
