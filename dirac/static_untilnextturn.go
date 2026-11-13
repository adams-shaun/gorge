package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Pump / PumpAll Duration$ UntilYourNextTurn (CR 611.2b): a keyword grant
// recorded as an override with the next-turn boundary (ovNT, ovNTWho), which
// dropNextTurnOv ends at the cleanup before the controller's next turn. The
// lowering is kgen's (lower_effect.go); this file only
// carries the mutation switch the oracle's lockstep test flips.

// DebugPumpNTEndsEOT is the mutation switch (dirac/oracle
// TestPumpntMutation): an OpOverride/OpOverrideAll record whose duration is
// DurUntilYourNextTurn ends at end of turn instead, so the lockstep against
// gorge must diverge once such a grant survives into the opponent's turn.
var DebugPumpNTEndsEOT bool

// pumpNTEndsEOT reports whether the mutation is on for a next-turn override.
// It is the one bool guard in ovDurFl's path.
func pumpNTEndsEOT(dur int16) bool {
	return DebugPumpNTEndsEOT && dur == kop.DurUntilYourNextTurn
}
