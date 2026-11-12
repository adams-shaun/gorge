package effects

import (
	"github.com/adams-shaun/gorge/engine/decision"
)

// chainInvestigateHost records every posed ask in order and marks the host
// Suspended after each, so Resolve stops descending into the SubAbility$ walk
// exactly as rules.Engine does (the fx42AskHost shape, reused here).
type chainInvestigateHost struct {
	fakeHost
	asks      []*decision.Decision
	suspended bool
}

func (h *chainInvestigateHost) Ask(d *decision.Decision) bool {
	cp := *d
	cp.Options = append([]decision.Option(nil), d.Options...)
	h.asks = append(h.asks, &cp)
	h.suspended = true
	return true
}

func (h *chainInvestigateHost) Suspended() bool { return h.suspended }
