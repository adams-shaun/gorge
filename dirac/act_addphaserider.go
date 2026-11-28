package dirac

// The ExtraPhaseDelayedTrigger$ rider of DB$ AddPhase (gorge effAddPhase's
// ExtraPhaseRiders, events.Apply ExtraPhase -1; Moraug, Fury of Akoum's "at
// the beginning of that combat, untap all creatures you control"). The
// lowering queues a kop.AddPhaseHere combat (spliced after the main phase the
// grant resolves in, act_addphase.go) and registers the rider beside it as a
// dprog record of timing kop.DelayExtraCombat, flagged dpExtExtraCombat: never
// due while it waits. gorge registers the one-shot as the walk CONSUMES the
// grant and fires it at that combat's beginning of combat step; the kernel's
// walk, consuming an AddPhaseHere entry (addPhaseNext), arms the newest
// waiting record -- gorge consumes the newest pending grant, whose rider is
// the newest -- turning it into an ordinary beginning-of-combat record
// (dpExtBeginCombat, trig_delaybegincombat.go), due as the combat being
// entered begins and dropped at the turn change with the waiting ones (an
// unconsumed grant ends with the turn, its rider never registered).

// dpExtExtraCombat marks a waiting rider record (kop.DelayExtraCombat). It sits
// above the remembered-player upkeep range (dpExtRemUpkeep + a seat).
const dpExtExtraCombat uint8 = 32

// DebugExtraCombatRiderOff (TestHostedCmdrMoraugExtraCombatUntapLockstep's
// mutation): a consumed grant arms no rider, so the untap never fires.
var DebugExtraCombatRiderOff bool

// armExtraCombatRider arms the newest waiting rider (addPhaseNext, as it
// consumes an AddPhaseHere entry).
func (s *State) armExtraCombatRider() {
	if DebugExtraCombatRiderOff {
		return
	}
	for i := int(s.g.ndp) - 1; i >= 0; i-- {
		if s.g.dp[i].ext == dpExtExtraCombat {
			s.g.dp[i].ext = dpExtBeginCombat
			return
		}
	}
}
