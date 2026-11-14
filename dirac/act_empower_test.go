package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The unit tests: OpEmpower mints the u_empower_jace token even at
// N = 0, ignores a non-token Jace, reuses an existing Jace token instead of
// minting, and (with Doubling Season's two mints) asks which token gets the
// counters.

type empCards struct {
	mint, zero, jace CardID
	jaceSub          int
}

// empPoolIndex is the pool index of the card or token named name.
func empPoolIndex(t *testing.T, name string) int {
	t.Helper()
	set := cardsettest.Set(t, "empower")
	for i, n := range set.Names() {
		if n == name {
			return i
		}
	}
	t.Fatalf("%s not in the empower pool", name)
	return -1
}

func empScen(t *testing.T) (*scen, empCards) {
	t.Helper()
	set := cardsettest.Set(t, "empower")
	tbl, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	tok := set.PoolID(empPoolIndex(t, "u_empower_jace"))
	prog := func(n int) []kop.Ability {
		return []kop.Ability{{Kind: kop.AbSpell, Prog: []kop.Instr{{
			Op: kop.OpEmpower, Obj: kop.RefController, A: kop.Const(n), B: kop.Const(1), C: int16(tok),
		}}}}
	}
	jaceSub := -1
	for i, name := range tbl.Subtypes {
		if name == "Jace" {
			jaceSub = i
		}
	}
	if jaceSub < 0 {
		t.Fatal("the empower table has no Jace subtype")
	}
	n := CardID(len(tbl.Cards))
	c := empCards{mint: n, zero: n + 1, jace: n + 2, jaceSub: jaceSub}
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Empower 2", Types: kop.TSorcery, Abilities: prog(2)},
		kop.CardDef{Name: "Test Empower 0", Types: kop.TSorcery, Abilities: prog(0)},
		kop.CardDef{Name: "Test Jace Planeswalker", Types: kop.TPlaneswalker,
			Subtypes: kop.SubMask{}.With(jaceSub)})
	return newScenTable(t, tbl, Config{NoElide: true}), c
}

// empCast casts the sorcery c from p0's hand and passes both priorities.
func (sc *scen) empCast(t *testing.T, c CardID) {
	t.Helper()
	sc.empCastObj(t, sc.hand(c, 0))
}

// empCastObj casts an object already placed in a hand.
func (sc *scen) empCastObj(t *testing.T, o Obj) {
	t.Helper()
	sc.do(t, AkCast, TCode(o), scAny)
	sc.do(t, AkPass, scAny, scAny)
	sc.do(t, AkPass, scAny, scAny)
}

// TestEmpowerMint: no Jace token -> one u_empower_jace token, named "Jace Token"
// and carrying the Jace subtype, with N loyalty counters.
func TestEmpowerMint(t *testing.T) {
	sc, c := empScen(t)
	o := sc.hand(c.mint, 0)
	before := sc.s.nobj
	sc.empCastObj(t, o)
	if sc.s.nobj != before+1 {
		t.Fatalf("%d objects created, want 1", sc.s.nobj-before)
	}
	var tok Obj
	for o := before; o < sc.s.nobj; o++ {
		tok = Obj(o)
	}
	sl := sc.slot(tok)
	if !sc.onBF(tok) {
		t.Fatal("the minted token is not on the battlefield")
	}
	if n := sc.s.perms[sl].ctrLow(int(kop.CtrLoyalty)); n != 2 {
		t.Fatalf("loyalty %d, want 2", n)
	}
	if !sc.s.hasSubGen(sc.k, sl, uint8(c.jaceSub)) {
		t.Error("the minted token lacks the Jace subtype")
	}
	set := cardsettest.Set(t, "empower")
	wantName := set.PoolID(empPoolIndex(t, "name:Jace Token"))
	if nc, ok := sc.s.FeatNameCard(tok); !ok || nc != wantName {
		t.Errorf("name card %d (%v), want the Jace Token name card %d", nc, ok, wantName)
	}
}

// TestEmpowerZeroStillMints: N = 0 still creates the token, places no
// counters, and the 0-loyalty planeswalker dies to state-based actions.
func TestEmpowerZeroStillMints(t *testing.T) {
	sc, c := empScen(t)
	o := sc.hand(c.zero, 0)
	before := sc.s.nobj
	sc.empCastObj(t, o)
	if sc.s.nobj != before+1 {
		t.Fatalf("%d objects created, want 1 (N = 0 still mints)", sc.s.nobj-before)
	}
	tok := Obj(before)
	if !sc.onBF(tok) {
		t.Fatal("the minted token is not on the battlefield before SBAs")
	}
	if n := sc.s.perms[sc.slot(tok)].ctrLow(int(kop.CtrLoyalty)); n != 0 {
		t.Fatalf("loyalty %d, want 0", n)
	}
	sc.s.sba(sc.k)
	if sc.onBF(tok) {
		t.Error("the 0-loyalty token survived the state-based actions")
	}
}

// TestEmpowerExistingTokenNoMint: an existing Jace token takes the counters
// and no second token is minted.
func TestEmpowerExistingTokenNoMint(t *testing.T) {
	sc, c := empScen(t)
	sc.empCast(t, c.mint) // mint a 2-loyalty Jace token
	tok := Obj(sc.s.nobj - 1)
	o := sc.hand(c.mint, 0)
	before := sc.s.nobj
	sc.empCastObj(t, o)
	if sc.s.nobj != before {
		t.Fatalf("%d objects created, want 0 (an existing Jace token was reused)", sc.s.nobj-before)
	}
	if n := sc.s.perms[sc.slot(tok)].ctrLow(int(kop.CtrLoyalty)); n != 4 {
		t.Fatalf("loyalty %d, want 4", n)
	}
}

// TestEmpowerIgnoresNonTokenJace: a non-token Jace planeswalker is not a
// candidate; the token is minted and takes the counters.
func TestEmpowerIgnoresNonTokenJace(t *testing.T) {
	sc, c := empScen(t)
	jace := sc.perm(c.jace, 0)
	sc.s.perms[sc.slot(jace)].addCtrN(sc.s, int(kop.CtrLoyalty), 3)
	o := sc.hand(c.mint, 0)
	before := sc.s.nobj
	sc.empCastObj(t, o)
	if sc.s.nobj != before+1 {
		t.Fatalf("%d objects created, want 1 (the non-token Jace is no candidate)", sc.s.nobj-before)
	}
	if n := sc.s.perms[sc.slot(jace)].ctrLow(int(kop.CtrLoyalty)); n != 3 {
		t.Errorf("the non-token Jace has %d loyalty counters, want 3", n)
	}
	if n := sc.s.perms[sc.slot(Obj(before))].ctrLow(int(kop.CtrLoyalty)); n != 2 {
		t.Errorf("the minted token has %d loyalty counters, want 2", n)
	}
}

// TestEmpowerPicksAmongMints: Doubling Season doubles the one mint, and the
// two tokens produce the counter_pick ask; only the answered token gets the
// counters.
func TestEmpowerPicksAmongMints(t *testing.T) {
	sc, c := empScen(t)
	set := cardsettest.Set(t, "empower")
	sc.perm(set.PoolID(empPoolIndex(t, "Doubling Season")), 0)
	o := sc.hand(c.mint, 0)
	before := sc.s.nobj
	sc.empCastObj(t, o)
	if got := sc.s.nobj - before; got != 2 {
		t.Fatalf("%d objects created, want 2 (Doubling Season)", got)
	}
	d := sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChGenBolster || d.N != 2 {
		t.Fatalf("decision kind %d sub %d with %d options, want the empower pick over 2 tokens", d.Kind, d.Sub, d.N)
	}
	first, second := Obj(before), Obj(before+1)
	sc.do(t, AkChoose, TCode(second), scAny)
	if n := sc.s.perms[sc.slot(second)].ctrLow(int(kop.CtrLoyalty)); n != 4 {
		t.Fatalf("the answered token has %d loyalty counters, want 4 (Doubling Season doubles counters too)", n)
	}
	if n := sc.s.perms[sc.slot(first)].ctrLow(int(kop.CtrLoyalty)); n != 0 {
		t.Errorf("the unpicked token has %d loyalty counters, want 0", n)
	}
}
