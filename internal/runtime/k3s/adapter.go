package k3s

import (
	"context"
	"errors"

	"github.com/prodops-chronicles/prodops/internal/runtime"
)

// ErrNotImplemented is returned by all k3s adapter methods in v1.0.
// Replace with actual k8s client-go calls in v2.0.
var ErrNotImplemented = errors.New("k3s runtime not implemented in v1.0")

// Adapter is a stub that satisfies runtime.RuntimeAdapter.
// In v2.0 this will use k8s.io/client-go to apply manifests.
type Adapter struct{}

func New() *Adapter { return &Adapter{} }

func (a *Adapter) StartModule(_ context.Context, _ string) error {
	return ErrNotImplemented
}

func (a *Adapter) StopModule(_ context.Context, _ string) error {
	return ErrNotImplemented
}

func (a *Adapter) ModuleStatus(_ context.Context, _ string) (runtime.PodStatus, error) {
	return runtime.PodStatus{}, ErrNotImplemented
}

func (a *Adapter) ListRunning(_ context.Context) ([]runtime.PodStatus, error) {
	return nil, ErrNotImplemented
}

func (a *Adapter) WriteModuleDefinition(_ context.Context, _ string) error {
	return ErrNotImplemented
}

func (a *Adapter) RemoveModuleDefinition(_ context.Context, _ string) error {
	return ErrNotImplemented
}
