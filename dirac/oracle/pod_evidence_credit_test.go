package oracle

import (
	"slices"
	"testing"
)

// TestPodEvidenceCreditsCommanderOnlyKey: Command Tower's mana ability adds one
// mana of a colour in its controller's commander colour identity (CR 903.4). A
// two-seat game with no commander never uses the key, so Evidence leaves the card
// out. A clean four-seat pod game has commanders, uses the key, and its evidence
// row credits the card: the ability-key rule is the same at four seats (OD-4
// stage 2), and a pod row needs EvidencePodMinPlays clean games.
func TestPodEvidenceCreditsCommanderOnlyKey(t *testing.T) {
	const (
		card         = "Command Tower"
		commanderKey = "f0/a0" // the colour-identity mana ability
		otherKey     = "f0/a1"
	)
	keys := []string{commanderKey, otherKey}

	// Two seats, no commander: the identity mana key never fires.
	twoSeat := coverageOf(card, EvidenceMinPlays, keys, otherKey)
	cc := twoSeat.Cards[card]
	if cc == nil {
		t.Fatalf("%s has no coverage row: %v", card, twoSeat.names())
	}
	// Preconditions: the key is in the rule's inventory and is the unused one;
	// the played key differs, so the two runs below cannot both credit.
	if !slices.Contains(cc.Keys, commanderKey) || !slices.Contains(cc.Keys, otherKey) {
		t.Fatalf("%s keys %v do not hold %q and %q", card, cc.Keys, commanderKey, otherKey)
	}
	if cc.Used[commanderKey] != 0 || cc.Used[otherKey] == 0 {
		t.Fatalf("%s uses: commander key %d, other key %d; want 0 and non-zero",
			card, cc.Used[commanderKey], cc.Used[otherKey])
	}
	if got := twoSeat.EvidenceForSeats(2); len(got) != 0 {
		t.Fatalf("two-seat evidence = %v, want none: the commander-only key is unused", got)
	}

	// Four seats: the commander's colour identity fires the key.
	pod := coverageOf(card, EvidenceMinPlays, keys, commanderKey, otherKey)
	if used := pod.Cards[card].Used[commanderKey]; used == 0 {
		t.Fatalf("pod coverage does not use %q: %v", commanderKey, pod.Cards[card].Used)
	}
	pod.Games = 1
	if got, want := pod.EvidenceForSeats(4), []string{card}; !slices.Equal(got, want) {
		t.Errorf("four-seat evidence = %v, want %v: the pod game credits the commander-only key", got, want)
	}

	// The protected floor: a pod row counts only with EvidencePodMinPlays clean
	// four-seat games.
	if EvidencePodMinPlays != 1 {
		t.Fatalf("EvidencePodMinPlays = %d, want 1", EvidencePodMinPlays)
	}
	pod.Games = EvidencePodMinPlays - 1
	if got := pod.EvidenceForSeats(4); len(got) != 0 {
		t.Errorf("four-seat evidence with %d clean games = %v, want none", pod.Games, got)
	}
}
