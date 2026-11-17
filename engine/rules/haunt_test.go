package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// hauntDrain answers every pending decision until the stack is empty and
// priority is back: a KTarget ask picks the option pick accepts (and fails the
// test when none does); anything else passes / takes option 0.
func hauntDrain(t *testing.T, e *Engine, pick func(decision.Option) bool) {
	t.Helper()
	for i := 0; i < 80 && !e.G.Over; i++ {
		d := e.Pending()
		if d == nil {
			return
		}
		if d.Kind == decision.KPriority {
			if len(e.G.Stack) == 0 {
				return
			}
			passUntilNonPriority2(t, e, d)
			continue
		}
		choice := 0
		if d.Kind == decision.KTarget {
			choice = -1
			for _, o := range d.Options {
				if pick(o) {
					choice = o.Index
				}
			}
			if choice < 0 {
				t.Fatalf("target decision %q (src %d) has no acceptable option: %d options, first %+v", d.Prompt, d.Source, len(d.Options), d.Options[0].Label)
			}
		}
		submitChoices(t, e, choice)
	}
}

func hauntObj(id state.ObjID) func(decision.Option) bool {
	return func(o decision.Option) bool { return o.Obj == id }
}

func hauntPlayer(p state.PlayerID) func(decision.Option) bool {
	return func(o decision.Option) bool { return o.Obj == 0 && o.Player == p }
}

// passUntilNonPriority2 passes priority once on the pending decision d.
func passUntilNonPriority2(t *testing.T, e *Engine, d *decision.Decision) {
	t.Helper()
	for _, o := range d.Options {
		if o.Kind == "pass" {
			submitChoices(t, e, o.Index)
			return
		}
	}
	t.Fatalf("priority decision with no pass option: %+v", d)
}

func hauntKill(t *testing.T, e *Engine, id state.ObjID) {
	t.Helper()
	e.emit(events.Event{Kind: events.Damage, Obj: id, Amount: 99})
	e.checkStateBased()
	e.priorityRound()
}

// CR 702.55a-c: Blind Hunter dies, is exiled haunting a creature, and when that
// creature dies the exiled card's ability triggers from exile.
func TestHauntExilesAndTriggersOnTheHauntedCreature(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := proliferateEngine(t, reg, "Blind Hunter", "Grizzly Bears")
	hunter := putNamedOnBattlefield(t, e, "Blind Hunter")
	bears := putNamedOnBattlefield(t, e, "Grizzly Bears")
	// Settle Blind Hunter's own "enters" trigger (target the opponent) before
	// measuring life.
	e.priorityRound()
	hauntDrain(t, e, hauntPlayer(1))
	hauntKill(t, e, hunter)
	hauntDrain(t, e, hauntObj(bears))
	ho := e.G.Obj(hunter)
	if ho == nil || ho.Zone != state.ZExile {
		t.Fatalf("Blind Hunter = %+v, want it exiled", ho)
	}
	if ho.Haunting != bears {
		t.Fatalf("Blind Hunter haunts %d, want the Grizzly Bears %d", ho.Haunting, bears)
	}
	mine, theirs := e.G.Players[0].Life, e.G.Players[1].Life
	hauntKill(t, e, bears)
	hauntDrain(t, e, hauntPlayer(1))
	if got := e.G.Players[1].Life; got != theirs-2 {
		t.Errorf("opponent life = %d, want %d (lost 2)", got, theirs-2)
	}
	if got := e.G.Players[0].Life; got != mine+2 {
		t.Errorf("controller life = %d, want %d (gained 2)", got, mine+2)
	}
	replayCheck(t, e, cfg)
}

// kw:Haunt is expanded by cards/kw_haunt.go; the corpus reader must also see
// it as supported.
func TestHauntCardsAreSupported(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	for _, name := range []string{
		"Absolver Thrull", "Belfry Spirit", "Benediction of Moons", "Blind Hunter", "Cry of Contrition",
		"Exhumer Thrull", "Graven Dominator", "Orzhov Euthanist", "Orzhov Pontiff", "Seize the Soul",
	} {
		c, ok := reg.Lookup(name)
		if !ok {
			t.Fatalf("%s: not in the corpus", name)
		}
		if miss := reg.Unsupported(c, effects.Supported()); len(miss) != 0 {
			t.Errorf("%s unsupported: %v", name, miss)
		}
	}
}

// CR 702.55b: a Haunt sorcery that resolves is exiled haunting a creature, and
// when that creature dies its effect happens from exile.
func TestHauntSpellExilesAfterResolvingAndTriggers(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := proliferateEngine(t, reg, "Cry of Contrition", "Grizzly Bears")
	bears := putNamedOnBattlefield(t, e, "Grizzly Bears")
	addMana(t, e, 0, "B")
	before := len(e.G.Zone(state.ZHand, 1))
	cry := castFixtureNamed(t, e, "Cry of Contrition")
	// The spell's own effect (target player discards) comes first, then the
	// haunt trigger's creature target. Both asks are answered here.
	hauntDrain(t, e, func(o decision.Option) bool { return hauntObj(bears)(o) || hauntPlayer(1)(o) })
	if got := len(e.G.Zone(state.ZHand, 1)); got != before-1 {
		t.Fatalf("opponent hand = %d after Cry of Contrition resolved, want %d (the spell's own discard)", got, before-1)
	}
	co := e.G.Obj(cry)
	if co == nil || co.Zone != state.ZExile || co.Haunting != bears {
		t.Fatalf("Cry of Contrition = %+v, want it exiled haunting the Grizzly Bears %d", co, bears)
	}
	hand := len(e.G.Zone(state.ZHand, 1))
	hauntKill(t, e, bears)
	hauntDrain(t, e, hauntPlayer(1))
	if got := len(e.G.Zone(state.ZHand, 1)); got != hand-1 {
		t.Errorf("opponent hand = %d, want %d (discarded one when the haunted creature died)", got, hand-1)
	}
	replayCheck(t, e, cfg)
}
