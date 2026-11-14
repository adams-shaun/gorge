package dirac

import (
	"testing"
	"unsafe"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// mayColdScen is a scenario whose table has a card granting play permissions
// over its remembered set, so the cold buffer holds past maxMay records.
func mayColdScen(t *testing.T, withGrant bool) (*scen, Obj) {
	sc, _ := newBLBScenTweak(t, []cardset.Entry{{Name: "Plains", Count: 8}}, func(tbl *kop.Table, id map[string]CardID) {
		if !withGrant {
			return
		}
		cd := &tbl.Cards[id["Plains"]]
		cd.Abilities = append(append([]kop.Ability(nil), cd.Abilities...), kop.Ability{Kind: kop.AbActivated, Prog: []kop.Instr{
			{Op: kop.OpMayPlay, Obj: kop.RefRemembered, C: int16(kop.ZoneExile), A: kop.Const(kop.DurYourNextTurn)},
		}})
	})
	s := sc.s
	o := s.pl[0].lib[0]
	s.detach(o)
	s.objs[o].zone = kop.ZoneExile
	s.g.rem[0], s.g.nrem = o, 1
	return sc, o
}

// TestMayCapCold: a table that grants play permissions over a remembered set
// holds maxMay+maxMayCold of them, the records past maxMay in the cold buffer;
// a turn's expiry keeps the survivors in order across that boundary and the
// next record past the capacity stalls. A table without such a grant keeps
// maxMay and no cold bytes.
func TestMayCapCold(t *testing.T) {
	plain, _ := mayColdScen(t, false)
	if c := plain.s.mayCap(); c != maxMay || plain.k.gen.anyMayCold || coldMay.count(plain.s.cold.layout) != 0 {
		t.Fatalf("a table with no remembered-set grant: capacity %d, flag %v, cold records %d; want %d, false, 0", c, plain.k.gen.anyMayCold, coldMay.count(plain.s.cold.layout), maxMay)
	}
	sc, o := mayColdScen(t, true)
	s := sc.s
	if !sc.k.gen.anyMayCold || s.mayCap() != maxMay+maxMayCold || s.mayCap() != 48 {
		t.Fatalf("flag %v, capacity %d; want true, 48", sc.k.gen.anyMayCold, s.mayCap())
	}
	in := kop.Instr{Op: kop.OpMayPlay, Obj: kop.RefRemembered, C: int16(kop.ZoneExile), A: kop.Const(kop.DurYourNextTurn)}
	s.g.on = true
	for i := 0; i < s.mayCap(); i++ {
		s.turn = uint16(i + 1)
		s.mayPlayRemembered(sc.k, &in, uint8(i%2))
	}
	if int(s.g.nmay) != 48 || s.g.stalled != 0 {
		t.Fatalf("after 48 grants: %d records, stall %d", s.g.nmay, s.g.stalled)
	}
	for i := 0; i < 48; i++ {
		if r := s.mayAt(i); r.obj != o || int(r.ctrl) != i%2 || int(r.turn) != i+1 {
			t.Fatalf("record %d = %+v, want obj %d ctrl %d turn %d", i, *r, o, i%2, i+1)
		}
	}
	s.mayPlayRemembered(sc.k, &in, 0)
	if int(s.g.nmay) != 48 || s.g.stalled != StallMayPlay {
		t.Fatalf("the 49th grant: %d records, stall %d; want 48 records and StallMayPlay", s.g.nmay, s.g.stalled)
	}
	// player 0's records granted before the turn counter reads 100 expire
	// at their controller's turn; player 1's stay
	s.active, s.turn = 0, 100
	s.cleanupGen(sc.k)
	if int(s.g.nmay) != 24 {
		t.Fatalf("after the expiry: %d records, want player 1's 24", s.g.nmay)
	}
	for i := 0; i < 24; i++ {
		if r := s.mayAt(i); r.ctrl != 1 || int(r.turn) != 2*i+2 {
			t.Fatalf("survivor %d = %+v, want ctrl 1 turn %d", i, *r, 2*i+2)
		}
	}
	var d Decision
	h := s.DecisionHash(&d)
	s.mayAt(23).turn++
	if s.DecisionHash(&d) == h {
		t.Fatalf("a cold record's change left the hash alone")
	}
	// the hot records end at maxMay: the bytes after g.may (a dead
	// delayed-program slot here) stay out of the hash however many cold
	// records are live
	h = s.DecisionHash(&d)
	if s.g.ndp != 0 || int(s.g.nmay) <= maxMay {
		t.Fatalf("precondition: %d live delayed programs, %d records; want 0 and past %d", s.g.ndp, s.g.nmay, maxMay)
	}
	*(*byte)(unsafe.Pointer(&s.g.dp[0])) ^= 0xff
	if s.DecisionHash(&d) != h {
		t.Fatalf("a dead delayed-program slot after the hot records moved the hash")
	}
}
