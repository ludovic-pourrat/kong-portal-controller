package proxy

import (
	"context"
	"fmt"
	services "kong-portal-controller/internal/kong"
	"kong-portal-controller/internal/store"
	developer "kong-portal-controller/pkg/apis/v1"
	"sync"
	"time"

	"github.com/blang/semver/v4"
	"github.com/go-logr/logr"
	"github.com/kong/go-kong/kong"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"kong-portal-controller/internal/dataplane/configuration"
	"kong-portal-controller/internal/metrics"
)

// -----------------------------------------------------------------------------
// Client Go Cached Proxy Resolver - Public Functions
// -----------------------------------------------------------------------------

// NewCacheBasedProxy will provide a new Proxy object. Note that this starts some background goroutines and the caller
// is resonsible for marking the provided context.Context as "Done()" to shut down the background routines. A "stagger"
// time duration is provided to indicate how often the background routines will sync developer to the Kong Admin API.
func NewCacheBasedProxyWithStagger(logger logr.Logger,
	kongConfig configuration.Kong,
	controllerClassName string,
	enableReverseSync bool,
	proxyRequestTimeout time.Duration,
	store store.CacheStores,
	service services.FileService,
	context context.Context,
) (Proxy, error) {
	proxy := &CachedProxyResolver{

		kongConfig:        kongConfig,
		enableReverseSync: enableReverseSync,

		store:   store,
		service: service,
		ctx:     context,

		logger: logger,

		controllerClassName: controllerClassName,

		proxyRequestTimeout: proxyRequestTimeout,

		configApplied: false,
	}

	// initialize the proxy
	if err := proxy.initialize(); err != nil {
		return nil, err
	}

	return proxy, nil
}

// -----------------------------------------------------------------------------
// Client Go Cached Proxy Resolver - Private Types
// -----------------------------------------------------------------------------

// CachedProxyResolver represents the cached objects and Kong DSL developer.
//
// This implements the Proxy interface to provide asynchronous, non-blocking updates to
// the Kong Admin API for controller-runtime based controller managers.
//
// This object's attributes are immutable (private), and it is threadsafe.
type CachedProxyResolver struct {
	// configApplied is true if config has been applied at least once
	configApplied      bool
	configAppliedMutex sync.RWMutex

	// kong developer
	kongConfig        configuration.Kong
	enableReverseSync bool
	dbmode            string
	version           semver.Version

	// context
	ctx context.Context

	// kong store
	store store.CacheStores
	// kong service
	service services.FileService

	promMetrics *metrics.CtrlFuncMetrics

	// server developer, flow control, channels and utility attributes
	controllerClassName string
	proxyRequestTimeout time.Duration

	logger logr.Logger
}

// -----------------------------------------------------------------------------
// Client Go Cached Proxy Resolver - Public Methods - Interface Implementation
// -----------------------------------------------------------------------------

func (p *CachedProxyResolver) UpdateObject(obj client.Object) error {

	switch obj := obj.(type) {
	// ----------------------------------------------------------------------------
	// Kong API Support
	// ----------------------------------------------------------------------------
	case *developer.KongFile:
		p.store.Update(obj)
		_, err := p.service.Update(p.ctx, Build(obj))
		return err
	default:
		return fmt.Errorf("cannot add unsupported kind %q to the store", obj.GetObjectKind().GroupVersionKind())
	}
}

func (p *CachedProxyResolver) DeleteObject(obj client.Object) error {
	switch obj := obj.(type) {
	// ----------------------------------------------------------------------------
	// Kong API Support
	// ----------------------------------------------------------------------------
	case *developer.KongFile:
		p.store.Delete(obj)
		_, err := p.service.Delete(p.ctx, Build(obj))
		return err
	default:
		return fmt.Errorf("cannot add unsupported kind %q to the store", obj.GetObjectKind().GroupVersionKind())
	}
}

func (p *CachedProxyResolver) ObjectExists(obj client.Object) (bool, error) {
	switch obj := obj.(type) {
	// ----------------------------------------------------------------------------
	// Kong API Support
	// ----------------------------------------------------------------------------
	case *developer.KongFile:
		file, err := p.service.Get(p.ctx, Build(obj))
		if err != nil {
			return false, err
		}
		if file != nil {
			return true, nil
		} else {
			return false, nil
		}
	default:
		return false, fmt.Errorf("cannot add unsupported kind %q to the store", obj.GetObjectKind().GroupVersionKind())
	}
}

func (p *CachedProxyResolver) ObjectExistsInCache(obj client.Object) (client.Object, bool, error) {
	cached, exists, err := p.store.Get(obj)
	if cached != nil {
		return cached.(client.Object), exists, err
	} else {
		return nil, exists, err
	}
}

func (p *CachedProxyResolver) NeedLeaderElection() bool {
	if p.dbmode == "off" {
		return false
	} else {
		return true
	}
}

func (p *CachedProxyResolver) Start(ctx context.Context) error {
	return nil
}

func (p *CachedProxyResolver) IsReady() bool {
	// If the proxy is has no database, it is only ready after a successful sync
	// Otherwise, it has no developer loaded
	if p.dbmode == "off" {
		p.configAppliedMutex.RLock()
		defer p.configAppliedMutex.RUnlock()
		return p.configApplied
	}
	// If the proxy has a database, it is ready immediately
	// It will load existing developer from the database
	return true
}

// -----------------------------------------------------------------------------
// Client Go Cached Proxy Resolver - Private Methods - Server Utils
// -----------------------------------------------------------------------------

// initialize validates connectivity with the Kong proxy and some of the developer options thereof
// and populates several local attributes given retrieved developer data from the proxy root config.
//
// Note: this must be run (and must succeed) in order to successfully start the cache server.
func (p *CachedProxyResolver) initialize() error {
	// download the kong root developer (and validate connectivity to the proxy API)
	root, err := p.kongRootWithTimeout()
	if err != nil {
		return err
	}

	// pull the proxy developer out of the root config and validate it
	proxyConfig, ok := root["configuration"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid root configuration, expected a map[string]interface{} got %T", proxyConfig["configuration"])
	}

	// validate the database developer for the proxy and check for supported database configurations
	dbmode, ok := proxyConfig["database"].(string)
	if !ok {
		return fmt.Errorf("invalid database developer, expected a string got %t", proxyConfig["database"])
	}
	switch dbmode {
	case "off", "":
		p.kongConfig.InMemory = true
	case "postgres":
		p.kongConfig.InMemory = false
	default:
		return fmt.Errorf("%s is not a supported database backend", dbmode)
	}

	// validate the proxy version
	proxySemver, err := kong.ParseSemanticVersion(kong.VersionFromInfo(root))
	if err != nil {
		return err
	}

	// store the gathered developer options
	p.kongConfig.Version = proxySemver
	p.dbmode = dbmode
	p.version = proxySemver
	p.promMetrics = metrics.NewCtrlFuncMetrics()

	return nil
}

// kongRootWithTimeout provides the root developer from Kong, but uses a configurable timeout to avoid long waits if the Admin API
// is not yet ready to respond. If a timeout error occurs, the caller is responsible for providing a retry mechanism.
func (p *CachedProxyResolver) kongRootWithTimeout() (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), p.proxyRequestTimeout)
	defer cancel()
	return p.kongConfig.Client.Root(ctx)
}

// Build file object
func Build(kongFile *developer.KongFile) (file *services.File) {
	var expectedContent string
	var expectedPath string
	if kongFile.Spec.Kind == developer.CONTENT {
		expectedPath = "content/" + kongFile.Spec.Path + "/" + kongFile.Spec.Name
		expectedContent = "---\n" +
			"title: " + kongFile.Spec.Title + "\n" +
			"layout: " + kongFile.Spec.Layout + "\n" +
			"---\n" +
			kongFile.Spec.Content
	} else if kongFile.Spec.Kind == developer.ASSET {
		expectedPath = "base/assets/" + kongFile.Spec.Path + "/" + kongFile.Spec.Name
		expectedContent = kongFile.Spec.Content
	} else if kongFile.Spec.Kind == developer.SPECIFICATION {
		expectedPath = "specs/" + kongFile.Spec.Path + "/" + kongFile.Spec.Name
		expectedContent = kongFile.Spec.Content
	}
	return &services.File{
		Path:     &expectedPath,
		Contents: &expectedContent,
	}
}
