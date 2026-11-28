package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// A RefSet Sacrifice or Discard whose members are all players is gorge's
// effSacrifice and effDiscard over Defined$ Player: every player chooses in
// turn from the controller and what they chose moves at once (CR 101.4), so
// one member's ask must run the walk on to the members after it, not end it
// (value_setexpr.go execSet).

// setSacScen builds an fdn table whose Savannah Lions carry one RefSet
// ability "each player sacrifices a creature" (OpSacrifice over
// SetEachPlayer, Amount$ 1 through a creature filter), and puts two Lions on
// each battlefield (a1/a2 seat 0, b1/b2 seat 1).
func setSacScen(t *testing.T) (sc *scen, lions CardID, ab int, a1, a2, b1, b2 Obj) {
	t.Helper()
	set := cardsettest.Set(t, "fdn")
	deck := []cardset.Entry{{Name: "Savannah Lions", Count: 2}, {Name: "Island", Count: 4}}
	if i, ok := set.Lookup("Savannah Lions"); !ok || !set.Supported()[i] {
		t.Skip("Savannah Lions not in the FDN table")
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range da {
		if tbl.Cards[c].Name == "Savannah Lions" {
			lions = c
		}
	}
	fCre := kop.FilterID(len(tbl.Filters))
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature})
	tbl.Sets = []kop.SetExpr{{{Kind: kop.SetEachPlayer}}}
	cd := tbl.Cards[lions]
	ab = len(cd.Abilities)
	cd.Abilities = append(cd.Abilities, kop.Ability{
		Prog:    []kop.Instr{{Op: kop.OpSacrifice, Obj: kop.RefSet, A: kop.Const(1), F: fCre}},
		SetRefs: []kop.SetRef{{PC: 0, Set: 0}},
	})
	tbl.Cards[lions] = cd
	sc = newScenTable(t, tbl, Config{NoElide: true})
	if !sc.k.gen.anyUnion {
		t.Fatal("precondition: a table with a RefSet subject takes the union arm")
	}
	a1, a2 = sc.perm(lions, 0), sc.perm(lions, 0)
	b1, b2 = sc.perm(lions, 1), sc.perm(lions, 1)
	for _, o := range []Obj{a1, a2, b1, b2} {
		if !sc.onBF(o) {
			t.Fatalf("precondition: %d not on the battlefield", o)
		}
	}
	return sc, lions, ab, a1, a2, b1, b2
}

// TestExecSetResumesAfterAsk: a RefSet sacrifice over SetEachPlayer asks each
// player in turn order from the controller, keeps every pick on the
// battlefield until all have chosen (CR 101.4), and sacrifices exactly one
// creature per player. Before the fix the walk stopped at player 0's ask and
// player 1 was never asked.
func TestExecSetResumesAfterAsk(t *testing.T) {
	sc, lions, ab, a1, a2, b1, b2 := setSacScen(t)
	s := sc.s
	e := stackEntry{kind: seAbility, obj: a1, srcVer: s.objs[a1].ver, card: lions, ab: uint8(ab), ctrl: 0, trigObj: NoObj}
	for i := range e.tgt {
		e.tgt[i] = NoTarget
	}
	s.stack[0] = e
	s.sp = 1
	s.resolveTop(sc.k)
	d := s.Step()
	if d.Kind != DkChoose || d.Sub != ChSacrifice || d.Player != 0 {
		t.Fatalf("first ask kind %d sub %d player %d, want DkChoose ChSacrifice 0 (the controller first)", d.Kind, d.Sub, d.Player)
	}
	sc.do(t, AkChoose, TCode(a1), scAny)
	if !sc.onBF(a1) || !sc.onBF(b1) {
		t.Fatalf("a pick moved before every player chose: a1 %v b1 %v on the battlefield (CR 101.4: all go at once)", sc.onBF(a1), sc.onBF(b1))
	}
	d = s.Step()
	if d.Kind != DkChoose || d.Sub != ChSacrifice || d.Player != 1 {
		t.Fatalf("second ask kind %d sub %d player %d, want DkChoose ChSacrifice 1: the walk stopped at the first member's ask", d.Kind, d.Sub, d.Player)
	}
	sc.do(t, AkChoose, TCode(b1), scAny)
	if sc.onBF(a1) || sc.onBF(b1) {
		t.Errorf("both players' picks not sacrificed: a1 %v b1 %v on the battlefield", sc.onBF(a1), sc.onBF(b1))
	}
	if !sc.onBF(a2) || !sc.onBF(b2) {
		t.Errorf("a member sacrificed more than its pick: a2 %v b2 %v", sc.onBF(a2), sc.onBF(b2))
	}
}

// setDiscardScen builds an fdn table with one sorcery whose program is a
// RefSet discard of two cards (OpDiscard over SetEachPlayer, NumCards$ 2,
// ChooseSelf) and gives each seat a three-card hand.
func setDiscardScen(t *testing.T) (sc *scen, src CardID, h [2][3]Obj) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	if int(v.Plains) >= len(tbl.Cards) {
		t.Skip("table too small for the scenario library")
	}
	src = CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Set Discard", Types: kop.TSorcery,
		Abilities: []kop.Ability{{Kind: kop.AbSpell, Prog: []kop.Instr{{
			Op: kop.OpDiscard, Obj: kop.RefSet, A: kop.Const(2), C: kop.ChooseSelf}},
			SetRefs: []kop.SetRef{{PC: 0, Set: 0}}}}})
	tbl.Sets = []kop.SetExpr{{{Kind: kop.SetEachPlayer}}}
	sc = newScenTable(t, tbl, Config{NoElide: true})
	if !sc.k.gen.anyUnion {
		t.Fatal("precondition: a table with a RefSet subject takes the union arm")
	}
	o := sc.obj(src, 0)
	sc.s.objs[o].zone = kop.ZoneStack
	sc.s.push(stackEntry{kind: seSpell, obj: o, ctrl: 0, card: src, trigObj: NoObj})
	sc.s.res.e = stackEntry{kind: seSpell, obj: o, ctrl: 0, card: src, trigObj: NoObj}
	sc.s.res.pc = 0
	for p := uint8(0); p < 2; p++ {
		for i := range h[p] {
			h[p][i] = sc.hand(v.Plains, p)
		}
	}
	return sc, src, h
}

// TestExecSetDiscardResumesAfterAsk: a RefSet discard over SetEachPlayer asks
// each player in turn from the controller, holds the first player's picks in
// hand while the second chooses (gorge's held discard, CR 101.4), and moves
// both players' picks once the last has chosen.
func TestExecSetDiscardResumesAfterAsk(t *testing.T) {
	sc, _, h := setDiscardScen(t)
	sc.s.runProg(sc.k)
	d := sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChDiscard || d.Player != 0 {
		t.Fatalf("first ask kind %d sub %d player %d, want DkChoose ChDiscard 0 (the controller first)", d.Kind, d.Sub, d.Player)
	}
	sc.do(t, AkChoose, TCode(h[0][0]), scAny)
	sc.do(t, AkChoose, TCode(h[0][1]), scAny)
	if sc.s.objs[h[0][0]].zone != kop.ZoneHand || sc.s.objs[h[0][1]].zone != kop.ZoneHand {
		t.Fatalf("player 0's picks moved before player 1 chose: %v %v (gorge holds them, CR 101.4)",
			sc.s.objs[h[0][0]].zone, sc.s.objs[h[0][1]].zone)
	}
	d = sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChDiscard || d.Player != 1 {
		t.Fatalf("second ask kind %d sub %d player %d, want DkChoose ChDiscard 1: the walk stopped at the first member's ask", d.Kind, d.Sub, d.Player)
	}
	sc.do(t, AkChoose, TCode(h[1][0]), scAny)
	sc.do(t, AkChoose, TCode(h[1][1]), scAny)
	if sc.s.pl[0].handN != 1 || sc.s.pl[1].handN != 1 {
		t.Fatalf("hands %d/%d, want 1/1", sc.s.pl[0].handN, sc.s.pl[1].handN)
	}
	for _, o := range []Obj{h[0][0], h[0][1], h[1][0], h[1][1]} {
		if sc.s.objs[o].zone != kop.ZoneGraveyard {
			t.Errorf("picked card %d in zone %v, want the graveyard", o, sc.s.objs[o].zone)
		}
	}
	for _, o := range []Obj{h[0][2], h[1][2]} {
		if sc.s.objs[o].zone != kop.ZoneHand {
			t.Errorf("unpicked card %d in zone %v, want the hand", o, sc.s.objs[o].zone)
		}
	}
}
