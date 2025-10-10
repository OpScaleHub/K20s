/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUTHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ThresholdSpec defines the thresholds for a resource.
type ThresholdSpec struct {
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	Min int32 `json:"min"`
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	Max int32 `json:"max"`
}

// ResourceOptimizerProfileSpec defines the desired state of ResourceOptimizerProfile
type ResourceOptimizerProfileSpec struct {
	// +kubebuilder:validation:Required
	Selector metav1.LabelSelector `json:"selector"`

	CPUThresholds ThresholdSpec `json:"cpuThresholds"`

	// +kubebuilder:validation:Enum=Scale;Resize;Recommend
	OptimizationPolicy string `json:"optimizationPolicy"`

	// CooldownPeriod is the duration the controller will wait before taking another scaling action.
	// Defaults to 5 minutes if not specified.
	// +optional
	// +kubebuilder:validation:Type=string
	CooldownPeriod *metav1.Duration `json:"cooldownPeriod,omitempty"`

	// MinCPU is the minimum CPU request that can be set by the Resize policy.
	// +optional
	MinCPU *resource.Quantity `json:"minCPU,omitempty"`

	// MaxCPU is the maximum CPU request that can be set by the Resize policy.
	// +optional
	MaxCPU *resource.Quantity `json:"maxCPU,omitempty"`
}

// ActionDetail records the details of the last action taken by the controller.
type ActionDetail struct {
	Type      string      `json:"type"`
	Timestamp metav1.Time `json:"timestamp"`
	// +optional
	Details string `json:"details,omitempty"`
}

// ResourceOptimizerProfileStatus defines the observed state of ResourceOptimizerProfile.
type ResourceOptimizerProfileStatus struct {
	ObservedMetrics map[string]string `json:"observedMetrics,omitempty"`
	// +optional
	LastAction      *ActionDetail `json:"lastAction,omitempty"`
	Recommendations []string      `json:"recommendations,omitempty"`
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// ResourceOptimizerProfile is the Schema for the resourceoptimizerprofiles API
type ResourceOptimizerProfile struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ResourceOptimizerProfileSpec   `json:"spec,omitempty"`
	Status ResourceOptimizerProfileStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ResourceOptimizerProfileList contains a list of ResourceOptimizerProfile
type ResourceOptimizerProfileList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ResourceOptimizerProfile `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ResourceOptimizerProfile{}, &ResourceOptimizerProfileList{})
}
