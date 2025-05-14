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
)

// KarpenterMachinePoolSpec defines the desired state of KarpenterMachinePool.
type KarpenterMachinePoolSpec struct {
	// The name or the Amazon Resource Name (ARN) of the instance profile associated
	// with the IAM role for the instance. The instance profile contains the IAM
	// role.
	IamInstanceProfile string `json:"iamInstanceProfile,omitempty"`
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
