package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// forageManaName is the blb pool's one forage Combo Any source.
const forageManaName = "Thornvault Forager"

// TestForagePairsRefuseBesideAMixGap: Thornvault Forager is a Combo Any source, so a
// matchup whose decks hold it beside a filter, a grant or a tap-another source is
// refused at its start (StallComboAnyMix) and Init stalls the same way; the pairs
// without it play. The precondition is that both outcomes occur.
func TestForagePairsRefuseBesideAMixGap(t *testing.T) {
	set := cardsettest.Set(t, "blb")
	sup := set.Supported()
	var names []string
	forage := false
	for i := 0; i < set.NumCards(); i++ {
		if sup[i] {
			names = append(names, set.Names()[i])
			forage = forage || set.Names()[i] == forageManaName
		}
	}
	if !forage {
		t.Fatalf("%s is not a supported blb card", forageManaName)
	}
	refused, played := compareRefusalWithInit(t, set, names, 15, 200, 2)
	if refused == 0 || played == 0 {
		t.Errorf("%d refused, %d played: the pairs must fall both ways", refused, played)
	}
}

// TestForageMixStallNeedsAForageDeck: the whole blb pool table holds the Forager
// beside gap kinds the Combo Any search does not chain, but a game whose decks hold
// none of its cards never reaches the mix, so it is not stalled; a deck holding the
// Forager stalls at Init by name.
func TestForageMixStallNeedsAForageDeck(t *testing.T) {
	set := cardsettest.Set(t, "blb")
	tbl, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	id := func(name string) CardID {
		i, ok := set.Lookup(name)
		if !ok {
			t.Fatalf("%s not in the blb pool", name)
		}
		return CardID(set.PoolID(i))
	}
	forage, plain := id(forageManaName), id("Festival of Embers")
	for _, c := range []struct {
		name string
		lib  CardID
		want uint8
	}{{"no forage source in a deck", plain, StallNone}, {"a forage source in the decks", forage, StallComboAnyMix}} {
		sc := newScenTableLib(t, tbl, c.lib, Config{NoElide: true})
		k := sc.k
		if !k.caMixForage || !k.gen.caMix || !k.forageCA(forage) || k.forageCA(plain) {
			t.Fatalf("%s: the pool table is not a forage-only Combo Any mix (caMixForage %v caMix %v)", c.name, k.caMixForage, k.gen.caMix)
		}
		if got := sc.s.StallReason(); got != c.want {
			t.Errorf("%s: stall %s, want %s", c.name, StallName(got), StallName(c.want))
		}
	}
}
