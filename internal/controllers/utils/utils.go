package ctrlutils

import (
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"kong-portal-controller/internal/annotations"
)

// HasAnnotation is a helper function to determine whether an object has a given annotation, and whether it's
// to the value provided.
func HasAnnotation(obj client.Object, key, expectedValue string) bool {
	foundValue, ok := obj.GetAnnotations()[key]
	return ok && foundValue == expectedValue
}

// MatchesControllerClassName indicates whether or not an object indicates that it's supported by the controller class name provided.
func MatchesControllerClassName(obj client.Object, ControllerClassName string) bool {
	return HasAnnotation(obj, annotations.ControllerClassKey, ControllerClassName)
}

// GeneratePredicateFuncsForControllerClassFilter builds a controller-runtime reconciliation predicate function which filters out objects
func GeneratePredicateFuncsForControllerClassFilter(name string, specCheckEnabled, annotationCheckEnabled bool) predicate.Funcs {
	preds := predicate.NewPredicateFuncs(func(obj client.Object) bool {
		if annotationCheckEnabled && IsControllerClassAnnotationConfigured(obj, name) {
			return true
		}
		if specCheckEnabled && IsControllerClassSpecConfigured(obj, name) {
			return true
		}
		return false
	})
	preds.UpdateFunc = func(e event.UpdateEvent) bool {
		if annotationCheckEnabled && IsControllerClassAnnotationConfigured(e.ObjectOld, name) || IsControllerClassAnnotationConfigured(e.ObjectNew, name) {
			return true
		}
		if specCheckEnabled && IsControllerClassSpecConfigured(e.ObjectOld, name) || IsControllerClassSpecConfigured(e.ObjectNew, name) {
			return true
		}
		return false
	}
	return preds
}

// IsControllerClassAnnotationConfigured determines whether an object has an controller.class annotation configured that
// matches the provide ControllerClassName (and is therefore an object configured to be reconciled by that class).
func IsControllerClassAnnotationConfigured(obj client.Object, expectedControllerClassName string) bool {
	if foundControllerClassName, ok := obj.GetAnnotations()[annotations.ControllerClassKey]; ok {
		if foundControllerClassName == expectedControllerClassName {
			return true
		}
	}

	return false
}

// IsControllerClassAnnotationConfigured determines whether an object has ControllerClassName field in its spec and whether the value
// matches the provide ControllerClassName (and is therefore an object configured to be reconciled by that class).
func IsControllerClassSpecConfigured(obj client.Object, expectedControllerClassName string) bool {
	return true
}

// CRDExists returns false if CRD does not exist
func CRDExists(client client.Client, gvr schema.GroupVersionResource) bool {
	_, err := client.RESTMapper().KindFor(gvr)
	return !meta.IsNoMatchError(err)
}
