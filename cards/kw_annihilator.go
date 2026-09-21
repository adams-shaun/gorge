// Keyword expansion split out of keywords.go so that tickets touching
// different keywords stop colliding on one file. Each expander is
// registered by head; a duplicate head panics (registerKeyword).

package cards

func kwAnnihilator(f *Face, i int, k, head, param string, has func(kind, line string) bool) {
	// CR 702.86: each time this creature attacks, its defending
	// player sacrifices the stated number of permanents. The count
	// rides the Annihilator$ marker itself rather than Amount$: the
	// generated trigger is this repo's own shape (no raw corpus card
	// carries an Annihilator$ param), and keeping Amount$ off the
	// expansion leaves api:Sacrifice.Amount genuinely unread for the
	// ordinary Sacrifice lines the parameter census still labels.
	f.addKeywordTrigger(head, k, "Mode$ Attacks | ValidCard$ Card.Self | TriggerDescription$ Annihilator",
		"DB$ Sacrifice | Defined$ TriggeredDefendingPlayer | SacValid$ Permanent | Annihilator$ "+param, has)
}

func init() { registerKeyword(kwAnnihilator, "Annihilator") }
