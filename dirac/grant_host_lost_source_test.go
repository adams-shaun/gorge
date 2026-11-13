package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestGrantHostSkipsAStaleGrantorThatLostItsAbilities: two copies of a global
// grantor, the timestamp-older one in the higher slot and with its abilities
// removed. globalGrantsGen emits only the live copy's grant (CR 613.6), so the
// offered activation's host is the live copy, not the older dead one. Before
// the eligibility filter, the oldest-timestamp scan named the dead copy.
func TestGrantHostSkipsAStaleGrantorThatLostItsAbilities(t *testing.T) {
	sc, c := empScen(t)
	set := cardsettest.Set(t, "empower")
	avatar := set.PoolID(empPoolIndex(t, "Avatar of Burgeoning Echoes"))
	// The granted loyalty ability the Avatar's global static names.
	ab := uint8(255)
	for i := range sc.k.t.Cards[avatar].Abilities {
		a := &sc.k.t.Cards[avatar].Abilities[i]
		if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatContinuous && a.Static.Grant != 0 {
			ab = a.Static.Grant - 1
			break
		}
	}
	if ab == 255 || sc.k.t.Cards[avatar].Abilities[ab].Kind != kop.AbActivated {
		t.Fatalf("precondition: the Avatar's global static does not name an activated ability (%d)", ab)
	}

	live := sc.perm(avatar, 0) // slot 0, the newer copy
	stale := sc.perm(avatar, 0)
	victim := sc.perm(c.jace, 0) // the planeswalker both copies grant
	sc.s.lstamp[sc.slot(live)] = 2
	sc.s.lstamp[sc.slot(stale)] = 1
	// Precondition: with both copies live, timestamp order already wins over
	// slot order, so the scan under test is the one being exercised.
	if got, want := sc.s.grantHostGen(sc.k, sc.slot(victim), avatar, ab), sc.slot(stale); got != want {
		t.Fatalf("precondition: host %d, want the older live copy in slot %d", got, want)
	}
	sc.s.perms[sc.slot(stale)].ovr.flags |= ovLoseAll
	if got, want := sc.s.grantHostGen(sc.k, sc.slot(victim), avatar, ab), sc.slot(live); got != want {
		t.Fatalf("host after the older copy lost its abilities = %d, want the live copy in slot %d", got, want)
	}
}
