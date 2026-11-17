package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Reinforce X—{X}{W}{W} (Swell of Courage): the announced X sizes the counters.
func TestReinforceXCountsXPaid(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	for _, name := range []string{"Swell of Courage", "Wren's Run Hydra"} {
		c, ok := reg.Lookup(name)
		if !ok {
			t.Fatalf("%s: not in the corpus", name)
		}
		if miss := reg.Unsupported(c, effects.Supported()); len(miss) != 0 {
			t.Errorf("%s unsupported: %v", name, miss)
		}
	}
	e, cfg := proliferateEngine(t, reg, "Swell of Courage")
	bear := putNamedOnBattlefield(t, e, "Grizzly Bears")
	sw := moveToHandByName(t, e, 0, "Swell of Courage")
	addMana(t, e, 0, "WWWW")
	offered := false
	for _, o := range e.Pending().Options {
		if o.Kind == "ability" && o.Obj == sw {
			offered = true
			submitChoices(t, e, o.Index)
			break
		}
	}
	if !offered {
		t.Fatalf("no Reinforce activation offered from hand: %+v", e.Pending().Options)
	}
	askedX := false
	for range 6 {
		d := e.Pending()
		if d == nil || d.Kind == decision.KPriority {
			break
		}
		idx := 0
		switch d.Kind {
		case decision.KChoose:
			askedX = true
			idx = len(d.Options) - 1 // the largest X the four W can pay
		default:
			for _, o := range d.Options {
				if o.Obj == bear {
					idx = o.Index
				}
			}
		}
		submitChoices(t, e, idx)
	}
	if !askedX {
		t.Fatal("precondition: no X announcement was asked")
	}
	passUntilStackEmpty(t, e, 60)
	if z := e.G.Obj(sw).Zone; z != state.ZGraveyard {
		t.Fatalf("Swell of Courage zone = %s, want graveyard", z)
	}
	if got := e.G.Obj(bear).Counter("P1P1"); got != 2 {
		t.Fatalf("bear P1P1 = %d, want 2 (X=2)", got)
	}
	replayCheck(t, e, cfg)
}
