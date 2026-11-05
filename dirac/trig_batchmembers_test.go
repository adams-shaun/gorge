package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestBatchMembersReachThePick: Kaya, Spirits' Justice's exile line queues once
// for a graveyard sweep of two creature cards to exile (gorge's zone batch),
// and the one instance carries both cards as its batch members; the pick
// lists them in exile order (gorge cloneChoiceCandidates), not any other.
func TestBatchMembersReachThePick(t *testing.T) {
	sc, id := newBatchPoolScen(t, []cardset.Entry{
		{Name: "Kaya, Spirits' Justice", Count: 1}, {Name: "Grizzly Bears", Count: 2},
		{Name: "Plains", Count: 8}, {Name: "Swamp", Count: 8},
	})
	kaya, bears := id["Kaya, Spirits' Justice"], id["Grizzly Bears"]
	c := sc.perm(kaya, 0)
	var ab uint8
	var pick kop.Instr
	found := false
	for i := range sc.k.t.Cards[kaya].Abilities {
		a := &sc.k.t.Cards[kaya].Abilities[i]
		if readsBatchPool(a) {
			ab, found = uint8(i), true
			for _, in := range a.Prog {
				if in.Ext() == kop.ExtCloneCopy && in.B.N&kop.ClonePick != 0 {
					pick = in
				}
			}
		}
	}
	if !found || pick.F == 0 {
		t.Fatal("Kaya's table has no batch pick")
	}
	sc.grave(bears, 0, 2)
	g := sc.s.pl[0]
	if g.graveN != 2 {
		t.Fatalf("precondition: %d cards in the graveyard, want 2", g.graveN)
	}
	first, second := g.grave[0], g.grave[1]
	var f kop.FilterID
	for i, fl := range sc.k.t.Filters {
		if fl.Zone == kop.ZoneGraveyard && fl.Types == 0 && fl.Or == 0 {
			f = kop.FilterID(i)
			break
		}
	}
	if f == 0 {
		t.Fatal("no graveyard filter in the pool")
	}
	in := kop.Instr{Op: kop.OpChangeZoneAll, C: int16(kop.ZoneExile), F: f}
	sc.s.npend = 0
	sc.s.res.e.ctrl = 0
	sc.s.execChangeZoneAll(sc.k, &in, false)
	if n := batchPend(sc, c, ab); n != 1 {
		t.Fatalf("a two-card sweep to exile queued %d triggers, want 1", n)
	}
	var pt *pendingTrig
	for i := 0; i < int(sc.s.npend); i++ {
		if sc.s.pend[i].obj == c && sc.s.pend[i].ab == ab {
			pt = &sc.s.pend[i]
		}
	}
	if pt == nil || pt.grp == 0 {
		t.Fatal("the queued instance holds no batch member set")
	}
	set := sc.s.cold.atkGrp[pt.grp-1]
	if !set.has(first) || !set.has(second) {
		t.Fatalf("members %v lack a swept card (%d, %d)", set, first, second)
	}
	sc.s.res.e.grp = pt.grp
	var d Decision
	if n := sc.s.cloneBatchCands(sc.k, &pick, &d); n != 2 {
		t.Fatalf("the pick lists %d cards, want both swept creatures", n)
	}
	if sc.s.g.batchLatch != 0 {
		t.Fatalf("batch state left open: %#x", sc.s.g.batchLatch)
	}
}

// newBatchPoolScen is newLeftGraveyardScen over the zonebatchclone pool.
func newBatchPoolScen(t *testing.T, deck []cardset.Entry) (*scen, map[string]CardID) {
	t.Helper()
	set := cardsettest.Set(t, "zonebatchclone")
	tbl, da, _, err := set.Matchup(deck, deck)
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
