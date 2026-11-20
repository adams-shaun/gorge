package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// definedSetCards are the cards T07's Defined$ set grammar newly lowers
// (lower_definedset.go), one per grammar form the corpus reaches: each plays in
// the synthetic pool that holds it, against itself, with basics of the
// colour it needs. A pool is found by searching for the one whose table holds
// the card, so a new lowering that moves the hash cut cannot break the test.
//
//	Acolyte Hybrid               Remembered Controller (Destroy, RememberDestroyed)
//	Introduction to Annihilation Remembered Controller (ChangeZone, RememberLKI)
//	Season of the Burrow         Remembered Controller on a Charm mode (BLB)
//	Plaguecrafter                Remembered Controller complement (Non)
//	Provoke the Trolls           Remembered.<filter>
//	Ashenmoor Liege              TriggeredSourceSAController (LoseLife)
//
// Wall of Nets (Valid <filter> on a remembering Exile, kop OpExileEx) is not
// here: its first play panicked in execSet (RefSet is not served by OpExileEx),
// and kgen now refuses the shape at build (TestDefinedSetKopShape).
//
// The other forms (union, Targeted.<q>, the LKICopy and Valid zone spellings)
// lower no card the corpus builds today: an object union refuses on an op that
// is not idempotent (TestDefinedSetUnionOverlap) and a multi-object
// Targeted.<q> slot refuses (TestDefinedSetTargetedSlots); the set kernel
// itself has its own unit cover (dirac/value_seteval_test.go). dirac is
// two-seat, so there is no 4-seat pod to play.
var definedSetCards = []struct{ card, basic string }{
	{"Acolyte Hybrid", "Mountain"},
	{"Introduction to Annihilation", "Plains"},
	{"Season of the Burrow", "Plains"},
	{"Plaguecrafter", "Swamp"},
	{"Provoke the Trolls", "Mountain"},
	{"Ashenmoor Liege", "Swamp"},
}

// definedSetPool is the synthetic pool whose table holds both the card and its
// basic land, found by trying each pool's Matchup.
func definedSetPool(t *testing.T, card, basic string) string {
	t.Helper()
	deck := []cardset.Entry{{Name: card, Count: 10}, {Name: basic, Count: 30}}
	for _, name := range cardsettest.SynthNames(t) {
		if _, _, _, err := cardsettest.Synth(t, name).Matchup(deck, deck); err == nil {
			return name
		}
	}
	t.Fatalf("no synthetic pool holds %s", card)
	return ""
}

// TestDefinedSetLockstep plays every card in definedSetCards for 12 seeds, both
// autopay surfaces, and fails on the first divergence.
func TestDefinedSetLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	for _, c := range definedSetCards {
		t.Run(c.card, func(t *testing.T) {
			pool := definedSetPool(t, c.card, c.basic)
			deck := []cardset.Entry{{Name: c.card, Count: 10}, {Name: c.basic, Count: 30}}
			t.Logf("%s plays in %s", c.card, pool)
			fixtureLockstep(t, pool, [2][]cardset.Entry{deck, deck}, 12)
		})
	}
}
