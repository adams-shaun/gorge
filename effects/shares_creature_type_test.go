package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/state"
)

// The sharesCreatureTypeWith leaf (Heirloom Blade's
// "Creature.sharesCreatureTypeWith TriggeredCardLKICopy"): the token joins
// the sharesCardTypeWith family's referent switch, the intersection is over
// CREATURE subtypes (not card types), and UnknownPredicates goes through the
// SAME classifier as the matcher — done-criterion 1, so the census cannot
// disagree with the matcher.

// sharesCreatureTypeFixture builds a game with the dead trigger card (a
// Bear), a matching Bear Cub and a non-matching Hill Giant in seat 0's
// library. Returns (game, dead, cub, giant).
func sharesCreatureTypeFixture(t *testing.T) (*state.Game, state.ObjID, state.ObjID, state.ObjID) {
	t.Helper()
	g := state.NewGame(names(2))
	dead := g.AddObject(mkCard(t, "Name:Bear\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"), 0)
	cub := g.AddObject(mkCard(t, "Name:Bear Cub\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"), 0)
	giant := g.AddObject(mkCard(t, "Name:Hill Giant\nTypes:Creature Giant\nPT:3/3\nOracle:x\n"), 0)
	g.SetZone(state.ZGraveyard, 0, []state.ObjID{dead.ID})
	g.SetZone(state.ZLibrary, 0, []state.ObjID{cub.ID, giant.ID})
	return g, dead.ID, cub.ID, giant.ID
}

// TestSharesCreatureTypeWithClassified keeps the seven recognised referents
// silent to UnknownPredicates and everything else fail-closed-loud.
func TestSharesCreatureTypeWithClassified(t *testing.T) {
	for _, ref := range []string{"RememberedCard", "Remembered", "RememberedLKI",
		"TriggeredCard", "TriggeredCardLKICopy", "Targeted", "Self"} {
		spec := "Creature.sharesCreatureTypeWith " + ref
		if got := UnknownPredicates(spec); len(got) != 0 {
			t.Errorf("UnknownPredicates(%q) = %v, want []", spec, got)
		}
	}
	// The exotic referents the brief scopes OUT stay unknown (fail closed).
	for _, spec := range []string{
		"Creature.sharesCreatureTypeWith Sacrificed",
		"Creature.sharesCreatureTypeWith TriggeredAttacker",
		"Creature.sharesCreatureTypeWithAll Tapped",
	} {
		if got := UnknownPredicates(spec); len(got) == 0 {
			t.Errorf("UnknownPredicates(%q) = [], want the token reported", spec)
		}
	}
	// The card-type reading keeps its own classification.
	if got := UnknownPredicates("Creature.sharesCardTypeWith RememberedCard"); len(got) != 0 {
		t.Errorf("UnknownPredicates card-type = %v, want []", got)
	}
}

// TestSharesCreatureTypeWithMatchesTheTriggerCard is the matcher half: a Bear
// candidate shares the dead Bear's subtype; a Giant does not; an unbound
// referent matches nothing (fail closed, never widened).
func TestSharesCreatureTypeWithMatchesTheTriggerCard(t *testing.T) {
	g, dead, cub, giant := sharesCreatureTypeFixture(t)
	sc := SpecContext{You: 0}
	sc.TriggerCard = dead
	if !MatchesSpecCtx(g, "Creature.sharesCreatureTypeWith TriggeredCardLKICopy", cub, sc) {
		t.Errorf("Bear Cub must share the dead Bear's creature type")
	}
	if MatchesSpecCtx(g, "Creature.sharesCreatureTypeWith TriggeredCardLKICopy", giant, sc) {
		t.Errorf("Hill Giant must not share the dead Bear's creature type")
	}
	scUnbound := SpecContext{You: 0}
	if MatchesSpecCtx(g, "Creature.sharesCreatureTypeWith TriggeredCardLKICopy", cub, scUnbound) {
		t.Errorf("an unbound referent must match nothing (fail closed)")
	}
	// The card-type reading is unchanged by the creature twin: both Bear Cub
	// and Hill Giant are Creatures, so the CARD-type spec matches both.
	if !MatchesSpecCtx(g, "Creature.sharesCardTypeWith TriggeredCardLKICopy", giant, sc) {
		t.Errorf("Hill Giant shares the CARD type Creature with the dead Bear")
	}
}
