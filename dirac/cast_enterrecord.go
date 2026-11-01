package dirac

// DebugSpentIgnoreReplaced (TestEntercastMutation/spent): the enter-time
// spend read ignores which object is entering.
var DebugSpentIgnoreReplaced bool

// DebugWasCastYourGraveAnyZone (TestEntercastMutation/grave):
// wasCastFromYourGraveyard ignores the cast zone.
var DebugWasCastYourGraveAnyZone bool

// spentOfSpell is ExCastManaSpent for a resolving spell entry e: 0 when
// another object is entering during e's resolution (gorge: a cheated-in
// permanent reads 0; the exConverge guard, mana_converge.go:144).
func (s *State) spentOfSpell(e *stackEntry) int {
	if r := s.g.replaced; r != NoObj && r != e.obj && !DebugSpentIgnoreReplaced {
		return 0
	}
	return int(e.trigAmt)
}
