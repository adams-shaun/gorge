// Keyword expansion split out of keywords.go so tickets touching different
// keywords do not collide on one file.
package cards

// MeleePumpCount is the synthesized Melee pump's count: the trigger's
// fire-time player capture, read explicitly. Rules captures one player ref
// per distinct opponent attacked in the declaration (rules/melee.go) as the
// trigger's capture; the plain Remembered heads (Count$RememberedNumber)
// exclude a trigger's capture, so the body names it through effects'
// TriggeredCapturedPlayers ref. events.Apply rebuilds a granted instance's
// body from the same constant, so the printed and granted shapes agree.
const MeleePumpCount = "TriggeredCapturedPlayers$Amount"

func kwMelee(f *Face, i int, k, head, param string, has func(kind, line string) bool) {
	// CR 702.121: each instance triggers when this creature attacks. The
	// declaration-wide distinct opponent count is captured as player references
	// as the trigger's capture by rules, so it survives the trigger push and
	// stack delay; the pump reads that capture explicitly (MeleePumpCount).
	f.addKeywordTrigger(head, k, "Mode$ Attacks | ValidCard$ Card.Self | TriggerDescription$ Melee",
		"DB$ Pump | Defined$ Self | NumAtt$ "+MeleePumpCount+" | NumDef$ "+MeleePumpCount, has)
}

func init() { registerKeyword(kwMelee, "Melee") }
