package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// newCasualtyBatchScen is a scenario over the czallrem pool (Blood Spatter Analysis,
// Raging Goblin, Day of Judgment, Plains).
func newCasualtyBatchScen(t *testing.T) (*scen, map[string]CardID) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Blood Spatter Analysis", Count: 1}, {Name: "Raging Goblin", Count: 4}, {Name: "Day of Judgment", Count: 1}, {Name: "Plains", Count: 8}}
	kczallremSet := cardsettest.Set(t, "czallrem")
	for _, e := range deck {
		if i, ok := kczallremSet.Lookup(e.Name); !ok || !kczallremSet.Supported()[i] {
			t.Skipf("%s not supported in the czallrem table", e.Name)
		}
	}
	tbl, da, _, err := kczallremSet.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	var lib [20]CardID
	for i := range lib {
		lib[i] = id["Plains"]
	}
	s := new(State)
	s.Init(tbl, lib[:], lib[:], 1, Config{NoElide: true})
	for p := uint8(0); p < 2; p++ {
		pls := &s.pl[p]
		for pls.handN > 0 {
			o := pls.hand[0]
			s.detach(o)
			s.objs[o].zone = kop.ZoneLibrary
			pls.libBottom(o)
		}
	}
	s.turn, s.active, s.step, s.prio, s.passes, s.ctl = 2, 0, kop.StepMain1, 0, 0, ctlPriority
	return &scen{s: s, k: s.k()}, id
}

// TestSBACasualtyBatch: gorge's destroyLethalDamage brackets the SBA
// pass's creature casualties in one zone batch (CR 704.3, 603.2c), so Blood
// Spatter Analysis's "whenever one or more creatures die" queues once for two
// creatures killed by lethal damage together (a Pyroclasm), as it does for a
// Day of Judgment (leaveAll). Two separate SBA passes are two batches.
func TestSBACasualtyBatch(t *testing.T) {
	sc, id := newCasualtyBatchScen(t)
	bs, rg, doj := id["Blood Spatter Analysis"], id["Raging Goblin"], id["Day of Judgment"]
	_ = doj
	w := sc.perm(bs, 0)
	ab := batchAb(t, sc, bs, kop.Tf2Batch)
	kill := func(n int) {
		for i := 0; i < n; i++ {
			sc.s.perms[sc.slot(sc.perm(rg, uint8(i%2)))].damage = 9
		}
	}
	sc.s.npend = 0
	kill(2)
	sc.s.sba(sc.k)
	if n := batchPend(sc, w, ab); n != 1 {
		t.Fatalf("two creatures dying to lethal damage in one pass queued %d triggers, want 1", n)
	}
	sc.s.npend = 0
	kill(1)
	sc.s.sba(sc.k)
	if n := batchPend(sc, w, ab); n != 1 {
		t.Fatalf("a later pass's casualty queued %d triggers, want 1", n)
	}
	sc.s.npend = 0
	var m slotMask
	m = m.with(sc.slot(sc.perm(rg, 0)))
	m = m.with(sc.slot(sc.perm(rg, 1)))
	sc.s.leaveAll(sc.k, m, kop.ZoneGraveyard)
	if n := batchPend(sc, w, ab); n != 1 {
		t.Fatalf("a leaveAll of two creatures to the graveyard queued %d triggers, want 1", n)
	}
	if sc.s.g.batchLatch != 0 {
		t.Fatalf("batch state left open: %#x", sc.s.g.batchLatch)
	}
}

// TestCantDrawEmptyLibrary: a player who can't draw (Mornsong Aria: players
// can't draw cards) does not lose to a draw from an empty library: gorge
// replaces the empty-library mark of a player under drawForbidden with a Note
// (replacement.go; CR 121.6). Without the Aria the loss still happens.
func TestCantDrawEmptyLibrary(t *testing.T) {
	deck := []cardset.Entry{{Name: "Mornsong Aria", Count: 1}, {Name: "Plains", Count: 8}}
	kpbanSet := cardsettest.Set(t, "pban")
	for _, aria := range []bool{true, false} {
		tbl, da, _, err := kpbanSet.Matchup(deck, deck)
		if err != nil {
			t.Fatal(err)
		}
		id := map[string]CardID{}
		for _, c := range da {
			id[tbl.Cards[c].Name] = c
		}
		var lib [20]CardID
		for i := range lib {
			lib[i] = id["Plains"]
		}
		s := new(State)
		s.Init(tbl, lib[:], lib[:], 1, Config{NoElide: true})
		sc := &scen{s: s, k: s.k()}
		s.turn, s.active, s.step, s.prio, s.passes, s.ctl = 2, 0, kop.StepMain1, 0, 0, ctlPriority
		if aria {
			sc.perm(id["Mornsong Aria"], 0)
		}
		s.pl[1].libLo = s.pl[1].libHi
		s.draw(1)
		s.sba(sc.k)
		if lost := s.pl[1].lost; lost == aria {
			t.Fatalf("aria=%v: the player with an empty library lost=%v", aria, lost)
		}
	}
}
