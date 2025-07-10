/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

// NodePoolSpec defines the configuration for a Karpenter NodePool
type NodePoolSpec struct {
	// Template contains the template of possibilities for the provisioning logic to launch a NodeClaim with.
	// NodeClaims launched from this NodePool will often be further constrained than the template specifies.
	// +required
	Template NodeClaimTemplate `json:"template"`

	// Disruption contains the parameters that relate to Karpenter's disruption logic
	// +kubebuilder:default:={consolidateAfter: "0s"}
	// +optional
	Disruption Disruption `json:"disruption"`

	// Limits define a set of bounds for provisioning capacity.
	// +optional
	Limits Limits `json:"limits,omitempty"`

	// Weight is the priority given to the nodepool during scheduling. A higher
	// numerical weight indicates that this nodepool will be ordered
	// ahead of other nodepools with lower weights. A nodepool with no weight
	// will be treated as if it is a nodepool with a weight of 0.
	// +kubebuilder:validation:Minimum:=1
	// +kubebuilder:validation:Maximum:=100
	// +optional
	Weight *int32 `json:"weight,omitempty"`
}

type NodeClaimTemplate struct {
	ObjectMeta `json:"metadata,omitempty"`
	// +required
	Spec NodeClaimTemplateSpec `json:"spec"`
}

type ObjectMeta struct {
	// Map of string keys and values that can be used to organize and categorize
	// (scope and select) objects. May match selectors of replication controllers
	// and services.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// Annotations is an unstructured key value map stored with a resource that may be
	// set by external tools to store and retrieve arbitrary metadata. They are not
	// queryable and should be preserved when modifying objects.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// NodeClaimTemplateSpec describes the desired state of the NodeClaim in the Nodepool
// NodeClaimTemplateSpec is used in the NodePool's NodeClaimTemplate, with the resource requests omitted since
// users are not able to set resource requests in the NodePool.
type NodeClaimTemplateSpec struct {
	// Taints will be applied to the NodeClaim's node.
	// +optional
	Taints []v1.Taint `json:"taints,omitempty"`
	// StartupTaints are taints that are applied to nodes upon startup which are expected to be removed automatically
	// within a short period of time, typically by a DaemonSet that tolerates the taint. These are commonly used by
	// daemonsets to allow initialization and enforce startup ordering.  StartupTaints are ignored for provisioning
	// purposes in that pods are not required to tolerate a StartupTaint in order to have nodes provisioned for them.
	// +optional
	StartupTaints []v1.Taint `json:"startupTaints,omitempty"`
	// Requirements are layered with GetLabels and applied to every node.
	// +kubebuilder:validation:XValidation:message="requirements with operator 'In' must have a value defined",rule="self.all(x, x.operator == 'In' ? x.values.size() != 0 : true)"
	// +kubebuilder:validation:XValidation:message="requirements operator 'Gt' or 'Lt' must have a single positive integer value",rule="self.all(x, (x.operator == 'Gt' || x.operator == 'Lt') ? (x.values.size() == 1 && int(x.values[0]) >= 0) : true)"
	// +kubebuilder:validation:XValidation:message="requirements with 'minValues' must have at least that many values specified in the 'values' field",rule="self.all(x, (x.operator == 'In' && has(x.minValues)) ? x.values.size() >= x.minValues : true)"
	// +kubebuilder:validation:MaxItems:=100
	// +required
	Requirements []NodeSelectorRequirementWithMinValues `json:"requirements" hash:"ignore"`
	// TerminationGracePeriod is the maximum duration the controller will wait before forcefully deleting the pods on a node, measured from when deletion is first initiated.
	//
	// Warning: this feature takes precedence over a Pod's terminationGracePeriodSeconds value, and bypasses any blocked PDBs or the karpenter.sh/do-not-disrupt annotation.
	//
	// This field is intended to be used by cluster administrators to enforce that nodes can be cycled within a given time period.
	// When set, drifted nodes will begin draining even if there are pods blocking eviction. Draining will respect PDBs and the do-not-disrupt annotation until the TGP is reached.
	//
	// Karpenter will preemptively delete pods so their terminationGracePeriodSeconds align with the node's terminationGracePeriod.
	// If a pod would be terminated without being granted its full terminationGracePeriodSeconds prior to the node timeout,
	// that pod will be deleted at T = node timeout - pod terminationGracePeriodSeconds.
	//
	// The feature can also be used to allow maximum time limits for long-running jobs which can delay node termination with preStop hooks.
	// If left undefined, the controller will wait indefinitely for pods to be drained.
	// +kubebuilder:validation:Pattern=`^([0-9]+(s|m|h))+$`
	// +kubebuilder:validation:Type="string"
	// +optional
	TerminationGracePeriod *metav1.Duration `json:"terminationGracePeriod,omitempty"`
	// ExpireAfter is the duration the controller will wait
	// before terminating a node, measured from when the node is created. This
	// is useful to implement features like eventually consistent node upgrade,
	// memory leak protection, and disruption testing.
	// +kubebuilder:default:="720h"
	// +kubebuilder:validation:Pattern=`^(([0-9]+(s|m|h))+|Never)$`
	// +kubebuilder:validation:Type="string"
	// +kubebuilder:validation:Schemaless
	// +optional
	ExpireAfter NillableDuration `json:"expireAfter,omitempty"`
}

// A node selector requirement with min values is a selector that contains values, a key, an operator that relates the key and values
// and minValues that represent the requirement to have at least that many values.
type NodeSelectorRequirementWithMinValues struct {
	v1.NodeSelectorRequirement `json:",inline"`
	// This field is ALPHA and can be dropped or replaced at any time
	// MinValues is the minimum number of unique values required to define the flexibility of the specific requirement.
	// +kubebuilder:validation:Minimum:=1
	// +kubebuilder:validation:Maximum:=50
	// +optional
	MinValues *int `json:"minValues,omitempty"`
}

type NodeClassReference struct {
	// Kind of the referent; More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds"
	// +kubebuilder:validation:XValidation:rule="self != ''",message="kind may not be empty"
	// +required
	Kind string `json:"kind"`
	// Name of the referent; More info: http://kubernetes.io/docs/user-guide/identifiers#names
	// +kubebuilder:validation:XValidation:rule="self != ''",message="name may not be empty"
	// +required
	Name string `json:"name"`
	// API version of the referent
	// +kubebuilder:validation:XValidation:rule="self != ''",message="group may not be empty"
	// +kubebuilder:validation:Pattern=`^[^/]*$`
	// +required
	Group string `json:"group"`
}

type Limits v1.ResourceList

type ConsolidationPolicy string

const (
	ConsolidationPolicyWhenEmpty                ConsolidationPolicy = "WhenEmpty"
	ConsolidationPolicyWhenEmptyOrUnderutilized ConsolidationPolicy = "WhenEmptyOrUnderutilized"
)

// DisruptionReason defines valid reasons for disruption budgets.
// +kubebuilder:validation:Enum={Underutilized,Empty,Drifted}
type DisruptionReason string

// Budget defines when Karpenter will restrict the
// number of Node Claims that can be terminating simultaneously.
type Budget struct {
	// Reasons is a list of disruption methods that this budget applies to. If Reasons is not set, this budget applies to all methods.
	// Otherwise, this will apply to each reason defined.
	// allowed reasons are Underutilized, Empty, and Drifted.
	// +optional
	Reasons []DisruptionReason `json:"reasons,omitempty"`
	// Nodes dictates the maximum number of NodeClaims owned by this NodePool
	// that can be terminating at once. This is calculated by counting nodes that
	// have a deletion timestamp set, or are actively being deleted by Karpenter.
	// This field is required when specifying a budget.
	// This cannot be of type intstr.IntOrString since kubebuilder doesn't support pattern
	// checking for int nodes for IntOrString nodes.
	// Ref: https://github.com/kubernetes-sigs/controller-tools/blob/55efe4be40394a288216dab63156b0a64fb82929/pkg/crd/markers/validation.go#L379-L388
	// +kubebuilder:validation:Pattern:="^((100|[0-9]{1,2})%|[0-9]+)$"
	// +kubebuilder:default:="10%"
	Nodes string `json:"nodes" hash:"ignore"`
	// Schedule specifies when a budget begins being active, following
	// the upstream cronjob syntax. If omitted, the budget is always active.
	// Timezones are not supported.
	// This field is required if Duration is set.
	// +kubebuilder:validation:Pattern:=`^(@(annually|yearly|monthly|weekly|daily|midnight|hourly))|((.+)\s(.+)\s(.+)\s(.+)\s(.+))$`
	// +optional
	Schedule *string `json:"schedule,omitempty" hash:"ignore"`
	// Duration determines how long a Budget is active since each Schedule hit.
	// Only minutes and hours are accepted, as cron does not work in seconds.
	// If omitted, the budget is always active.
	// This is required if Schedule is set.
	// This regex has an optional 0s at the end since the duration.String() always adds
	// a 0s at the end.
	// +kubebuilder:validation:Pattern=`^((([0-9]+(h|m))|([0-9]+h[0-9]+m))(0s)?)$`
	// +kubebuilder:validation:Type="string"
	// +optional
	Duration *metav1.Duration `json:"duration,omitempty" hash:"ignore"`
}

type Disruption struct {
	// ConsolidateAfter is the duration the controller will wait
	// before attempting to terminate nodes that are underutilized.
	// Refer to ConsolidationPolicy for how underutilization is considered.
	// +kubebuilder:validation:Pattern=`^(([0-9]+(s|m|h))+|Never)$`
	// +kubebuilder:validation:Type="string"
	// +kubebuilder:validation:Schemaless
	// +required
	ConsolidateAfter NillableDuration `json:"consolidateAfter"`
	// ConsolidationPolicy describes which nodes Karpenter can disrupt through its consolidation
	// algorithm. This policy defaults to "WhenEmptyOrUnderutilized" if not specified
	// +kubebuilder:default:="WhenEmptyOrUnderutilized"
	// +kubebuilder:validation:Enum:={WhenEmpty,WhenEmptyOrUnderutilized}
	// +optional
	ConsolidationPolicy ConsolidationPolicy `json:"consolidationPolicy,omitempty"`
	// Budgets is a list of Budgets.
	// If there are multiple active budgets, Karpenter uses
	// the most restrictive value. If left undefined,
	// this will default to one budget with a value to 10%.
	// +kubebuilder:validation:XValidation:message="'schedule' must be set with 'duration'",rule="self.all(x, has(x.schedule) == has(x.duration))"
	// +kubebuilder:default:={{nodes: "10%"}}
	// +kubebuilder:validation:MaxItems=50
	// +optional
	Budgets []Budget `json:"budgets,omitempty" hash:"ignore"`
}

// ConsolidateUnderSpec defines when to consolidate under
type ConsolidateUnderSpec struct {
	// CPUUtilization specifies the CPU utilization threshold
	// +optional
	CPUUtilization *string `json:"cpuUtilization,omitempty"`

	// MemoryUtilization specifies the memory utilization threshold
	// +optional
	MemoryUtilization *string `json:"memoryUtilization,omitempty"`
}

// LimitsSpec defines the limits for a NodePool
type LimitsSpec struct {
	// CPU specifies the CPU limit
	// +optional
	CPU *resource.Quantity `json:"cpu,omitempty"`

	// Memory specifies the memory limit
	// +optional
	Memory *resource.Quantity `json:"memory,omitempty"`
}

// RequirementSpec defines a requirement for a NodePool
type RequirementSpec struct {
	// Key specifies the requirement key
	Key string `json:"key"`

	// Operator specifies the requirement operator
	Operator string `json:"operator"`

	// Values specifies the requirement values
	// +optional
	Values []string `json:"values,omitempty"`
}

// TaintSpec defines a taint for a NodePool
type TaintSpec struct {
	// Key specifies the taint key
	Key string `json:"key"`

	// Value specifies the taint value
	// +optional
	Value *string `json:"value,omitempty"`

	// Effect specifies the taint effect
	Effect string `json:"effect"`
}

// EC2NodeClassSpec defines the configuration for a Karpenter EC2NodeClass
type EC2NodeClassSpec struct {
	// Name is the ami name in EC2.
	// This value is the name field, which is different from the name tag.
	AMIName string `json:"amiName,omitempty"`
	// Owner is the owner for the ami.
	// You can specify a combination of AWS account IDs, "self", "amazon", and "aws-marketplace"
	AMIOwner string `json:"amiOwner,omitempty"`

	// SecurityGroups specifies the security groups to use
	// +optional
	SecurityGroups map[string]string `json:"securityGroups,omitempty"`

	// Subnets specifies the subnets to use
	// +optional
	Subnets map[string]string `json:"subnets,omitempty"`
}

// KarpenterMachinePoolSpec defines the desired state of KarpenterMachinePool.
type KarpenterMachinePoolSpec struct {
	// The name or the Amazon Resource Name (ARN) of the instance profile associated
	// with the IAM role for the instance. The instance profile contains the IAM
	// role.
	IamInstanceProfile string `json:"iamInstanceProfile,omitempty"`

	// NodePool specifies the configuration for the Karpenter NodePool
	// +optional
	NodePool *NodePoolSpec `json:"nodePool,omitempty"`

	// EC2NodeClass specifies the configuration for the Karpenter EC2NodeClass
	// +optional
	EC2NodeClass *EC2NodeClassSpec `json:"ec2NodeClass,omitempty"`

	// ProviderIDList are the identification IDs of machine instances provided by the provider.
	// This field must match the provider IDs as seen on the node objects corresponding to a machine pool's machine instances.
	// +optional
	ProviderIDList []string `json:"providerIDList,omitempty"`
}

// KarpenterMachinePoolStatus defines the observed state of KarpenterMachinePool.
type KarpenterMachinePoolStatus struct {
	// Ready is true when the provider resource is ready.
	// +optional
	Ready bool `json:"ready"`

	// Replicas is the most recently observed number of replicas
	// +optional
	Replicas int32 `json:"replicas"`

	// Conditions defines current service state of the KarpenterMachinePool.
	// +optional
	Conditions capi.Conditions `json:"conditions,omitempty"`

	// NodePoolReady indicates whether the NodePool is ready
	// +optional
	NodePoolReady bool `json:"nodePoolReady"`

	// EC2NodeClassReady indicates whether the EC2NodeClass is ready
	// +optional
	EC2NodeClassReady bool `json:"ec2NodeClassReady"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:metadata:annotations="helm.sh/resource-policy=keep"
// https://release-1-2.cluster-api.sigs.k8s.io/developer/providers/contracts#api-version-labels
// +kubebuilder:metadata:labels="cluster.x-k8s.io/v1beta1=v1alpha1"
// +kubebuilder:printcolumn:name="Ready",type=boolean,JSONPath=`.status.ready`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// KarpenterMachinePool is the Schema for the karpentermachinepools API.
type KarpenterMachinePool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KarpenterMachinePoolSpec   `json:"spec,omitempty"`
	Status KarpenterMachinePoolStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// KarpenterMachinePoolList contains a list of KarpenterMachinePool.
type KarpenterMachinePoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KarpenterMachinePool `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KarpenterMachinePool{}, &KarpenterMachinePoolList{})
}

func (in *KarpenterMachinePool) GetConditions() capi.Conditions {
	return in.Status.Conditions
}

func (in *KarpenterMachinePool) SetConditions(conditions capi.Conditions) {
	in.Status.Conditions = conditions
}
