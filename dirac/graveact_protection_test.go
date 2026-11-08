package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

func graveActTdm(t *testing.T, names ...string) (*scen, map[string]CardID) {
	t.Helper()
	var deck []cardset.Entry
	set := cardsettest.Synth(t, "std-tdm")
	for _, n := range names {
		if i, ok := set.Lookup(n); !ok || !set.Supported()[i] {
			t.Skipf("%s not in the std-tdm table", n)
		}
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
	return newScenTable(t, tbl, Config{NoElide: true}), id
}

// TestRenewTargetWithProtectionNotOffered: Renew works from the graveyard
// and targets, so its source is the black card there: a creature with
// protection from black (Ureni, the Song Unending: white and black) is not a
// legal target (CR 115.2, 702.16b), and with no other creature the ability is
// not offered; an unprotected creature restores the offer.
func TestRenewTargetWithProtectionNotOffered(t *testing.T) {
	sc, id := graveActTdm(t, "Alchemist's Assistant", "Ureni, the Song Unending", "Swamp")
	sc.perm(id["Swamp"], 0)
	sc.perm(id["Swamp"], 0)
	sc.perm(id["Ureni, the Song Unending"], 1)
	renew := sc.obj(id["Alchemist's Assistant"], 0)
	sc.s.place(renew, kop.ZoneGraveyard, false)
	sc.s.ctl = ctlPriority
	if n := sc.graveActivations(renew); n != 0 {
		t.Fatalf("Renew offered %d times with only a protection-from-black creature to target, want 0", n)
	}
	sc.perm(id["Alchemist's Assistant"], 1)
	sc.s.ctl = ctlPriority
	if n := sc.graveActivations(renew); n != 1 {
		t.Fatalf("Renew offered %d times with an unprotected creature to target, want 1", n)
	}
}
