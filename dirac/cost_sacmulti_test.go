package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// multiSacScen builds a generated table with one extra card whose activated
// ability is Priest of Forgotten Gods' shape: "each targeted player
// sacrifices a creature, then you draw a card" (Defined$ Targeted over a
// player target slot, Amount$ 1). The source is an artifact, so it is never
// one of the creature candidates; each seat gets p0/p1 Grizzly Bears.
func multiSacScen(t *testing.T, p0, p1 int) (*scen, CardID) {
	t.Helper()
	tbl := *v.Table
	tbl.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	tbl.Filters = append([]kop.Filter(nil), v.Table.Filters...)
	tbl.Exprs = []kop.Expr{} // non-nil: the generated interpreter
	f := kop.FilterID(len(tbl.Filters))
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature})
	rite := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Rite", Types: kop.TArtifact,
		Abilities: []kop.Ability{{Kind: kop.AbActivated, Prog: []kop.Instr{
			{Op: kop.OpSacrifice, Obj: kop.RefTarget0, A: kop.Const(1), F: f},
			{Op: kop.OpDraw, Obj: kop.RefController, A: kop.Const(1)},
		}}}})
	sc := newScenTable(t, &tbl, Config{NoElide: true})
	for i := 0; i < p0; i++ {
		sc.perm(v.GrizzlyBears, 0)
	}
	for i := 0; i < p1; i++ {
		sc.perm(v.GrizzlyBears, 1)
	}
	return sc, rite
}

// multiSacResolve starts the test ability's resolution with players as the
// target slot 0 picks (the first in tgt[0], the rest as extra targets).
func multiSacResolve(t *testing.T, sc *scen, rite CardID, players ...uint8) {
	t.Helper()
	src := sc.perm(rite, 0)
	e := stackEntry{kind: seAbility, obj: src, srcVer: sc.s.objs[src].ver, card: rite, ab: 0, ctrl: 0, nt: 1}
	e.tgt[0] = uint8(PlayerTarget0 + TCode(players[0]))
	if len(players) > 1 {
		x := sc.s.xtMut(&e)
		x.n = uint8(len(players) - 1)
		for i, p := range players[1:] {
			x.slot[i], x.code[i], x.ver[i] = 0, uint8(PlayerTarget0+TCode(p)), 0
		}
	}
	sc.s.stack[0] = e
	sc.s.sp = 1
	sc.s.resolveTop(sc.k)
}

// multiSacAsk is the next decision, required to be player p's sacrifice.
func multiSacAsk(t *testing.T, sc *scen, p uint8) *Decision {
	t.Helper()
	d := sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChSacrifice || d.Player != p {
		t.Fatalf("decision kind %d sub %d player %d, want DkChoose ChSacrifice %d", d.Kind, d.Sub, d.Player, p)
	}
	return &d
}

// multiSacPick answers with player p's first creature.
func multiSacPick(t *testing.T, sc *scen, d *Decision, p uint8) {
	t.Helper()
	for _, a := range d.Options[:d.N] {
		if a.Kind() != AkChoose {
			continue
		}
		if sc.s.objs[Obj(a.Source())].owner() == p {
			sc.s.Apply(a)
			return
		}
	}
	t.Fatalf("player %d's sacrifice offer has no creature: %v", p, d.Options[:d.N])
}

// TestMultiPlayerSacrificeAsksEachPlayer: a sacrifice over two targeted
// players asks P0, flushes P0's sacrifice, then asks P1, and only then runs
// the rest of the program (DB$ Draw). Before the fix execEachTarget dropped
// the suspension: the whole program resolved with no ask at all.
func TestMultiPlayerSacrificeAsksEachPlayer(t *testing.T) {
	sc, rite := multiSacScen(t, 2, 2)
	h0, h1 := sc.s.pl[0].handN, sc.s.pl[1].handN
	multiSacResolve(t, sc, rite, 0, 1)

	d0 := multiSacAsk(t, sc, 0)
	if sc.s.pl[0].handN != h0 {
		t.Fatalf("the program ran before the first answer: hand 0 %d, want %d", sc.s.pl[0].handN, h0)
	}
	multiSacPick(t, sc, d0, 0)
	if sc.s.pl[0].graveN != 0 || sc.s.pl[1].graveN != 0 {
		t.Fatalf("after P0's answer: graveyards %d/%d, want 0/0 (all go at once, CR 101.4)",
			sc.s.pl[0].graveN, sc.s.pl[1].graveN)
	}
	if sc.s.pl[0].handN != h0 {
		t.Fatalf("the program ran before the second answer: hand 0 %d, want %d", sc.s.pl[0].handN, h0)
	}

	d1 := multiSacAsk(t, sc, 1)
	multiSacPick(t, sc, d1, 1)
	if sc.s.pl[0].graveN != 1 || sc.s.pl[1].graveN != 1 {
		t.Fatalf("after both answers: graveyards %d/%d, want 1/1", sc.s.pl[0].graveN, sc.s.pl[1].graveN)
	}
	if sc.s.pl[0].handN != h0+1 || sc.s.pl[1].handN != h1 {
		t.Fatalf("after both answers: hands %d/%d, want %d/%d (the controller draws once)",
			sc.s.pl[0].handN, sc.s.pl[1].handN, h0+1, h1)
	}
}

// TestMultiPlayerSacrificeOneTarget: one targeted player keeps the ordinary
// path (no extra target, no execEachTarget): the ask is posed and the
// program runs after the answer.
func TestMultiPlayerSacrificeOneTarget(t *testing.T) {
	sc, rite := multiSacScen(t, 2, 2)
	h0 := sc.s.pl[0].handN
	multiSacResolve(t, sc, rite, 0)
	d := multiSacAsk(t, sc, 0)
	if sc.s.pl[0].handN != h0 {
		t.Fatalf("the program ran before the answer: hand 0 %d, want %d", sc.s.pl[0].handN, h0)
	}
	multiSacPick(t, sc, d, 0)
	if sc.s.pl[0].graveN != 1 || sc.s.pl[1].graveN != 0 {
		t.Fatalf("graveyards %d/%d, want 1/0", sc.s.pl[0].graveN, sc.s.pl[1].graveN)
	}
	if sc.s.pl[0].handN != h0+1 {
		t.Fatalf("hand 0 %d, want %d after the draw", sc.s.pl[0].handN, h0+1)
	}
}

// TestMultiPlayerSacrificeNoChoice: with one creature each and Amount 1, no
// choice is real: both players sacrifice without an ask and the program
// still runs.
func TestMultiPlayerSacrificeNoChoice(t *testing.T) {
	sc, rite := multiSacScen(t, 1, 1)
	h0 := sc.s.pl[0].handN
	multiSacResolve(t, sc, rite, 0, 1)
	if d := sc.s.Step(); d.Kind != DkPriority {
		t.Fatalf("decision kind %d sub %d, want DkPriority (no sacrifice is a choice)", d.Kind, d.Sub)
	}
	if sc.s.pl[0].graveN != 1 || sc.s.pl[1].graveN != 1 {
		t.Fatalf("graveyards %d/%d, want 1/1", sc.s.pl[0].graveN, sc.s.pl[1].graveN)
	}
	if sc.s.pl[0].handN != h0+1 {
		t.Fatalf("hand 0 %d, want %d after the draw", sc.s.pl[0].handN, h0+1)
	}
}

// TestMultiPlayerSacrificePickOrder: the ask order is the slot's target pick
// order, not seat order or APNAP. The caster (P0) targeting P1 then P0 asks
// P1 first, then P0, and the caster draws only after both answers.
func TestMultiPlayerSacrificePickOrder(t *testing.T) {
	sc, rite := multiSacScen(t, 2, 2)
	h0 := sc.s.pl[0].handN
	multiSacResolve(t, sc, rite, 1, 0) // the first target is seat 1

	d1 := multiSacAsk(t, sc, 1)
	if sc.s.pl[0].handN != h0 || sc.s.pl[1].graveN != 0 {
		t.Fatalf("the program or a sacrifice ran before the first answer: hand 0 %d, graveyard 1 %d",
			sc.s.pl[0].handN, sc.s.pl[1].graveN)
	}
	multiSacPick(t, sc, d1, 1)
	if sc.s.pl[1].graveN != 0 || sc.s.pl[0].graveN != 0 || sc.s.pl[0].handN != h0 {
		t.Fatalf("after P1's answer: graveyards %d/%d, hand 0 %d, want 0/0 and %d (P0 not asked yet)",
			sc.s.pl[0].graveN, sc.s.pl[1].graveN, sc.s.pl[0].handN, h0)
	}

	d0 := multiSacAsk(t, sc, 0)
	multiSacPick(t, sc, d0, 0)
	if sc.s.pl[0].graveN != 1 || sc.s.pl[1].graveN != 1 || sc.s.pl[0].handN != h0+1 {
		t.Fatalf("after both answers: graveyards %d/%d, hand 0 %d, want 1/1 and %d",
			sc.s.pl[0].graveN, sc.s.pl[1].graveN, sc.s.pl[0].handN, h0+1)
	}
}
