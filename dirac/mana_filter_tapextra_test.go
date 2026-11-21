package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// fpScenPerm puts each named pool card once onto seat 0's battlefield
// (untapped, not sick) and returns the slots, keyed by card name.
func fpScenPerm(t *testing.T, pool string, perms []string, lib string) (*scen, map[string]Slot) {
	t.Helper()
	set := cardsettest.Set(t, pool)
	deck := make([]cardset.Entry, 0, len(perms)+1)
	for _, n := range perms {
		deck = append(deck, cardset.Entry{Name: n, Count: 1})
	}
	deck = append(deck, cardset.Entry{Name: lib, Count: 20})
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id[lib], Config{NoElide: true})
	sl := map[string]Slot{}
	for _, n := range perms {
		sl[n] = sc.slot(sc.perm(id[n], 0))
	}
	return sc, sl
}

// TestFreeBranchPruneFailsOpenOnTapExtraCarrier: freeBranchHopeless's reach
// scan reads only printed production, the pool and other sources' mana
// abilities, never a StatTapExtra carrier's extra (mana_tapextra.go), so the
// scan must fail open on a table with one: the free tap's extra is then a
// route the branch's continuations could use, and pruning the branch would
// drop a live line (the Vault of the Archangel repro's prune, on a table with
// no carrier, keeps firing --
// dirac/oracle/mbc_vault_of_the_archangel_activation_lockstep_test.go). The
// control run is Crystal Grotto alone, where the prune does fire: the free
// {C} tap, the pool and every other untapped permanent make no {G}.
func TestFreeBranchPruneFailsOpenOnTapExtraCarrier(t *testing.T) {
	t.Parallel()
	sc, sl := fpScenPerm(t, "reduceab", []string{"Crystal Grotto"}, "Forest")
	fs := filtSearch{p: 0, own: NoSlot, usefulOK: true, need: [kop.NumManaKinds]bool{kop.G: true}}
	if !fs.freeBranchHopeless(sc.k, sc.s, sl["Crystal Grotto"]) {
		t.Fatal("precondition: with only Crystal Grotto's free {C} tap on the board, the needed {G} pip is unreachable and the free branch is pruned")
	}
	sc, sl = fpScenPerm(t, "manasecond", []string{"Baxter Building", "Groundchuck & Dirtbag"}, "Wastes")
	fs = filtSearch{p: 0, own: NoSlot, usefulOK: true, need: [kop.NumManaKinds]bool{kop.G: true}}
	if fs.freeBranchHopeless(sc.k, sc.s, sl["Baxter Building"]) {
		t.Fatal("the free branch pruned with Groundchuck & Dirtbag on the board: its StatTapExtra extra makes the free {C} tap bring the needed {G}, a route the scan does not model")
	}
}
