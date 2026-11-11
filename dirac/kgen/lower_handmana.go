package kgen

import "strings"

// handManaRefusal refuses a mana ability whose cost exiles its own source from
// the hand (Simian / Elvish Spirit Guide). gorge plays it as a manual
// "activate" at priority (CR 605.1a, 605.2a: the ability functions from the
// hand) and never prices the hand source into a cast's payment plan, while the
// kernel has no manual mana (dirac/oracle doc.go): a lowering would keep a
// creature whose only other text is unreachable.
func handManaRefusal(kind int, cost string) error {
	if kind != akMana || !strings.Contains(cost, "ExileFromHand<1/CARDNAME") {
		return nil
	}
	return refuse("exile cost %q on a hand-zone mana ability: the kernel has no manual mana (gorge only activates it by hand, never pays a cast with it)", cost)
}
