package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// announcedTargetKept: the witness fork t left spell c without a target (the
// plan sacrificed one), yet the cast stays offered when its one target is
// announced before the mana is paid (CR 601.2c-h) and some candidate of the
// board s the witness started from survives every permanent the plan
// consumed. Confound ("Counter target spell that targets a creature") over
// Larger Than Life that targets the one Eldrazi Spawn the plan sacrifices for
// {C}: the Spawn is not Confound's target, the Larger Than Life is. A spell
// whose every candidate is consumed stays refused (Larger Than Life on the
// Spawn alone). A spell with a second target slot is read by its first slot
// alone, as gorge's survivorsCanTarget reads it (sa's first target, its
// minimum): Incremental Growth's three creatures with the Eldrazi Scion one
// of them, sacrificed for the fifth mana after it was announced, is offered
// (CR 601.2c-h; the other two stay legal targets). What the survivors must make legal is the survivor
// census targetsKept documents once, below. It mirrors gorge's
// witnessAnnouncesBeforePaying (engine/rules/potential_witness_targets.go).
func (s *State) announcedTargetKept(k *prep, t *State, c CardID, p uint8) bool {
	ab := spellAb(k, c)
	sp := tspec(k, c, ab, 0)
	if sp == nil {
		return false
	}
	if k.gen != nil && s.g.probe.kind == 0 {
		s.g.probe = probe{kind: probeSpell, src: NoSlot, card: c, ctrl: p}
		defer func() { s.g.probe = probe{} }()
	}
	return s.targetsKept(k, p, sp, t, NoObj)
}

// DebugMinTargetsOneSurvivor reads every slot's minimum as one survivor
// (targetsKept), the single-target census a "two target" spell was once judged
// by. Test-only; the min-target lockstep's mutation test sets it.
var DebugMinTargetsOneSurvivor bool

// targetsKept is gorge's survivorsCanTarget over sp's candidates on the board
// s: those the witness fork t left standing (a player is consumed by no
// payment) must still make a legal declaration beside the slot's resolved
// minimum. The census is gorge's targetChoiceFeasible (engine/rules/
// legal_targets.go) with the capacities its tail applies to a full one
// (hasTargetsMin's: the survivors come from one seat, from that many seats,
// or hold one creature type that often), not a plain count. A minimum that
// reads an unannounced X binds nothing here (gorge's targetOfferMin dynamic
// early-true): such a slot needs one survivor. self is the cast object the
// census leaves out (CR 115.5), NoObj for none.
func (s *State) targetsKept(k *prep, p uint8, sp *kop.TargetSpec, t *State, self Obj) bool {
	var buf [MaxOptions]TCode
	n := s.targetCands(k, sp, p, NoSlot, self, &buf)
	need := 1
	if sp.TFlags&kop.TgfBoundsV == 0 && int(sp.Min) > need && !DebugMinTargetsOneSurvivor {
		need = int(sp.Min)
	}
	// the survivors, compacted in place over the candidate buffer
	m := 0
	for i := 0; i < n; i++ {
		tc := buf[i]
		if isPlayerOrNone(tc) {
			buf[m] = tc
			m++
		} else if o := Obj(tc); t.objs[o].zone == s.objs[o].zone && t.objs[o].ver == s.objs[o].ver {
			buf[m] = tc
			m++
		}
	}
	if m < need {
		return false
	}
	if need == 1 {
		return true // the dynamic (or one-survivor) minimum binds nothing
	}
	return s.tgtCapsHold(k, sp, buf[:m]) // the capacities over the survivors
}
