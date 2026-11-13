package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// The storage-battery X mana ability (SubCounter<X/KIND>, Amount$ X) lowers
// for the single-colour cards (the fee, combination and any-colour shapes
// stay refused in kgen; see dirac/kgen/storageland_test.go). City of Shadows
// is the card the STORAGE counter-kind mapping also unlocks: its {T}: Add {C}
// for each storage counter is a plain dynamic production, not a SubCounter X
// cost.
//
// Each card lives in a tail synthetic pool. The lockstep plays its land,
// counter-placement and (City's) production abilities decision for decision.
// The X activation itself is exercised by dirac's TestStoreManaGap and
// TestStorePaysSpell, whose fixture pays a {B} spell from Bottomless Vault's
// stored counters, and by TestStorageXPaysSpellLockstep, whose real-game
// deck casts a {1}{B} spell paid by the Vault's counters.
var kStorageX = []string{
	"Bottomless Vault", "Mercadian Bazaar", "Mage-Ring Network", "Icatian Store",
	"Dwarven Hold", "Hollow Trees", "Rushwood Grove", "Fountain of Cho",
	"Saprazzan Cove", "Sand Silos", "Subterranean Hangar", "City of Shadows",
}

type storageXFixture struct {
	card  string
	pool  string
	decks [2][]cardset.Entry
}

func storageXFixtures(t *testing.T) []storageXFixture {
	t.Helper()
	pools := cardsettest.SynthNames(t)
	var out []storageXFixture
	for _, card := range kStorageX {
		pool := ""
		for _, name := range pools {
			s := cardsettest.Synth(t, name)
			if _, ok := s.Lookup(card); ok {
				pool = name
				break
			}
		}
		if pool == "" {
			t.Fatalf("%s: no synthetic pool holds it", card)
		}
		out = append(out, storageXFixture{
			card: card, pool: pool,
			decks: [2][]cardset.Entry{
				{{Name: card, Count: 40}},
				{{Name: "Plains", Count: 40}},
			},
		})
	}
	return out
}

// TestStorageXLockstep plays every newly lowered storage land in a synthetic
// pool, alternating the two option surfaces, and fails on the first
// divergence.
func TestStorageXLockstep(t *testing.T) {
	for _, f := range storageXFixtures(t) {
		t.Run(f.card, func(t *testing.T) {
			t.Logf("%s: pool %s", f.card, f.pool)
			fixtureLockstep(t, f.pool, f.decks, 10)
		})
	}
}
