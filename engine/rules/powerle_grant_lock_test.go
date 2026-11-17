package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// castOnTarget casts a one-target spell from seat 0's hand through the real
// cast flow, aiming its target ask at tgt, and resolves it.
func castOnTarget(t *testing.T, e *Engine, name, mana string, tgt state.ObjID) {
	t.Helper()
	addMana(t, e, 0, mana)
	castSpellOption(t, e, name)
	for i := 0; i < 30; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatalf("no decision while casting %s", name)
		}
		if d.Kind == decision.KTarget {
			idx := indexOfObjOption(d, tgt)
			if idx < 0 {
				t.Fatalf("%s: no target option for %d: %+v", name, tgt, d.Options)
			}
			submitChoices(t, e, idx)
			continue
		}
		if d.Kind == decision.KPriority {
			break
		}
		submitChoices(t, e, d.Options[0].Index)
	}
	resolveStack(t, e)
}

// A resolution-generated continuous effect's affected set is determined when
// the effect begins and never changes afterwards (CR 611.2c: "this works
// differently than a continuous effect from a static ability"). Temur Charm
// mode 3 ("Creatures with power 3 or less can't block this turn") resolves
// with a Giant-Growthed Grizzly Bears at power 5: it is not in the set, so a
// Disfigure that then shrinks it to power 3 must not make it unable to block.
// A second Grizzly Bears that was power 2 at resolution is in the set.
func TestPowerleGrantLocksItsSetAtBegin(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := corpusEngineCfg(t, reg,
		[]*cards.Card{lookup(t, reg, "Grizzly Bears"), lookup(t, reg, "Runeclaw Bear"), lookup(t, reg, "Giant Growth"), lookup(t, reg, "Disfigure"), lookup(t, reg, "Temur Charm")},
		nil)
	big := moveCorpusCard(t, e, "Grizzly Bears", 0, state.ZBattlefield)
	small := moveCorpusCard(t, e, "Runeclaw Bear", 0, state.ZBattlefield)
	moveCorpusCard(t, e, "Giant Growth", 0, state.ZHand)
	moveCorpusCard(t, e, "Disfigure", 0, state.ZHand)
	moveCorpusCard(t, e, "Temur Charm", 0, state.ZHand)
	if e.G.Active != 0 {
		driveToStep(t, e, e.G.Turn+1, 0, state.StepMain1)
	}

	castOnTarget(t, e, "Giant Growth", "G", big)
	if p := e.Derived(big).Power; p != 5 {
		t.Fatalf("precondition: pumped Bears power = %d, want 5", p)
	}

	addMana(t, e, 0, "GUR")
	castSpellOption(t, e, "Temur Charm")
	answered := false
	for i := 0; i < 40; i++ {
		d := e.Pending()
		if d == nil {
			break
		}
		if d.Kind == decision.KPriority {
			break
		}
		if d.Kind == decision.KModes {
			pick := -1
			for _, o := range d.Options {
				if strings.Contains(o.Label, "can't block") {
					pick = o.Index
				}
			}
			if pick < 0 {
				t.Fatalf("no can't-block mode option: %+v", d.Options)
			}
			submitChoices(t, e, pick)
			answered = true
			continue
		}
		submitChoices(t, e, d.Options[0].Index)
	}
	if !answered {
		t.Fatal("the charm's mode ask never appeared")
	}
	resolveStack(t, e)
	if !derivedKWHolds(e, small, "can't block") {
		t.Fatalf("precondition: the power-2 Bears was not granted can't-block: %v", e.Keywords(small))
	}
	if derivedKWHolds(e, big, "can't block") {
		t.Fatalf("precondition: the power-5 Bears was granted can't-block: %v", e.Keywords(big))
	}

	castOnTarget(t, e, "Disfigure", "B", big)
	if p := e.Derived(big).Power; p != 3 {
		t.Fatalf("precondition: Disfigured Bears power = %d, want 3 (inside the gate)", p)
	}
	if derivedKWHolds(e, big, "can't block") {
		t.Errorf("a creature that was power 5 when the charm resolved gained can't-block after shrinking to power 3: %v", e.Keywords(big))
	}
	if !derivedKWHolds(e, small, "can't block") {
		t.Errorf("the power-2 Bears lost its can't-block: %v", e.Keywords(small))
	}
	replayCheck(t, e, cfg)
}
