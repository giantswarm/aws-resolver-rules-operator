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

	// NodePoolCreatedCondition indicates whether the NodePool resource has been successfully
	// created or updated in the workload cluster. This doesn't mean the NodePool is ready
	// to provision nodes, just that the resource exists.
	NodePoolCreatedCondition capi.ConditionType = "NodePoolCreated"

	// EC2NodeClassCreatedCondition indicates whether the EC2NodeClass resource has been
	// successfully created or updated in the workload cluster. This doesn't mean the
	// EC2NodeClass is ready for use, just that the resource exists.
	EC2NodeClassCreatedCondition capi.ConditionType = "EC2NodeClassCreated"

	// BootstrapDataReadyCondition indicates whether the bootstrap user data has been
	// successfully uploaded to S3 and is ready for use by Karpenter nodes.
	BootstrapDataReadyCondition capi.ConditionType = "BootstrapDataReady"

	// VersionSkewValidCondition indicates whether the Kubernetes version skew policy
	// is satisfied (worker nodes don't use newer versions than control plane).
	VersionSkewValidCondition capi.ConditionType = "VersionSkewValid"

	// ReadyCondition indicates the overall readiness of the KarpenterMachinePool.
	// This is True when all necessary Karpenter resources are created and configured.
	ReadyCondition capi.ConditionType = "Ready"
)

// Condition reasons used by various controllers
const (
	// Generic reasons used across controllers
	InitializingReason = "Initializing"
	ReadyReason        = "Ready"
	NotReadyReason     = "NotReady"

	// KarpenterMachinePool controller reasons
	NodePoolCreationFailedReason        = "NodePoolCreationFailed"
	NodePoolCreationSucceededReason     = "NodePoolCreated"
	EC2NodeClassCreationFailedReason    = "EC2NodeClassCreationFailed"
	EC2NodeClassCreationSucceededReason = "EC2NodeClassCreated"
	BootstrapDataUploadFailedReason     = "BootstrapDataUploadFailed"
	BootstrapDataSecretNotFoundReason   = "BootstrapDataSecretNotFound"
	BootstrapDataSecretInvalidReason    = "BootstrapDataSecretInvalid"
	BootstrapDataUploadSucceededReason  = "BootstrapDataUploaded"
	VersionSkewBlockedReason            = "VersionSkewBlocked"
	VersionSkewValidReason              = "VersionSkewValid"
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

func MarkNodePoolCreated(setter capiconditions.Setter) {
	capiconditions.MarkTrue(setter, NodePoolCreatedCondition)
}

func MarkNodePoolNotCreated(setter capiconditions.Setter, reason, message string) {
	capiconditions.MarkFalse(setter, NodePoolCreatedCondition, reason, capi.ConditionSeverityError, message, nil)
}

func MarkEC2NodeClassCreated(setter capiconditions.Setter) {
	capiconditions.MarkTrue(setter, EC2NodeClassCreatedCondition)
}

func MarkEC2NodeClassNotCreated(setter capiconditions.Setter, reason, message string) {
	capiconditions.MarkFalse(setter, EC2NodeClassCreatedCondition, reason, capi.ConditionSeverityError, message, nil)
}

func MarkBootstrapDataReady(setter capiconditions.Setter) {
	capiconditions.MarkTrue(setter, BootstrapDataReadyCondition)
}

func MarkBootstrapDataNotReady(setter capiconditions.Setter, reason, message string) {
	capiconditions.MarkFalse(setter, BootstrapDataReadyCondition, reason, capi.ConditionSeverityError, message, nil)
}

func MarkVersionSkewValid(setter capiconditions.Setter) {
	capiconditions.MarkTrue(setter, VersionSkewValidCondition)
}

func MarkVersionSkewInvalid(setter capiconditions.Setter, reason, message string) {
	capiconditions.MarkFalse(setter, VersionSkewValidCondition, reason, capi.ConditionSeverityError, message, nil)
}

func MarkKarpenterMachinePoolReady(setter capiconditions.Setter) {
	capiconditions.MarkTrue(setter, ReadyCondition)
}

func MarkKarpenterMachinePoolNotReady(setter capiconditions.Setter, reason, message string) {
	capiconditions.MarkFalse(setter, ReadyCondition, reason, capi.ConditionSeverityError, message, nil)
}
