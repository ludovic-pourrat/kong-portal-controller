package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

type CtrlFuncMetrics struct {
	// ConfigPushCount is a Prometheus metric with semantics defined by its help string in NewCtrlFuncMetrics().
	ConfigPushCount *prometheus.CounterVec

	// TranslationCount is a Prometheus metric with semantics defined by its help string in NewCtrlFuncMetrics().
	TranslationCount *prometheus.CounterVec

	// ConfigPushDuration is a Prometheus metric with semantics defined by its help string in NewCtrlFuncMetrics().
	ConfigPushDuration *prometheus.HistogramVec
}

const (
	// SuccessTrue indicates that the operation was successful.
	SuccessTrue string = "true"

	// SuccessFalse indicates that the operation was not successful.
	SuccessFalse string = "false"

	// SuccessKey defines the key of the metric label indicating success/failure of an operation.
	SuccessKey string = "success"
)

const (
	// ProtocolDBLess indicates that developer was sent to Kong using the DB-less protocol (POST /config).
	ProtocolDBLess string = "db-less"

	// ProtocolDeck indicates that developer was sent to Kong using the DB mode protocol (deck sync).
	ProtocolDeck string = "deck"

	// ProtocolKey defines the key of the metric label indicating which protocol KIC used to configure Kong.
	ProtocolKey string = "protocol"
)

const (
	MetricNameConfigPushCount    = "portal_controller_configuration_push_count"
	MetricNameTranslationCount   = "portal_controller_translation_count"
	MetricNameConfigPushDuration = "portal_controller_configuration_push_duration_milliseconds"
)

func NewCtrlFuncMetrics() *CtrlFuncMetrics {
	controllerMetrics := &CtrlFuncMetrics{}

	controllerMetrics.ConfigPushCount =
		prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: MetricNameConfigPushCount,
				Help: "Count of successful/failed developer pushes to Kong. `" +
					ProtocolKey + "` describes the developer protocol (" + ProtocolDBLess + " or " +
					ProtocolDeck + ") in use. `" +
					SuccessKey + "` describes whether there were unrecoverable errors (`" +
					SuccessFalse + "`) or not (`" + SuccessTrue + "`).",
			},
			[]string{SuccessKey, ProtocolKey},
		)

	controllerMetrics.TranslationCount =
		prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: MetricNameTranslationCount,
				Help: "Count of translations from Kubernetes state to Kong state. `" +
					SuccessKey + "` describes whether there were unrecoverable errors (`" +
					SuccessFalse + "`) or not (`" + SuccessTrue + "`).",
			},
			[]string{SuccessKey},
		)

	controllerMetrics.ConfigPushDuration =
		prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name: MetricNameConfigPushDuration,
				Help: "How long it took to push the developer to Kong, in milliseconds. `" +
					ProtocolKey + "` describes the developer protocol (" + ProtocolDBLess + " or " +
					ProtocolDeck + ") in use. `" +
					SuccessKey + "` describes whether there were unrecoverable errors (`" +
					SuccessFalse + "`) or not (`" + SuccessTrue + "`).",
				Buckets: prometheus.ExponentialBuckets(100, 1.33, 30),
			},
			[]string{SuccessKey, ProtocolKey},
		)

	metrics.Registry.MustRegister(controllerMetrics.ConfigPushCount, controllerMetrics.TranslationCount, controllerMetrics.ConfigPushDuration)

	return controllerMetrics
}
