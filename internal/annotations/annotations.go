/*
Copyright 2017 The Kubernetes Authors.

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

package annotations

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

type ClassMatching int

const (
	IgnoreClassMatch       ClassMatching = iota
	ExactOrEmptyClassMatch ClassMatching = iota
	ExactClassMatch        ClassMatching = iota
)

const (
	ControllerClassKey = AnnotationPrefix + "/controller.class"

	AnnotationPrefix = "developer.konghq.com"

	// DefaultControllerClass defines the default class used
	// by Kong's portal controller.
	DefaultControllerClass = "kong"
)

func validController(controllerAnnotationValue, controllerClass string, handling ClassMatching) bool {
	switch handling {
	case IgnoreClassMatch:
		// class is not considered at all. any value, even a mismatch, is valid
		return true
	case ExactOrEmptyClassMatch:
		// aka lazy. exact match desired, but empty permitted
		return controllerAnnotationValue == "" || controllerAnnotationValue == controllerClass
	case ExactClassMatch:
		// what it says on the tin
		// this may be another place we want to return a warning, since an empty-class resource will never be valid
		return controllerAnnotationValue == controllerClass
	default:
		panic("controller class handling option received")
	}
}

// ControllerClassValidatorFunc returns a function which can validate if an Object
// belongs to the controllerClass or not.
func ControllerClassValidator(
	controllerClass string) func(obj *metav1.ObjectMeta, handling ClassMatching) bool {

	return func(obj *metav1.ObjectMeta, handling ClassMatching) bool {
		controller := obj.GetAnnotations()[ControllerClassKey]
		return validController(controller, controllerClass, handling)
	}
}
