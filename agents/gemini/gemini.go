package gemini

import "context"
import "mtrx/internal/agents"
import "mtrx/internal/events"

type Adapter struct{}

func (a *Adapter) ID() string                               { return "gemini" }
func (a *Adapter) Name() string                             { return "gemini" }
func (a *Adapter) Detect(ctx context.Context) (bool, error) { return false, nil }
func (a *Adapter) Sources() []agents.Source                 { return nil }
func (a *Adapter) Collect(ctx context.Context, src agents.Source) ([]events.Event, error) {
	return nil, nil
}
func (a *Adapter) Watch(ctx context.Context, src agents.Source, emit func(events.Event)) error {
	<-ctx.Done()
	return nil
}
