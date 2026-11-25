package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// dfcCounterEngine deals a corpus-only game with the transforming card and a
// Counterspell in seat 0's deck, then moves each to its starting zone.
func dfcCounterEngine(t *testing.T, name string, from state.Zone) (*Engine, Config, state.ObjID, state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	e, cfg := proliferateEngine(t, reg, name, "Counterspell")
	id := putNamed(t, e, 0, name, from)
	counter := putNamed(t, e, 0, "Counterspell", state.ZHand)
	return e, cfg, id, counter
}

// counterSpell counters the spell id and drains the stack.
func counterSpell(t *testing.T, e *Engine, counter, id state.ObjID) {
	t.Helper()
	submitChoices(t, e, castOptionFor(t, e, counter).Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("counter target ask: %+v", d)
	}
	tgt := -1
	for _, o := range d.Options {
		if o.Obj == id {
			tgt = o.Index
		}
	}
	if tgt < 0 {
		t.Fatalf("no option targeting the spell: %+v", d.Options)
	}
	submitChoices(t, e, tgt)
	passUntilStackEmpty(t, e, 40)
}

// CR 712.8a: a transforming double-faced card not on the battlefield or the
// stack has only its front face's characteristics. Both back-face cast routes
// are covered: a countered Disturb cast leaves the stack and its Disturb
// replacement sends it to exile (CR 702.146b), while a countered More Than
// Meets the Eye converted cast (CR 702.162a) goes to the graveyard. Either way
// the card there is its front face again.
func TestDFCLeavesStackFrontFace(t *testing.T) {
	t.Parallel()
	t.Run("disturb into exile", func(t *testing.T) {
		e, cfg, id, counter := dfcCounterEngine(t, "Baithook Angler", state.ZGraveyard)
		addMana(t, e, 0, "UU")
		opt := disturbOption(t, e, id)
		if opt == nil {
			t.Fatalf("precondition: disturb cast not offered: %+v", castOptions(t, e))
		}
		submitChoices(t, e, opt.Index)
		o := e.G.Obj(id)
		if o.Zone != state.ZStack || o.Face().Name != "Hook-Haunt Drifter" {
			t.Fatalf("precondition: zone %s face %q, want the back face on the stack", o.Zone, o.Face().Name)
		}
		addMana(t, e, 0, "UU")
		counterSpell(t, e, counter, id)
		o = e.G.Obj(id)
		if o.Zone != state.ZExile {
			t.Fatalf("countered Disturb spell zone=%s, want ZExile", o.Zone)
		}
		if got := o.Face().Name; got != "Baithook Angler" {
			t.Fatalf("countered Disturb card in exile shows %q, want its front face Baithook Angler", got)
		}
		replayCheck(t, e, cfg)
	})
	t.Run("converted into the graveyard", func(t *testing.T) {
		e, cfg, id, counter := dfcCounterEngine(t, optimusFront, state.ZHand)
		addMana(t, e, 0, "UURWG")
		opt := convertedOption(t, e, id)
		if opt == nil {
			t.Fatalf("precondition: converted cast not offered: %+v", mtmteOpts(castOptions(t, e)))
		}
		submitChoices(t, e, opt.Index)
		o := e.G.Obj(id)
		if o.Zone != state.ZStack || o.Face().Name != optimusBack {
			t.Fatalf("precondition: zone %s face %q, want the back face on the stack", o.Zone, o.Face().Name)
		}
		addMana(t, e, 0, "UU")
		counterSpell(t, e, counter, id)
		o = e.G.Obj(id)
		if o.Zone != state.ZGraveyard {
			t.Fatalf("countered converted spell zone=%s, want ZGraveyard", o.Zone)
		}
		if got := o.Face().Name; got != optimusFront {
			t.Fatalf("countered converted card in the graveyard shows %q, want its front face %s", got, optimusFront)
		}
		replayCheck(t, e, cfg)
	})
}
