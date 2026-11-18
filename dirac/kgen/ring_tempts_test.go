package kgen

import (
	"strings"
	"testing"
)

// ringTemptsCards is the class api:RingTemptsYou refused (CR 701.54): the
// twenty top cards by deck-hits.
var ringTemptsCards = []string{
	"Boromir, Warden of the Tower", "Call of the Ring", "Scroll of Isildur",
	"Frodo, Adventurous Hobbit", "One Ring to Rule Them All", "In the Darkness Bind Them",
	"Sauron, Lord of the Rings", "Sauron, the Dark Lord", "War of the Last Alliance",
	"Frodo Baggins", "Claim the Precious", "Sméagol, Helpful Guide", "Inherited Envelope",
	"Fiery Inscription", "Aragorn, Company Leader", "Slip On the Ring",
	"Soothing of Sméagol", "Ringsight", "Samwise the Stouthearted", "Birthday Escape",
}

// ringTemptsRefused maps a class card the Ring's lowering leaves refused to a
// fragment of its measured refusal reason. None of the reasons is the Ring's:
// each is a separate gap the card carries beside it.
var ringTemptsRefused = map[string]string{
	// "put your choice of a counter from among first strike, vigilance,
	// deathtouch, and lifelink": a PutCounter whose kind is chosen.
	"Aragorn, Company Leader": "PutCounter without CounterType",
	// "As long as it is your Ring-bearer, it must be blocked if able": a
	// Continuous static granting the must-be-blocked hidden keyword.
	"Frodo Baggins": `AddHiddenKeyword "CARDNAME must be blocked if able."`,
	// "Search your library for a card that shares a color with a legendary
	// creature you control": a ChangeZone ChangeType$ naming SharesColorWith.
	"Ringsight": "unknown property",
	// "Put that card onto the battlefield tapped under your control": a
	// DigUntil with GainControl$.
	"Sméagol, Helpful Guide": "unread params [GainControl]",
}

// TestLowerRingTemptsYou lowers every card in the class from the corpus and
// asserts it has no refusal, or (ringTemptsRefused) only the named reason.
func TestLowerRingTemptsYou(t *testing.T) {
	seen := 0
	for _, c := range lowerNamed(t, ringTemptsCards) {
		seen++
		want, left := ringTemptsRefused[c.Name]
		if left && len(c.Refusals) == 0 {
			t.Errorf("%s: no longer refused; drop it from ringTemptsRefused", c.Name)
		}
		for _, r := range c.Refusals {
			if strings.Contains(r.Reason, "RingTempts") {
				t.Errorf("%s: the Ring is still refused: %s", c.Name, r.Reason)
			}
			if !left || !strings.Contains(r.Reason, want) {
				t.Errorf("%s: refused: %s", c.Name, r.Reason)
			}
		}
	}
	if seen != len(ringTemptsCards) {
		t.Fatalf("lowered %d of %d cards", seen, len(ringTemptsCards))
	}
}
