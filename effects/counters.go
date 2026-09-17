package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func init() {
	Register("PutCounter", effPutCounter)
	Register("RemoveCounterAll", effRemoveCounterAll)
	Register("Regenerate", effRegenerate)
}

func effPutCounter(h Host, c *Ctx, sa *cards.SA) {
	n := Num(h, c, sa, "CounterNum", 1)
	if n < 0 {
		n = 0
	}
	kind := sa.Params["CounterType"]
	if kind == "" {
		kind = "P1P1"
	}
	// ETB$ True (the K:etbCounter expansion's body, Wishclaw Talisman and
	// every "enters with N counters" card): the counters are placed on the
	// ENTERING object as it enters, so the target does not have to be a
	// settled battlefield permanent yet. The replacement machinery runs the
	// body after the entry move in the ordinary flow, but a body reached
	// while the object is still mid-entry must place the counters anyway,
	// not skip on the battlefield precondition.
	etb := strings.EqualFold(strings.TrimSpace(sa.Params["ETB"]), "True")
	for _, t := range Defined(h, c, sa) {
		if t.IsPlayer {
			// A player target takes a PLAYER counter (energy's "you get {E}{E}{E}",
			// poison's "gets a poison counter"): the same instruction an object
			// target takes, but on the PlayerCounterChange event the engine's
			// player-counter state folds through. Skipping these (the pre-fix
			// behaviour) silently dropped the whole instruction -- the corpus
			// carries 156 player-targeted PutCounter lines.
			if p := PlayerOf(h, c, t); int(p) >= 0 && int(p) < len(h.Game().Players) {
				h.Emit(events.Event{Kind: events.PlayerCounterChange, Player: p,
					Counter: kind, Amount: n})
			}
			continue
		}
		o := h.Game().Obj(t.Obj)
		if o == nil || (o.Zone != state.ZBattlefield && !etb) {
			continue
		}
		h.Emit(events.Event{Kind: events.CounterChange, Obj: o.ID, Counter: kind, Amount: n})
	}
}

// effRemoveCounterAll sweeps ValidCards$ (default "Permanent") on the
// battlefield and removes CounterType$ counters from each match: CounterNum$
// (default 1) of them, or every counter of that kind the object actually has
// when AllCounters$ is "True". state.Object.AddCounter already clamps at
// zero, so removing more than an object has is harmless either way; the
// AllCounters$ case reads the object's own count first purely to avoid an
// event whose Amount overstates what changed.
func effRemoveCounterAll(h Host, c *Ctx, sa *cards.SA) {
	kind := sa.Params["CounterType"]
	if kind == "" {
		return
	}
	spec := sa.Params["ValidCards"]
	if spec == "" {
		spec = "Permanent"
	}
	all := sa.Params["AllCounters"] == "True"
	n := Num(h, c, sa, "CounterNum", 1)
	if n < 0 {
		n = 0
	}
	g := h.Game()
	for _, p := range g.AliveFrom(0) {
		for _, id := range g.Zone(state.ZBattlefield, p) {
			if !MatchesSpecCtx(g, spec, id, c.SpecContext(c.Controller)) {
				continue
			}
			amt := n
			if all {
				amt = g.Obj(id).Counter(kind)
			}
			if amt <= 0 {
				continue
			}
			h.Emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: kind, Amount: -amt})
		}
	}
}

// effRegenerate grants a this-turn shield consumed by ReplaceDestruction.
func effRegenerate(h Host, c *Ctx, sa *cards.SA) {
	for _, t := range Defined(h, c, sa) {
		if t.IsPlayer {
			continue
		}
		o := h.Game().Obj(t.Obj)
		if o == nil || o.Zone != state.ZBattlefield {
			continue
		}
		h.Emit(events.Event{Kind: events.CounterChange, Obj: o.ID, Counter: "Shield", Amount: 1})
	}
}
