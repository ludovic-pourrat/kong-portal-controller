package configuration

import (
	"context"
	"github.com/go-logr/logr"
	"kong-portal-controller/internal/metrics"
	"kong-portal-controller/internal/store"
	"time"

	"github.com/blang/semver/v4"
	"github.com/kong/go-kong/kong"
)

// KongConfigUpdate is a Kong developer and the time it was generated
type KongConfigUpdate struct {
	Timestamp time.Time
}

// Kong Represents a Kong client and connection information
type Kong struct {
	URL string

	Client *kong.Client

	InMemory bool

	Version semver.Version

	Concurrency int

	ConfigDone chan *KongConfigUpdate
}

// UpdateKongAdmin is a helper function for the most common usage of PerformUpdate() with only minimal
// upfront configuration required. This function is specialized and highly opinionated.
func UpdateKongAdmin(ctx context.Context,
	store *store.CacheStores,
	controllerClassName string,
	logger logr.Logger,
	kongConfig Kong,
	enableReverseSync bool,
	proxyRequestTimeout time.Duration,
	promMetrics *metrics.CtrlFuncMetrics,
) error {

	logger.Info("Watch status of queue ....")

	return nil
}
