package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The wider restricted mana-fee carrier Orb of Dragonkind: "{1}, {T}: Add two
// mana in any combination of colors. Spend this mana only to cast Dragon
// spells or activate abilities of Dragons." Two Orb seats: the deck is
// Mountain-light, so gorge's payment reaches for the fee unit (its tier funds
// a fee source only once the plain lands fall short) and Orb's two restricted
// mana (the fee paid from a Mountain's mana, CR 605.3b) fund Shivan Dragon's
// {4}{R}{R} or Furnace Whelp's {2}{R}{R} and {R} pump. Castle Garenbrig is not
// in the pod: its own fee tests carry it, and a board of only Castles diverges
// on its own (the fee funded from the other Castles' {T}: Add {G}). One seat
// carries Great Hall of the Citadel's restricted Combo Any fee pair beside its
// census {T}: Add {C} (restrComboWide), and one Hydro-Channeler's free {U}
// beside its any-colour fee alternative (restrFeePairOf).
func restrictedManaFeeWideDecks() [4][]cardset.Entry {
	orb := []cardset.Entry{
		{Name: "Orb of Dragonkind", Count: 16}, {Name: "Shivan Dragon", Count: 8}, {Name: "Furnace Whelp", Count: 6},
		{Name: "Mountain", Count: 10},
	}
	// Interplanar Beacon's "{1}, {T}: Add two mana of different colors" (planeswalker
	// spells only) beside its plain {T}: Add {C}: two Beacons in the deck, a
	// Mountain-heavy build and a few Islands, so Jace Beleren's {1}{U}{U} is
	// reachable through the pair (one {U} and one other colour per tap) and a
	// pair alone never pays {U}{U}.
	beacon := []cardset.Entry{
		{Name: "Interplanar Beacon", Count: 2}, {Name: "Jace Beleren", Count: 8}, {Name: "Mountain", Count: 14},
		{Name: "Island", Count: 6}, {Name: "Shock", Count: 5}, {Name: "Hill Giant", Count: 5},
	}
	hall := []cardset.Entry{
		{Name: "Great Hall of the Citadel", Count: 16}, {Name: "Kamahl, Pit Fighter", Count: 8},
		{Name: "Isamaru, Hound of Konda", Count: 6}, {Name: "Mountain", Count: 6}, {Name: "Plains", Count: 4},
	}
	return [4][]cardset.Entry{orb, hall, beacon, hydroChannelerDeck()}
}

// hydroChannelerDeck: Hydro-Channeler's "{T}: Add {U}" and "{1}, {T}: Add one
// mana of any color" (both instant and sorcery spells only) beside Islands and
// a few Mountains, spent on Lightning Bolt and Divination: the free {U} is
// gorge's census unit, the any-colour fee alternative its cast window's (the
// fee paid from an Island's mana, CR 605.3b), one tap between them.
func hydroChannelerDeck() []cardset.Entry {
	return []cardset.Entry{
		{Name: "Hydro-Channeler", Count: 12}, {Name: "Island", Count: 12}, {Name: "Mountain", Count: 4},
		{Name: "Lightning Bolt", Count: 8}, {Name: "Divination", Count: 4},
	}
}

// restrictedManaFeeWideSeeds: seeds that spend the carriers' restricted fee
// mana (Orb by seed 7; Hydro-Channeler from the first; Interplanar Beacon's on
// 16, 22, 26, 28 and 35: the pair needs two Beacons on one board, one's
// {T}: Add {C} paying the other's {1}, which gorge's prefix search allows and a
// Mountain's {R} it does not). Games run to 120 turns so a seat reaches a
// Dragon or a planeswalker. Seeds 3, 8 and 11 cast a Bolt only if one Hydro's
// {U} could pay another's {1} fee (it cannot, CR 106.6); seed 5 casts one
// only if a summoning-sick Hydro books no census entry (CR 302.6).
var restrictedManaFeeWideSeeds = []uint64{1, 2, 3, 4, 5, 6, 7, 8, 11, 16, 22, 26, 28, 35}

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
	for _, name := range []string{"Orb of Dragonkind", "Great Hall of the Citadel", "Hydro-Channeler", "Interplanar Beacon"} {
		if sum.byName[name] == 0 {
			t.Fatalf("no %s restricted mana spent over the seeds (%v)", name, sum.byName)
		}
	}
}
