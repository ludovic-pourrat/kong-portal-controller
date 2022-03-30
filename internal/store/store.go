package store

import (
	"fmt"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
	"kong-portal-controller/internal/annotations"
	developer "kong-portal-controller/pkg/apis/v1"
	"reflect"
	"sync"
)

// ErrNotFound error is returned when a lookup results in no resource.
// This type is meant to be used for error handling using `errors.As()`.
type ErrNotFound struct {
	message string
}

func (e ErrNotFound) Error() string {
	if e.message == "" {
		return "not found"
	}
	return e.message
}

func keyFunc(obj interface{}) (string, error) {
	v := reflect.Indirect(reflect.ValueOf(obj))
	name := v.FieldByName("Name")
	namespace := v.FieldByName("Namespace")
	return namespace.String() + "/" + name.String(), nil
}

// KongStore is the interface that wraps the required methods to gather information about Kong
type KongStore interface {
	GetKongFile(namespace, name string) (*developer.KongFile, error)

	ListKongFiles() ([]*developer.KongFile, error)
}

// Store implements Storer and can be used to list Ingress, Services
// and other resources from k8s APIserver. The backing stores should
// be synced and updated by the caller.
// It is controllerClass filter aware.
type Store struct {
	stores CacheStores

	controllerClass string

	isValidControllerClass func(objectMeta *metav1.ObjectMeta, handling annotations.ClassMatching) bool

	logger logr.Logger
}

// CacheStores stores cache.Store for all Kinds of k8s objects that
// the Ingress Controller reads.
type CacheStores struct {
	KongFiles cache.Store

	l *sync.RWMutex

	logger logr.Logger
}

// NewCacheStores is a convenience function for CacheStores to initialize all attributes with new cache stores
func NewCacheStores(logger logr.Logger) (c CacheStores) {
	c.KongFiles = cache.NewStore(keyFunc)
	c.l = &sync.RWMutex{}
	c.logger = logger
	return
}

// Get checks if there's already some version of the provided object present in the cache.
func (c CacheStores) Get(obj runtime.Object) (item interface{}, exists bool, err error) {
	c.l.RLock()
	defer c.l.RUnlock()

	switch obj := obj.(type) {
	// ----------------------------------------------------------------------------
	// Kong API Support
	// ----------------------------------------------------------------------------
	case *developer.KongFile:
		return c.KongFiles.Get(obj)

	}
	return nil, false, fmt.Errorf("%T is not a supported cache object type", obj)
}

// Add stores a provided runtime.Object into the CacheStore if it's of a supported type.
// The CacheStore must be initialized (see NewCacheStores()) or this will panic.
func (c CacheStores) Add(obj runtime.Object) error {
	c.l.Lock()
	defer c.l.Unlock()

	switch obj := obj.(type) {
	// ----------------------------------------------------------------------------
	// Kong API Support
	// ----------------------------------------------------------------------------
	case *developer.KongFile:
		return c.KongFiles.Add(obj)
	default:
		return fmt.Errorf("cannot add unsupported kind %q to the store", obj.GetObjectKind().GroupVersionKind())
	}
}

// Update stores a provided runtime.Object into the CacheStore if it's of a supported type.
// The CacheStore must be initialized (see NewCacheStores()) or this will panic.
func (c CacheStores) Update(obj runtime.Object) error {
	c.l.Lock()
	defer c.l.Unlock()

	switch obj := obj.(type) {
	// ----------------------------------------------------------------------------
	// Kong API Support
	// ----------------------------------------------------------------------------
	case *developer.KongFile:
		return c.KongFiles.Update(obj)
	default:
		return fmt.Errorf("cannot add unsupported kind %q to the store", obj.GetObjectKind().GroupVersionKind())
	}
}

// Delete removes a provided runtime.Object from the CacheStore if it's of a supported type.
// The CacheStore must be initialized (see NewCacheStores()) or this will panic.
func (c CacheStores) Delete(obj runtime.Object) error {
	c.l.Lock()
	defer c.l.Unlock()

	switch obj := obj.(type) {
	// ----------------------------------------------------------------------------
	// Kong API Support
	// ----------------------------------------------------------------------------
	case *developer.KongFile:
		return c.KongFiles.Delete(obj)
	default:
		return fmt.Errorf("cannot delete unsupported kind %q from the store", obj.GetObjectKind().GroupVersionKind())
	}
}

// GetKongFile returns the 'name' KongFile resource in namespace.
func (s Store) GetKongFile(namespace, name string) (*developer.KongFile, error) {
	key := fmt.Sprintf("%v/%v", namespace, name)
	p, exists, err := s.stores.KongFiles.GetByKey(key)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrNotFound{fmt.Sprintf("KongFile %v not found", key)}
	}
	return p.(*developer.KongFile), nil
}

// ListKongFiles returns all KongFile resources
func (s Store) ListKongFiles() ([]*developer.KongFile, error) {

	var KongFiles []*developer.KongFile
	err := cache.ListAll(s.stores.KongFiles,
		labels.NewSelector(),
		func(ob interface{}) {
			p, ok := ob.(*developer.KongFile)
			if ok && s.isValidControllerClass(&p.ObjectMeta, annotations.ExactOrEmptyClassMatch) {
				KongFiles = append(KongFiles, p)
			}
		})
	if err != nil {
		return nil, err
	}
	return KongFiles, nil
}

// ListAllKongFiles returns all KongFile resources
func (s Store) ListAllKongFiles() ([]*developer.KongFile, error) {

	var KongFiles []*developer.KongFile
	err := cache.ListAll(s.stores.KongFiles,
		labels.NewSelector(),
		func(ob interface{}) {
			p, ok := ob.(*developer.KongFile)
			if ok {
				KongFiles = append(KongFiles, p)
			}
		})
	if err != nil {
		return nil, err
	}
	return KongFiles, nil
}
