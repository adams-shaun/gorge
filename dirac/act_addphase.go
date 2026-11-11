package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// DB$ AddPhase (gorge effAddPhase, rules/turn.go
// extraPhaseBoundary, events/apply_turn.go foldExtraPhase).
//
// gorge keeps a queue of grants, one entry per granted phase, in creation
// order; the walk leaving step L (1) completes every consumed entry whose
// range ends at L, (2) consumes the MOST RECENTLY created pending entry
// spliced at L (or, when step 1 completed one, at the last completed entry's
// splice point) and enters its first step, (3) else resumes at the last
// completed entry's AfterStep+1, (4) else steps on. The kernel keeps the same
// queue run-length coded: genState.xph holds an entry per run of identical
// pending grants (Obeka's N upkeeps are one entry with n = N; a grant that
// matches the LAST entry's splice point and kind, when that entry has no
// phase running, joins it). The coding loses nothing: gorge picks pending
// grants from the back, and N identical grants at the back are N consecutive
// picks of an indistinguishable entry; a consumed entry keeps its place, so a
// run with one phase running is [pending x n-1, consumed] in gorge and
// {n-1, consumed} here, completion clearing the bit and keeping the n-1 in
// place. A fifth distinct entry is a loud stall.
//
// xph[i].at: bits 0-3 the splice point (a Step), bits 4-5 the kind
// (kop.AddPhaseCombat/Upkeep/End/CombatMain), bit 6 a phase of this entry is
// running, bit 7 (CombatMain only) the entry's additional main phase is the
// one running. xph[i].n: the grants of the entry still pending.
//
// kop.AddPhaseCombatMain (FollowedBy$ Main2, Relentless Assault): an extra
// combat followed by an additional main phase, splice point the main phase it
// resolves in. Cast in the second main phase the combat resumes at that main
// phase and the ordinary walk takes the end step after it. Cast in the first,
// gorge (extraPhaseBoundary, followedByMainBeforeCombat) splices the main
// phase as a one-step grant that resumes at the ordinary combat; here the
// entry stays queued with bit 7 set for the main phase and resumes at its
// splice point's successor. The kind holds the queue to its own entry: any
// other grant beside it is a loud stall (the interleavings of gorge's
// scan are not reproduced).
//
// The walk is advance's alone (as gorge's is advanceStep's tail): the jumps
// outside it (declare attackers with no attacker, regular combat damage to
// end of combat, first strike to regular damage) skip it, as gorge's do.
// phCnt counts the combats (low nibble) and end steps (high nibble) begun
// this turn, saturating at 15, as the StepChange fold does.

const (
	xphConsumed = 1 << 6
	xphFollow   = 1 << 7 // a CombatMain entry past its combat: its additional main phase is the one running
	xphAtMask   = 0x3f   // splice point and kind
)

// StallAddPhase: a fifth distinct pending extra-phase entry, or a run of more
// than 255 identical grants (genState.xph).
const StallAddPhase = 131

func init() { stallNames[StallAddPhase] = "addphase" }

// DebugAddPhaseResumeWrong (TestAddphaseMutation): a completed extra phase
// resumes at its own last step's successor instead of the splice point's.
var DebugAddPhaseResumeWrong bool

// prepAddPhase reads the table: some program is ExtAddPhase, some trigger is
// Tf3FirstAttack, or some expression reads the phase ledger.
func prepAddPhase(t *kop.Table, g *genPrep) {
	for i := range t.Exprs {
		if t.Exprs[i].Kind == kop.ExPhaseCount {
			g.anyAddPhase = true
		}
	}
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			a := &t.Cards[i].Abilities[j]
			if a.Kind == kop.AbTriggered && a.Trig.Flags3&kop.Tf3FirstAttack != 0 {
				g.anyAddPhase = true
			}
			for _, in := range a.Prog {
				if in.Op == kop.OpExt && in.Ext() == kop.ExtAddPhase {
					g.anyAddPhase = true
				}
			}
		}
	}
}

// addPhaseUses reports the arena mechanic a Tf3FirstAttack trigger reads: the
// attackers declared this turn (arenaUses).
func addPhaseUses(t *kop.Table, u *[numMech]bool) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			if a := &t.Cards[i].Abilities[j]; a.Kind == kop.AbTriggered && a.Trig.Flags3&kop.Tf3FirstAttack != 0 {
				u[mechAttackedObj] = true
			}
		}
	}
}

// addPhaseEntry and addPhaseEnd are an extra phase kind's first and last step.
func addPhaseEntry(kind uint8) kop.Step {
	switch kind {
	case kop.AddPhaseUpkeep:
		return kop.StepUpkeep
	case kop.AddPhaseEnd:
		return kop.StepEnd
	}
	return kop.StepBeginCombat
}

func addPhaseEnd(at uint8) kop.Step {
	if at&xphFollow != 0 {
		return kop.StepMain2
	}
	switch at >> 4 & 3 {
	case kop.AddPhaseUpkeep:
		return kop.StepUpkeep
	case kop.AddPhaseEnd:
		return kop.StepEnd
	}
	return kop.StepEndCombat
}

// addPhaseGrant is ExtAddPhase: n grants (B; none for n <= 0, as gorge) of
// the kind in C's low bits, spliced after the step in C's high nibble.
func (s *State) addPhaseGrant(k *prep, in *kop.Instr) {
	n := s.value(k, in.B)
	if n <= 0 {
		return
	}
	at := uint8(in.C>>4)&15 | uint8(in.C&3)<<4
	g := &s.g
	if in.C&3 == kop.AddPhaseCombatMain {
		// the splice point is the step the grant resolves in, a main phase
		// (the lowering's ConditionPhases$ gate keeps it there)
		at = uint8(s.step) | kop.AddPhaseCombatMain<<4
		if s.step != kop.StepMain1 && s.step != kop.StepMain2 || g.nxph > 0 || n > 1 {
			s.capFull(StallAddPhase)
			return
		}
	} else {
		for i := 0; i < int(g.nxph); i++ {
			if g.xph[i].at>>4&3 == kop.AddPhaseCombatMain {
				s.capFull(StallAddPhase)
				return
			}
		}
	}
	if g.nxph > 0 && g.xph[g.nxph-1].at == at {
		e := &g.xph[g.nxph-1]
		if t := int(e.n) + n; t <= 255 {
			e.n = uint8(t)
			return
		}
		e.n = 255
		s.capFull(StallAddPhase)
		return
	}
	if int(g.nxph) == len(g.xph) || n > 255 {
		s.capFull(StallAddPhase)
		return
	}
	g.xph[g.nxph].at, g.xph[g.nxph].n = at, uint8(n)
	g.nxph++
}

// addPhaseNext is gorge's extraPhaseBoundary: the step advance enters after
// the one being left (next is the natural successor), counting the combats and
// end steps begun.
//
//go:noinline
func (s *State) addPhaseNext(next kop.Step) kop.Step {
	leaving := s.step
	if leaving == kop.StepCleanup || leaving == kop.StepAttackers && !s.declared {
		return next
	}
	g := &s.g
	completed := false
	var compAfter, resume, forced kop.Step
	for i := 0; i < int(g.nxph); {
		e := &g.xph[i]
		if e.at&xphConsumed != 0 && addPhaseEnd(e.at) == leaving {
			compAfter = kop.Step(e.at & 15)
			resume = compAfter + 1
			if DebugAddPhaseResumeWrong {
				resume = leaving + 1
			}
			if e.at>>4&3 == kop.AddPhaseCombatMain && e.at&xphFollow == 0 {
				if compAfter == kop.StepMain1 {
					// the additional main phase before the ordinary combat
					e.at |= xphFollow
					forced = kop.StepMain2
					i++
					continue
				}
				resume = kop.StepMain2
			}
			completed = true
			e.at &^= xphConsumed
			if e.n == 0 {
				copy(g.xph[i:], g.xph[i+1:g.nxph])
				g.nxph--
				g.xph[g.nxph].at, g.xph[g.nxph].n = 0, 0
				continue
			}
		}
		i++
	}
	found := forced != 0
	if found {
		next = forced
	}
	for i := int(g.nxph) - 1; i >= 0 && !found; i-- {
		e := &g.xph[i]
		sp := kop.Step(e.at & 15)
		if e.n == 0 || sp != leaving && !(completed && sp == compAfter) {
			continue
		}
		if e.at&xphConsumed != 0 {
			s.capFull(StallLowering) // a second phase of one run beginning while the first runs: not reachable by the kinds
			break
		}
		e.n--
		e.at |= xphConsumed
		next, found = addPhaseEntry(e.at>>4&3), true
		break
	}
	if !found && completed && resume <= kop.StepCleanup {
		next = resume
	}
	switch next {
	case kop.StepBeginCombat:
		if g.phCnt&15 < 15 {
			g.phCnt++
		}
	case kop.StepEnd:
		if g.phCnt>>4 < 15 {
			g.phCnt += 16
		}
	}
	return next
}

// addPhaseNewTurn ends the turn's extra phases and ledger (gorge's TurnChange
// clears the queue and the counters).
func (s *State) addPhaseNewTurn() {
	s.g.xph = [4]struct{ at, n uint8 }{}
	s.g.nxph, s.g.phCnt = 0, 0
}

// phaseCount is ExPhaseCount.
func (s *State) phaseCount(x *kop.Expr) int {
	if x.N == kop.PhaseCountCombats {
		return int(s.g.phCnt & 15)
	}
	n := int(s.g.phCnt >> 4)
	if s.step == kop.StepEnd && n > 0 {
		n-- // Forge getNumEndOfTurn() - (is(END_OF_TURN) ? 1 : 0)
	}
	return n
}

// attackedBefore reports whether o was declared as an attacker earlier this
// turn: the mark finishAttacks sets after the declaration's triggers, so on
// the first declaration it is not there yet (gorge's AttacksThisTurn == 1).
//
//go:noinline
func (s *State) attackedBefore(k *prep, o Obj) bool {
	return s.mObj(k, mechAttackedObj, o)
}
