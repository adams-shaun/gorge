// Keyword expansion split out of keywords.go so that tickets touching
// different keywords stop colliding on one file. Each expander is
// registered by head; a duplicate head panics (registerKeyword).

package cards

func kwReplicate(f *Face, i int, k, head, param string, has func(kind, line string) bool) {
	// CR 702.56a: "As an additional cost to cast this spell, you may pay
	// [cost] any number of times" and "When you cast this spell, if a
	// replicate cost was paid for it, copy it for each time its replicate
	// cost was paid." The cast flow poses the count ask (engine/rules/cast.go's
	// replicateAsk, one KChoose before the payment window) and records the
	// count on the pay-time CastInfo (FlagReplicated's Amount); this trigger
	// reads Count$ReplicatePaid off the cast spell, so a DECLINED replicate
	// does not trigger (CR 603.4). The copies keep their targets
	// (MayChooseTarget$), the same Storm-shaped stand-in.
	f.addKeywordTrigger(head, k, "Mode$ SpellCast | ValidCard$ Card.Self | TriggerZones$ Stack | CheckSVar$ Count$ReplicatePaid | TriggerDescription$ Replicate",
		"DB$ CopySpellAbility | Defined$ TriggeredSpellAbility | Amount$ Count$ReplicatePaid | MayChooseTarget$ True", has)
}

func init() { registerKeyword(kwReplicate, "Replicate") }
