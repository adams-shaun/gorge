package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// afScen builds the divergence's mana board: Shambling Vent's {1}{W}{B}
// animation (a land's ability, so Blossoming Tortoise's "activated abilities
// of lands cost {1} less" reduces it to {W}{B}), Crystal Grotto (a filter
// whose fee the reduction takes to nothing, so its any-colour line is the
// source's own mana) and Rootrider Faun ({1}, {T}: one mana of any colour, a
// paid filter whose fee a Mountain pays), beside two untapped Mountains and
// the tapped lands the game had spent. Scarecrow Guide is in the deck but not
// on the battlefield: its first-ability filter is what makes the table's
// anyFiltFirst branch live, the shape the divergence was found on.
func afScen(t *testing.T, grottoTapped bool) (*scen, Obj) {
	t.Helper()
	deck := []cardset.Entry{
		{Name: "Shambling Vent", Count: 1},
		{Name: "Blossoming Tortoise", Count: 1},
		{Name: "Crystal Grotto", Count: 1},
		{Name: "Rootrider Faun", Count: 1},
		{Name: "Scarecrow Guide", Count: 1},
		{Name: "Mountain", Count: 4},
		{Name: "Forest", Count: 6},
	}
	set := cardsettest.Set(t, "reduceab")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id["Forest"], Config{NoElide: true, GorgeAutopaySurface: true})
	vent := sc.perm(id["Shambling Vent"], 0)
	sc.perm(id["Blossoming Tortoise"], 0)
	grotto := sc.perm(id["Crystal Grotto"], 0)
	sc.perm(id["Rootrider Faun"], 0)
	sc.perm(id["Mountain"], 0)
	sc.perm(id["Mountain"], 0)
	for _, o := range []Obj{
		sc.perm(id["Mountain"], 0),
		sc.perm(id["Mountain"], 0),
		sc.perm(id["Forest"], 0),
		sc.perm(id["Forest"], 0),
		sc.perm(id["Forest"], 0),
		sc.perm(id["Forest"], 0),
	} {
		sc.s.perms[sc.slot(o)].flags |= pfTapped
	}
	if grottoTapped {
		sc.s.perms[sc.slot(grotto)].flags |= pfTapped
	}
	return sc, vent
}

// afOffered steps to the priority decision and reports whether the Vent's
// animation is offered.
func afOffered(t *testing.T, sc *scen, vent Obj) bool {
	t.Helper()
	d := sc.s.Step()
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkActivate && Obj(a.Source()) == vent {
			return true
		}
	}
	return false
}

// TestActivatedAbilityPaidFilterSecondColour (CR 601.2g, 602.2b): an
// activated ability whose two coloured pips need a free filter (Crystal
// Grotto) and a paid one (Rootrider Faun's {1}, {T}) is offered; with the
// Grotto tapped the paid filter alone can make only one of the pips, so the
// {W}{B} animation is not.
func TestActivatedAbilityPaidFilterSecondColour(t *testing.T) {
	sc, vent := afScen(t, false)
	if !afOffered(t, sc, vent) {
		t.Fatal("Shambling Vent's {W}{B} animation not offered: the Grotto and the Faun's paid filter pay both coloured pips")
	}

	sc, vent = afScen(t, true)
	if afOffered(t, sc, vent) {
		t.Fatal("Shambling Vent's animation offered with the Grotto tapped: one Faun activation cannot make both {W} and {B}")
	}
}
