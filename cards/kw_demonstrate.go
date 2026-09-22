// Keyword expansion split out of keywords.go so that tickets touching
// different keywords stop colliding on one file. Each expander is
// registered by head; a duplicate head panics (registerKeyword).

package cards

func kwDemonstrate(f *Face, i int, k, head, param string, has func(kind, line string) bool) {
	// CR 702.152: "Whenever you cast this spell, you may copy it. If you
	// do, choose an opponent to also copy it. Each copy becomes a token."
	// The trigger is the Storm/Conspire shape -- Mode$ SpellCast over the
	// card's own cast (PutOnStack, deferred to fireDeferredCastTrigger's
	// post-payment walk like every "when you cast" trigger) -- whose body
	// is the Demonstrate API (effects/demonstrate.go): the optional
	// election and the opponent choice are its own mid-resolution asks,
	// and the copies are ordinary StackCopy mints, so a creature-spell
	// copy becomes a token through the standing CR 707.10g fold when it
	// resolves (events/apply.go's Move case) -- the reminder text's
	// "each copy becomes a token". A copy is never a cast (no PutOnStack),
	// so the copy cannot re-fire the trigger.
	f.addKeywordTrigger(head, k,
		"Mode$ SpellCast | ValidCard$ Card.Self | TriggerZones$ Stack | TriggerDescription$ Demonstrate",
		"DB$ Demonstrate | Defined$ TriggeredSpellAbility", has)
}

func init() { registerKeyword(kwDemonstrate, "Demonstrate") }
