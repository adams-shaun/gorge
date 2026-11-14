package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestPreventFromCompetesWithDoubler: an unblocked attacker granted
// KwPreventCombatDamageFrom (Maze of Ith's Effect) whose controller has a
// Twinflame Tyrant hits the opponent. In gorge the Tyrant's doubling and the
// Effect's prevention are two matches for the one DamageDone event, so the
// CR 616.1 order ask is posed to the opponent over both, and the event deals
// 0 whichever is picked first.
func TestPreventFromCompetesWithDoubler(t *testing.T) {
	for _, pickPrevention := range []bool{false, true} {
		deck := []cardset.Entry{{Name: "Twinflame Tyrant", Count: 1}, {Name: "Erudite Wizard", Count: 1},
			{Name: "Eager Trufflesnout", Count: 4}, {Name: "Mountain", Count: 12}}
		sc, id := replScen(t, deck)
		s := sc.s
		w := sc.perm(id["Erudite Wizard"], 0)
		ty := sc.perm(id["Twinflame Tyrant"], 0)
		s.perms[sc.slot(w)].eotKw |= kop.KwPreventCombatDamageFrom
		was := sc.k.gen.anyPrevFrom
		sc.k.gen.anyPrevFrom = true
		defer func() { sc.k.gen.anyPrevFrom = was }()

		sc.passUntil(t, kop.StepAttackers)
		sc.do(t, AkAttack, TCode(w), scAny)
		sc.do(t, AkDone, scAny, scAny)

		var asked bool
		for i := 0; i < 30; i++ {
			d := s.Step()
			switch {
			case d.Kind == DkChoose && d.Sub == ChGenReplOrder:
				if asked {
					t.Fatalf("pick %v: a second order ask", pickPrevention)
				}
				asked = true
				if d.Player != 1 || d.N != 2 {
					t.Fatalf("pick %v: ask player %d with %d options, want the opponent's two", pickPrevention, d.Player, d.N)
				}
				if s.pl[1].life != 20 {
					t.Fatalf("pick %v: life %d before the answer, want 20", pickPrevention, s.pl[1].life)
				}
				var pick *Action
				for j := 0; j < int(d.N); j++ {
					src, md := Obj(d.Options[j].Source()), d.Options[j].Mode()
					if src == ty && md != 0 || src == w && md != 1 || src != ty && src != w {
						t.Fatalf("pick %v: option %d is %d mode %d, want the Tyrant (mode 0) and the attacker's prevention (mode 1)", pickPrevention, j, src, md)
					}
					if (src == w) == pickPrevention {
						pick = &d.Options[j]
					}
				}
				if pick == nil {
					t.Fatalf("pick %v: no option to pick", pickPrevention)
				}
				s.Apply(*pick)
			case d.Kind == DkPriority:
				if !asked {
					if s.step == kop.StepDamage {
						t.Fatalf("pick %v: combat damage was dealt without the order ask", pickPrevention)
					}
					s.Apply(MakeAction(AkPass, 0, 0, 0))
					continue
				}
				if s.pl[1].life != 20 {
					t.Fatalf("pick %v: life %d after the answer, want 20 (the event is prevented)", pickPrevention, s.pl[1].life)
				}
				goto next
			default:
				t.Fatalf("pick %v: unexpected decision kind %d sub %d player %d", pickPrevention, d.Kind, d.Sub, d.Player)
			}
		}
		t.Fatalf("pick %v: no priority after the answer", pickPrevention)
	next:
	}
}
