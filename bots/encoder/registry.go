package encoder

import (
	"fmt"
	"sort"
	"sync"

	"github.com/adams-shaun/gorge/bots/policynet"
)

var (
	mu     sync.RWMutex
	byName = map[string]ViewEncoder{}
	sorted []ViewEncoder // by Name; rebuilt on Register
)

// Register adds e under e.Schema().Name. Call it from init. An empty name or a
// duplicate name panics.
func Register(e ViewEncoder) {
	name := e.Schema().Name
	if name == "" {
		panic("encoder: Register with an empty name")
	}
	mu.Lock()
	defer mu.Unlock()
	if _, dup := byName[name]; dup {
		panic(fmt.Sprintf("encoder: %q registered twice", name))
	}
	byName[name] = e
	sorted = append(sorted, e)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Schema().Name < sorted[j].Schema().Name })
}

// Lookup returns the encoder registered under name.
func Lookup(name string) (ViewEncoder, bool) {
	mu.RLock()
	defer mu.RUnlock()
	e, ok := byName[name]
	return e, ok
}

// List returns every registered encoder sorted by name. The slice is a copy.
func List() []ViewEncoder {
	mu.RLock()
	defer mu.RUnlock()
	return append([]ViewEncoder(nil), sorted...)
}

// Resolve is what loading a net does: find the encoder a header names and
// refuse a hash mismatch or a Hidden encoder when the context is a seat.
func Resolve(name string, hash uint64, seat bool) (ViewEncoder, error) {
	e, ok := Lookup(name)
	if !ok {
		return nil, fmt.Errorf("encoder %q is not registered", name)
	}
	s := e.Schema()
	if got := s.Hash(); got != hash {
		return nil, fmt.Errorf("encoder %q: net hash %#016x, registered %#016x", name, hash, got)
	}
	if seat && s.Hidden {
		return nil, fmt.Errorf("encoder %q reads hidden information and cannot be seated", name)
	}
	return e, nil
}

// ForName returns the encoder a net names: the registered encoder called name,
// or, for a net from before the header (name ""), the built-in encoder of its
// feature set. It is nil when the named encoder is not registered (a net that
// loaded checks this already) or the feature set has no built-in.
func ForName(name string, fs policynet.FeatureSet) ViewEncoder {
	if name != "" {
		e, _ := Lookup(name)
		return e
	}
	e, _ := Lookup(fs.String())
	return e
}

// ForModel is ForName for the model a call site holds: the encoder its
// network names, else its FeatureSet's. A nil model returns nil.
func ForModel(m *policynet.Model) ViewEncoder {
	if m == nil {
		return nil
	}
	return ForName(m.Encoder, m.Features)
}
