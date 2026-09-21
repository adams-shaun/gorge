// Keyword expansion split out of keywords.go so that tickets touching
// different keywords stop colliding on one file. Each expander is
// registered by head; a duplicate head panics (registerKeyword).

package cards

func kwConspire(f *Face, i int, k, head, param string, has func(kind, line string) bool) {
	// CR 702.78a is two abilities: a static "as you cast this spell,
	// you may tap two untapped creatures you control that share a color
	// with it" (the cast flow's "conspired" offer + conspireAsk, which
	// records the tap into the pending cast) and a triggered "when you
	// do, copy it". This expansion is the second half, the Replicate
	// shape verbatim except the amount head: Count$Conspired is 1 only
	// when the tap was actually paid (the pay-time FlagConspired
	// CastInfo), so a DECLINED/plain cast resolves the trigger with
	// Amount 0 and effCopySpellAbility emits nothing. The copies keep
	// their targets (MayChooseTarget$), the same Storm-shaped stand-in
	// the M4 copy-target task owns.
	f.addKeywordTrigger(head, k,
		"Mode$ SpellCast | ValidCard$ Card.Self | TriggerZones$ Stack | TriggerDescription$ Conspire",
		"DB$ CopySpellAbility | Defined$ TriggeredSpellAbility | Amount$ Count$Conspired | MayChooseTarget$ True", has)
}

func init() { registerKeyword(kwConspire, "Conspire") }
