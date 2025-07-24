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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

// KarpenterMachinePoolSpec defines the desired state of KarpenterMachinePool.
type KarpenterMachinePoolSpec struct {
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
	// Replicas is the most recently observed number of replicas
	// +optional
	Replicas int32 `json:"replicas"`

	// Conditions defines current service state of the KarpenterMachinePool.
	// +optional
	Conditions capi.Conditions `json:"conditions,omitempty"`
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
