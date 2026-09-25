// Keyword expansion split out of keywords.go so that tickets touching
// different keywords stop colliding on one file. Each expander is
// registered by head; a duplicate head panics (registerKeyword).

package cards

func kwCipher(f *Face, i int, k, head, param string, has func(kind, line string) bool) {
	// CR 702.99a: "Cipher is a static ability that functions while the spell
	// with cipher is on the stack. 'Then you may exile this spell card
	// encoded on a creature you control.'" The word "Then" makes it a
	// reflexive triggered ability (CR 603.12) that triggers as the spell
	// resolves -- so it fires on the spell card's own stack->graveyard move,
	// NOT on the cast (a cast-time trigger would resolve BEFORE the spell;
	// the Demonstrate/Storm Shape is deliberately not this one). The
	// TriggerZones$ Stack gate is satisfied by the move's From look-back
	// (rules/trigmatch_zone.go). A spell that is countered never makes that
	// move, so no encode is offered (CR 702.99c, the counterspell ruling).
	//
	// The body is the api:Cipher primitive (effects/cipher.go): the optional
	// exile-encoded offer and the encoded association live there, and the
	// combat-damage copy trigger the association grants is rules' business
	// (rules/trigger_granted.go's checkCipherTriggers plus the queue's
	// __kwCipher: payload, exactly the granted-keyword shape Conspire and
	// Demonstrate use).
	f.addKeywordTrigger(head, k,
		"Mode$ ChangesZone | Origin$ Stack | Destination$ Graveyard | ValidCard$ Card.Self | TriggerZones$ Stack | TriggerDescription$ Cipher",
		"DB$ Cipher | Defined$ Self", has)
}

func init() { registerKeyword(kwCipher, "Cipher") }
