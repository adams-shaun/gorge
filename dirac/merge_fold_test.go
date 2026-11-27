package dirac

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// mergeFoldTable builds a generated table with one survivable creature, a
// program that carries ExtMergePermanent (so prepMerge sizes coldMerge), and
// six component cards.
func mergeFoldTable(t *testing.T) (*kop.Table, CardID, []CardID) {
	t.Helper()
	tbl, w := monstrosityTable(t, 1, false)
	tbl.Cards[w].Abilities = append(tbl.Cards[w].Abilities,
		kop.Ability{Kind: kop.AbActivated, Prog: []kop.Instr{{Op: kop.OpExt, A: kop.Const(int(kop.ExtMergePermanent))}}})
	ids := make([]CardID, 6)
	for i := range ids {
		ids[i] = CardID(len(tbl.Cards))
		tbl.Cards = append(tbl.Cards, kop.CardDef{
			Name: fmt.Sprintf("Test Merge Component %d", i), Types: kop.TCreature, Power: 1, Toughness: 1,
		})
	}
	return tbl, w, ids
}

func mergeScen(t *testing.T) (*scen, CardID, []CardID) {
	t.Helper()
	tbl, w, ids := mergeFoldTable(t)
	return newScenTable(t, tbl, Config{NoElide: true}), w, ids
}

// mergeNow runs one ExtMergePermanent instruction: surv is the target (the
// pile's survivor) and src is the mutating spell's object.
func mergeNow(sc *scen, surv, src Obj, top bool) {
	in := kop.Instr{Op: kop.OpExt, Obj: kop.RefTarget0, A: kop.Const(int(kop.ExtMergePermanent))}
	if top {
		in.C = 1
	}
	sc.s.res.e.obj = src
	sc.s.res.e.tgt[0] = TCode(surv)
	sc.s.exec(sc.k, &in)
}

// mergeRecAt finds the pile record topped by surv.
func mergeRecAt(s *State, surv Obj) (*mergeRec, bool) {
	recs := coldMerge.of(s)
	for i := range recs {
		if recs[i].top == objWord(surv)+1 {
			return &recs[i], true
		}
	}
	return nil, false
}

// TestMergePileFoldTopUnder: a top merge demotes the old top card onto the
// spell's object and an under merge parks the spell's object, in pile order;
// the split puts the components in that order before the top, and a full
// pile stalls with StallMerge.
func TestMergePileFoldTopUnder(t *testing.T) {
	t.Run("top then under", func(t *testing.T) {
		sc, w, ids := mergeScen(t)
		s := sc.s
		surv := sc.perm(w, 0)
		sl := sc.slot(surv)
		if got := s.cardAt(sl); got != w {
			t.Fatalf("survivor card %d, want %d", got, w)
		}
		src1, src2 := sc.obj(ids[0], 0), sc.obj(ids[1], 0)
		if s.objs[src1].zone != kop.ZoneNone || s.objs[src2].zone != kop.ZoneNone {
			t.Fatalf("component objects start in zones %d/%d, want ZoneNone", s.objs[src1].zone, s.objs[src2].zone)
		}

		mergeNow(sc, surv, src1, true)
		if got := s.cardAt(sl); got != ids[0] {
			t.Fatalf("top merge left card %d, want the spell's %d", got, ids[0])
		}
		r, ok := mergeRecAt(s, surv)
		if !ok {
			t.Fatal("no merge record after the top merge")
		}
		if r.n != 1 || r.tm != 1 || r.und[0] != objWord(src1)+1 {
			t.Fatalf("after top: n=%d tm=%d und[0]=%d, want 1/1/%d", r.n, r.tm, r.und[0], src1+1)
		}
		if s.objs[src1].card != w || s.objs[src1].zone != kop.ZoneNone {
			t.Fatalf("demoted holder card/zone %d/%d, want %d/ZoneNone", s.objs[src1].card, s.objs[src1].zone, w)
		}

		mergeNow(sc, surv, src2, false)
		if got := s.cardAt(sl); got != ids[0] {
			t.Fatalf("under merge changed the top card to %d, want %d", got, ids[0])
		}
		r, ok = mergeRecAt(s, surv)
		if !ok {
			t.Fatal("no merge record after the under merge")
		}
		if r.n != 2 || r.tm != 2 || r.und[0] != objWord(src1)+1 || r.und[1] != objWord(src2)+1 {
			t.Fatalf("after under: n=%d tm=%d und=%v, want 2/2/[%d %d]", r.n, r.tm, r.und, src1+1, src2+1)
		}
		if s.objs[src2].card != ids[1] || s.objs[src2].zone != kop.ZoneNone {
			t.Fatalf("under holder card/zone %d/%d, want %d/ZoneNone", s.objs[src2].card, s.objs[src2].zone, ids[1])
		}

		s.removePerm(sc.k, sl, kop.ZoneGraveyard, false)
		if _, ok := mergeRecAt(s, surv); ok {
			t.Fatal("the split left the pile record live")
		}
		want := []Obj{src1, src2, surv}
		for i, o := range want {
			if s.objs[o].zone != kop.ZoneGraveyard {
				t.Errorf("component %d is in zone %d, want the graveyard", o, s.objs[o].zone)
			}
			if s.pl[0].grave[i] != o {
				t.Errorf("graveyard[%d] is object %d, want %d (pile order before the top)", i, s.pl[0].grave[i], o)
			}
		}
	})

	t.Run("debug keep", func(t *testing.T) {
		sc, w, ids := mergeScen(t)
		s := sc.s
		surv := sc.perm(w, 0)
		sl := sc.slot(surv)
		src := sc.obj(ids[0], 0)
		mergeNow(sc, surv, src, true)
		if s.objs[src].zone != kop.ZoneNone {
			t.Fatalf("precondition: the demoted holder is in zone %d, want ZoneNone", s.objs[src].zone)
		}

		DebugMergeKeep = true
		defer func() { DebugMergeKeep = false }()
		s.removePerm(sc.k, sl, kop.ZoneGraveyard, false)
		if s.objs[src].zone != kop.ZoneNone {
			t.Fatalf("DebugMergeKeep moved the component to zone %d", s.objs[src].zone)
		}
		if _, ok := mergeRecAt(s, surv); ok {
			t.Fatal("DebugMergeKeep left the pile record live")
		}
		if s.objs[surv].zone != kop.ZoneGraveyard {
			t.Fatalf("the top is in zone %d, want the graveyard", s.objs[surv].zone)
		}
	})

	t.Run("capacity", func(t *testing.T) {
		sc, w, ids := mergeScen(t)
		s := sc.s
		surv := sc.perm(w, 0)
		src := make([]Obj, 5)
		for i := range src {
			src[i] = sc.obj(ids[i], 0)
		}
		mergeNow(sc, surv, src[0], true)
		for i := 1; i < mergeCap; i++ {
			mergeNow(sc, surv, src[i], false)
		}
		r, ok := mergeRecAt(s, surv)
		if !ok || r.n != mergeCap {
			t.Fatalf("pile has n=%d (record %v), want %d", r.n, ok, mergeCap)
		}
		if s.mergeRoom(surv) {
			t.Fatal("precondition: mergeRoom still reports room at the cap")
		}
		if s.mergeFold(sc.k, surv, src[mergeCap], false, 1) {
			t.Fatal("a fifth component folded past mergeCap")
		}
		if got := s.StallReason(); got != StallMerge {
			t.Fatalf("stall reason %q, want %q", StallName(got), StallName(StallMerge))
		}
		if r.n != mergeCap {
			t.Fatalf("the refused fold changed n to %d", r.n)
		}
	})
}

// TestMergePileCommanderRetarget: a top merge moves each seat's commander
// designation with its card, in both directions.
func TestMergePileCommanderRetarget(t *testing.T) {
	sc, w, ids := mergeScen(t)
	s := sc.s
	surv := sc.perm(w, 0)
	src := sc.obj(ids[0], 0)
	if s.objs[surv].zone != kop.ZoneBattlefield || s.objs[src].zone != kop.ZoneNone {
		t.Fatalf("precondition: survivor/commander zones %d/%d, want battlefield/ZoneNone", s.objs[surv].zone, s.objs[src].zone)
	}
	s.pl[0].cmd = surv
	s.pl[1].cmd = src
	if s.pl[0].cmd == s.pl[1].cmd {
		t.Fatal("precondition: both seats name the same object")
	}

	mergeNow(sc, surv, src, true)
	if s.pl[0].cmd != src {
		t.Fatalf("seat 0's commander is object %d, want the demoted holder %d", s.pl[0].cmd, src)
	}
	if s.pl[1].cmd != surv {
		t.Fatalf("seat 1's commander is object %d, want the survivor %d", s.pl[1].cmd, surv)
	}
}
