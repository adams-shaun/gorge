package effects

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// Phases (CR 702.25 phasing) — registered for the pieces of the mechanic this
// build can represent without a phased-out game layer, loud about the rest.
//
// This engine has NO representation for "phased out" (no flag, no zone, no
// visibility exclusion), so a phase-out cannot be modelled faithfully here and
// a phase-in has nothing to reverse. What this registration DOES do is the
// bookkeeping the corpus's chains actually read downstream:
//
//   - RememberAffected$ True (Out of Time's enters trigger, the bare
//     K:Vanishing carrier): the affected objects join the source object's
//     persistent event-backed Remembered list AND the chain ctx, so the
//     chained DB$ PutCounter's SVar X:Count$RememberedSize counts the real
//     affected set -- the card's own dynamic counter mechanism, which the
//     bare Vanishing clock then ticks.
//   - PhaseInOrOut$ True (the phase-in half): a loud no-op Note. The creatures
//     a card like Out of Time would return stay where they always were.
//
// The phase-out itself degrades to ONE loud Note per resolution: the affected
// objects stay on the battlefield, fully visible and targetable for the whole
// window a real game would hide them. That is the recorded deviation (never a
// Known-approximations row): the count a chained body reads is correct, and
// the END state of a "phase out until X leaves" chain matches a real game (the
// creatures were never hidden, and a real phase-in would put them back), but
// everything mid-flight -- combat, targeting, upkeep phase-in -- is wider than
// CR 702.25. A faithful phased-out layer is its own ticket.
func init() {
	Register("Phases", effPhases)
}

func effPhases(h Host, c *Ctx, sa *cards.SA) {
	// PhaseInOrOut$ True: the phase-in half. This build marks nothing phased
	// out, so there is nothing to reverse; say so once per resolution.
	if strings.EqualFold(strings.TrimSpace(sa.Params["PhaseInOrOut"]), "True") {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
			Text: "Phases: phase-in runs as a no-op (this build marks nothing phased out)"})
		return
	}
	g := h.Game()
	// The affected set. AllValid$ <spec> sweeps every battlefield object the
	// spec admits (the whole battlefield across all controllers -- Out of
	// Time untaps and phases out "all creatures"); anything else goes through
	// the shared Defined$ resolver (Defined$ Self, Defined$ Remembered,
	// Defined$ Valid <filter>, the pre-asked ValidTgts$ set). A form that
	// resolves to nothing is a no-op, silently.
	var affected []state.Target
	if spec := strings.TrimSpace(sa.Params["AllValid"]); spec != "" {
		for i := range g.Objs {
			o := &g.Objs[i]
			if o.Zone == state.ZBattlefield && MatchesObjectCtx(g, spec, o, c.SpecContext(c.Controller)) {
				affected = append(affected, state.Target{Obj: o.ID})
			}
		}
	} else {
		affected = Defined(h, c, sa)
	}
	// RememberAffected$ True: the affected objects join the source object's
	// persistent event-backed Remembered list and the chain ctx, so a chained
	// body's Count$RememberedSize (and Card.IsRemembered) reads the real
	// affected count. Only battlefield objects are remembered -- a phasing
	// effect can only ever have affected permanents.
	if strings.EqualFold(strings.TrimSpace(sa.Params["RememberAffected"]), "True") {
		for _, t := range affected {
			if t.IsPlayer {
				continue
			}
			if o := g.Obj(t.Obj); o == nil || o.Zone != state.ZBattlefield {
				continue
			}
			c.Remembered = append(c.Remembered, t)
			eventRemember(h, c, t.Obj)
		}
	}
	if len(affected) == 0 {
		return
	}
	text := "Phases: phase-out not represented (" + strconv.Itoa(len(affected)) +
		" affected object(s) stay on the battlefield)"
	if wont := strings.TrimSpace(sa.Params["WontPhaseInNormal"]); wont != "" {
		text += "; WontPhaseInNormal$ " + wont + " unrepresented"
	}
	h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Text: text})
}
