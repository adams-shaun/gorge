package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// ReplBeginPhase (the printed phase-skip, gorge replacement_phase.go): a
// battlefield R:Event$ BeginPhase | Phase$ <p> | Skip$ True line (Necropotence,
// Eon Hub, Stasis). gorge's replacement pass rewrites the proposed StepChange
// into the landing step (applyBeginPhaseReplacement: skipFirstDrawStep(step+1),
// whose CR 103.8a turn-1 two-player rule drawStepSkipped also reads) and
// re-matches the landing StepChange, so a chain of skips keeps skipping with
// the step numbers strictly increasing and Cleanup is never skipped; the
// skipped StepChange never enters the log, so no Phase trigger of a skipped
// phase sees it. The kernel consults the same scan at the boundary
// (beginTurn's untap entry and advance's landing step). Every matched
// candidate only prevents the event, so gorge's CR 616.1 order ask over two
// or more of them (posePhaseOrderChoice) cannot change the outcome: the scan
// order stands in for it. ValidPlayer$ You (ReplPhYou) scopes the skip to the
// replacement controller's own turns -- the active player of the phase being
// skipped; a line without it applies every turn. An Optional$ line or one
// with a ReplaceWith$ body (Fasting) and a Hellbent gate (Gibbering Descent)
// refuse at their lowering.

// DebugBeginPhaseOff skips the boundary scan (the mutation: the kernel runs a
// phase gorge skips).
var DebugBeginPhaseOff bool

// tableHasReplKind reports whether any card's abilities carry the kind.
func tableHasReplKind(t *kop.Table, kind kop.ReplKind) bool {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			if a := &t.Cards[i].Abilities[j]; a.Kind == kop.AbReplacement && a.Repl.Kind == kind {
				return true
			}
		}
	}
	return false
}

func init() {
	genNeed(func(t *kop.Table) bool { return tableHasReplKind(t, kop.ReplBeginPhase) })
	genNeed(func(t *kop.Table) bool {
		return tableHasReplKind(t, kop.ReplLifeReduced) || tableHasReplKind(t, kop.ReplDamageMinus) ||
			tableHasReplKind(t, kop.ReplDamageSet) || tableHasReplKind(t, kop.ReplDamageInstead)
	})
}

// phaseSkipLanded is the step a skip of next lands in, before any skip of
// that one is consulted (gorge applyBeginPhaseReplacement's landing
// StepChange; skipFirstDrawStep). Cleanup is never skipped: the turn's
// 514.1/514.2 work makes the next turn begin correctly, and no corpus line
// names it (gorge bounds the emission here).
func (s *State) phaseSkipLanded(k *prep, next kop.Step) kop.Step {
	if next == kop.StepCleanup {
		return next
	}
	landed := next + 1
	if landed == kop.StepDraw && k.gen != nil && k.gen.anyBeginPhase && s.turn == 1 && s.nSeats() == 2 {
		landed = kop.StepMain1 // CR 103.8a: the first draw step does not happen
	}
	return landed
}

// phaseSkipNext rewrites the step about to begin onto the first later step no
// live skip replaces (the chain: gorge re-matches each landing StepChange).
// It returns next unchanged for a table without the kind, a landed Cleanup,
// and a mutation hook.
func (s *State) phaseSkipNext(k *prep, next kop.Step) kop.Step {
	g := k.gen
	if g == nil || !g.anyBeginPhase || DebugBeginPhaseOff {
		return next
	}
	landed := next
	for {
		skipped := false
		for m := s.statM; m.any(); m = m.rest() {
			sl := m.first()
			if g.replBits2[s.cardAt(sl)]&rb2BeginPhase == 0 || s.genLost(k, sl) || s.faceDown(k, sl) {
				continue
			}
			cd := &k.t.Cards[s.cardAt(sl)]
			for i := range cd.Abilities {
				r := &cd.Abilities[i].Repl
				if cd.Abilities[i].Kind != kop.AbReplacement || r.Kind != kop.ReplBeginPhase || r.A != int16(landed) {
					continue
				}
				if r.C&kop.ReplPhYou != 0 && s.perms[sl].ctrl != s.active {
					continue // ValidPlayer$ You: the controller's own turns only
				}
				if r.Cond != 0 && !s.condEval(k, r.Cond, evalCtx{src: s.perms[sl].obj, srcSl: sl, ctrl: s.perms[sl].ctrl}) {
					continue
				}
				landed = s.phaseSkipLanded(k, landed)
				skipped = true
				break
			}
			if skipped {
				break
			}
		}
		if !skipped {
			return landed
		}
	}
}

// beginTurnPhaseSkip is the turn's untap entry: a live skip of the untap step
// enters the upkeep at once (gorge's applyBeginPhaseReplacement on the
// StepChange beginTurn would propose). The summoning sickness the untap step
// would clear still clears: gorge clears it at the TurnChange, which ran
// before the skipped StepChange (gorge beginTurn; skipUntapTurn's loop).
func (s *State) beginTurnPhaseSkip(k *prep) {
	if k.gen == nil || !k.gen.anyBeginPhase || DebugBeginPhaseOff {
		return
	}
	if s.phaseSkipNext(k, kop.StepUntap) != kop.StepUntap {
		s.clearSickness()
		// the untap skip lands in the upkeep, which a live skip of its own
		// rewrites in turn (gorge re-matches each landing StepChange)
		s.step = s.phaseSkipNext(k, kop.StepUpkeep)
	}
}

// clearSickness is skipUntapTurn's body: the active player's tapped or
// sickened permanents lose their summoning sickness (CR 302.6's clearing at
// the untap step a skip stands in for).
func (s *State) clearSickness() {
	for m := s.ctrlM[s.active]; m.any(); m = m.rest() {
		sl := m.first()
		p := &s.perms[sl]
		if p.flags&pfSick != 0 {
			p.flags &^= pfSick
			s.touchSl(sl)
		}
	}
}
