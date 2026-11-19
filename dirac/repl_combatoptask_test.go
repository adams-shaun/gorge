package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestStaticPreventCombatOptionalAsk: a lone Optional$ shield (Battletide
// Alchemist) is asked of its controller at a combat damage event too (CR
// 616.1g, gorge replacementAskPlayer), not the damaged player; the pick
// prevents its amount and the skip option (mode ro.n) deals all of it.
func TestStaticPreventCombatOptionalAsk(t *testing.T) {
	const attackerSeat = uint8(0)
	for _, tc := range []struct {
		name string
		skip bool
		want int
	}{{"pick", false, 1}, {"skip", true, 3}} {
		t.Run(tc.name, func(t *testing.T) {
			tb := newSPTable()
			// the host sits with the attacker, so the ask's decider (its
			// controller, OptionalDecider$ You) differs from the damaged
			// player, as gorge's replacementAskPlayer poses it
			h := tb.host("Test Alchemist", kop.Repl{Kind: kop.ReplPreventDamageN, C: kop.ReplSPYou | kop.ReplSPOpp | kop.ReplSPOptional, A: 2})
			sc := tb.scen(t)
			host := sc.perm(h, attackerSeat)
			giant := sc.perm(v.HillGiant, attackerSeat)
			s := sc.s
			p := &s.perms[sc.slot(giant)]
			p.flags |= pfAttacking
			p.atk = TCode(PlayerTarget1)
			s.active, s.nAttackers = attackerSeat, 1
			life := s.pl[1].life
			if !s.combatDamage(sc.k, false) {
				t.Fatal("no ask posed: the walk dealt the event without the Optional$ line's may")
			}
			ro := &s.g.ro
			if ro.n != 1 || ro.spv&spOptionalAsk == 0 || ro.cand[0] != host || ro.p != attackerSeat || s.pl[1].life != life {
				t.Fatalf("ask: n %d spv %#x cand %v player %d, life moved %d; want the lone optional host asked of its controller, no damage yet",
					ro.n, ro.spv, ro.cand[0], ro.p, life-s.pl[1].life)
			}
			var d Decision
			s.replOrderOptions(&d)
			if d.Player != attackerSeat || d.N != 2 {
				t.Fatalf("options: player %d, %d options; want the host's controller with the line and its skip", d.Player, d.N)
			}
			pick := MakeAction(AkChoose, TCode(host), 0, 0)
			if tc.skip {
				pick = d.Options[1]
				if pick.Mode() != ro.n {
					t.Fatalf("the second option's mode %d, want the skip's %d", pick.Mode(), ro.n)
				}
			}
			s.applyReplOrder(pick)
			if s.combatDamage(sc.k, false) {
				t.Fatal("the pass asked again after the answer")
			}
			if got := int(life - s.pl[1].life); got != tc.want {
				t.Fatalf("the player took %d, want %d", got, tc.want)
			}
			if ro.n != 0 || s.g.ro.on || s.g.ro.spv != 0 {
				t.Fatalf("ask left behind: n %d on %v spv %#x", ro.n, s.g.ro.on, s.g.ro.spv)
			}
		})
	}
}
