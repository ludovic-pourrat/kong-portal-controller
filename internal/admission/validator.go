package admission

import (
	"context"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	developer "kong-portal-controller/pkg/apis/v1"
)

// KongValidator validates Kong entities.
type KongValidator interface {
	ValidateKongFile(ctx context.Context, plugin developer.KongFile) (bool, string, error)
}

// KongHTTPValidator implements KongValidator interface to validate Kong
// entities using the Admin API of Kong.
type KongHTTPValidator struct {
	Logger        logr.Logger
	ManagerClient client.Client
}

// NewKongHTTPValidator provides a new KongHTTPValidator object provided a
// controller-runtime client which will be used to retrieve reference objects
// such as consumer credentials secrets. If you do not pass a cached client
// here, the performance of this validator can get very poor at high scales.
func NewKongHTTPValidator(
	logger logr.Logger,
	managerClient client.Client,
) KongHTTPValidator {
	return KongHTTPValidator{
		Logger:        logger,
		ManagerClient: managerClient,
	}
}

// ValidateKongFile checks if the developer CRD is valid.
func (validator KongHTTPValidator) ValidateKongFile(
	ctx context.Context,
	kongFile developer.KongFile,
) (bool, string, error) {
	validator.Logger.Info("Validating resource", "namespace", kongFile.Namespace, "name", kongFile.Name)
	if kongFile.Name == "" {
		return false, ErrKongFileNameEmpty, nil
	}
	if kongFile.Spec.Name == "" {
		return false, ErrKongFileSpecNameEmpty, nil
	}
	if kongFile.Spec.Path == "" {
		return false, ErrKongFileSpecPathEmpty, nil
	}
	if kongFile.Spec.Kind == developer.CONTENT {
		if kongFile.Spec.Title == "" {
			return false, ErrKongFileSpecTitleEmpty, nil
		}
		if kongFile.Spec.Layout == "" {
			return false, ErrKongFileSpecLayoutEmpty, nil
		}
	}
	return true, "", nil
}
