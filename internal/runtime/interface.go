package runtime

import "context"

// RuntimeAdapter abstracts the container orchestration layer.
// The service layer and handlers only import this interface —
// never the compose or k3s packages directly.
type RuntimeAdapter interface {
	StartModule(ctx context.Context, moduleID string) error
	StopModule(ctx context.Context, moduleID string) error
	ModuleStatus(ctx context.Context, moduleID string) (PodStatus, error)
	ListRunning(ctx context.Context) ([]PodStatus, error)
	WriteModuleDefinition(ctx context.Context, moduleID string) error
	RemoveModuleDefinition(ctx context.Context, moduleID string) error
}

type PodStatus struct {
	ModuleID string
	Running  bool
	Healthy  bool
	ImageTag string
}
