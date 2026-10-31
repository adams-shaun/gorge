package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// An either-or additional cost (Eaten Alive's "sacrifice a creature or pay
// {3}{B}") is paid however the spell is cast: flashback replaces only the
// mana cost (CR 702.34a), and a cast "without paying its mana cost" skips
// only the mana cost (CR 118.9d), so both still sacrifice the creature.
// gorge used to charge it on the plain and may-play casts only (dirac
// census pairs 2359 flashback granted by Sphinx of Forgotten Lore; 616 and
// 9954 Etali, Primal Storm's free cast).
func TestEitherOrAdditionalCostOnEveryCast(t *testing.T) {
	answer := func(e *Engine, spell, victim, fodder state.ObjID) {
		for i := 0; i < 10; i++ {
			d := e.Pending()
			if d == nil || d.Kind == decision.KPriority {
				return
			}
			pick := 0
			for _, o := range d.Options {
				switch {
				case o.Obj == spell && o.Obj != 0:
					pick = o.Index
				case d.Kind == decision.KTarget && o.Obj == victim:
					pick = o.Index
				case d.Kind != decision.KTarget && o.Obj == fodder:
					pick = o.Index
				}
			}
			submitChoices(t, e, pick)
		}
	}
	setup := func(spellSrc string) (*Engine, state.ObjID, state.ObjID, state.ObjID) {
		e := mayPlayBase(t)
		spell := graveCard(e, card(t, spellSrc), 0, 0)
		victim := onBoardGrant(t, e, 1, "Name:Victim Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
		fodder := onBoardGrant(t, e, 0, "Name:Fodder Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
		return e, spell, victim, fodder
	}
	check := func(t *testing.T, e *Engine, victim, fodder state.ObjID) {
		t.Helper()
		if z := e.G.Obj(fodder).Zone; z != state.ZGraveyard {
			t.Fatalf("the cast did not sacrifice the creature for its additional cost: Fodder Bear in %s", z)
		}
		if z := e.G.Obj(victim).Zone; z != state.ZExile {
			t.Fatalf("Victim Bear in %s, want exile", z)
		}
	}

	t.Run("flashback", func(t *testing.T) {
		e, spell, victim, fodder := setup(eatenAliveLike + "K:Flashback:B\n")
		addMana(t, e, 0, "B")
		e.priorityRound()
		idx := -1
		for _, o := range e.Pending().Options {
			if o.Kind == "cast" && o.Obj == spell && o.Mode == "flashback" {
				idx = o.Index
			}
		}
		if idx < 0 {
			t.Fatalf("flashback cast not offered: %+v", e.Pending().Options)
		}
		submitChoices(t, e, idx)
		answer(e, spell, victim, fodder)
		fdnCensusDrain(t, e)
		check(t, e, victim, fodder)
	})

	t.Run("free play", func(t *testing.T) {
		e, spell, victim, fodder := setup(eatenAliveLike)
		caster := handCard(e, card(t, "Name:Free Caster\nManaCost:B\nTypes:Sorcery\nA:SP$ Play | Valid$ Card.namedEaten Alive Probe+YouOwn | ValidZone$ Graveyard | WithoutManaCost$ True | Optional$ True\nOracle:x\n"), 0)
		addMana(t, e, 0, "B")
		e.priorityRound()
		idx := -1
		for _, o := range e.Pending().Options {
			if o.Kind == "cast" && o.Obj == caster {
				idx = o.Index
			}
		}
		if idx < 0 {
			t.Fatalf("Free Caster not offered: %+v", e.Pending().Options)
		}
		submitChoices(t, e, idx)
		answer(e, caster, victim, fodder)
		for i := 0; i < 40 && len(e.G.Stack) > 0; i++ {
			d := e.Pending()
			if d == nil {
				t.Fatal("no decision while draining")
			}
			if d.Kind == decision.KPriority {
				for _, o := range d.Options {
					if o.Kind == "pass" {
						submitChoices(t, e, o.Index)
						break
					}
				}
				continue
			}
			answer(e, spell, victim, fodder)
		}
		check(t, e, victim, fodder)
	})
}
