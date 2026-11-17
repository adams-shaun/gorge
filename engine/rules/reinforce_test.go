package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// CR 702.77a: Reinforce N—{cost} is "{cost}, Discard this card: Put N +1/+1
// counters on target creature", activated only from hand.
func TestReinforcePutsCountersFromHand(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := proliferateEngine(t, reg, "Bannerhide Krushok")
	bear := putNamedOnBattlefield(t, e, "Grizzly Bears")
	krushok := moveToHandByName(t, e, 0, "Bannerhide Krushok")
	if got := e.G.Obj(bear).Counter("P1P1"); got != 0 {
		t.Fatalf("precondition: bear P1P1 = %d, want 0", got)
	}
	addMana(t, e, 0, "1G")
	var opt decision.Option
	for _, o := range e.Pending().Options {
		if o.Kind == "ability" && o.Obj == krushok {
			opt = o
		}
	}
	if opt.Kind != "ability" {
		t.Fatalf("no Reinforce activation offered from hand: %+v", e.Pending().Options)
	}
	submitChoices(t, e, opt.Index)
	// Answer the target prompt with the bear, whatever else is pending.
	for range 6 {
		d := e.Pending()
		if d == nil || d.Kind == decision.KPriority {
			break
		}
		idx := 0
		for _, o := range d.Options {
			if o.Obj == bear {
				idx = o.Index
			}
		}
		submitChoices(t, e, idx)
	}
	if z := e.G.Obj(krushok).Zone; z != state.ZGraveyard {
		t.Fatalf("Krushok zone = %s, want graveyard (discarded as the cost)", z)
	}
	passUntilStackEmpty(t, e, 60)
	if got := e.G.Obj(bear).Counter("P1P1"); got != 2 {
		t.Fatalf("bear P1P1 = %d, want 2 (Reinforce 2)", got)
	}
	replayCheck(t, e, cfg)
}

func TestReinforceCardsAreSupported(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	for _, name := range []string{"Bannerhide Krushok", "Break Ties", "Earthbrawn"} {
		c, ok := reg.Lookup(name)
		if !ok {
			t.Fatalf("%s: not in the corpus", name)
		}
		if miss := reg.Unsupported(c, effects.Supported()); len(miss) != 0 {
			t.Errorf("%s unsupported: %v", name, miss)
		}
	}
}
