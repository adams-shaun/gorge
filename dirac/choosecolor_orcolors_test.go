package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// runChooseColorRes runs one ExtChooseColorRes instruction with C = c in a
// resolution of an Archnemesis (any permanent: the op reads only the
// resolution) and returns the state after it.
func runChooseColorRes(t *testing.T, c int16) *State {
	t.Helper()
	set := cardsettest.Set(t, "cursecurses")
	deck := []cardset.Entry{{Name: "Archnemesis", Count: 4}, {Name: "Island", Count: 36}}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, cd := range da {
		id[tbl.Cards[cd].Name] = cd
	}
	sc := newScenTableLib(t, tbl, id["Island"], Config{})
	o := sc.perm(id["Archnemesis"], 0)
	s := sc.s
	s.res = resCtx{e: stackEntry{kind: seAbility, obj: o, srcVer: s.objs[o].ver,
		card: s.objs[o].card, ab: 0, ctrl: 0, trigObj: NoObj}, active: true, lastTok: NoObj, eachObj: NoObj}
	in := kop.Instr{Op: kop.OpExt, A: kop.Const(int(kop.ExtChooseColorRes)), C: c}
	s.exec(sc.k, &in)
	return s
}

// TestChooseColorOrColors: a ChooseColor | OrColors$ True resolution
// (Quickchange, Shyft) poses no ask and records white, the answer gorge's
// effChooseColor falls back to for the shape it cannot ask (OrColors$ is
// "unaskable": a Note, then the first-WUBRG colour). The plain instruction
// (C = 0) still asks the controller for one of five colours, so the no-ask
// assertion has a control it can fail against.
func TestChooseColorOrColors(t *testing.T) {
	ask := runChooseColorRes(t, 0)
	if ask.res.ask != raChooseColorRes {
		t.Fatalf("the plain ExtChooseColorRes posed ask %d, want raChooseColorRes %d", ask.res.ask, raChooseColorRes)
	}
	if ask.res.chosen != 0 {
		t.Fatalf("the plain ExtChooseColorRes recorded %d before any answer", ask.res.chosen)
	}
	s := runChooseColorRes(t, 1)
	if s.res.ask != raNone {
		t.Errorf("OrColors ask %d, want none", s.res.ask)
	}
	if s.res.chosen != 1 || s.res.col != 1 {
		t.Errorf("OrColors recorded chosen=%d col=%d, want white (1, 1)", s.res.chosen, s.res.col)
	}
}
