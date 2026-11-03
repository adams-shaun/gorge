package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// Produced$ Combo ColorIdentity (CR 903.4: Arcane Signet, Command Tower, Commander's
// Sphere) adds mana of the colours of the player's commander. A game without a
// commander has none, and gorge's payment plan never counts the source: the lowered
// ability must be a source of no colour, or the kernel taps it for {C} where gorge taps
// a land. Each pair is one the kernel once played differently, with the card on the
// battlefield.
func TestCommanderIdentityManaIsNoSource(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		card string
		pair int
	}{
		{"Arcane Signet", 25},
		{"Command Tower", 9},
		{"Commander's Sphere", 191},
	} {
		t.Run(c.card, func(t *testing.T) {
			t.Parallel()
			e := &cardset.RefusedEntry{Card: c.card, Repro: cardset.RefusedRepro{Pool: "std-trc", Surface: "autopay", Seed: 1, Pair: c.pair, Policy: "random"}}
			r := openRefusedRun(t, e)
			if r.diverges(PairingAt(r.pool, c.pair, r.seed)) {
				t.Errorf("%s: std-trc pair %d diverges on a commander-identity mana source", c.card, c.pair)
			}
		})
	}
}
