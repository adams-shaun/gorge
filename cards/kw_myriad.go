// Keyword expansion split out of keywords.go so that tickets touching
// different keywords stop colliding on one file. Each expander is
// registered by head; a duplicate head panics (registerKeyword).

package cards

func kwMyriad(f *Face, i int, k, head, param string, has func(kind, line string) bool) {
	// CR 702.109: "Whenever this creature attacks, for each opponent other
	// than the defending player, you may create a token that's a copy of
	// this creature tapped and attacking that player." An Attacks trigger
	// whose body creates the myriad per-other-opponent token copies; the
	// token copy creation is the engine's Myriad effect (a focused
	// implementation: the copies are minted and attack the respective
	// opponent).
	f.addKeywordTrigger(head, k, "Mode$ Attacks | ValidCard$ Card.Self | Myriad$ True | TriggerDescription$ Myriad",
		"DB$ Myriad", has)
}

func init() { registerKeyword(kwMyriad, "Myriad") }
