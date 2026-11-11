package kgen

import "strings"

// handManaRefusal refuses a mana ability whose cost exiles its own source from
// the hand (Simian / Elvish Spirit Guide; CR 605.1a, 605.2a: the ability
// functions from the hand). gorge lists the guide in a manual payment window
// (pay.ManaSourceIDs walks the hand) and the oracle's window answer takes it
// when no battlefield source makes the owed colour, while the kernel's window
// sources are battlefield slots only: an inert lowering would diverge there
// (oracle TestSpiritGuideGorgeWindow measures it).
func handManaRefusal(kind int, cost string) error {
	if kind != akMana || !strings.Contains(cost, "ExileFromHand<1/CARDNAME") {
		return nil
	}
	return refuse("exile cost %q on a hand-zone mana ability: the kernel has no hand source in its mana window (gorge lists the guide in a payment window and the oracle answers with it)", cost)
}
