// Keyword expansion split out of keywords.go so that tickets touching
// different keywords stop colliding on one file. Each expander is
// registered by head; a duplicate head panics (registerKeyword).

package cards

func kwPersist(f *Face, i int, k, head, param string, has func(kind, line string) bool) {
	// CR 702.77, the mirror of Undying: the dies-condition reads
	// -1/-1 counters off the LKI and the return grants one.
	f.addKeywordTrigger(head, k, "Mode$ ChangesZone | Origin$ Battlefield | Destination$ Graveyard | ValidCard$ Card.Self+counters_EQ0_M1M1 | TriggerDescription$ Persist",
		"DB$ ChangeZone | Defined$ TriggeredNewCardLKICopy | Origin$ Graveyard | Destination$ Battlefield | WithCountersType$ M1M1 | WithCountersAmount$ 1", has)
}

func init() { registerKeyword(kwPersist, "Persist") }
