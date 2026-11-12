// Zimone's reduction paying into {X} against gorge's X-ask ceiling.
package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

func TestXCeilingReducePaysX(t *testing.T) {
	t.Parallel()

	reg, set := kcastSetup(t)
	md := [2][]cardset.Entry{
		{{Name: "Zimone, Infinite Analyst", Count: 8}, {Name: "Ivy Elemental", Count: 12}, {Name: "Forest", Count: 10}, {Name: "Island", Count: 10}},
		{{Name: "Zimone, Infinite Analyst", Count: 8}, {Name: "Ivy Elemental", Count: 8}, {Name: "Mind Spring", Count: 4}, {Name: "Forest", Count: 10}, {Name: "Island", Count: 10}},
	}
	decks := kcastDeck(t, reg, md)
	tbl, _, _, err := set.Matchup(md[0], md[1])
	if err != nil {
		t.Fatal(err)
	}
	bad := 0
	for seed := uint64(1); seed <= 600; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, _ := NewMatchup(reg, [2]string{"a", "b"}, decks, 1)
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, eagerPolicy(seed, map[string]int{}), RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 30})
		CloseSide(k)
		if d := res.Div; d != nil {
			bad++
			if bad <= 3 {
				t.Logf("seed %d: diverged at %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	// gorge's xAsk stops at pool + graveyard + convoke credit + 1, raised by
	// the cast's composed reductions (CR 601.2f: a reduction is mana the
	// announced X can spend); the kernel's ask stops at the same ceiling
	// (cast.go bsX, the clamp lifted by XRed in the same change), so every
	// X either side offers, the other does too. 14/600 games diverged before
	// the clamp, and again after gorge raised the bound without the kernel
	// following
	if bad != 0 {
		t.Fatalf("diverged %d/600", bad)
	}
}
