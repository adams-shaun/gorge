package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// TestRestrictedManaFeeCormelaMirrorLockstep plays the two-seat mirror of
// Cormela, Glamour Thief ("{1}, {T}: Add {U}{B}{R}", instants and sorceries
// only) beside Divination and Lightning Bolt on the full option surface: the
// fee unit's three mana float before Bolt's target ask exactly as gorge's
// prefix search floats them, and the unit's colours count nothing in the
// remainder keys (gorge's census never lists a restricted fee ability).
func TestRestrictedManaFeeCormelaMirrorLockstep(t *testing.T) {
	p := restrictedManaFeePod(t)
	d := []cardset.Entry{
		{Name: "Cormela, Glamour Thief", Count: 12}, {Name: "Island", Count: 7}, {Name: "Swamp", Count: 7},
		{Name: "Mountain", Count: 6}, {Name: "Divination", Count: 4}, {Name: "Lightning Bolt", Count: 4},
	}
	p.decks = [][]cardset.Entry{d, d}
	fee := 0
	cormela := 0
	for _, seed := range []uint64{1, 2, 3, 4, 5, 6} {
		res, c := p.playSurface(t, seed, 40, "full")
		if res.Div != nil {
			restrictedManaFail(t, seed, res)
		}
		if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("seed %d: kernel stopped: %s", seed, res.Stall)
		}
		fee += c.byClass[rmFee]
		cormela += c.byName["Cormela, Glamour Thief"]
	}
	if fee == 0 || cormela == 0 {
		t.Fatalf("no Cormela mana was spent over the seeds: fee %d, Cormela %d", fee, cormela)
	}
}
