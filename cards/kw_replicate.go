// Keyword expansion split out of keywords.go so that tickets touching
// different keywords stop colliding on one file. Each expander is
// registered by head; a duplicate head panics (registerKeyword).

package cards

func kwReplicate(f *Face, i int, k, head, param string, has func(kind, line string) bool) {
	// CR 702.55a: "you may pay an additional [cost] any number of
	// times as you cast this spell. If you do, copy it for each time
	// you paid its replicate cost." The cast flow poses the count ask
	// (rules/cast.go's replicateAsk, one KChoose before the payment
	// window) and records the count on the pay-time CastInfo
	// (FlagReplicated's Amount); this trigger reads Count$ReplicatePaid
	// off the cast spell, so a DECLINED replicate resolves the trigger
	// with Amount 0 and effCopySpellAbility's loop emits nothing. The
	// copies keep their targets (MayChooseTarget$), the same
	// Storm-shaped stand-in the M4 copy-target task owns.
	f.addKeywordTrigger(head, k, "Mode$ SpellCast | ValidCard$ Card.Self | TriggerZones$ Stack | TriggerDescription$ Replicate",
		"DB$ CopySpellAbility | Defined$ TriggeredSpellAbility | Amount$ Count$ReplicatePaid | MayChooseTarget$ True", has)
}

func init() { registerKeyword(kwReplicate, "Replicate") }
