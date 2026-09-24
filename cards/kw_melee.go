// Keyword expansion split out of keywords.go so tickets touching different
// keywords do not collide on one file.
package cards

func kwMelee(f *Face, i int, k, head, param string, has func(kind, line string) bool) {
	// CR 702.121: each instance triggers when this creature attacks. The
	// declaration-wide distinct opponent count is captured as player references
	// in Remembered by rules, so it survives the trigger push and stack delay.
	f.addKeywordTrigger(head, k, "Mode$ Attacks | ValidCard$ Card.Self | TriggerDescription$ Melee",
		"DB$ Pump | Defined$ Self | NumAtt$ Count$RememberedNumber | NumDef$ Count$RememberedNumber", has)
}

func init() { registerKeyword(kwMelee, "Melee") }
