package manager

import (
	"context"
	"fmt"
	"github.com/bombsimon/logrusr/v2"
	"kong-portal-controller/internal/dataplane/configuration"
	"kong-portal-controller/internal/kong"
	"kong-portal-controller/internal/store"
	"strings"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"kong-portal-controller/internal/admission"
	"kong-portal-controller/internal/dataplane/proxy"
	"kong-portal-controller/internal/util"
)

// -----------------------------------------------------------------------------
// Controller Manager - Setup Utility Functions
// -----------------------------------------------------------------------------

func setupLoggers(c *Config) (logr.Logger, error) {
	customizedLogger, err := util.MakeLogger(c.LogLevel, c.LogFormat)
	if err != nil {
		return logr.Logger{}, fmt.Errorf("failed to make logger: %w", err)
	}

	logger := logrusr.New(customizedLogger)
	ctrl.SetLogger(logger)

	return logger, nil
}

func setupControllerOptions(logger logr.Logger, c *Config, scheme *runtime.Scheme) (ctrl.Options, error) {
	// some controllers may require additional namespaces to be cached and this
	// is currently done using the global manager client cache.
	//
	// See: https://github.com/Kong/kong-portal-controller/issues/2004
	requiredCacheNamespaces := make([]string, 0)

	// if publish service has been provided the namespace for it should be
	// watched so that controllers can see updates to the service.
	if c.PublishService != "" {
		publishServiceSplit := strings.SplitN(c.PublishService, "/", 3)
		if len(publishServiceSplit) != 2 {
			return ctrl.Options{}, fmt.Errorf("--publish-service was expected to be in format <namespace>/<name> but got %s", c.PublishService)
		}
		requiredCacheNamespaces = append(requiredCacheNamespaces, publishServiceSplit[0])
	}

	var leaderElection bool
	logger.Info("Database mode detected, enabling leader election")
	leaderElection = true

	// configure the general controller options
	controllerOpts := ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     c.MetricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: c.ProbeAddr,
		LeaderElection:         leaderElection,
		LeaderElectionID:       c.LeaderElectionID,
		SyncPeriod:             &c.SyncPeriod,
	}

	// configure the controller caching options
	if len(c.WatchNamespaces) == 0 {
		// if there are no configured watch namespaces, then we're watching ALL namespaces
		// and we don't have to bother individually caching any particular namespaces
		controllerOpts.Namespace = corev1.NamespaceAll
	} else {
		// in all other cases we are a multi-namespace setup and must watch all the
		// c.WatchNamespaces and additionalNamespacesToCache defined namespaces.
		// this mode does not set the Namespace option, so the manager will default to watching all namespaces
		// MultiNamespacedCacheBuilder imposes a filter on top of that watch to retrieve scoped resources
		// from the watched namespaces only.
		logger.Info("manager set up with multiple namespaces", "namespaces", c.WatchNamespaces)
		controllerOpts.NewCache = cache.MultiNamespacedCacheBuilder(append(c.WatchNamespaces, requiredCacheNamespaces...))
	}

	if len(c.LeaderElectionNamespace) > 0 {
		controllerOpts.LeaderElectionNamespace = c.LeaderElectionNamespace
	}

	return controllerOpts, nil
}

func setupKongConfig(ctx context.Context, logger logr.Logger, c *Config) (configuration.Kong, error) {
	kongClient, err := c.GetKongClient(ctx)
	if err != nil {
		return configuration.Kong{}, fmt.Errorf("unable to build kong api client: %w", err)
	}

	cfg := configuration.Kong{
		URL:         c.KongAdminURL,
		Concurrency: c.Concurrency,
		Client:      kongClient,
		ConfigDone:  make(chan *configuration.KongConfigUpdate),
	}

	return cfg, nil
}

func setupProxyServer(ctx context.Context,
	logger logr.Logger,
	mgr manager.Manager,
	kongConfig configuration.Kong,
	c *Config,
) (proxy.Proxy, error) {

	timeoutDuration, err := time.ParseDuration(fmt.Sprintf("%gs", c.ProxyTimeoutSeconds))
	if err != nil {
		logger.Error(err, "%s is not a valid number of seconds to the timeout config for the kong client")
		return nil, err
	}

	service := kong.NewFileService(kongConfig.Client)

	store := store.NewCacheStores(logger)

	proxyServer, err := proxy.NewCacheBasedProxyWithStagger(logger,
		kongConfig,
		c.ControllerClassName,
		c.EnableReverseSync,
		timeoutDuration,
		store,
		service,
		ctx)
	if err != nil {
		return nil, err
	}

	err = mgr.Add(proxyServer)
	if err != nil {
		return nil, err
	}

	return proxyServer, nil
}

func setupAdmissionServer(ctx context.Context, managerConfig *Config, managerClient client.Client) error {
	customizedLogger, err := util.MakeLogger(managerConfig.LogLevel, managerConfig.LogFormat)
	if err != nil {
		return err
	}

	logger := logrusr.New(customizedLogger)

	srv, err := admission.MakeTLSServer(&managerConfig.AdmissionServer, &admission.RequestHandler{
		Validator: admission.NewKongHTTPValidator(
			logger,
			managerClient,
		),
		Logger: logger,
	})
	if err != nil {
		return err
	}
	go func() {
		err := srv.ListenAndServeTLS("", "")
		logger.Error(err, "admission webhook server stopped")
	}()
	return nil
}
