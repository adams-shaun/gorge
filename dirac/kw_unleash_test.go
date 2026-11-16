package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// kw_unleash.go on a hand-built table: the printed K:Unleash poses the
// as-enters counter election on every face-up entry, "yes" enters with one
// +1/+1 counter (an entry-counter put), "no" enters plain, and a permanent
// carrying the keyword and at least one +1/+1 counter can't block while the
// counter is there (from any source: a -1/-1 counter annihilating it
// releases the gate).

// unleashScen is a scen over the fdn table plus a printed-Unleash 2/2, both
// hands emptied back into Plains-only libraries.
func unleashScen(t *testing.T) (*scen, CardID) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	plains := CardID(0)
	for i := range tbl.Cards {
		if tbl.Cards[i].Name == "Plains" {
			plains = CardID(i)
		}
	}
	if tbl.Cards[plains].Name != "Plains" {
		t.Fatal("fdn table has no Plains")
	}
	n := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{
		Name: "Test Unleasher", Types: kop.TCreature, Power: 2, Toughness: 2, Unleash: true,
	})
	if !tableHasUnleash(tbl) {
		t.Fatal("precondition: the table prints Unleash")
	}
	sc := newScenTableLib(t, tbl, plains, Config{NoElide: true})
	if !sc.k.unleash {
		t.Fatal("an Unleash table does not turn on the unleash paths")
	}
	return sc, n
}

// unleashAnswer answers the queued ask of the permanent at o: yes puts the
// +1/+1 counter on. It fails unless the next decision is that ask with
// exactly the two options.
func (sc *scen) unleashAnswer(t *testing.T, o Obj, yes bool) {
	t.Helper()
	d := sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChGenUnleash || d.N != 2 {
		t.Fatalf("unleash ask = kind %d sub %d n %d, want DkChoose/ChGenUnleash/2", d.Kind, d.Sub, d.N)
	}
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		if a.Source() != TCode(o) || a.Mode() != b2u8(i == 0) {
			t.Fatalf("unleash option %d = src %d mode %d, want src %d, mode %d order", i, a.Source(), a.Mode(), TCode(o), i)
		}
	}
	idx := 1
	if yes {
		idx = 0
	}
	sc.s.Apply(d.Options[idx])
}

func b2u8(b bool) uint8 {
	if b {
		return 1
	}
	return 0
}

// toBlockers passes priority until the blockers decision, failing on any
// other ask.
func (sc *scen) toBlockers(t *testing.T) *Decision {
	t.Helper()
	for i := 0; i < 50; i++ {
		d := sc.s.Step()
		switch d.Kind {
		case DkBlockers:
			return &d
		case DkPriority:
			sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
		default:
			t.Fatalf("decision kind %d before the blockers ask", d.Kind)
		}
	}
	t.Fatal("never reached the blockers ask")
	return nil
}

// blockersOf lists the blocker objects the blockers decision offers against
// attacker a.
func blockersOf(d *Decision, a Obj) (out []Obj) {
	for i := 0; i < int(d.N); i++ {
		if x := d.Options[i]; x.Kind() == AkBlock && Obj(x.Target()) == a {
			out = append(out, Obj(x.Source()))
		}
	}
	return out
}

// permUnleash puts card c onto seat p's battlefield and answers its as-enters
// Unleash ask (the generated ask queue holds four; gorge asks at the move, so
// each entry is answered before the next is queued).
func (sc *scen) permUnleash(t *testing.T, c CardID, p uint8, yes bool) Obj {
	t.Helper()
	o := sc.perm(c, p)
	sc.unleashAnswer(t, o, yes)
	return o
}

// TestUnleashChoiceAndBlockGate drives the choice and the gate on the
// generated pool: "yes" enters with a +1/+1 counter and cannot block, "no"
// can, and a -1/-1 counter annihilating the +1/+1 counter releases it again.
func TestUnleashChoiceAndBlockGate(t *testing.T) {
	sc, ck := unleashScen(t)
	c0 := sc.permUnleash(t, ck, 0, true)
	c1 := sc.permUnleash(t, ck, 0, false)
	c4 := sc.permUnleash(t, ck, 0, false)
	c2 := sc.permUnleash(t, ck, 1, true)
	c3 := sc.permUnleash(t, ck, 1, false)
	for o, want := range map[Obj]int{c0: 1, c1: 0, c4: 0, c2: 1, c3: 0} {
		if n := sc.s.perms[sc.slot(o)].ctrN(sc.s, int(kop.CtrP1P1)); n != want {
			t.Fatalf("cackler %d: %d +1/+1 counters, want %d", o, n, want)
		}
	}
	// Seat 0's turn: the plain c1 attacks; the countered c2 is refused as a
	// blocker, the plain c3 offered.
	sc.passUntil(t, kop.StepAttackers)
	sc.do(t, AkAttack, TCode(c1), scAny)
	sc.do(t, AkDone, scAny, scAny)
	d := sc.toBlockers(t)
	off := blockersOf(d, c1)
	if len(off) != 1 || off[0] != c3 {
		t.Fatalf("blockers against c1 = %v, want only c3 (the countered c2 must be refused)", off)
	}
	sc.s.Apply(MakeAction(AkDone, 0, 0, 0))
	// Seat 1's turn: c3 attacks; the countered c0 is refused, the plain c4
	// (c1 is still tapped from its attack) offered.
	sc.passUntil(t, kop.StepMain2)
	sc.passUntil(t, kop.StepAttackers)
	sc.do(t, AkAttack, TCode(c3), scAny)
	sc.do(t, AkDone, scAny, scAny)
	d = sc.toBlockers(t)
	off = blockersOf(d, c3)
	if len(off) != 1 || off[0] != c4 {
		t.Fatalf("blockers against c3 = %v, want only c4 (the countered c0 must be refused)", off)
	}
	sc.s.Apply(MakeAction(AkDone, 0, 0, 0))
}

// TestUnleashCounterGoneBlocksAgain is the gate's release: the +1/+1 counter
// a -1/-1 counter annihilates restores blocking.
func TestUnleashCounterGoneBlocksAgain(t *testing.T) {
	sc, ck := unleashScen(t)
	sc.s.turn, sc.s.active = 3, 1 // seat 1 attacks
	c0 := sc.permUnleash(t, ck, 0, true)
	if n := sc.s.perms[sc.slot(c0)].ctrN(sc.s, int(kop.CtrP1P1)); n != 1 {
		t.Fatalf("precondition: c0 holds %d +1/+1 counters, want 1", n)
	}
	foe := sc.permUnleash(t, ck, 1, false)
	sc.passUntil(t, kop.StepAttackers)
	sc.do(t, AkAttack, TCode(foe), scAny)
	sc.do(t, AkDone, scAny, scAny)
	if d := sc.s.Step(); d.Kind != DkPriority {
		t.Fatalf("decision kind %d, want the post-attack priority round", d.Kind)
	}
	// A -1/-1 counter annihilates the +1/+1 counter (CR 704.5f) before the
	// blockers are declared: the gate releases, and the blockers decision
	// offers c0.
	p := &sc.s.perms[sc.slot(c0)]
	p.addCtrN(sc.s, int(kop.CtrM1M1), 1)
	sc.s.annihilateCtrs(sc.slot(c0))
	if n := p.ctrN(sc.s, int(kop.CtrP1P1)); n != 0 {
		t.Fatalf("the annihilation left %d +1/+1 counters", n)
	}
	d := sc.toBlockers(t)
	if off := blockersOf(d, foe); len(off) != 1 || off[0] != c0 {
		t.Fatalf("blockers after the annihilation = %v, want c0", off)
	}
	sc.s.Apply(MakeAction(AkDone, 0, 0, 0))
}
