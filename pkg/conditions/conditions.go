package conditions

import (
	gsannotation "github.com/giantswarm/k8smetadata/pkg/annotation"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1" //nolint:staticcheck
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	clusterv1conditions "sigs.k8s.io/cluster-api/util/conditions/deprecated/v1beta1"
	capiconditions "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions" //nolint:staticcheck

	"github.com/aws-resolver-rules-operator/pkg/util/annotations"
)

const (
	NetworkTopologyCondition clusterv1beta1.ConditionType = "NetworkTopologyReady"
	TransitGatewayCreated    clusterv1beta1.ConditionType = "TransitGatewayCreated"
	TransitGatewayAttached   clusterv1beta1.ConditionType = "TransitGatewayAttached"
	PrefixListEntriesReady   clusterv1beta1.ConditionType = "PrefixListEntriesReady"

	// NodePoolCreatedCondition indicates whether the NodePool resource has been successfully
	// created or updated in the workload cluster. This doesn't mean the NodePool is ready
	// to provision nodes, just that the resource exists.
	NodePoolCreatedCondition clusterv1beta1.ConditionType = "NodePoolCreated"

	// EC2NodeClassCreatedCondition indicates whether the EC2NodeClass resource has been
	// successfully created or updated in the workload cluster. This doesn't mean the
	// EC2NodeClass is ready for use, just that the resource exists.
	EC2NodeClassCreatedCondition clusterv1beta1.ConditionType = "EC2NodeClassCreated"

	// BootstrapDataReadyCondition indicates whether the bootstrap user data has been
	// successfully uploaded to S3 and is ready for use by Karpenter nodes.
	BootstrapDataReadyCondition clusterv1beta1.ConditionType = "BootstrapDataReady"

	// VersionSkewPolicySatisfiedCondition indicates whether the Kubernetes version skew policy
	// is satisfied (worker nodes don't use newer versions than control plane).
	VersionSkewPolicySatisfiedCondition clusterv1beta1.ConditionType = "VersionSkewPolicySatisfied"

	// ReadyCondition indicates the overall readiness of the KarpenterMachinePool.
	// This is True when all necessary Karpenter resources are created and configured.
	ReadyCondition clusterv1beta1.ConditionType = "Ready"
)

// Condition reasons used by various controllers
const (
	// Generic reasons used across controllers
	ReadyReason    = "Ready"
	NotReadyReason = "NotReady"

	// KarpenterMachinePool controller reasons
	NodePoolCreationFailedReason              = "NodePoolCreationFailed"
	NodePoolCreationSucceededReason           = "NodePoolCreated"
	EC2NodeClassCreationFailedReason          = "EC2NodeClassCreationFailed"
	EC2NodeClassCreationSucceededReason       = "EC2NodeClassCreated"
	BootstrapDataUploadFailedReason           = "BootstrapDataUploadFailed"
	BootstrapDataSecretNotFoundReason         = "BootstrapDataSecretNotFound"
	BootstrapDataSecretInvalidReason          = "BootstrapDataSecretInvalid"
	BootstrapDataSecretMissingReferenceReason = "BootstrapDataSecretMissingReference"
	BootstrapDataUploadSucceededReason        = "BootstrapDataUploaded"
	VersionSkewBlockedReason                  = "VersionSkewBlocked"
	VersionSkewValidReason                    = "VersionSkewValid"
)

func MarkReady(setter capiconditions.Setter, condition clusterv1beta1.ConditionType) {
	capiconditions.MarkTrue(setter, condition)
}

func MarkModeNotSupported(cluster *clusterv1.Cluster) {
	clusterv1conditions.MarkFalse(cluster, clusterv1.ConditionType(NetworkTopologyCondition), //nolint:staticcheck
		"ModeNotSupported", clusterv1.ConditionSeverityInfo,
		"The provided mode '%s' is not supported",
		annotations.GetAnnotation(cluster, gsannotation.NetworkTopologyModeAnnotation),
	)
}

func MarkVPCNotReady(cluster *clusterv1.Cluster) {
	clusterv1conditions.MarkFalse(cluster, clusterv1.ConditionType(NetworkTopologyCondition), //nolint:staticcheck
		"VPCNotReady",
		clusterv1.ConditionSeverityInfo,
		"The cluster's VPC is not yet ready",
	)
}

func MarkIDNotProvided(cluster *clusterv1.Cluster, id string) {
	clusterv1conditions.MarkFalse(cluster, clusterv1.ConditionType(NetworkTopologyCondition), //nolint:staticcheck
		"RequiredIDMissing",
		clusterv1.ConditionSeverityError,
		"The %s ID is missing from the annotations", id,
	)
}

func MarkNodePoolCreated(setter capiconditions.Setter) {
	capiconditions.Set(setter, &clusterv1beta1.Condition{
		Type:   NodePoolCreatedCondition,
		Status: "True", //nolint:goconst
		Reason: NodePoolCreationSucceededReason,
	})
}

func MarkNodePoolNotCreated(setter capiconditions.Setter, reason, message string) {
	capiconditions.MarkFalse(setter, NodePoolCreatedCondition, reason, clusterv1beta1.ConditionSeverityError, "%s", message)
}

func MarkEC2NodeClassCreated(setter capiconditions.Setter) {
	capiconditions.Set(setter, &clusterv1beta1.Condition{
		Type:   EC2NodeClassCreatedCondition,
		Status: "True",
		Reason: EC2NodeClassCreationSucceededReason,
	})
}

func MarkEC2NodeClassNotCreated(setter capiconditions.Setter, reason, message string) {
	capiconditions.MarkFalse(setter, EC2NodeClassCreatedCondition, reason, clusterv1beta1.ConditionSeverityError, "%s", message)
}

func MarkBootstrapDataReady(setter capiconditions.Setter) {
	capiconditions.Set(setter, &clusterv1beta1.Condition{
		Type:   BootstrapDataReadyCondition,
		Status: "True",
		Reason: ReadyReason,
	})
}

func MarkBootstrapDataNotReady(setter capiconditions.Setter, reason, message string) {
	capiconditions.MarkFalse(setter, BootstrapDataReadyCondition, reason, clusterv1beta1.ConditionSeverityError, "%s", message)
}

func MarkVersionSkewPolicySatisfied(setter capiconditions.Setter) {
	capiconditions.Set(setter, &clusterv1beta1.Condition{
		Type:   VersionSkewPolicySatisfiedCondition,
		Status: "True",
		Reason: VersionSkewValidReason,
	})
}

func MarkVersionSkewInvalid(setter capiconditions.Setter, reason, message string) {
	capiconditions.MarkFalse(setter, VersionSkewPolicySatisfiedCondition, reason, clusterv1beta1.ConditionSeverityError, "%s", message)
}

func MarkKarpenterMachinePoolReady(setter capiconditions.Setter) {
	capiconditions.Set(setter, &clusterv1beta1.Condition{
		Type:   ReadyCondition,
		Status: "True",
		Reason: ReadyReason,
	})
}

func MarkKarpenterMachinePoolNotReady(setter capiconditions.Setter, reason, message string) {
	capiconditions.MarkFalse(setter, ReadyCondition, reason, clusterv1beta1.ConditionSeverityError, "%s", message)
}
