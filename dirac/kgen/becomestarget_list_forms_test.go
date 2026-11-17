package kgen

import "testing"

// becomestargetlistForms: the BecomesTarget shapes this arm lowers beyond
// the single-filter watcher. FirstTime$ True on the ungated Card.Self self
// line rides the kernel's per-source Tf2FirstTime latch
// (dirac/trig_counterfirst.go); a ValidTarget$ comma list of permanent
// filters lowers as the OR of the alternatives. A list naming a player
// lowers too: the kernel's targetedOne walk now queues the player half
// (dirac/cast.go, trig_becomestarget.go becameTargetPlayer) and the filter
// carries an FPlayer alternative the walk matches with castTargetsPlayer.
// Dormant Gomazoa's single player target is the spell-source arm's player
// filter, now walked rather than inert.
var becomestargetlistForms = []formCase{
	{card: "Angelic Cub", form: "FirstTime$ True on Card.Self (Tf2FirstTime latch)", lir: "first=true"},
	{card: "Fey Steed", form: "ValidTarget$ list lowers as the OR of clauses", lir: "F={types=0x2 ctrl=1 | types=0x10 ctrl=1}"},
	{card: "Silverfur Partisan", form: "ValidTarget$ list over a spell source (Instant,Sorcery)", lir: "trigger BecomesTarget"},
	// Dormant Gomazoa: the single player target of a spell source.
	{card: "Dormant Gomazoa", form: "ValidTarget$ You (single player target)", lir: "F={players=1}"},
	// Kira's Counter body reads the targeting spell/ability as its trigger
	// referent (the queue's trigSA slot, trig_becomestarget.go) and counters
	// it rather than refusing.
	{card: "Kira, Great Glass-Spinner", form: "FirstTime$ trigger with a Counter TriggeredSourceSA body", lir: "Counter obj=TriggeredSource"},
	// a list with a player alternative (Loki's Player,Permanent) lowers as
	// the OR of the permanent clause and the player alternative.
	{card: "Loki, God of Mischief", form: "ValidTarget$ Player,Permanent (player alternative)", lir: "F={players=3 | types=0x9f}"},
	// Rayne's You,Permanent list and the same player alternative arm.
	{card: "Rayne, Academy Chancellor", form: "ValidTarget$ You,Permanent.YouCtrl (player alternative)", lir: "F={players=1 | types=0x9f ctrl=1}"},
}

func TestBecomesTargetListForms(t *testing.T) { checkForms(t, becomestargetlistForms) }
