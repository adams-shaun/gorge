package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The wider restricted mana-fee carrier Orb of Dragonkind: "{1}, {T}: Add two
// mana in any combination of colors. Spend this mana only to cast Dragon
// spells or activate abilities of Dragons." Four Orb seats: the deck is
// Mountain-light, so gorge's payment reaches for the fee unit (its tier funds
// a fee source only once the plain lands fall short) and Orb's two restricted
// mana (the fee paid from a Mountain's mana, CR 605.3b) fund Shivan Dragon's
// {4}{R}{R} or Furnace Whelp's {2}{R}{R} and {R} pump. Castle Garenbrig is not
// in the pod: its own fee tests carry it, and a board of only Castles diverges
// on its own (the fee funded from the other Castles' {T}: Add {G}).
func restrictedManaFeeWideDecks() [4][]cardset.Entry {
	orb := []cardset.Entry{
		{Name: "Orb of Dragonkind", Count: 16}, {Name: "Shivan Dragon", Count: 8}, {Name: "Furnace Whelp", Count: 6},
		{Name: "Mountain", Count: 10},
	}
	return [4][]cardset.Entry{orb, orb, orb, orb}
}

// restrictedManaFeeWideSeeds: Orb's restricted mana is spent on three of the
// eight (2, 5, 7), games run to 120 turns so a seat reaches a Dragon.
var restrictedManaFeeWideSeeds = []uint64{1, 2, 3, 4, 5, 6, 7, 8}

// newRestrictedManaFeeWidePod builds the four-deck pod.
func newRestrictedManaFeeWidePod(t *testing.T) restrictedManaPod {
	t.Helper()
	reg := synthReg(t)
	set, err := cardset.RawByName("restrictedmana")
	if err != nil {
		t.Fatal(err)
	}
	p := restrictedManaPod{reg: reg, set: set}
	for i, d := range restrictedManaFeeWideDecks() {
		c := 0
		for _, e := range d {
			c += e.Count
		}
		if c != 40 {
			t.Fatalf("deck %d: %d cards", i, c)
		}
		p.decks = append(p.decks, d)
	}
	return p
}

// TestRestrictedManaFeeWideLockstep plays four-seat pods of the wider fee
// carrier: no divergence and no stall, and Orb's restricted mana was spent
// over the seeds.
func TestRestrictedManaFeeWideLockstep(t *testing.T) {
	p := newRestrictedManaFeeWidePod(t)
	sum := restrictedManaCounts{byName: map[string]int{}}
	for _, seed := range restrictedManaFeeWideSeeds {
		res, c := p.play(t, seed, 120)
		if res.Div != nil {
			restrictedManaFail(t, seed, res)
		}
		if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("seed %d: kernel stopped: %s", seed, res.Stall)
		}
		for i := range sum.byClass {
			sum.byClass[i] += c.byClass[i]
		}
		for name, n := range c.byName {
			sum.byName[name] += n
		}
		t.Logf("seed %d: %d decisions, over %v %s; spent fee %v by %v", seed, res.Decisions, res.Over, res.Stall, c.byClass, c.byName)
	}
	t.Logf("restricted fee mana spent over the seeds: %v", sum.byName)
	if sum.byName["Orb of Dragonkind"] == 0 {
		t.Fatalf("no restricted fee mana spent over the seeds (%v)", sum.byName)
	}
}
