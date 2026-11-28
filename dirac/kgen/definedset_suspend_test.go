package kgen

import (
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestDefinedSetSuspendGuard: the kernel's execSet ends its walk when one
// member's run suspends on an ask, so the builder refuses a set that can hold
// more than one member under an op whose run can ask. It keeps a one-member
// set under the same op, a multi-member set under an op that never asks, and
// the all-player Sacrifice and Discard the kernel walks on the repNext chain
// (CR 101.4). The opponents of one player pass under Draw only (setOpponentOf,
// Bounty Board); the same complement spelled Non<ref> does not, and neither
// does that set under any other asking op.
func TestDefinedSetSuspendGuard(t *testing.T) {
	const two = "Defined$ You & Opponent"             // SetController SetEachOpponent SetUnion
	const non = "Defined$ NonTriggeredCardController" // SetTriggerPlayer SetPlayersComplement
	const one = "Defined$ CardController"             // SetSelf SetControllerOf
	const opp = "Defined$ TriggeredCardOpponent"      // SetTriggerPlayer SetPlayersComplement, the exempt set
	ab := func(line string) string { return definedSetSrc("A:AB$ " + line) }
	trig := func(line string) string {
		return definedSetSrc("T:Mode$ SpellCast | ValidCard$ Card | TriggerZones$ Battlefield | Execute$ ProbeTrig",
			"SVar:ProbeTrig:DB$ "+line)
	}
	cases := []struct {
		name, src string
		op        kop.Op
		refused   bool
	}{
		{"draw, two members", ab("Draw | Cost$ T | NumCards$ 1 | " + two), kop.OpDraw, true},
		{"draw, one member", ab("Draw | Cost$ T | NumCards$ 1 | " + one), kop.OpDraw, false},
		{"draw, a complement", trig("Draw | NumCards$ 1 | " + non), kop.OpDraw, true},
		// Draw's own arm reads the bare opponent spelling; the alias is the
		// set (Bounty Board)
		{"draw, the opponent of one player", trig("Draw | NumCards$ 1 | Defined$ Player.OpponentOf TriggeredCardController"), kop.OpDraw, false},
		{"damage, the opponent of one player", trig("DealDamage | NumDmg$ 1 | " + opp), kop.OpDealDamage, true},
		{"discard another player chooses, the opponent of one player",
			trig("Discard | Mode$ RevealYouChoose | NumCards$ 1 | " + opp), kop.OpDiscard, true},
		{"may sacrifice, the opponent of one player",
			trig("Sacrifice | SacValid$ Creature | Optional$ True | " + opp), kop.OpSacrifice, true},
		{"sacrifice, the opponent of one player",
			trig("Sacrifice | SacValid$ Creature | " + opp), kop.OpSacrifice, false},
		{"damage, two members", ab("DealDamage | Cost$ T | NumDmg$ 1 | " + two), kop.OpDealDamage, true},
		{"damage, one member", ab("DealDamage | Cost$ T | NumDmg$ 1 | " + one), kop.OpDealDamage, false},
		{"discard another player chooses, two members",
			ab("Discard | Cost$ T | Mode$ RevealYouChoose | NumCards$ 1 | " + two), kop.OpDiscard, true},
		{"discard another player chooses, one member",
			ab("Discard | Cost$ T | Mode$ RevealYouChoose | NumCards$ 1 | " + one), kop.OpDiscard, false},
		{"each player discards their own pick",
			ab("Discard | Cost$ T | Mode$ TgtChoose | NumCards$ 1 | " + two), kop.OpDiscard, false},
		{"each player sacrifices",
			ab("Sacrifice | Cost$ T | SacValid$ Creature | " + two), kop.OpSacrifice, false},
		{"each player may sacrifice",
			ab("Sacrifice | Cost$ T | SacValid$ Creature | Optional$ True | " + two), kop.OpSacrifice, true},
		{"gain life parks no ask in the walk", trig("GainLife | LifeAmount$ 1 | " + non), kop.OpGainLife, false},
		{"lose life never asks", trig("LoseLife | LifeAmount$ 1 | " + non), kop.OpLoseLife, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, lir := definedSetLIR(t, tc.src)
			if len(c.Refusals) != 0 || len(c.Sets) != 1 {
				t.Fatalf("precondition: one lowered set, refusals %q\n%s", abReasons(c), lir)
			}
			set := "SET 0 SetSelf SetControllerOf"
			switch {
			case strings.Contains(tc.src, two):
				set = "SET 0 SetController SetEachOpponent SetUnion"
			case strings.Contains(tc.src, non), strings.Contains(tc.src, "Opponent"):
				set = "SET 0 SetTriggerPlayer SetPlayersComplement"
			}
			if !strings.Contains(lir, set) {
				t.Fatalf("precondition: no %q line:\n%s", set, lir)
			}
			if len(c.Abilities) != 1 || len(c.Abilities[0].Prog) != 1 || c.Abilities[0].Prog[0].Obj.Kind != rSet {
				t.Fatalf("precondition: one instruction reading the set:\n%s", lir)
			}
			pt := newPoolTables(&lowered{cards: []*lCard{c}}, nil)
			b := buildCard(pt, c)
			if tc.refused {
				want := "Defined$ set of more than one member on kop " + tc.op.String()
				if !slices.ContainsFunc(b.refusals, func(r refusal) bool { return strings.Contains(r.Reason, want) }) {
					t.Fatalf("want a refusal naming %q, got %+v\n%s", want, b.refusals, lir)
				}
				if len(pt.sets) != 0 {
					t.Errorf("a refused set was interned: %v", pt.sets)
				}
				return
			}
			if len(b.refusals) != 0 {
				t.Fatalf("refused: %+v\n%s", b.refusals, lir)
			}
			a := b.def.Abilities[0]
			if len(a.SetRefs) != 1 || a.Prog[0].Op != tc.op || a.Prog[0].Obj != kop.RefSet {
				t.Fatalf("want a %s over RefSet, got op %s obj %v SetRefs %+v", tc.op, a.Prog[0].Op, a.Prog[0].Obj, a.SetRefs)
			}
		})
	}
}
