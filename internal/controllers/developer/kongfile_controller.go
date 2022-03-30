package developer

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrlutils "kong-portal-controller/internal/controllers/utils"
	"kong-portal-controller/internal/dataplane/proxy"
	"kong-portal-controller/internal/util"
	"sigs.k8s.io/controller-runtime/pkg/builder"

	"k8s.io/apimachinery/pkg/runtime"
	developerv1 "kong-portal-controller/pkg/apis/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// KongFileReconciler reconciles a KongFile object
type KongFileReconciler struct {
	client.Client

	Log    logr.Logger
	Scheme *runtime.Scheme
	Proxy  proxy.Proxy

	ControllerClassName string
}

//+kubebuilder:rbac:groups=developer.konghq.com,resources=kongFiles,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=developer.konghq.com,resources=kongFiles/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=developer.konghq.com,resources=kongFiles/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
func (r *KongFileReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("KongFile", req.NamespacedName)

	log.V(util.InfoLevel).Info("Reconciling resource", "namespace", req.Namespace, "name", req.Name)

	// get the relevant object
	obj := new(developerv1.KongFile)
	if err := r.Get(ctx, req.NamespacedName, obj); err != nil {
		if errors.IsNotFound(err) {
			obj.Namespace = req.Namespace
			obj.Name = req.Name
			result, exists, e := EnsureProxyDeleteObject(r.Proxy, obj)
			if e != nil {
				log.Error(e, "Resource fail to be deleted, retrying ...", "type", "KongFile", "namespace", req.Namespace, "name", req.Name)
			} else {
				if exists {
					log.V(util.InfoLevel).Info("Resource is deleted, its configuration will be removed", "type", "KongFile", "namespace", req.Namespace, "name", req.Name)
				}
			}
			return result, e
		}
		return ctrl.Result{}, err
	}

	// clean the object up if it's being deleted
	if !obj.DeletionTimestamp.IsZero() && time.Now().After(obj.DeletionTimestamp.Time) {
		log.V(util.InfoLevel).Info("Resource is being deleted, its configuration will be removed", "type", "KongFile", "namespace", req.Namespace, "name", req.Name)
		objectExistsInCache, err := r.Proxy.ObjectExists(obj)
		if err != nil {
			return ctrl.Result{}, err
		}
		if objectExistsInCache {
			if err := r.Proxy.DeleteObject(obj); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{Requeue: true}, nil // wait until the object is no longer present in the cache
		}
		return ctrl.Result{}, nil
	}

	// if the object is configured with our controller.class, then we need to ensure it's removed from the cache
	if !ctrlutils.MatchesControllerClassName(obj, r.ControllerClassName) {
		log.V(util.InfoLevel).Info("Object missing controller class, ensuring it's removed from configuration", "namespace", req.Namespace, "name", req.Name)
		return ctrl.Result{}, nil
	}

	if obj.Status.Validated == false {
		// update the kong Admin API with the changes
		log.V(util.InfoLevel).Info("Object missing, ensuring it's created into configuration",
			"namespace", req.Namespace,
			"name", req.Name,
			"status", obj.Status.Validated)

		if err := r.Proxy.UpdateObject(obj); err != nil {
			log.Error(err, "Failed to update resource")
			return ctrl.Result{}, err
		}
		// validated
		obj.Status.Validated = true

		log.V(util.InfoLevel).Info("Object validated, ensuring it's created into configuration",
			"namespace", req.Namespace,
			"name", req.Name,
			"status", obj.Status)

		// update status
		err := r.Status().Update(ctx, obj)
		if err != nil {
			log.Error(err, "Failed to update resource status")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *KongFileReconciler) SetupWithManager(mgr ctrl.Manager) error {
	preds := ctrlutils.GeneratePredicateFuncsForControllerClassFilter(r.ControllerClassName, false, true)

	return ctrl.NewControllerManagedBy(mgr).
		For(&developerv1.KongFile{}, builder.WithPredicates(preds)).Complete(r)
}

// EnsureProxyDeleteObject is a reconciliation helper to ensure that an object is removed from
// the backend proxy cache so that it gets removed from data-plane configurations.
func EnsureProxyDeleteObject(proxy proxy.Proxy, obj client.Object) (ctrl.Result, bool, error) {
	// check whether the object is at all present in the proxy cache.
	cached, objectExistsInCache, err := proxy.ObjectExistsInCache(obj)
	if err != nil {
		return ctrl.Result{}, false, err
	}

	// if the object is still present in the proxy cache, we need to keep trying to
	// remove it until its gone so that it gets removed from backend data-plane.
	if objectExistsInCache {
		if err := proxy.DeleteObject(cached); err != nil {
			return ctrl.Result{}, true, err
		}
		// the caller should requeue until the object is no longer present in the cache
		// to ensure removal was successful
		return ctrl.Result{Requeue: true}, true, nil
	} else {
		// if the object is not present in the proxy cache, we're all set
		return ctrl.Result{}, false, nil
	}

}
