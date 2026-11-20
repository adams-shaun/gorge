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

// The one-shot grants (Step$ Untap / Draw, Phase$ BeginCombat without Start$;
// kop.SkipPhaseUntap and the rest) are one byte per seat in the cold region
// coldSkip, the bits of gorge's PhaseSkips (state.SkipUntapNext, SkipDrawNext,
// SkipCombatOnce, SkipDrawTurn, SkipCombatTurn). The walk consumes a grant where
// gorge's skipGrantedStep does: beginStep for the untap and draw steps,
// skipCombatNext for a combat phase the ordinary walk enters (an extra combat
// spliced by addPhaseNext is neither skipped by nor consumes it); beginTurn
// drops the this-turn bits of every seat.
const (
	skUntap      uint8 = 1 << iota // the seat's next untap step
	skDraw                         // its next draw step
	skCombat                       // its next ordinary combat phase
	skDrawTurn                     // its draw step, only this turn (Duration$ EndOfTurn)
	skCombatTurn                   // its next combat phase, only this turn (Duration$ NextThisTurn)
)

// coldSkip is the per-seat one-shot grant byte, or nil when the table has no
// ExtSkipPhase (genPrep.anySkipPhase). A cold region: genState has no padding
// to spare without moving the pinned fixed layout.
var coldSkip = newCold[uint8]("skipPhase", func(g *genPrep) int {
	if g.anySkipPhase {
		return MaxSeats
	}
	return 0
})

// DebugSkipOneShotIgnored (TestSkipphaseStepsMutation): a one-shot grant is
// never armed.
var DebugSkipOneShotIgnored bool

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

// skipPhaseGrant is ExtSkipPhase: player Obj skips what in.C names (the
// kop.SkipPhase* grants).
func (s *State) skipPhaseGrant(k *prep, in *kop.Instr) {
	p := s.playerRef(k, in.Obj)
	if p >= MaxSeats {
		return
	}
	var bit uint8
	switch in.C {
	case kop.SkipPhaseAllCombat:
		s.g.cskip |= 1 << p
		return
	case kop.SkipPhaseUntap:
		bit = skUntap
	case kop.SkipPhaseDraw:
		bit = skDraw
	case kop.SkipPhaseCombat:
		bit = skCombat
	case kop.SkipPhaseDrawTurn:
		bit = skDrawTurn
	case kop.SkipPhaseCombatTurn:
		bit = skCombatTurn
	}
	if c := coldSkip.of(s); c != nil && !DebugSkipOneShotIgnored {
		c[p] |= bit
	}
}

// skipPhaseTurn is gorge's TurnChange fold (advancePhaseSkips): the turn of p
// begins.
func (s *State) skipPhaseTurn(p uint8) {
	live := s.g.cskip & 0x0f // the live bits end with the turn that ended
	if live&(1<<p) != 0 && !DebugSkipPhaseNeverLive {
		live = live&^(1<<p) | 1<<(4+p)
	}
	s.g.cskip = live
	c := coldSkip.of(s)
	for i := range c {
		c[i] &^= skDrawTurn | skCombatTurn // the this-turn grants expire with the turn
	}
}

// skipGranted is gorge's skipGrantedStep: the walk is about to enter step for
// the active player, who holds a one-shot grant for it; the grant is consumed
// (the this-turn form with it) and true is returned.
func (s *State) skipGranted(step kop.Step) bool {
	c := coldSkip.of(s)
	if c == nil || s.active >= MaxSeats || s.out(s.active) {
		return false
	}
	var m uint8
	switch step {
	case kop.StepUntap:
		m = skUntap
	case kop.StepDraw:
		m = skDraw | skDrawTurn
	case kop.StepBeginCombat:
		m = skCombat | skCombatTurn
	}
	if c[s.active]&m == 0 {
		return false
	}
	c[s.active] &^= m
	return true
}

// skipUntapStep is the skipped untap step: gorge's walk jumps to the upkeep
// with no untap and no Untap event. Summoning sickness (a flag here, derived
// from the turn an object entered there) still ends, as it does in the step.
func (s *State) skipUntapStep() {
	for m := s.ctrlM[s.active]; m.any(); m = m.rest() {
		sl := m.first()
		if p := &s.perms[sl]; p.flags&pfSick != 0 {
			p.flags &^= pfSick
			s.touchSl(sl)
		}
	}
	s.step = kop.StepUpkeep
}

// skipCombatNext is gorge's skipCombatPhase: the step advance enters after the
// one it leaves, the beginning of combat becoming the postcombat main phase
// while the active player skips combats. ordinary is false when next came out
// of the extra-phase splice, which a one-combat grant does not touch.
func (s *State) skipCombatNext(next kop.Step, ordinary bool) kop.Step {
	if next == kop.StepBeginCombat && ordinary && s.skipGranted(kop.StepBeginCombat) {
		next = kop.StepMain2
	}
	if next == kop.StepBeginCombat && s.g.cskip&(1<<(4+s.active)) != 0 {
		return kop.StepMain2
	}
	return next
}
