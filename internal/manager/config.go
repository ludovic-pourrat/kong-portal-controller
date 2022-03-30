package manager

import (
	"context"
	"fmt"
	"time"

	"github.com/kong/go-kong/kong"
	"github.com/spf13/pflag"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"kong-portal-controller/internal/adminapi"
	"kong-portal-controller/internal/admission"
	"kong-portal-controller/internal/annotations"
	"kong-portal-controller/internal/dataplane/proxy"
)

// -----------------------------------------------------------------------------
// Controller Manager - Config
// -----------------------------------------------------------------------------

// Config collects all developer that the controller manager takes from the environment.
type Config struct {
	// See flag definitions in RegisterFlags(...) for documentation of the fields defined here.

	// Logging configurations
	LogLevel            string
	LogFormat           string
	LogReduceRedundancy bool

	// Kong high-level controller manager configurations
	KongAdminAPIConfig adminapi.HTTPClientOpts
	KongAdminToken     string
	KongWorkspace      string
	AnonymousReports   bool
	EnableReverseSync  bool
	SyncPeriod         time.Duration

	// Kong Proxy configurations
	APIServerHost            string
	APIServerQPS             int
	APIServerBurst           int
	MetricsAddr              string
	ProbeAddr                string
	KongAdminURL             string
	ProxyTimeoutSeconds      float32
	KongCustomEntitiesSecret string

	// Kubernetes configurations
	KubeconfigPath          string
	ControllerClassName     string
	EnableLeaderElection    bool
	LeaderElectionNamespace string
	LeaderElectionID        string
	Concurrency             int
	FilterTags              []string
	WatchNamespaces         []string

	// Ingress status
	PublishService       string
	PublishStatusAddress []string

	// Admission Webhook server config
	AdmissionServer admission.ServerConfig

	// Diagnostics and performance
	EnableProfiling bool
}

// -----------------------------------------------------------------------------
// Controller Manager - Config - Methods
// -----------------------------------------------------------------------------

// FlagSet binds the provided Config to commandline flags.
func (c *Config) FlagSet() *pflag.FlagSet {

	flagSet := pflag.NewFlagSet("", pflag.ExitOnError)

	// Logging configurations
	flagSet.StringVar(&c.LogLevel, "log-level", "info", `Level of logging for the controller. Allowed values are trace, debug, info, warn, error, fatal and panic.`)
	flagSet.StringVar(&c.LogFormat, "log-format", "text", `Format of logs of the controller. Allowed values are text and json.`)
	flagSet.BoolVar(&c.LogReduceRedundancy, "debug-log-reduce-redundancy", false, `If enabled, repetitive log entries are suppressed. Built for testing environments - production use not recommended.`)
	flagSet.MarkHidden("debug-log-reduce-redundancy")

	// Kong high-level controller manager configurations
	flagSet.BoolVar(&c.KongAdminAPIConfig.TLSSkipVerify, "kong-admin-tls-skip-verify", false, "Disable verification of TLS certificate of Kong's Admin endpoint.")
	flagSet.StringVar(&c.KongAdminAPIConfig.TLSServerName, "kong-admin-tls-server-name", "", "SNI name to use to verify the certificate presented by Kong in TLS.")
	flagSet.StringVar(&c.KongAdminAPIConfig.CACertPath, "kong-admin-ca-cert-file", "", `Path to PEM-encoded CA certificate file to verify Kong's Admin SSL certificate.`)
	flagSet.StringVar(&c.KongAdminAPIConfig.CACert, "kong-admin-ca-cert", "", `PEM-encoded CA certificate to verify Kong's Admin SSL certificate.`)

	flagSet.StringSliceVar(&c.KongAdminAPIConfig.Headers, "kong-admin-header", nil, `add a header (key:value) to every Admin API call, this flag can be used multiple times to specify multiple headers`)
	flagSet.StringVar(&c.KongAdminToken, "kong-admin-token", "", `The Kong Enterprise RBAC token used by the controller.`)
	flagSet.StringVar(&c.KongWorkspace, "kong-workspace", "", "Kong Enterprise workspace to configure. Leave this empty if not using Kong workspaces.")
	flagSet.BoolVar(&c.AnonymousReports, "anonymous-reports", true, `Send anonymized usage data to help improve Kong`)
	flagSet.BoolVar(&c.EnableReverseSync, "enable-reverse-sync", false, `Send developer to Kong even if the developer checksum has not changed since previous update.`)
	flagSet.DurationVar(&c.SyncPeriod, "sync-period", time.Hour*48, `Relist and confirm cloud resources this often`) // 48 hours derived from controller-runtime defaults

	flagSet.StringVar(&c.KongAdminAPIConfig.TLSClientCertPath, "kong-admin-tls-client-cert-file", "", "mTLS client certificate file for authentication.")
	flagSet.StringVar(&c.KongAdminAPIConfig.TLSClientKeyPath, "kong-admin-tls-client-key-file", "", "mTLS client key file for authentication.")
	flagSet.StringVar(&c.KongAdminAPIConfig.TLSClientCert, "kong-admin-tls-client-cert", "", "mTLS client certificate for authentication.")
	flagSet.StringVar(&c.KongAdminAPIConfig.TLSClientKey, "kong-admin-tls-client-key", "", "mTLS client key for authentication.")

	// Kong Proxy and Proxy Cache configurations
	flagSet.StringVar(&c.APIServerHost, "apiserver-host", "", `The Kubernetes API server URL. If not set, the controller will use cluster config discovery.`)
	flagSet.IntVar(&c.APIServerQPS, "apiserver-qps", 100, "The Kubernetes API RateLimiter maximum queries per second")
	flagSet.IntVar(&c.APIServerBurst, "apiserver-burst", 300, "The Kubernetes API RateLimiter maximum burst queries per second")
	flagSet.StringVar(&c.MetricsAddr, "metrics-bind-address", fmt.Sprintf(":%v", MetricsPort), "The address the metric endpoint binds to.")
	flagSet.StringVar(&c.ProbeAddr, "health-probe-bind-address", fmt.Sprintf(":%v", HealthzPort), "The address the probe endpoint binds to.")
	flagSet.StringVar(&c.KongAdminURL, "kong-admin-url", "http://localhost:8001", `The Kong Admin URL to connect to in the format "protocol://address:port".`)
	flagSet.Float32Var(&c.ProxyTimeoutSeconds, "proxy-timeout-seconds", proxy.DefaultProxyTimeoutSeconds,
		"Define the rate (in seconds) in which the timeout developer will be applied to the Kong client.",
	)
	flagSet.StringVar(&c.KongCustomEntitiesSecret, "kong-custom-entities-secret", "", `A Secret containing custom entities for DB-less mode, in "namespace/name" format`)

	// Kubernetes configurations
	flagSet.StringVar(&c.KubeconfigPath, "kubeconfig", "", "Path to the kubeconfig file.")
	flagSet.StringVar(&c.ControllerClassName, "controller-class", annotations.DefaultControllerClass, `Name of the controller class to route through this controller.`)
	flagSet.StringVar(&c.LeaderElectionID, "election-id", "4g374a9e.konghq.com", `Election id to use for status update.`)
	flagSet.StringVar(&c.LeaderElectionNamespace, "election-namespace", "", `Leader election namespace to use when running outside a cluster`)
	flagSet.StringSliceVar(&c.FilterTags, "kong-admin-filter-tag", []string{"managed-by-portal-controller"}, "The tag used to manage and filter entities in Kong. This flag can be specified multiple times to specify multiple tags. This setting will be silently ignored if the Kong instance has no tags support.")
	flagSet.IntVar(&c.Concurrency, "kong-admin-concurrency", 10, "Max number of concurrent requests sent to Kong's Admin API.")
	flagSet.StringSliceVar(&c.WatchNamespaces, "watch-namespace", nil,
		`Namespace(s) to watch for Kubernetes resources. Defaults to all namespaces. To watch multiple namespaces, use
		a comma-separated list of namespaces.`)

	// Ingress status
	flagSet.StringVar(&c.PublishService, "publish-service", "", `Service fronting Ingress resources in "namespace/name"
			format. The controller will update Ingress status information with this Service's endpoints.`)
	flagSet.StringSliceVar(&c.PublishStatusAddress, "publish-status-address", []string{}, `User-provided addresses in
			comma-separated string format, for use in lieu of "publish-service" when that Service lacks useful address
			information (for example, in bare-metal environments).`)

	// Admission Webhook server config
	flagSet.StringVar(&c.AdmissionServer.ListenAddr, "admission-webhook-listen", "off",
		`The address to start admission controller on (ip:port).  Setting it to 'off' disables the admission controller.`)
	flagSet.StringVar(&c.AdmissionServer.CertPath, "admission-webhook-cert-file", "",
		`admission server PEM certificate file path; `+
			`if both this and the cert value is unset, defaults to `+admission.DefaultAdmissionWebhookCertPath)
	flagSet.StringVar(&c.AdmissionServer.KeyPath, "admission-webhook-key-file", "",
		`admission server PEM private key file path; `+
			`if both this and the key value is unset, defaults to `+admission.DefaultAdmissionWebhookKeyPath)
	flagSet.StringVar(&c.AdmissionServer.Cert, "admission-webhook-cert", "",
		`admission server PEM certificate value`)
	flagSet.StringVar(&c.AdmissionServer.Key, "admission-webhook-key", "",
		`admission server PEM private key value`)

	// Diagnostics
	flagSet.BoolVar(&c.EnableProfiling, "profiling", false, fmt.Sprintf("Enable profiling via web interface host:%v/debug/pprof/", DiagnosticsPort))

	flagSet.Int("stderrthreshold", 0, "DEPRECATED: has no effect and will be removed in future releases (see github issue #1297)")
	flagSet.Bool("update-status-on-shutdown", false, `DEPRECATED: no longer has any effect and will be removed in a later release (see github issue #1304)`)

	return flagSet
}

func (c *Config) GetKongClient(ctx context.Context) (*kong.Client, error) {
	if c.KongAdminToken != "" {
		c.KongAdminAPIConfig.Headers = append(c.KongAdminAPIConfig.Headers, "kong-admin-token:"+c.KongAdminToken)
	}
	httpclient, err := adminapi.MakeHTTPClient(&c.KongAdminAPIConfig)
	if err != nil {
		return nil, err
	}

	return adminapi.GetKongClientForWorkspace(ctx, c.KongAdminURL, c.KongWorkspace, httpclient)
}

func (c *Config) GetKubeconfig() (*rest.Config, error) {
	config, err := clientcmd.BuildConfigFromFlags(c.APIServerHost, c.KubeconfigPath)
	if err != nil {
		return nil, err
	}

	// Configure k8s client rate-limiting
	config.QPS = float32(c.APIServerQPS)
	config.Burst = c.APIServerBurst

	return config, err
}

func (c *Config) GetKubeClient() (client.Client, error) {
	conf, err := c.GetKubeconfig()
	if err != nil {
		return nil, err
	}
	return client.New(conf, client.Options{})
}
