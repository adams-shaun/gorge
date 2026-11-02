package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// forgetChangedBody runs the three ForgetChanged$ moves over remembered
// library cards and returns the remembered sets after each: an OpReturnEx
// (MvForgetChanged, RetFromLibrary) of Remembered onto the battlefield, an
// OpBounce (MvForgetChanged, BncFromLibrary) and the found card of an
// OpSearchEx (SrForgetChanged); kept are the cards each move left remembered.
func forgetChangedBody(t *testing.T) (ret, bnc, srch, kept []Obj) {
	t.Helper()
	sc, id := newRemParamsScen(t)
	s := sc.s
	s.res.e.obj = sc.obj(id["Plains"], 0) // the resolving spell
	pl := &s.pl[0]
	a, b, c := pl.lib[pl.libLo], pl.lib[pl.libLo+1], pl.lib[pl.libLo+2]
	set := func(os ...Obj) {
		s.g.nrem = uint8(copy(s.g.rem[:], os))
	}
	got := func() []Obj { return append([]Obj(nil), s.g.rem[:s.g.nrem]...) }

	// the resolving spell is no library card, so the return leaves it remembered
	unmoved := s.res.e.obj
	set(a, unmoved)
	in := kop.Instr{Op: kop.OpReturnEx, Obj: kop.RefRemembered, B: kop.Const(kop.RetFromLibrary),
		C: int16(kop.ZoneBattlefield) | kop.MvForgetChanged<<3}
	s.execGen(sc.k, &in)
	if s.objs[a].zone != kop.ZoneBattlefield {
		t.Fatalf("return: first remembered card in zone %d, want the battlefield", s.objs[a].zone)
	}
	ret = got()

	set(b, c)
	bn := kop.Instr{Op: kop.OpBounce, Obj: kop.RefRemembered, C: kop.MvForgetChanged | kop.BncFromLibrary}
	s.execGen(sc.k, &bn)
	if s.objs[b].zone != kop.ZoneHand {
		t.Fatalf("bounce: first remembered card in zone %d, want the hand", s.objs[b].zone)
	}
	bnc = got()

	// the search: c is found and moves to the battlefield; the card d stays
	d := pl.lib[pl.libLo+3]
	set(d, c)
	card := &sc.k.t.Cards[id["Plains"]]
	card.Abilities[0].Prog = []kop.Instr{{Op: kop.OpSearchEx, Obj: kop.RefController, A: kop.Const(1), B: kop.Const(kop.SrForgetChanged),
		C: int16(kop.ZoneBattlefield)}}
	s.res.pc = 0
	s.g.pickP, s.g.pickZ, s.g.pickF = 0, uint8(kop.ZoneBattlefield), 0
	s.resArm().look[0], s.g.nlook = c, 1
	s.moveFound(sc.k)
	if s.objs[c].zone != kop.ZoneBattlefield {
		t.Fatalf("search: found card in zone %d, want the battlefield", s.objs[c].zone)
	}
	srch = got()
	return ret, bnc, srch, []Obj{unmoved, c, d}
}

func sameObjs(a, b []Obj) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestForgetChangedDropsMoved: ForgetChanged$ True drops each moved card
// from the remembered set and keeps the unmoved one, order preserved (gorge
// eventForgetChanged / forgetRememberedOne).
func TestForgetChangedDropsMoved(t *testing.T) {
	ret, bnc, srch, kept := forgetChangedBody(t)
	for _, c := range []struct {
		name      string
		got, want []Obj
	}{{"return", ret, kept[:1]}, {"bounce", bnc, kept[1:2]}, {"search", srch, kept[2:]}} {
		if !sameObjs(c.got, c.want) {
			t.Errorf("%s: remembered %v, want %v (the moved card forgotten)", c.name, c.got, c.want)
		}
	}
}

// TestForgetChangedMutation: with DebugForgetChangedOff the moved card stays
// remembered, so TestForgetChangedDropsMoved would fail.
func TestForgetChangedMutation(t *testing.T) {
	DebugForgetChangedOff = true
	defer func() { DebugForgetChangedOff = false }()
	ret, bnc, srch, _ := forgetChangedBody(t)
	if len(ret) != 2 || len(bnc) != 2 || len(srch) != 2 {
		t.Fatalf("mutation: remembered sets %v %v %v, want the moved cards still present", ret, bnc, srch)
	}
}
