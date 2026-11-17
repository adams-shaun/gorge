package kgen

import "testing"

// becomestargetlistForms: the BecomesTarget shapes this arm lowers beyond
// the single-filter watcher. FirstTime$ True on the ungated Card.Self self
// line rides the kernel's per-source Tf2FirstTime latch
// (dirac/trig_counterfirst.go); a ValidTarget$ comma list of permanent
// filters lowers as the OR of the alternatives. A list naming a player
// refuses: the kernel's targetedOne walk sees object targets only
// (dirac/cast.go), so a lowered player half would silently never fire
// (becomesTargetListLockstep's PARTIAL item). Dormant Gomazoa's single
// player target predates this arm: it lowers inertly through the spell
// source arm's player filter, the pre-existing shape this ticket leaves
// alone (a refusal would strip its working untap statics).
var becomestargetlistForms = []formCase{
	{card: "Angelic Cub", form: "FirstTime$ True on Card.Self (Tf2FirstTime latch)", lir: "first=true"},
	{card: "Fey Steed", form: "ValidTarget$ list lowers as the OR of clauses", lir: "F={types=0x2 ctrl=1 | types=0x10 ctrl=1}"},
	{card: "Silverfur Partisan", form: "ValidTarget$ list over a spell source (Instant,Sorcery)", lir: "trigger BecomesTarget"},
	// Dormant Gomazoa: the single player target's pre-existing inert
	// lowering, unchanged by this ticket.
	{card: "Dormant Gomazoa", form: "ValidTarget$ You (single player target, pre-existing)", lir: "F={players=1}"},
	// Kira's trigger lowers, its body refuses: countering the targeting
	// spell or ability needs the targeting stack object as a trigger
	// referent, which the kernel queue does not carry on a non-ward
	// BecomesTarget line.
	{card: "Kira, Great Glass-Spinner", form: "FirstTime$ trigger lowers, its Counter body refuses", refused: "TriggeredSourceSA"},
	// a list with a player alternative (Loki's Player,Permanent) refuses.
	{card: "Loki, God of Mischief", form: "ValidTarget$ Player,Permanent (player alternative)", refused: "BecomesTarget"},
}

func TestBecomesTargetListForms(t *testing.T) { checkForms(t, becomestargetlistForms) }
