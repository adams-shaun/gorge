package rules

// Flickerform's end-step return (CR 603.7a/610.3): the {2}{W}{W} exile
// registers a delayed "at the beginning of the next end step" trigger whose
// body returns the exiled creature, then the Auras. The registration must
// carry the Effect's own ImprintCards$ capture -- the chain imprinted the
// enchanted creature, and the return body's ChangeType$ is
// Card.IsRemembered+IsImprinted -- so the delayed body's IsImprinted reads the
// registration's capture, not the host's live imprint, which the chain's
// DBCleanup (ClearImprinted$ True) cleared at resolution.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func TestFlickerformReturnsAtEndStep(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	flicker := lookup(t, reg, "Flickerform")
	memnite := lookup(t, reg, "Memnite")
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{flicker, memnite}, nil)
	bearer := moveByName(t, e, 0, "Memnite", state.ZBattlefield)
	aura := attachCorpusAura(t, e, 0, flicker, bearer)
	if e.G.Obj(bearer).Zone != state.ZBattlefield || e.G.Obj(aura).AttachedTo != bearer {
		t.Fatalf("precondition: Memnite zone=%s, Flickerform attached to %d, want attached to %d",
			e.G.Obj(bearer).Zone, e.G.Obj(aura).AttachedTo, bearer)
	}

	// The {2}{W}{W} activation's resolution chain: imprint the enchanted
	// creature, remember creature+Auras, exile the remembered set, register
	// the Effect's end-step trigger, clear the host. The cost payment is the
	// activation walk's work and does not affect the return.
	face := e.G.Obj(aura).Face()
	effects.Resolve(e, &effects.Ctx{Source: aura, Controller: 0, SVars: face.SVars}, face.Abilities[0])
	if z := e.G.Obj(bearer).Zone; z != state.ZExile {
		t.Fatalf("precondition: exiled Memnite zone=%s, want exile", z)
	}
	if z := e.G.Obj(aura).Zone; z != state.ZExile {
		t.Fatalf("precondition: exiled Flickerform zone=%s, want exile", z)
	}
	if got := len(e.G.Delayed); got != 1 {
		t.Fatalf("precondition: %d delayed registrations, want the end-step promise", got)
	}

	e.pending = nil
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepEnd})
	if len(e.pendingTriggers) != 1 {
		t.Fatalf("the end step queued %d triggers, want Flickerform's return", len(e.pendingTriggers))
	}
	e.putTriggersOnStack()
	e.resolveTop()
	if z := e.G.Obj(bearer).Zone; z != state.ZBattlefield {
		t.Fatalf("returned Memnite zone=%s, want the battlefield: the delayed return matched nothing", z)
	}
	if z := e.G.Obj(aura).Zone; z != state.ZBattlefield {
		t.Fatalf("returned Flickerform zone=%s, want the battlefield", z)
	}
	if at := e.G.Obj(aura).AttachedTo; at != bearer {
		t.Fatalf("Flickerform attached to %d, want the returned Memnite %d", at, bearer)
	}
	if n := len(e.G.Zone(state.ZExile, 0)); n != 0 {
		t.Fatalf("p0 exile holds %d cards after the return, want empty", n)
	}
	replayCheck(t, e, cfg)
}
