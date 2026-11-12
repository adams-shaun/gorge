package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestGrantHostSkipsAStaleGrantorThatLostItsAbilities: two copies of a global
// grantor, the timestamp-older one in the higher slot and with its abilities
// removed. globalGrantsGen emits only the live copy's grant (CR 613.6), so the
// offered activation's host is the live copy, not the older dead one. Before
// the eligibility filter, the oldest-timestamp scan named the dead copy.
func TestGrantHostSkipsAStaleGrantorThatLostItsAbilities(t *testing.T) {
	const avatar, garruk = "Avatar of Burgeoning Echoes", "Garruk, Curse Breaker"
	set := cardsettest.Synth(t, "std-fra")
	for _, n := range []string{avatar, garruk} {
		if i, ok := set.Lookup(n); !ok || !set.Supported()[i] {
			t.Skipf("%s not in the std-fra table", n)
		}
	}
	var deck []cardset.Entry
	for _, n := range []string{avatar, garruk} {
		deck = append(deck, cardset.Entry{Name: n, Count: 1})
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	// The granted loyalty ability the Avatar's global static names.
	av := id[avatar]
	ab := uint8(255)
	for i := range tbl.Cards[av].Abilities {
		a := &tbl.Cards[av].Abilities[i]
		if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatContinuous && a.Static.Grant != 0 {
			ab = a.Static.Grant - 1
			break
		}
	}
	if ab == 255 || tbl.Cards[av].Abilities[ab].Kind != kop.AbActivated {
		t.Fatalf("precondition: %s's grant does not name an activated ability (%d)", avatar, ab)
	}

	sc := newScenTableLib(t, tbl, av, Config{NoElide: true})
	live := sc.perm(av, 0)  // slot 0, the newer copy
	stale := sc.perm(av, 0) // slot 1, the older copy
	victim := sc.perm(id[garruk], 0)
	sc.s.lstamp[sc.slot(live)] = 2
	sc.s.lstamp[sc.slot(stale)] = 1
	// Precondition: with both copies live, the timestamp order already wins
	// over slot order, so the scan under test is the one being exercised.
	if got, want := sc.s.grantHostGen(sc.k, sc.slot(victim), av, ab), sc.slot(stale); got != want {
		t.Fatalf("precondition: host %d, want the older live copy in slot %d", got, want)
	}
	sc.s.perms[sc.slot(stale)].ovr.flags |= ovLoseAll
	if got, want := sc.s.grantHostGen(sc.k, sc.slot(victim), av, ab), sc.slot(live); got != want {
		t.Fatalf("host after the older copy lost its abilities = %d, want the live copy in slot %d", got, want)
	}
}
