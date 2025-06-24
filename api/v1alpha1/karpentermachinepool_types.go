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
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

// NodePoolSpec defines the configuration for a Karpenter NodePool
type NodePoolSpec struct {
	// Disruption specifies the disruption behavior for the node pool
	// +optional
	Disruption *DisruptionSpec `json:"disruption,omitempty"`

	// Limits specifies the limits for the node pool
	// +optional
	Limits *LimitsSpec `json:"limits,omitempty"`

	// Requirements specifies the requirements for the node pool
	// +optional
	Requirements []RequirementSpec `json:"requirements,omitempty"`

	// Taints specifies the taints to apply to nodes in this pool
	// +optional
	Taints []TaintSpec `json:"taints,omitempty"`

	// Labels specifies the labels to apply to nodes in this pool
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// Weight specifies the weight of this node pool
	// +optional
	Weight *int32 `json:"weight,omitempty"`
}

// DisruptionSpec defines the disruption behavior for a NodePool
type DisruptionSpec struct {
	// ConsolidateAfter specifies when to consolidate nodes
	// +optional
	ConsolidateAfter *metav1.Duration `json:"consolidateAfter,omitempty"`

	// ConsolidationPolicy specifies the consolidation policy
	// +optional
	ConsolidationPolicy *string `json:"consolidationPolicy,omitempty"`

	// ConsolidateUnder specifies when to consolidate under
	// +optional
	ConsolidateUnder *ConsolidateUnderSpec `json:"consolidateUnder,omitempty"`
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
	// AMIID specifies the AMI ID to use
	// +optional
	AMIID *string `json:"amiId,omitempty"`

	// SecurityGroups specifies the security groups to use
	// +optional
	SecurityGroups []string `json:"securityGroups,omitempty"`

	// Subnets specifies the subnets to use
	// +optional
	Subnets []string `json:"subnets,omitempty"`

	// UserData specifies the user data to use
	// +optional
	UserData *string `json:"userData,omitempty"`

	// Tags specifies the tags to apply to EC2 instances
	// +optional
	Tags map[string]string `json:"tags,omitempty"`
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
