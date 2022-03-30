package manager

import (
	"kong-portal-controller/internal/controllers/developer"
	"reflect"

	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	ctrlutils "kong-portal-controller/internal/controllers/utils"
	"kong-portal-controller/internal/dataplane/proxy"
	konghqcomv1 "kong-portal-controller/pkg/apis/v1"
)

// -----------------------------------------------------------------------------
// Controller Manager - Controller Definition Interfaces
// -----------------------------------------------------------------------------

// Controller is a Kubernetes controller that can be plugged into Manager.
type Controller interface {
	SetupWithManager(ctrl.Manager) error
}

// AutoHandler decides whether the specific controller shall be enabled (true) or disabled (false).
type AutoHandler func(client.Client) bool

// ControllerDef is a specification of a Controller that can be conditionally registered with Manager.
type ControllerDef struct {
	Enabled     bool
	AutoHandler AutoHandler
	Controller  Controller
}

// Name returns a human-readable name of the controller.
func (c *ControllerDef) Name() string {
	return reflect.TypeOf(c.Controller).String()
}

// MaybeSetupWithManager runs SetupWithManager on the controller if it is enabled
// and its AutoHandler (if any) indicates that it can load
func (c *ControllerDef) MaybeSetupWithManager(mgr ctrl.Manager) error {
	if !c.Enabled {
		return nil
	}

	if c.AutoHandler != nil {
		if enable := c.AutoHandler(mgr.GetClient()); !enable {
			return nil
		}
	}
	return c.Controller.SetupWithManager(mgr)
}

// -----------------------------------------------------------------------------
// Controller Manager - Controller Setup Functions
// -----------------------------------------------------------------------------

func setupControllers(mgr manager.Manager, proxy proxy.Proxy, c *Config) ([]ControllerDef, error) {

	controllers := []ControllerDef{
		{
			Enabled: true,
			AutoHandler: crdExistsChecker{GVR: schema.GroupVersionResource{
				Group:    konghqcomv1.SchemeGroupVersion.Group,
				Version:  konghqcomv1.SchemeGroupVersion.Version,
				Resource: "kongfile",
			}}.CRDExists,
			Controller: &developer.KongFileReconciler{
				Client:              mgr.GetClient(),
				Log:                 ctrl.Log.WithName("controllers").WithName("KongFile"),
				Scheme:              mgr.GetScheme(),
				Proxy:               proxy,
				ControllerClassName: c.ControllerClassName,
			},
		},
	}

	return controllers, nil
}

// crdExistsChecker verifies whether the resource type defined by GVR is supported by the k8s apiserver.
type crdExistsChecker struct {
	GVR schema.GroupVersionResource
}

// CRDExists returns true iff the apiserver supports the specified group/version/resource.
func (c crdExistsChecker) CRDExists(r client.Client) bool {
	return ctrlutils.CRDExists(r, c.GVR)
}
