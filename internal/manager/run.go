// Package manager implements the controller manager for all controllers
package manager

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"

	"kong-portal-controller/internal/manager/metadata"
	"kong-portal-controller/internal/util"
	developer "kong-portal-controller/pkg/apis/v1"
)

// -----------------------------------------------------------------------------
// Controller Manager - Setup & Run
// -----------------------------------------------------------------------------

// Run starts the controller manager and blocks until it exits.
func Run(ctx context.Context, c *Config) error {
	logger, err := setupLoggers(c)
	if err != nil {
		return err
	}
	logger.Info("Kong Portal Controller Starting ....")
	setupLog := ctrl.Log.WithName("setup")
	setupLog.Info("Starting controller manager", "release", metadata.Release, "repo", metadata.Repo, "commit", metadata.Commit)
	setupLog.V(util.DebugLevel).Info("The controller class name has been set", "value", c.ControllerClassName)
	setupLog.V(util.DebugLevel).Info("Building the manager runtime scheme and loading apis into the scheme")
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(developer.AddToScheme(scheme))

	if c.EnableLeaderElection {
		setupLog.V(0).Info("The --leader-elect flag is deprecated and no longer has any effect: leader election is set based on the Kong database setting")
	}

	setupLog.Info("Starting controller class", "name", c.ControllerClassName)

	setupLog.Info("Getting the kubernetes client")
	kubeconfig, err := c.GetKubeconfig()
	if err != nil {
		return fmt.Errorf("get kubeconfig from file %q: %w", c.KubeconfigPath, err)
	}

	setupLog.Info("Getting the kong admin api client")
	kongConfig, err := setupKongConfig(ctx, setupLog, c)
	if err != nil {
		return fmt.Errorf("unable to build the kong admin api developer: %w", err)
	}

	kongRoot, err := kongConfig.Client.Root(ctx)
	if err != nil {
		return fmt.Errorf("could not retrieve Kong admin root: %w", err)
	}
	kongRootConfig, ok := kongRoot["configuration"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid root configuration, expected a map[string]interface{} got %T",
			kongRoot["developer"])
	}
	dbmode, ok := kongRootConfig["database"].(string)
	if !ok {
		return fmt.Errorf("invalid database configuration, expected a string got %T", kongRootConfig["database"])
	}
	setupLog.Info("Configuration loaded",
		"mode", dbmode,
		"URL", kongConfig.URL)

	setupLog.Info("Configuring and building the controller manager")
	controllerOpts, err := setupControllerOptions(setupLog, c, scheme)
	if err != nil {
		return fmt.Errorf("unable to setup controller options: %w", err)
	}
	mgr, err := ctrl.NewManager(kubeconfig, controllerOpts)
	if err != nil {
		return fmt.Errorf("unable to start controller manager: %w", err)
	}

	setupLog.Info("Starting Admission Server")
	if err := setupAdmissionServer(ctx, c, mgr.GetClient()); err != nil {
		return err
	}

	setupLog.Info("Initializing Proxy Cache Server")
	proxy, err := setupProxyServer(ctx, setupLog, mgr, kongConfig, c)
	if err != nil {
		return fmt.Errorf("unable to initialize proxy cache server: %w", err)
	}

	setupLog.Info("Starting Enabled Controllers")
	controllers, err := setupControllers(mgr, proxy, c)
	if err != nil {
		return fmt.Errorf("unable to setup controller as expected %w", err)
	}
	for _, c := range controllers {
		if err := c.MaybeSetupWithManager(mgr); err != nil {
			return fmt.Errorf("unable to create controller %q: %w", c.Name(), err)
		}
	}

	//+kubebuilder:scaffold:builder
	
	setupLog.Info("Starting health check servers")
	if err := mgr.AddHealthzCheck("health", healthz.Ping); err != nil {
		return fmt.Errorf("unable to setup healthz: %w", err)
	}
	if err := mgr.AddReadyzCheck("check", func(_ *http.Request) error {
		if !proxy.IsReady() {
			return errors.New("proxy not yet configured")
		}
		return nil
	}); err != nil {
		return fmt.Errorf("unable to setup readyz: %w", err)
	}

	setupLog.Info("Starting manager")
	return mgr.Start(ctx)
}
