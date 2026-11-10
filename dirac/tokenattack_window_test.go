package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestTokenAttackingCombatWindow: TokenAttacking$ True with no trigger in
// play enters its tokens attacking only from declare attackers to end of
// combat of the minting player's own turn (gorge enteringDefenderFallback);
// anywhere else (a Branch's attacking arm forced in a main phase, the other
// player's turn) the tokens still enter, but not attacking, and no defender
// is asked.
func TestTokenAttackingCombatWindow(t *testing.T) {
	for _, c := range []struct {
		name   string
		step   kop.Step
		active uint8
		attack bool
	}{
		{"main 1", kop.StepMain1, 0, false},
		{"begin combat", kop.StepBeginCombat, 0, false},
		{"declare attackers", kop.StepAttackers, 0, true},
		{"declare blockers", kop.StepBlockers, 0, true},
		{"combat damage", kop.StepDamage, 0, true},
		{"end of combat", kop.StepEndCombat, 0, true},
		{"main 2", kop.StepMain2, 0, false},
		{"end step", kop.StepEnd, 0, false},
		{"the opponent's declare blockers", kop.StepBlockers, 1, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			sc, prog, ids := tokenAttackAskScen(t, 2)
			s := sc.s
			// a walker would make the in-window mint ask; out of the window
			// gorge asks nothing whatever the opponent controls
			sc.perm(ids[0], 1)
			s.step, s.active = c.step, c.active
			before := s.nobj
			if c.attack {
				s.exec(sc.k, &prog[0])
				if s.res.ask != raGenEnterDefender {
					t.Fatalf("in the window: ask %d, want the CR 508.4 defender ask", s.res.ask)
				}
				return
			}
			if !s.exec(sc.k, &prog[0]) || s.res.ask != raNone {
				t.Fatalf("outside the window: ask %d, want the mint done unasked", s.res.ask)
			}
			if s.nobj != before+2 {
				t.Fatalf("%d tokens made, want 2", s.nobj-before)
			}
			for m := s.occ; m.any(); m = m.rest() {
				if p := &s.perms[m.first()]; p.flags&pfAttacking != 0 {
					t.Fatalf("slot %d enters attacking outside the controller's combat", m.first())
				}
			}
		})
	}
}
