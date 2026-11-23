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

// The one-shot shapes (kop.SkipPhaseUntap and its siblings, the C operand of
// ExtSkipPhase): the player's next untap step, next draw step, next combat
// phase and next combat phase this turn, gorge's SkipUntapPending,
// SkipDrawPending, SkipNextCombat and SkipCombatThisTurn. One byte per seat in
// the cold region coldSkipOne (laid out only for a table whose ExtSkipPhase
// carries a one-shot shape), so State grows by nothing. The untap skip is
// consumed as the turn begins (gorge beginTurn: the turn enters upkeep, the
// untap step's untap, phasing and day/night actions do not happen; the
// summoning sickness TurnChange clears still ends), the draw skip as the draw
// step would begin (gorge skipPendingStep) and the combat ones as the
// beginning of combat would be entered, never for an extra combat phase that
// was spliced in.
const (
	skUntap   uint8 = 1 << iota // next untap step
	skDraw                      // next draw step
	skCombat                    // next combat phase
	skThisTrn                   // next combat phase, this turn only
)

// coldSkipOne is the one-shot skip bits, MaxSeats bytes.
var coldSkipOne = newCold[uint8]("skipPhaseOne", nil)

// DebugSkipUntapIgnored (TestSkipPhaseUntapMutation): a pending untap skip is
// consumed but the untap step happens anyway.
var DebugSkipUntapIgnored bool

// prepSkipPhaseOne lays the region out for a table with a one-shot ExtSkipPhase.
func prepSkipPhaseOne(t *kop.Table, p *prep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			for _, in := range t.Cards[i].Abilities[j].Prog {
				if in.Op == kop.OpExt && in.Ext() == kop.ExtSkipPhase && in.C != kop.SkipPhaseAllCombats {
					coldSkipOne.set(&p.cold, MaxSeats)
					return
				}
			}
		}
	}
}

// skipPhaseGrant is ExtSkipPhase: each player Obj names skips what C says.
func (s *State) skipPhaseGrant(k *prep, in *kop.Instr) {
	q := s.players(k, in.Obj)
	for i := uint8(0); i < q.n; i++ {
		p := q.s[i]
		if s.out(p) {
			continue // gorge grants nothing to a player who has lost
		}
		var bit uint8
		switch in.C {
		case kop.SkipPhaseAllCombats:
			s.g.cskip |= 1 << p
			continue
		case kop.SkipPhaseUntap:
			bit = skUntap
		case kop.SkipPhaseDraw:
			bit = skDraw
		case kop.SkipPhaseNextCombat:
			bit = skCombat
		case kop.SkipPhaseThisTurn:
			bit = skThisTrn
		}
		if r := coldSkipOne.of(s); bit != 0 && len(r) > int(p) {
			r[p] |= bit
		}
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
	r := coldSkipOne.of(s)
	for i := range r {
		r[i] &^= skThisTrn // a this-turn grant ends with its turn
	}
}

// skipUntapTurn is gorge's beginTurn untap skip: a pending untap skip of the
// seat whose turn has begun is consumed and the turn enters its upkeep step
// at once. The sickness the untap step would clear still clears (gorge clears
// it at TurnChange).
func (s *State) skipUntapTurn() {
	r := coldSkipOne.of(s)
	if len(r) <= int(s.active) || r[s.active]&skUntap == 0 {
		return
	}
	r[s.active] &^= skUntap
	if DebugSkipUntapIgnored {
		return
	}
	for m := s.ctrlM[s.active]; m.any(); m = m.rest() {
		sl := m.first()
		if p := &s.perms[sl]; p.flags&pfSick != 0 {
			p.flags &^= pfSick
			s.touchSl(sl)
		}
	}
	s.step = kop.StepUpkeep
	if k := s.k(); k.gen != nil && k.gen.anyBeginPhase {
		s.step = s.phaseSkipNext(k, kop.StepUpkeep) // the landing StepChange meets a printed upkeep skip (repl_beginphase.go)
	}
}

// drawStepSkipped reports whether the draw step about to begin does not happen:
// CR 103.8a skips the starting player's first draw step in a two-player game
// only (CR 103.8c: no one skips it at more), and a pending draw skip of the
// active player (gorge skipPendingStep, which runs after skipFirstDrawStep) is
// consumed.
func (s *State) drawStepSkipped(k *prep) bool {
	if s.turn == 1 && s.nSeats() == 2 {
		return true
	}
	if k.gen == nil || !k.gen.anySkipPhase {
		return false
	}
	r := coldSkipOne.of(s)
	if len(r) <= int(s.active) || r[s.active]&skDraw == 0 {
		return false
	}
	r[s.active] &^= skDraw
	return true
}

// skipCombatNext is gorge's skipCombatPhase and skipPendingStep: the step
// advance enters after the one it leaves, the beginning of combat becoming the
// postcombat main phase while the active player skips combats, or has a
// one-shot next-combat skip and the combat is the ordinary walk's (spliced:
// an extra combat phase was queued in, gorge's extraPhaseBoundary).
func (s *State) skipCombatNext(next kop.Step, spliced bool) kop.Step {
	if next != kop.StepBeginCombat {
		return next
	}
	if s.g.cskip&(1<<(4+s.active)) != 0 {
		return kop.StepMain2
	}
	if r := coldSkipOne.of(s); !spliced && len(r) > int(s.active) && r[s.active]&(skCombat|skThisTrn) != 0 {
		r[s.active] &^= skCombat | skThisTrn
		return kop.StepMain2
	}
	return next
}
