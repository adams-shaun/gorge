package rules

import "github.com/adams-shaun/gorge/engine/effects"

// Recover (CR 702.59) needs no rules-side code of its own: the trigger is
// the ordinary ChangesZone shape (engine/cards/kw_recover.go), and the
// pay-or-else resolution is the generic UnlessCost$ gate with
// UnlessSwitched$ True (engine/effects/unless.go) driving a ChangeZone
// return, with the SubAbility$ exile branch on a decline. This init is the
// supported-primitive marker, the way engine/effects/crew.go is for
// kw:Crew.
func init() { effects.RegisterNonAPI("kw:Recover") }
