// Keyword expansion split out of keywords.go so that tickets touching
// different keywords stop colliding on one file. Each expander is
// registered by head; a duplicate head panics (registerKeyword).

package cards

func kwAfterlife(f *Face, i int, k, head, param string, has func(kind, line string) bool) {
	// CR 702.132a: "When this creature dies, create N 1/1 white and
	// black Spirit creature tokens with flying." One ChangesZone
	// death trigger (the Undying shape: Origin$ Battlefield,
	// Destination$ Graveyard, ValidCard$ Card.Self) whose effect mints
	// the Spirit tokens from the existing wb_1_1_spirit_flying token
	// script; effToken's default owner is the resolving controller,
	// so the tokens enter under the dying creature's controller. The
	// param is a bare literal count on every corpus line (measured:
	// 11 files, values 1/2/3, no trailing fields), so it is spliced
	// in verbatim as TokenAmount$. addKeywordTrigger already guards
	// idempotency via the KeywordLine tag, so a second Link() of a
	// cached face cannot double-add the trigger.
	f.addKeywordTrigger(head, k, "Mode$ ChangesZone | Origin$ Battlefield | Destination$ Graveyard | ValidCard$ Card.Self | TriggerDescription$ Afterlife",
		"DB$ Token | TokenScript$ wb_1_1_spirit_flying | TokenAmount$ "+param, has)
}

func init() { registerKeyword(kwAfterlife, "Afterlife") }
