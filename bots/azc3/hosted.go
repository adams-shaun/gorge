package azc3

import (
	"errors"
	"fmt"
	"sync"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/azredeal"
	"github.com/adams-shaun/gorge/bots/gonet"
	"github.com/adams-shaun/gorge/bots/policynet"
	"github.com/adams-shaun/gorge/bots/searchprobe"
	"github.com/adams-shaun/gorge/engine/seat"
)

// Hosted is one hosted az policy: the az-redeal search seat over one .dzgw
// network with the search spec the self-play loop trains and measures
// (Config, closed under a belief by ClosedConfig). az-c3 is one; az-c4
// (bots/azc4) is another that only ever plays closed lists. Each registers
// only when enabled: a network is a deployment artifact, not a source file.
type Hosted struct {
	spec    HostedSpec
	mu      sync.Mutex
	enabled bool
	model   *policynet.Model
	belief  *searchprobe.Belief
}

// HostedSpec is what tells hosted az policies apart.
type HostedSpec struct {
	// Info is the registry entry. Info.Name is the policy key and the
	// prefix of the factory's errors.
	Info bots.Info
	// ErrPrefix prefixes Enable's errors ("azc3: already enabled").
	ErrPrefix string
	// Closed makes the policy closed-list only: Enable refuses (EnableClosed
	// is the way in), the belief can never be cleared, and the factory
	// refuses to build a seat without one. Info must then declare
	// ClosedLists, which lets the host seat it against a human -- and the
	// host checks every seat it builds rather than trusting the declaration
	// (server/host/closedlist.go).
	Closed bool
}

// NewHosted returns an unregistered hosted policy.
func NewHosted(spec HostedSpec) *Hosted { return &Hosted{spec: spec} }

// Enable loads the network at path and registers the policy with open lists
// (until SetBelief closes them). It must run before the server resolves its
// policy flags, once per process; a path that is not a valid .dzgw network
// is an error and registers nothing. A closed-only policy is refused here:
// it must start with its belief (EnableClosed).
func (h *Hosted) Enable(path string) error {
	if h.spec.Closed {
		return fmt.Errorf("%s: closed-list only: enable it with a belief", h.spec.ErrPrefix)
	}
	return h.enable(path, nil)
}

// EnableClosed is Enable with the opponents' lists closed under b from the
// first seat on. b must be non-nil.
func (h *Hosted) EnableClosed(path string, b *searchprobe.Belief) error {
	if b == nil {
		return fmt.Errorf("%s: no belief: the opponents' lists must be closed", h.spec.ErrPrefix)
	}
	return h.enable(path, b)
}

func (h *Hosted) enable(path string, b *searchprobe.Belief) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.enabled {
		return fmt.Errorf("%s: already enabled", h.spec.ErrPrefix)
	}
	if h.spec.Closed != h.spec.Info.ClosedLists {
		return fmt.Errorf("%s: Closed %v but Info.ClosedLists %v", h.spec.ErrPrefix, h.spec.Closed, h.spec.Info.ClosedLists)
	}
	m, err := gonet.Model(path)
	if err != nil {
		return fmt.Errorf("%s: %w", h.spec.ErrPrefix, err)
	}
	h.model = m
	if b != nil {
		h.belief = b
	}
	bots.Register(bots.Entry{Info: h.spec.Info, New: h.newSeat})
	h.enabled = true
	return nil
}

// SetBelief closes the opponents' deck lists for every seat built afterwards
// (ClosedConfig); nil restores the open lists. A closed-only policy keeps
// its belief: nil is ignored there.
func (h *Hosted) SetBelief(b *searchprobe.Belief) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if b == nil && h.spec.Closed {
		return
	}
	h.belief = b
}

// newSeat is the registry factory. The seat pays its own mana through the
// engine's automatic payment plans, as it did in training, so a table built
// without them is refused here rather than played at a handicap.
func (h *Hosted) newSeat(o bots.Options) (seat.Seat, error) {
	name := h.spec.Info.Name
	if !o.AutoPayMana {
		return nil, fmt.Errorf("%s needs automatic mana payment (gorged -bot-auto-mana, the default)", name)
	}
	h.mu.Lock()
	m, b := h.model, h.belief
	h.mu.Unlock()
	if m == nil {
		return nil, fmt.Errorf("%s: no network loaded (Enable was not called)", name)
	}
	if h.spec.Closed && b == nil {
		return nil, errors.New(name + ": no belief: the opponents' lists must be closed")
	}
	return azredeal.NewWithModel(o, ClosedConfig(b), m)
}
