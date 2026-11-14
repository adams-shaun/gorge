package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// embColdScen is a scenario whose Plains makes an emblem from the ability of
// the given kind.
func embColdScen(t *testing.T, kind kop.AbilityKind) *scen {
	t.Helper()
	sc, _ := newBLBScenTweak(t, []cardset.Entry{{Name: "Plains", Count: 8}}, func(tbl *kop.Table, id map[string]CardID) {
		tbl.Emblems = append(tbl.Emblems[:len(tbl.Emblems):len(tbl.Emblems)], kop.Emblem{})
		cd := &tbl.Cards[id["Plains"]]
		cd.Abilities = append(append([]kop.Ability(nil), cd.Abilities...), kop.Ability{Kind: kind, Prog: []kop.Instr{
			{Op: kop.OpCreateEmblem, Obj: kop.RefController, C: int16(len(tbl.Emblems) - 1)},
		}})
	})
	sc.s.g.on = true
	return sc
}

// embColdMake runs the table's OpCreateEmblem n times for player p, each
// with its own source card and object.
func embColdMake(sc *scen, p uint8, from, n int) {
	in := kop.Instr{Op: kop.OpCreateEmblem, Obj: kop.RefController, C: int16(len(sc.k.t.Emblems) - 1)}
	for i := from; i < from+n; i++ {
		sc.s.res.e.ctrl, sc.s.res.e.card, sc.s.res.e.obj = p, CardID(i+1), Obj(i+1)
		sc.s.execGen(sc.k, &in)
	}
}

// TestEmblemCapCold: a table whose activated or triggered ability makes an
// emblem holds maxEmbTotal of them per player, those past maxEmb in the cold
// buffer, in creation order through a compaction; the next one stalls. A
// table that can make one only from a spell keeps maxEmb and no cold bytes.
func TestEmblemCapCold(t *testing.T) {
	once := embColdScen(t, kop.AbSpell)
	if c := once.s.embCap(); c != maxEmb || once.k.gen.anyEmbCold || coldEmb.count(once.s.cold.layout) != 0 {
		t.Fatalf("a spell-only table: capacity %d, flag %v, cold records %d; want %d, false, 0", c, once.k.gen.anyEmbCold, coldEmb.count(once.s.cold.layout), maxEmb)
	}
	embColdMake(once, 0, 0, maxEmb+1)
	if once.s.g.nemb[0] != maxEmb || once.s.g.stalled != StallEmblem {
		t.Fatalf("a spell-only table: %d emblems, stall %d; want %d and StallEmblem", once.s.g.nemb[0], once.s.g.stalled, maxEmb)
	}

	sc := embColdScen(t, kop.AbActivated)
	s := sc.s
	if !sc.k.gen.anyEmbCold || s.embCap() != maxEmbTotal || s.embCap() != 8 {
		t.Fatalf("flag %v, capacity %d; want true, 8", sc.k.gen.anyEmbCold, s.embCap())
	}
	embColdMake(sc, 0, 0, 8)
	embColdMake(sc, 1, 100, 8)
	if s.g.nemb[0] != 8 || s.g.nemb[1] != 8 || s.g.stalled != 0 {
		t.Fatalf("after 8 each: %d and %d emblems, stall %d", s.g.nemb[0], s.g.nemb[1], s.g.stalled)
	}
	for p, base := range []int{0, 100} {
		for i := 0; i < 8; i++ {
			r := s.embGet(uint8(p), i)
			if r.idx != uint8(len(sc.k.t.Emblems)) || r.c != CardID(base+i+1) || r.obj != Obj(base+i+1) || r.t == 0 || (i > 0 && r.t <= s.embStamp(uint8(p), i-1)) {
				t.Fatalf("player %d emblem %d = %+v, want card %d object %d and a timestamp past the one before", p, i, r, base+i+1, base+i+1)
			}
		}
	}
	embColdMake(sc, 0, 8, 1)
	if s.g.nemb[0] != 8 || s.g.stalled != StallEmblem {
		t.Fatalf("the ninth emblem: %d emblems, stall %d; want 8 and StallEmblem", s.g.nemb[0], s.g.stalled)
	}
	if !s.heldReferenced(Obj(8)) || !s.heldReferenced(Obj(105)) {
		t.Fatal("an emblem's source in the cold buffer is not held")
	}
	h := s.genViewHash(0)
	s.embRecAt(1, 7).num++
	if s.genViewHash(0) == h {
		t.Fatal("a cold emblem's change left the view hash alone")
	}
	var cp State
	cp.CopyFrom(s)
	if !cp.SameGame(s) || cp.embGet(1, 7) != s.embGet(1, 7) {
		t.Fatal("a copy lost the cold emblems")
	}

	// emblems 1 and 6 end; bits 0, 2, 4, 5 and 7 were set, and their survivors
	// are now emblems 0, 1, 3, 4 and 5
	before := [8]embRec{}
	for i := range before {
		before[i] = s.embGet(0, i)
	}
	s.g.embF[0] = 0b1011_0101
	s.embCompact(0, 1<<1|1<<6)
	if s.g.nemb[0] != 6 || s.g.embF[0] != 0b0011_1011 {
		t.Fatalf("after the compaction: %d emblems, bits %08b; want 6 and 00111011", s.g.nemb[0], s.g.embF[0])
	}
	for i, from := range []int{0, 2, 3, 4, 5, 7} {
		if s.embGet(0, i) != before[from] {
			t.Fatalf("survivor %d = %+v, want the emblem that was %d: %+v", i, s.embGet(0, i), from, before[from])
		}
	}
	for i := 6; i < 8; i++ {
		if s.embGet(0, i) != (embRec{}) {
			t.Fatalf("freed emblem %d = %+v, want zero", i, s.embGet(0, i))
		}
	}
}
