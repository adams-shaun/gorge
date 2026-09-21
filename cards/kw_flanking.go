// Keyword expansion split out of keywords.go so that tickets touching
// different keywords stop colliding on one file. Each expander is
// registered by head; a duplicate head panics (registerKeyword).

package cards

func kwFlanking(f *Face, i int, k, head, param string, has func(kind, line string) bool) {
	// CR 702.25a: whenever this creature becomes blocked by a creature
	// without flanking, that blocker gets -1/-1 until end of turn (the
	// controller of the trigger is the attacker's controller, which the
	// queue's Source/Controller already are since the source IS the
	// attacker). One instance per (attacker, non-flanking blocker) pair,
	// mirrored from Forge's CardFactoryUtil expansion (Mode$
	// AttackerBlockedByCreature | ValidBlocker$ Creature.withoutFlanking);
	// the pair hook is rules/trigger_match.go's
	// attackerBlockedByPairCandidates, the per-attacker
	// attackerBlockedCandidates shape narrowed per blocker. CR 702.25b's
	// one-trigger-per-instance is out of measured scope: all 30 corpus
	// carriers are a bare single K:Flanking.
	f.addKeywordTrigger(head, k,
		"Mode$ AttackerBlockedByCreature | ValidCard$ Card.Self | ValidBlocker$ Creature.withoutFlanking | TriggerZones$ Battlefield | TriggerDescription$ Flanking",
		"DB$ Pump | Defined$ TriggeredBlockerLKICopy | NumAtt$ -1 | NumDef$ -1", has)
}

func init() { registerKeyword(kwFlanking, "Flanking") }
