package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// A player read of Targeted names the chain's target players only (CR 608.2c):
// a creature targeted by a later link adds its controller to nothing.
func TestTargetedHandOwnerIgnoresALaterLinksCreature(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := corpusEngine(t, reg,
		[]*cards.Card{lookup(t, reg, "Aggressive Negotiations"), lookup(t, reg, "Grizzly Bears"), lookup(t, reg, "Shock")},
		[]*cards.Card{lookup(t, reg, "Lightning Bolt")})
	toMain1(t, e)
	if e.G.Active != 0 {
		passUntil(t, e, func() bool { return e.G.Active == 0 && e.G.Step == state.StepMain1 })
	}
	moveByName(t, e, 0, "Aggressive Negotiations", state.ZHand)
	bears := moveByName(t, e, 0, "Grizzly Bears", state.ZBattlefield)
	shock := moveByName(t, e, 0, "Shock", state.ZHand)
	bolt := moveByName(t, e, 1, "Lightning Bolt", state.ZHand)
	addMana(t, e, 0, "CCB")
	e.pending = nil
	e.priorityRound()
	submitChoices(t, e, castByName(t, e, 0, "Aggressive Negotiations").Index)
	for e.Pending().Kind == decision.KTarget {
		d := e.Pending()
		picked := -1
		for _, o := range d.Options {
			if (o.Obj == 0 && o.Player == 1) || o.Obj == bears {
				picked = o.Index
			}
		}
		if picked < 0 {
			t.Fatalf("no wanted target in %+v", d.Options)
		}
		submitChoices(t, e, picked)
	}
	passUntilStackEmpty(t, e, 10)
	if got := e.G.Obj(bolt).Zone; got != state.ZExile {
		t.Fatalf("Lightning Bolt in zone %v, want exile", got)
	}
	if got := e.G.Obj(shock).Zone; got != state.ZHand {
		t.Fatalf("the caster's Shock moved to zone %v", got)
	}
	if got := e.G.Obj(bears).Counter("P1P1"); got != 1 {
		t.Fatalf("Grizzly Bears has %d +1/+1 counters, want 1", got)
	}
}

// Blightning names one discard of two cards, whichever target it took.
func TestBlightningDiscardsTwoAtEitherTarget(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	for _, tc := range []struct {
		name string
		at   string
	}{{"player", ""}, {"planeswalker", "Ajani Goldmane"}} {
		e := corpusEngine(t, reg, []*cards.Card{lookup(t, reg, "Blightning")}, []*cards.Card{lookup(t, reg, "Ajani Goldmane")})
		toMain1(t, e)
		if e.G.Active != 0 {
			passUntil(t, e, func() bool { return e.G.Active == 0 && e.G.Step == state.StepMain1 })
		}
		moveByName(t, e, 0, "Blightning", state.ZHand)
		var pw state.ObjID
		if tc.at != "" {
			pw = moveByName(t, e, 1, tc.at, state.ZBattlefield)
		}
		addMana(t, e, 0, "CBR")
		e.pending = nil
		e.priorityRound()
		before := len(e.G.Zone(state.ZHand, 1))
		if before < 4 {
			t.Fatalf("%s: fixture hand holds %d cards", tc.name, before)
		}
		submitChoices(t, e, castByName(t, e, 0, "Blightning").Index)
		d := e.Pending()
		picked := -1
		for _, o := range d.Options {
			if (pw == 0 && o.Obj == 0 && o.Player == 1) || (pw != 0 && o.Obj == pw) {
				picked = o.Index
			}
		}
		if picked < 0 {
			t.Fatalf("%s: target not offered: %+v", tc.name, d.Options)
		}
		submitChoices(t, e, picked)
		for i := 0; i < 20 && len(e.G.Stack) > 0; i++ {
			d := e.Pending()
			switch d.Kind {
			case decision.KPriority:
				submitChoices(t, e, passOptionIndex(t, d))
			default:
				if d.Player != 1 {
					t.Fatalf("%s: discard asked seat %d", tc.name, d.Player)
				}
				choices := []int{}
				for j := 0; j < int(d.Min); j++ {
					choices = append(choices, j)
				}
				submitChoices(t, e, choices...)
			}
		}
		if got := before - len(e.G.Zone(state.ZHand, 1)); got != 2 {
			t.Errorf("%s: discarded %d cards, want 2", tc.name, got)
		}
	}
}
