package agents

import (
	"context"
	"mtrx/internal/events"
)

type Source struct {
	Path    string
	Adapter string
}

type Adapter interface {
	ID() string
	Name() string
	Detect(ctx context.Context) (bool, error)
	Sources() []Source
	Collect(ctx context.Context, source Source) ([]events.Event, error)
	Watch(ctx context.Context, source Source, emit func(events.Event)) error
}
