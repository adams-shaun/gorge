package dirac

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// A restricted "{T}, Sacrifice another <filter>: Add <one fixed colour>" mana
// source (Master of Dark Rites' three {B}, Vampire, Cleric and Demon spells
// only): gorge's census defers it (pay.PaymentPlanShapeTierOf: a Sac part that
// is not the source), and the tap-another search funds the play it pays —
// the activation sacrifices one other creature in a scripted prefix and the
// mana floats restricted, spent exactly by the payments its restriction admits
// (CR 106.6, 602.2b; dirac mana_tapanother.go sacXDo's restricted arm).

// restrSacXDeck is the scenario's deck.
func restrSacXDeck() []cardset.Entry {
	return []cardset.Entry{
		{Name: "Master of Dark Rites", Count: 1}, {Name: "Vampire Nighthawk", Count: 1},
		{Name: "Grizzly Bears", Count: 1}, {Name: "Mountain", Count: 1}, {Name: "Swamp", Count: 1},
	}
}

// restrSacXTable is the restrictedmana pool's table over restrSacXDeck, with
// Master of Dark Rites' restriction replaced by f when patch is set (the
// scenario's own refusal filter).
func restrSacXTable(t *testing.T, patch bool, f kop.Filter) (*kop.Table, map[string]CardID) {
	t.Helper()
	set := cardsettest.Set(t, "restrictedmana")
	tbl, da, _, err := set.Matchup(restrSacXDeck(), restrSacXDeck())
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	if !patch {
		return tbl, id
	}
	tb := *tbl
	tb.Filters = slices.Clone(tbl.Filters)
	tb.Cards = slices.Clone(tbl.Cards)
	tb.Cards[id["Master of Dark Rites"]].Abilities = slices.Clone(tbl.Cards[id["Master of Dark Rites"]].Abilities)
	found := false
	for j := range tb.Cards[id["Master of Dark Rites"]].Abilities {
		if a := &tb.Cards[id["Master of Dark Rites"]].Abilities[j]; a.Kind == kop.AbMana && a.ManaRestrict != 0 {
			f.Or = 0
			tb.Filters[a.ManaRestrict] = f
			found = true
		}
	}
	if !found {
		t.Fatal("Master of Dark Rites has no restricted mana ability in the table")
	}
	return &tb, id
}

// TestRestrictedSacXGapPaysAndWithholds: the tap-another search activates
// Master of Dark Rites (sacrificing one other creature) to fund a Vampire
// spell, the floats spent and none left; a spell the restriction refuses stays
// withheld, and beside sources that pay the cast plainly the Master is
// untouched.
func TestRestrictedSacXGapPaysAndWithholds(t *testing.T) {
	// (a) the floats alone pay Vampire Nighthawk ({2}{B}{B}) beside a
	// Mountain: the Bears are sacrificed, the floats spent, the pool empty.
	tbl, id := restrSacXTable(t, false, kop.Filter{})
	sc := newScenTableLib(t, tbl, id["Mountain"], Config{NoElide: true})
	master := sc.perm(id["Master of Dark Rites"], 0)
	bears := sc.perm(id["Grizzly Bears"], 0)
	sc.perm(id["Mountain"], 0)
	nighthawk := sc.hand(id["Vampire Nighthawk"], 0)
	if sc.s.objs[master].zone != kop.ZoneBattlefield {
		t.Fatal("(a) precondition: the Master is not on the battlefield")
	}
	if !restrSacCast(sc, nighthawk) {
		t.Fatal("(a) the Nighthawk withheld with only the Master's gap: want it offered")
	}
	if z := sc.s.objs[bears].zone; z != kop.ZoneGraveyard {
		t.Fatalf("(a) the Bears are in zone %d, want the graveyard (sacrificed)", z)
	}
	if sc.s.perms[sc.s.objs[master].slot].flags&pfTapped == 0 {
		t.Fatal("(a) the Master is untapped after its activation")
	}
	if sc.s.restrN(0) != 0 || sc.s.pl[0].pool != ([kop.NumManaKinds]uint8{}) {
		t.Fatalf("(a) after the payment: restricted %v pool %v, want both spent", sc.s.rb[0], sc.s.pl[0].pool)
	}

	// (b) the restriction refuses creature spells: withheld, the Master
	// untouched (untapped, the Bears alive).
	tblB, idb := restrSacXTable(t, true, kop.Filter{Types: kop.TArtifact})
	scB := newScenTableLib(t, tblB, idb["Mountain"], Config{NoElide: true})
	masterB := scB.perm(idb["Master of Dark Rites"], 0)
	bearsB := scB.perm(idb["Grizzly Bears"], 0)
	scB.perm(idb["Mountain"], 0)
	nighthawkB := scB.hand(idb["Vampire Nighthawk"], 0)
	if restrSacCast(scB, nighthawkB) {
		t.Fatal("(b) the Nighthawk offered on an artifact-only gap: want it withheld")
	}
	if scB.s.perms[scB.s.objs[masterB].slot].flags&pfTapped != 0 || scB.s.objs[bearsB].zone != kop.ZoneBattlefield {
		t.Fatal("(b) the Master activated without paying")
	}

	// (c) beside three Swamps and a Mountain the Nighthawk pays plainly: the
	// Master is not activated, no mana floats.
	scC := newScenTableLib(t, tbl, id["Mountain"], Config{NoElide: true})
	masterC := scC.perm(id["Master of Dark Rites"], 0)
	scC.perm(id["Grizzly Bears"], 0)
	scC.perm(id["Mountain"], 0)
	scC.perm(id["Swamp"], 0)
	scC.perm(id["Swamp"], 0)
	scC.perm(id["Swamp"], 0)
	nighthawkC := scC.hand(id["Vampire Nighthawk"], 0)
	if !restrSacCast(scC, nighthawkC) {
		t.Fatal("(c) the Nighthawk withheld beside three Swamps and a Mountain: want it offered")
	}
	if scC.s.perms[scC.s.objs[masterC].slot].flags&pfTapped != 0 || scC.s.restrN(0) != 0 {
		t.Fatalf("(c) the Master was activated (tapped) or floated restricted mana %v beside the Swamps; the printed sources pay", scC.s.rb[0])
	}
}
