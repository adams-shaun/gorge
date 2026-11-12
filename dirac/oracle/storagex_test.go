package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// The storage-battery X mana ability (SubCounter<X/KIND>, Amount$ X) lowers
// as the kernel's counter-X gap for the single-colour cards; the fee-bearing
// combination shapes stay refused in kgen. Each of these cards lives in a
// tail synthetic pool, so play it in a land-only fixture: its land and
// counter-placement abilities resolve decision for decision, and the X
// ability's primitive is covered by dirac's TestStoreManaGap.
var kStorageX = []string{
	"Bottomless Vault", "Mercadian Bazaar", "Mage-Ring Network", "Icatian Store",
	"Dwarven Hold", "Haruspex", "Hollow Trees", "Rushwood Grove", "Fountain of Cho",
	"Saprazzan Cove", "Sand Silos", "Subterranean Hangar",
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
