package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// A RefSet Draw whose members are all players is gorge's effDraw over a
// player list: every member draws, whichever member's draw poses an ask
// first (a dredge choice, CR 702.52a; a draw replacement order, CR 616.1).

// setDrawScen stands a four-seat game on a table whose one sorcery draws a
// card for each opponent of its controller (the Bounty Board set) and then
// gains its controller 100 life. extra appends cards after the sorcery; thug
// is a Dredge 4 card.
func setDrawScen(t *testing.T, extra ...kop.CardDef) (sc *scen, spell, thug CardID) {
	t.Helper()
	tb := newSPTable()
	spell = CardID(len(tb.tbl.Cards))
	tb.tbl.Cards = append(tb.tbl.Cards, kop.CardDef{Name: "Test Set Draw", Types: kop.TSorcery,
		Abilities: []kop.Ability{{Kind: kop.AbSpell, Prog: []kop.Instr{
			{Op: kop.OpDraw, Obj: kop.RefSet, A: kop.Const(1)},
			{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(100)},
		}, SetRefs: []kop.SetRef{{PC: 0, Set: 0}}}}})
	thug = CardID(len(tb.tbl.Cards))
	tb.tbl.Cards = append(tb.tbl.Cards, kop.CardDef{Name: "Test Dredge", Types: kop.TCreature, Dredge: 4})
	tb.tbl.Cards = append(tb.tbl.Cards, extra...)
	tb.tbl.Sets = []kop.SetExpr{{{Kind: kop.SetController}, {Kind: kop.SetPlayersComplement}}}
	var decks [MaxSeats][]CardID
	for p := range decks {
		d := make([]CardID, 20)
		for i := range d {
			d[i] = v.Plains
		}
		decks[p] = d
	}
	s := new(State)
	s.InitN(&tb.tbl, decks, 4, 1, Config{NoElide: true})
	s.turn, s.active, s.step, s.prio, s.passes, s.ctl = 2, 0, kop.StepMain1, 0, 0, ctlPriority
	sc = &scen{s: s, k: s.k()}
	if !sc.k.gen.anyUnion || s.nSeats() != 4 {
		t.Fatalf("precondition: a four-seat table with a RefSet subject (anyUnion %v, seats %d)", sc.k.gen.anyUnion, s.nSeats())
	}
	return sc, spell, thug
}

// resolveSetDraw puts the sorcery on the stack under seat 0 and runs its
// program to the first ask.
func resolveSetDraw(sc *scen, spell CardID) {
	s := sc.s
	o := sc.obj(spell, 0)
	s.objs[o].zone = kop.ZoneStack
	e := stackEntry{kind: seSpell, obj: o, ctrl: 0, card: spell, trigObj: NoObj}
	for i := range e.tgt {
		e.tgt[i] = NoTarget
	}
	s.push(e)
	s.res = resCtx{e: e, active: true, lastTok: NoObj, eachObj: NoObj}
	s.runProg(sc.k)
}

func setDrawHands(s *State) (h [4]int) {
	for p := range h {
		h[p] = int(s.pl[p].handN)
	}
	return h
}

// TestExecSetDrawAskRunsEveryMember: the first member's draw asks, and after
// the answers every opponent has drawn, the controller has not, and the
// program went on past the draw. Before the fix the walk ended at the first
// member's ask.
func TestExecSetDrawAskRunsEveryMember(t *testing.T) {
	// seat 1, the first member, answers a dredge ask: declined (it draws) or
	// taken (it mills four and returns the card instead of drawing).
	for _, tc := range []struct {
		name string
		mode uint8
	}{{"dredge declined", 1}, {"dredge taken", 0}} {
		t.Run(tc.name, func(t *testing.T) {
			sc, spell, thug := setDrawScen(t)
			s := sc.s
			g := sc.grave(thug, 1, 1)[0]
			if !dredgeOn(sc.k) || s.dredgeCands(sc.k, 1, nil) != 1 {
				t.Fatal("precondition: seat 1 has no dredge candidate")
			}
			hand, life, lib := setDrawHands(s), s.pl[0].life, s.pl[1].libLen()
			resolveSetDraw(sc, spell)
			d := sc.expect(t, DkChoose, ChGenBranch)
			if d.Player != 1 || d.N != 2 {
				t.Fatalf("first ask player %d options %d, want seat 1's two-way dredge ask", d.Player, d.N)
			}
			if got := setDrawHands(s); got != hand {
				t.Fatalf("precondition: hands %v moved before the first member's answer (were %v)", got, hand)
			}
			s.Apply(d.Options[tc.mode])
			if s.g.stalled != 0 {
				t.Fatalf("stalled = %d", s.g.stalled)
			}
			want := [4]int{hand[0], hand[1] + 1, hand[2] + 1, hand[3] + 1}
			if got := setDrawHands(s); got != want {
				t.Errorf("hands %v, want %v: every opponent gets its card, the controller none", got, want)
			}
			wantLib, wantZone := lib-1, kop.ZoneGraveyard
			if tc.mode == 0 {
				wantLib, wantZone = lib-4, kop.ZoneHand
			}
			if got := s.pl[1].libLen(); got != wantLib || s.objs[g].zone != wantZone {
				t.Errorf("seat 1 library %d (want %d), dredge card in zone %v (want %v)", got, wantLib, s.objs[g].zone, wantZone)
			}
			if s.pl[0].life != life+100 {
				t.Errorf("controller life %d, want %d: the program stopped at the draw", s.pl[0].life, life+100)
			}
		})
	}
	// every member's draw meets two competing draw replacements, so each
	// opponent in turn orders its own (CR 616.1).
	t.Run("draw replacement order", func(t *testing.T) {
		repl := func(op kop.Op, ref kop.Ref, n int) kop.CardDef {
			return kop.CardDef{Name: "Test Draw Replacement", Types: kop.TEnchantment,
				Abilities: []kop.Ability{{Kind: kop.AbReplacement, Repl: kop.Repl{Kind: kop.ReplDrawProgram, C: kop.ReplDrawAnyPlayer},
					Prog: []kop.Instr{{Op: op, Obj: ref, A: kop.Const(n)}}}}}
		}
		sc, spell, thug := setDrawScen(t, repl(kop.OpGainLife, kop.RefTriggerPlayer, 3), repl(kop.OpLoseLife, kop.RefTriggerPlayer, 1))
		s := sc.s
		gain, lose := sc.perm(thug+1, 0), sc.perm(thug+2, 0)
		if !sc.onBF(gain) || !sc.onBF(lose) || !drawOrderOn(sc.k) {
			t.Fatal("precondition: both draw replacements in play on a table that orders them")
		}
		var life [4]int
		for p := range life {
			life[p] = int(s.pl[p].life)
		}
		hand := setDrawHands(s)
		resolveSetDraw(sc, spell)
		for p := uint8(1); p < 4; p++ {
			d := drawOrderAsk(t, sc)
			if d.Player != p || d.N != 2 {
				t.Fatalf("ask player %d options %d, want seat %d's two-way order ask", d.Player, d.N, p)
			}
			s.applyResolveGen(sc.k, d.Options[0]) // the gain program
			if s.g.stalled != 0 {
				t.Fatalf("stalled = %d after seat %d's answer", s.g.stalled, p)
			}
		}
		for p, want := range [4]int{life[0] + 100, life[1] + 3, life[2] + 3, life[3] + 3} {
			if got := int(s.pl[p].life); got != want {
				t.Errorf("seat %d life %d, want %d", p, got, want)
			}
		}
		if got := setDrawHands(s); got != hand {
			t.Errorf("hands %v, want %v: every draw was replaced", got, hand)
		}
	})
}
