/*
Copyright 2022 Kong, Inc..

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Kind string

const (
	CONTENT       Kind = "CONTENT"
	SPECIFICATION      = "SPECIFICATION"
	ASSET              = "ASSET"
)

// KongFileSpec defines the desired state of KongFile
type KongFileSpec struct {

	// KongFile layout
	Layout string `json:"layout,omitempty" yaml:"layout,omitempty"`

	// KongFile path
	Path string `json:"path,omitempty" yaml:"path,omitempty"`

	// KongFile title
	Title string `json:"title,omitempty" yaml:"title,omitempty"`

	// KongFile name
	Name string `json:"name,omitempty" yaml:"name,omitempty"`

	// KongFile name
	Content string `json:"content,omitempty" yaml:"content,omitempty"`

	// KongFile kind
	Kind Kind `json:"kind,omitempty" yaml:"kind,omitempty"`
}

// KongFileStatus defines the observed state of KongFile
type KongFileStatus struct {
	Validated bool `json:"validated,omitempty" yaml:"validated,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// KongFile is the Schema for the kongFiles API
type KongFile struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KongFileSpec   `json:"spec,omitempty"`
	Status KongFileStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// KongFileList contains a list of KongFile
type KongFileList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KongFile `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KongFile{}, &KongFileList{})
}
