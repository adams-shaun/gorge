package dirac

// announcedTargetKept: the witness fork t left spell c without a target (the
// plan sacrificed one), yet the cast stays offered when its one target is
// announced before the mana is paid (CR 601.2c-h) and some candidate of the
// board s the witness started from survives every permanent the plan
// consumed. Confound ("Counter target spell that targets a creature") over
// Larger Than Life that targets the one Eldrazi Spawn the plan sacrifices for
// {C}: the Spawn is not Confound's target, the Larger Than Life is. A spell
// whose every candidate is consumed stays refused (Larger Than Life on the
// Spawn alone), as does a spell with a second target slot, which this
// census does not read. It mirrors gorge's witnessAnnouncesBeforePaying
// (engine/rules/potential_witness_targets.go).
func (s *State) announcedTargetKept(k *prep, t *State, c CardID, p uint8) bool {
	ab := spellAb(k, c)
	sp := tspec(k, c, ab, 0)
	if sp == nil || tspec(k, c, ab, 1) != nil {
		return false
	}
	if k.gen != nil && s.g.probe.kind == 0 {
		s.g.probe = probe{kind: probeSpell, src: NoSlot, card: c, ctrl: p}
		defer func() { s.g.probe = probe{} }()
	}
	var buf [MaxOptions]TCode
	n := s.targetCands(k, sp, p, NoSlot, NoObj, &buf)
	for i := 0; i < n; i++ {
		if isPlayerOrNone(buf[i]) {
			return true
		}
		o := Obj(buf[i])
		if t.objs[o].zone == s.objs[o].zone && t.objs[o].ver == s.objs[o].ver {
			return true
		}
	}
	return false
}
