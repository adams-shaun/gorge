package extern

import (
	"context"
	"fmt"
	"sync"

	"github.com/adams-shaun/gorge/draft/draftlab"
)

// Registry resolves role specs for one lab: "ext:" specs (ParseSpec's
// shape) dial their endpoint once per process and share the one Agent
// across every role and every concurrent game, while anything else goes to
// the lab's built-ins unchanged. A spec may pin a model version
// ("ext:exec:agent@gen7"); a pinned version that the dialed agent does not
// serve is a draftlab.ErrVersion error — the same freeze rule as the
// built-ins.
//
// Resolution is NOT goroutine-safe: resolve every spec before the run's
// workers start. The returned Named values (and every Agent) are.
type Registry struct {
	ctx  context.Context
	lab  *draftlab.Lab
	opts Options

	mu     sync.Mutex // guards the slices below between resolutions
	agents []*Agent   // dial order
	byKey  map[string]*Agent
}

// NewRegistry builds a Registry over the lab. The ctx bounds every dial it
// performs (an exec child is started with exec.CommandContext, so
// cancelling it kills the child and ends its session).
func NewRegistry(ctx context.Context, l *draftlab.Lab, o Options) *Registry {
	return &Registry{ctx: ctx, lab: l, opts: o, byKey: map[string]*Agent{}}
}

// Drafter resolves a drafter spec: "ext:..." dials (once per endpoint) and
// wraps the agent; anything else is draftlab.ParseAgent plus the lab's
// built-in drafter.
func (r *Registry) Drafter(spec string) (draftlab.NamedDrafter, error) {
	if !IsSpec(spec) {
		ref, err := draftlab.ParseAgent(spec)
		if err != nil {
			return draftlab.NamedDrafter{}, fmt.Errorf("extern: registry: %w", err)
		}
		return r.lab.Drafter(ref)
	}
	a, err := r.agentFor(spec)
	if err != nil {
		return draftlab.NamedDrafter{}, err
	}
	return a.Drafter(r.lab)
}

// Builder resolves a builder spec; built-ins pass through the lab.
func (r *Registry) Builder(spec string) (draftlab.NamedBuilder, error) {
	if !IsSpec(spec) {
		ref, err := draftlab.ParseAgent(spec)
		if err != nil {
			return draftlab.NamedBuilder{}, fmt.Errorf("extern: registry: %w", err)
		}
		return r.lab.Builder(ref)
	}
	a, err := r.agentFor(spec)
	if err != nil {
		return draftlab.NamedBuilder{}, err
	}
	return a.Builder(r.lab)
}

// Player resolves a player spec; built-ins pass through the lab.
func (r *Registry) Player(spec string) (draftlab.NamedPlayer, error) {
	if !IsSpec(spec) {
		ref, err := draftlab.ParseAgent(spec)
		if err != nil {
			return draftlab.NamedPlayer{}, fmt.Errorf("extern: registry: %w", err)
		}
		return r.lab.Player(ref)
	}
	a, err := r.agentFor(spec)
	if err != nil {
		return draftlab.NamedPlayer{}, err
	}
	return a.Player(r.lab)
}

// Agents lists every dialed agent in dial order.
func (r *Registry) Agents() []*Agent {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]*Agent(nil), r.agents...)
}

// Close closes every dialed agent in dial order and returns the first
// error.
func (r *Registry) Close() error {
	var first error
	for _, a := range r.Agents() {
		if err := a.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

// agentFor resolves the one Agent an ext: spec names: dialing the endpoint
// once per (transport, target) key, sharing it across roles, and checking
// a pinned model version against the hello's. An agent whose version does
// not match a pin is closed again and never registered: a later spec with
// the matching pin dials fresh.
func (r *Registry) agentFor(spec string) (*Agent, error) {
	ep, pinned, err := ParseSpec(spec)
	if err != nil {
		return nil, err
	}
	key := ep.Transport + ":" + ep.Target
	r.mu.Lock()
	a, ok := r.byKey[key]
	r.mu.Unlock()
	if ok {
		if pinned != "" && pinned != a.hello.ModelVersion {
			return nil, r.versionError(a, pinned)
		}
		return a, nil
	}
	a, err = Dial(r.ctx, ep, r.opts)
	if err != nil {
		return nil, err
	}
	if pinned != "" && pinned != a.hello.ModelVersion {
		_ = a.Close()
		return nil, r.versionError(a, pinned)
	}
	r.mu.Lock()
	r.agents = append(r.agents, a)
	r.byKey[key] = a
	r.mu.Unlock()
	return a, nil
}

// versionError names the freeze refusal: the dialed agent's model_version
// is not the spec's pinned one.
func (r *Registry) versionError(a *Agent, pinned string) error {
	return fmt.Errorf("%w: agent %s serves model_version %q, spec pins %q",
		draftlab.ErrVersion, a.Ref().Name, a.hello.ModelVersion, pinned)
}
