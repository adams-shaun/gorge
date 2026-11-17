package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Tresserhorn's Lord, Returned's UnlessCost$ Sac<3/Creature> PayLife<3>
// Draw<3/Player.targetedBy> is ONE pay / decline election over three parts
// (kgen lower_unlesschain.go, kernel cost_unlesssacriders.go): the Sac part's
// picks, then life, the sacrifices and the draws in gorge's charge order. The
// card rides kcostTokensDecks, so TestCostTokensLockstep runs it; each
// mutation below breaks ONE part of the packed instruction and a seed must
// diverge, so no part is carried by the others.

// kcostTokensMutateSacRiders rewrites every unless instruction that carries
// the Sac riders: f receives the Sac count (A) and the packed riders and
// returns their replacements.
func kcostTokensMutateSacRiders(f func(a kop.ValueRef, life, draw int) (kop.ValueRef, int, int)) func(*kop.Table) {
	return func(tbl *kop.Table) {
		cards := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cards {
			abs := append([]kop.Ability(nil), cards[i].Abilities...)
			for j := range abs {
				prog := append([]kop.Instr(nil), abs[j].Prog...)
				for k := range prog {
					in := &prog[k]
					if (in.Op == kop.OpUnlessPaid || in.Op == kop.OpUnlessSwitched) && int(in.C&0xF) == kop.PaySac && kop.IsPaySacRidersB(in.B.N) {
						life, draw := kop.PaySacRidersFrom(in.B.N)
						a, life, draw := f(in.A, life, draw)
						in.A = a
						if n, ok := kop.PaySacRidersB(life, draw); ok {
							in.B = kop.Const(n)
						} else {
							in.B = kop.Const(0)
						}
					}
				}
				abs[j].Prog = prog
			}
			cards[i].Abilities = abs
		}
		tbl.Cards = cards
	}
}

// TestCostTokensLockstepMutationUnlessPartSacrifice: with the Sac count cut
// to one the kernel asks one pick where gorge asks three (and sacrifices one
// creature where gorge sacrifices three), so a seed must diverge.
func TestCostTokensLockstepMutationUnlessPartSacrifice(t *testing.T) {
	t.Parallel()
	mut := kcostTokensMutateSacRiders(func(a kop.ValueRef, life, draw int) (kop.ValueRef, int, int) {
		return kop.Const(1), life, draw
	})
	if n := lockstepRun(t, "costtokens", kcostTokensDecks(), 60, false, mut); n == 0 {
		t.Fatal("no seed diverged with the Sac part cut to one creature")
	}
}

// TestCostTokensLockstepMutationUnlessPartLife: with the life part dropped
// the payer keeps the 3 life gorge charges, so a seed must diverge.
func TestCostTokensLockstepMutationUnlessPartLife(t *testing.T) {
	t.Parallel()
	mut := kcostTokensMutateSacRiders(func(a kop.ValueRef, life, draw int) (kop.ValueRef, int, int) {
		return a, 0, draw
	})
	if n := lockstepRun(t, "costtokens", kcostTokensDecks(), 60, false, mut); n == 0 {
		t.Fatal("no seed diverged with the life part dropped")
	}
}

// TestCostTokensLockstepMutationUnlessPartDraw: with the draw part dropped
// the targeted opponent does not draw the three cards gorge draws for them,
// so a seed must diverge.
func TestCostTokensLockstepMutationUnlessPartDraw(t *testing.T) {
	t.Parallel()
	mut := kcostTokensMutateSacRiders(func(a kop.ValueRef, life, draw int) (kop.ValueRef, int, int) {
		return a, life, 0
	})
	if n := lockstepRun(t, "costtokens", kcostTokensDecks(), 60, false, mut); n == 0 {
		t.Fatal("no seed diverged with the draw part dropped")
	}
}
