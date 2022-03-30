package admission

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/go-logr/logr"
	"io"
	"net/http"
	"os"

	admission "k8s.io/api/admission/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	developer "kong-portal-controller/pkg/apis/v1"
)

var (
	scheme = runtime.NewScheme()
	codecs = serializer.NewCodecFactory(scheme)
)

const (
	DefaultAdmissionWebhookCertPath = "/admission-webhook/tls.crt"
	DefaultAdmissionWebhookKeyPath  = "/admission-webhook/tls.key"
)

type ServerConfig struct {
	ListenAddr string

	CertPath string
	Cert     string

	KeyPath string
	Key     string
}

func readKeyPairFiles(certPath, keyPath string) ([]byte, []byte, error) {
	cert, err := os.ReadFile(certPath)
	if err != nil {
		return nil, nil, fmt.Errorf("read cert from file %q: %w", certPath, err)
	}
	key, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, nil, fmt.Errorf("read key from file %q: %w", keyPath, err)
	}

	return cert, key, nil
}

func (sc *ServerConfig) toTLSConfig() (*tls.Config, error) {
	var cert, key []byte
	switch {
	case sc.CertPath == "" && sc.KeyPath == "" && sc.Cert != "" && sc.Key != "":
		cert, key = []byte(sc.Cert), []byte(sc.Key)

	case sc.CertPath != "" && sc.KeyPath != "" && sc.Cert == "" && sc.Key == "":
		var err error
		cert, key, err = readKeyPairFiles(sc.CertPath, sc.KeyPath)
		if err != nil {
			return nil, err
		}

	case sc.CertPath == "" && sc.KeyPath == "" && sc.Cert == "" && sc.Key == "":
		var err error
		cert, key, err = readKeyPairFiles(DefaultAdmissionWebhookCertPath, DefaultAdmissionWebhookKeyPath)
		if err != nil {
			return nil, err
		}

	default:
		return nil, fmt.Errorf("either cert/key files OR cert/key values must be provided, or none")
	}

	keyPair, err := tls.X509KeyPair(cert, key)
	if err != nil {
		return nil, fmt.Errorf("X509KeyPair error: %w", err)
	}
	return &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{keyPair},
	}, nil
}

func MakeTLSServer(config *ServerConfig, handler http.Handler) (*http.Server, error) {
	tlsConfig, err := config.toTLSConfig()
	if err != nil {
		return nil, err
	}
	return &http.Server{
		Addr:      config.ListenAddr,
		TLSConfig: tlsConfig,
		Handler:   handler,
	}, nil
}

// RequestHandler is an HTTP server that can validate Kong Ingress Controllers'
// Custom Resources using Kubernetes Admission Webhooks.
type RequestHandler struct {
	// Validator validates the entities that the k8s API-server asks
	// it the server to validate.
	Validator KongValidator

	Logger logr.Logger
}

// ServeHTTP parses AdmissionReview requests and responds back
// with the validation result of the entity.
func (a RequestHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Body == nil {
		a.Logger.Info("received request with empty body")
		http.Error(w, "admission review object is missing",
			http.StatusBadRequest)
		return
	}
	data, err := io.ReadAll(r.Body)
	if err != nil {
		a.Logger.Error(err, "failed to read request from client : %v", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	review := admission.AdmissionReview{}
	if err := json.Unmarshal(data, &review); err != nil {
		a.Logger.Error(err, "failed to parse AdmissionReview object: %v", err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	response, err := a.handleValidation(r.Context(), *review.Request)
	if err != nil {
		a.Logger.Error(err, "failed to run validation: %v", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	review.Response = response
	data, err = json.Marshal(review)
	if err != nil {
		a.Logger.Error(err, "failed to marshal response: %v", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_, err = w.Write(data)
	if err != nil {
		a.Logger.Error(err, "failed to write response: %v", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

var (
	kongFileGVResource = meta.GroupVersionResource{
		Group:    developer.SchemeGroupVersion.Group,
		Version:  developer.SchemeGroupVersion.Version,
		Resource: "kongfiles",
	}
)

func (a RequestHandler) handleValidation(ctx context.Context, request admission.AdmissionRequest) (
	*admission.AdmissionResponse, error) {
	var response admission.AdmissionResponse

	var ok bool
	var message string
	var err error

	switch request.Resource {
	case kongFileGVResource:
		plugin := developer.KongFile{}
		deserializer := codecs.UniversalDeserializer()
		_, _, err = deserializer.Decode(request.Object.Raw,
			nil, &plugin)
		if err != nil {
			return nil, err
		}

		ok, message, err = a.Validator.ValidateKongFile(ctx, plugin)
		if err != nil {
			return nil, err
		}

	default:
		return nil, fmt.Errorf("unknown resource type to validate: %s/%s %s",
			request.Resource.Group, request.Resource.Version,
			request.Resource.Resource)
	}
	response.UID = request.UID
	response.Allowed = ok
	response.Result = &meta.Status{
		Message: message,
	}
	if !ok {
		response.Result.Code = 400
	}
	return &response, nil
}
