// Keyword expansion split out of keywords.go so that tickets touching
// different keywords stop colliding on one file. Each expander is
// registered by head; a duplicate head panics (registerKeyword).

package cards

func kwRampage(f *Face, i int, k, head, param string, has func(kind, line string) bool) {
	// CR 702.23a: "Whenever this creature becomes blocked, it gets +N/+N
	// until end of turn for each creature blocking it beyond the first."
	// The trigger is Forge's (Mode$ AttackerBlocked | ValidCard$ Card.Self,
	// the same hook Bushido's second half uses); the amount mirrors the
	// corpus's own Johtull Wurm line: the creatures blocking the triggered
	// attacker, minus one, times N, read when the trigger resolves. N is the
	// printed parameter verbatim (all 12 corpus lines are numeric) and a
	// bare line expands nothing. param arrives already trimmed.
	n := param
	if n == "" {
		return
	}
	f.setSVar("__kwRampageY", "Count$Valid Creature.blockingTriggeredAttacker/Minus.1")
	f.setSVar("__kwRampageX", "SVar$__kwRampageY/Times."+n)
	f.addKeywordTrigger(head, k,
		"Mode$ AttackerBlocked | ValidCard$ Card.Self | TriggerDescription$ Rampage "+n,
		"DB$ Pump | Defined$ Self | NumAtt$ __kwRampageX | NumDef$ __kwRampageX", has)
}

func init() { registerKeyword(kwRampage, "Rampage") }
