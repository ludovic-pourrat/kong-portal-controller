package proxy

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// -----------------------------------------------------------------------------
// Proxy - Public Vars
// -----------------------------------------------------------------------------

const (
	// DefaultProxyTimeoutSeconds indicates the time.Duration allowed for responses to
	// come back from the backend proxy API.
	//
	// NOTE: the current default is based on observed latency in a CI environment using
	// the GKE cloud provider.
	DefaultProxyTimeoutSeconds float32 = 10.0

	// DefaultSyncSeconds indicates the time.Duration (minimum) that will occur between
	// updates to the Kong Proxy Admin API when using the NewProxy() constructor.
	// this 1s default was based on local testing wherein it appeared sub-second updates
	// to the Admin API could be problematic (or at least operate differently) based on
	// which storage backend was in use (i.e. "dbless", "postgres"). This is a workaround
	// for improvements we still need to investigate upstream.
	//
	// See Also: https://github.com/Kong/kong-portal-controller/issues/1398
	DefaultSyncSeconds float32 = 3.0
)

// -----------------------------------------------------------------------------
// Proxy - Public Types
// -----------------------------------------------------------------------------

// Proxy represents the Kong Proxy from the perspective of Kubernetes allowing
// callers to update and remove Kubernetes objects in the backend proxy without
// having to understand or be aware of Kong DSLs or how types are converted between
// Kubernetes and the Kong Admin API.
//
// NOTE: implementations of this interface are: threadsafe, non-blocking
type Proxy interface {
	// UpdateObject accepts a Kubernetes controller-runtime client.Object and adds/updates that to the developer cache.
	// It will be asynchronously converted into the upstream Kong DSL and applied to the Kong Admin API.
	// A status will later be added to the object whether the developer update succeeds or fails.
	UpdateObject(obj client.Object) error

	// DeleteObject accepts a Kubernetes controller-runtime client.Object and removes it from the developer cache.
	// The delete action will asynchronously be converted to Kong DSL and applied to the Kong Admin API.
	// A status will later be added to the object whether the developer update succeeds or fails.
	DeleteObject(obj client.Object) error

	// ObjectExists indicates if any version of the provided object is already present in the proxy.
	ObjectExists(obj client.Object) (bool, error)

	// ObjectExists indicates if any version of the provided object is already present in the proxy.
	ObjectExistsInCache(obj client.Object) (client.Object, bool, error)

	// IsReady returns true if the proxy is considered ready.
	// A ready proxy has developer available and can handle traffic.
	IsReady() bool

	manager.Runnable

	manager.LeaderElectionRunnable
}
