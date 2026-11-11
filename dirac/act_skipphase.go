package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// DB$ SkipPhase (gorge effSkipPhase, events.SkipPhase, state.Game.PhaseSkips,
// rules/turn.go skipCombatPhase; CR 614.10): the player skips every combat
// phase of their next turn (False Peace, Empty City Ruse). genState.cskip keeps
// one pending bit and one live bit per player: ExtSkipPhase sets the pending
// bit; the turn that player begins turns it into the live bit (and drops every
// other live bit, the turn that ended); advance, about to enter the beginning
// of combat on a live turn, enters the postcombat main phase instead, so the
// combat steps, the beginning-of-combat triggers and the phase ledger never
// see that combat (addPhaseNext applies it before the ledger counts). gorge
// also consumes and completes at once an extra combat granted to a live
// player; the kernel has no such case: the only lowered extra combat is
// spliced after the end of combat step, which a skipping player never reaches.

// DebugSkipPhaseNeverLive (TestSkipphaseMutation): a pending skip never goes
// live.
var DebugSkipPhaseNeverLive bool

// prepSkipPhase reads the table: some program is ExtSkipPhase.
func prepSkipPhase(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			for _, in := range t.Cards[i].Abilities[j].Prog {
				if in.Op == kop.OpExt && in.Ext() == kop.ExtSkipPhase {
					g.anySkipPhase = true
				}
			}
		}
	}
}

// skipPhaseGrant is ExtSkipPhase: player Obj skips their next turn's combats.
func (s *State) skipPhaseGrant(k *prep, in *kop.Instr) {
	if p := s.playerRef(k, in.Obj); p != 255 {
		s.g.cskip |= 1 << p
	}
}

// skipPhaseTurn is gorge's TurnChange fold (advancePhaseSkips): the turn of p
// begins.
func (s *State) skipPhaseTurn(p uint8) {
	c := s.g.cskip & 0x0f // the live bits end with the turn that ended
	if c&(1<<p) != 0 && !DebugSkipPhaseNeverLive {
		c = c&^(1<<p) | 1<<(4+p)
	}
	s.g.cskip = c
}

// skipCombatNext is gorge's skipCombatPhase: the step advance enters after the
// one it leaves, the beginning of combat becoming the postcombat main phase
// while the active player skips combats.
func (s *State) skipCombatNext(next kop.Step) kop.Step {
	if next == kop.StepBeginCombat && s.g.cskip&(1<<(4+s.active)) != 0 {
		return kop.StepMain2
	}
	return next
}
