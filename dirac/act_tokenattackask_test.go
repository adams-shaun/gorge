package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// A TokenAttacking$ True mint with two or more CR 508.4 defenders asks the
// minting player which one the tokens attack, once and before the mint (gorge
// effToken's resolveAttack, chooseEnteringDefender); the answer sets every
// token's defender and the instruction then runs with it. Without a second
// candidate the tokens attack the opponent player and nothing is asked.

// tokenAttackAskScen builds the scenario: a spell whose single instruction is
// an OpCreateTokenEx minting n attacking tokens (TokAttackAsk), combat set up
// for seat 0.
func tokenAttackAskScen(t *testing.T, n int) (*scen, []kop.Instr, [3]CardID) {
	t.Helper()
	var ids [3]CardID
	sc, prog := czScenWith(t, func(tbl *kop.Table) []kop.Instr {
		ids = [3]CardID{CardID(len(tbl.Cards)), CardID(len(tbl.Cards) + 1), CardID(len(tbl.Cards) + 2)}
		tbl.Cards = append(tbl.Cards,
			kop.CardDef{Name: "Test Walker A", Types: kop.TPlaneswalker, Loyalty: 3},
			kop.CardDef{Name: "Test Walker B", Types: kop.TPlaneswalker, Loyalty: 2},
			kop.CardDef{Name: "Test Soldier", Types: kop.TCreature | kop.TToken, Power: 1, Toughness: 1})
		tbl.Exprs = append(tbl.Exprs, kop.Expr{Kind: kop.ExConst, N: int16(n)})
		amt := kop.ValueRef{K: kop.VExpr, N: int16(len(tbl.Exprs) - 1)}
		return []kop.Instr{{Op: kop.OpCreateTokenEx, Obj: kop.RefController, A: amt, C: int16(ids[2]),
			B: kop.Const(kop.TokAttacking | kop.TokAttackAsk)}}
	})
	sc.s.step = kop.StepAttackers
	sc.s.active, sc.s.prio = 0, 0
	return sc, prog, ids
}

// attackers lists the attacking tokens' defender bytes in slot order.
func tokenAttackDefenders(s *State) []uint8 {
	var out []uint8
	for m := s.occ; m.any(); m = m.rest() {
		if p := &s.perms[m.first()]; p.flags&pfAttacking != 0 && p.flags&pfTapped == 0 {
			out = append(out, p.atk)
		}
	}
	return out
}

func TestTokenAttackingAsksDefenderBeforeMint(t *testing.T) {
	// no walker: no ask, both tokens attack the opponent player
	sc, prog, _ := tokenAttackAskScen(t, 2)
	s := sc.s
	s.exec(sc.k, &prog[0])
	if s.res.ask != raNone {
		t.Fatalf("ask %d, want none with only the opponent player to attack", s.res.ask)
	}
	if got := tokenAttackDefenders(s); len(got) != 2 || TCode(got[0]) != PlayerTarget0+1 || TCode(got[1]) != PlayerTarget0+1 {
		t.Fatalf("defenders %v, want two tokens attacking the opponent player", got)
	}

	for _, pickWalker := range []bool{false, true} {
		sc, prog, ids := tokenAttackAskScen(t, 2)
		s = sc.s
		wa, _ := sc.perm(ids[0], 1), sc.perm(ids[1], 1)
		before := s.nobj
		if s.exec(sc.k, &prog[0]) {
			t.Fatalf("pickWalker=%v: the mint ran without asking", pickWalker)
		}
		if s.res.ask != raGenEnterDefender || s.res.chosen != uint8(NoSlot) || s.res.askP != 0 {
			t.Fatalf("pickWalker=%v: ask %d chosen %d askP %d, want the CR 508.4 defender ask for seat 0", pickWalker, s.res.ask, s.res.chosen, s.res.askP)
		}
		if s.nobj != before {
			t.Fatalf("pickWalker=%v: a token was created before the ask was answered", pickWalker)
		}
		var d Decision
		if !s.resolveOptionsGen(sc.k, &d) || d.Sub != ChGenEnterDefender || d.N != 3 {
			t.Fatalf("pickWalker=%v: options sub %d n %d, want the entering-defender ask with three options", pickWalker, d.Sub, d.N)
		}
		a, want := d.Options[0], uint8(PlayerTarget0+1)
		if pickWalker {
			a, want = d.Options[1], uint8(wa)
		}
		s.applyAskCzParams(sc.k, a)
		if s.res.ask != raNone || s.g.tokDefSet {
			t.Fatalf("pickWalker=%v: ask %d tokDefSet %v after the answer, want the mint done and the answer consumed", pickWalker, s.res.ask, s.g.tokDefSet)
		}
		got := tokenAttackDefenders(s)
		if len(got) != 2 || got[0] != want || got[1] != want {
			t.Fatalf("pickWalker=%v: defenders %v, want both tokens attacking %d", pickWalker, got, want)
		}
	}
}
