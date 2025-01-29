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

package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// PocketbaseSpec defines the desired state of Pocketbase.
type PocketbaseSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Name is the name of the PocketBase instance.
	Name string `json:"name,omitempty"`

	// Image is the Docker image to use for the PocketBase instance.
	Image string `json:"image,omitempty"`

	// // Resources defines the resource requirements for the PocketBase instance.
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// ServiceAccountName is the name of the service account to use for the PocketBase instance.
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// Volumes defines the volumes for the PocketBase instance.
	Volumes VolumeConfig `json:"volumes,omitempty"`
}

type VolumeConfig struct {
	// Type is the type of storage to use (e.g., "local", "s3").
	StorageClassName string `json:"storageClassName,omitempty"`

	// AccessModes is the access modes for the volume.
	AccessModes []string `json:"accessModes,omitempty"`

	// StorageSize is the size of the storage.
	StorageSize string `json:"storageSize,omitempty"`

	// VolumeMountPath is the path where the volume is mounted.
	VolumeMountPath string `json:"volumeMountPath,omitempty"`

	// VolumeName is the name of the volume.
	VolumeName string `json:"volumeName,omitempty"`
}

// PocketbaseStatus defines the observed state of Pocketbase.
type PocketbaseStatus struct {
	// Conditions is a list of the current conditions of the PocketBase.
	Conditions []PocketbaseCondition `json:"conditions,omitempty"`
}

// PocketbaseCondition is a condition of the PocketBase.
type PocketbaseCondition struct {
	// Type is the type of the condition.
	Type string `json:"type"`

	// Status is the status of the condition.
	Status string `json:"status"`

	// LastTransitionTime is the last time the condition transitioned from one status to another.
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`

	// Reason is a one-word CamelCase reason for the condition's last transition.
	Reason string `json:"reason,omitempty"`

	// Message is a human-readable message indicating details about the condition.
	Message string `json:"message,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Pocketbase is the Schema for the pocketbases API.
type Pocketbase struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PocketbaseSpec   `json:"spec,omitempty"`
	Status PocketbaseStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// PocketbaseList contains a list of Pocketbase.
type PocketbaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Pocketbase `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Pocketbase{}, &PocketbaseList{})
}
