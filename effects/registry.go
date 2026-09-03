// Package effects implements the primitives Forge card scripts reference. It
// reaches the engine through the Host interface, so it never imports rules and
// the dependency graph stays acyclic.
package effects

import (
	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// Host is everything an effect may do to a game: read it, and propose events.
// Deliberately tiny — an effect that needs more is a sign the primitive is
// doing rules work that belongs in the rules package.
type Host interface {
	Game() *state.Game
	Emit(events.Event)
	// Rand is the engine's seeded generator. Effects that need randomness must
	// use it and nothing else, or replay breaks.
	Rand(n int) int
}

// Ctx carries the bindings a Forge script refers to during resolution.
type Ctx struct {
	Source     state.ObjID
	Controller state.PlayerID
	Targets    []state.Target
	Remembered []state.Target
}

type Effect func(h Host, c *Ctx, sa *cards.SA)

var registry = map[string]Effect{}

// Register installs an implementation for a Forge API name. Called from init
// functions in this package; re-registering replaces, which is what lets the
// plugin tier in M3 override a native primitive.
func Register(api string, fn Effect) { registry[api] = fn }

func unregister(apis ...string) {
	for _, a := range apis {
		delete(registry, a)
	}
}

// Supported reports the primitive set this build implements, in the same
// prefixed form cards.Face.Primitives uses, so it feeds straight into
// cards.Registry.Coverage.
func Supported() map[string]bool {
	out := make(map[string]bool, len(registry)+len(supportedNonAPI))
	for k := range registry {
		out["api:"+k] = true
	}
	for k := range supportedNonAPI {
		out[k] = true
	}
	return out
}

// supportedNonAPI holds keyword, trigger, static and replacement primitives,
// which are implemented in rules rather than as effect functions. Tasks 18-20
// fill it in.
var supportedNonAPI = map[string]bool{}

// RegisterNonAPI records a keyword, trigger, static or replacement primitive as
// implemented. The name must carry its prefix, e.g. "kw:Flying".
func RegisterNonAPI(prefixed ...string) {
	for _, p := range prefixed {
		supportedNonAPI[p] = true
	}
}

const maxChain = 32

// Resolve runs an ability and every sub-ability chained beneath it.
func Resolve(h Host, c *Ctx, sa *cards.SA) {
	for d := 0; sa != nil && d < maxChain; d, sa = d+1, sa.Sub {
		fn, ok := registry[sa.API]
		if !ok {
			// Unimplemented primitives must be loud but harmless: deck-build
			// validation is supposed to have caught this already.
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
				Text: "unimplemented API " + sa.API})
			continue
		}
		fn(h, c, sa)
	}
}
