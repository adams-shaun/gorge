package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// Spend-restricted mana from a source whose cost is more than {T} in the
// payment class its restriction names: Jeweled Lotus's "{T}, Sacrifice this:
// Add three mana of any one color. Spend this mana only to cast your
// commander" (RestrictValid$ Spell.IsCommander+YouOwn), the sacrificing
// last-resort unit kop.RestrictedSacSelfShape lowers (mana_restrictedcost.go,
// restrictedSacSelfUnit). The Constructed restrictedmana pods carry the Lotus
// but never spend it: no object there is a commander. These pods are Commander
// games, so the restriction's object path (W7IsCommander in a Commander game)
// is the one the payment meets.

// restrictedManaCommanderDecks are the commander pod's two 40-card decks. Deck
// index 0 is the seat's commander; its {W} is the only white in the deck (the
// lands make green), so Isamaru can be paid from a Jeweled Lotus's three white
// alone, forcing the restricted sacrificing unit to be spent on a commander
// cast.
func restrictedManaCommanderDecks() [2][]cardset.Entry {
	return [2][]cardset.Entry{
		{
			{Name: "Isamaru, Hound of Konda", Count: 1}, {Name: "Jeweled Lotus", Count: 4},
			{Name: "Llanowar Elves", Count: 4}, {Name: "Elvish Mystic", Count: 4},
			{Name: "Grizzly Bears", Count: 4}, {Name: "Craw Wurm", Count: 4},
			{Name: "Giant Growth", Count: 3}, {Name: "Forest", Count: 16},
		},
		{
			{Name: "Isamaru, Hound of Konda", Count: 1}, {Name: "Jeweled Lotus", Count: 4},
			{Name: "Llanowar Visionary", Count: 4}, {Name: "Elvish Archdruid", Count: 4},
			{Name: "Elvish Champion", Count: 3}, {Name: "Grizzly Bears", Count: 4},
			{Name: "Giant Growth", Count: 4}, {Name: "Forest", Count: 16},
		},
	}
}

// restrictedManaCommanderPod is a Commander pod over the restrictedmana
// fixture: every seat's deck index 0 is its commander.
type restrictedManaCommanderPod struct {
	reg   *cards.Registry
	set   cardset.Set
	decks [][]cardset.Entry
}

func newRestrictedManaCommanderPod(t *testing.T, n int) restrictedManaCommanderPod {
	t.Helper()
	all := restrictedManaCommanderDecks()
	// The corpus skip comes before the fixture table read: a fresh clone has no
	// .cards (and so no tables), and must SKIP rather than fail on RawByName.
	reg := synthReg(t)
	set, err := cardset.RawByName("restrictedmana")
	if err != nil {
		t.Fatal(err)
	}
	p := restrictedManaCommanderPod{reg: reg, set: set}
	for i := 0; i < n; i++ {
		c := 0
		for _, e := range all[i] {
			c += e.Count
		}
		if c != 40 {
			t.Fatalf("deck %d: %d cards", i, c)
		}
		p.decks = append(p.decks, all[i])
	}
	return p
}

// repro is the pod as a Repro: every seat is a Commander game and its
// commander is deck index 0.
func (p restrictedManaCommanderPod) repro(seed uint64) *Repro {
	expand := func(d []cardset.Entry) []string {
		var out []string
		for _, e := range d {
			for j := 0; j < e.Count; j++ {
				out = append(out, e.Name)
			}
		}
		return out
	}
	r := &Repro{Format: reproFormat, Decks: [2]string{"rc0", "rc1"}, Cards: [2][]string{expand(p.decks[0]), expand(p.decks[1])},
		Seed: seed, PassElision: true, Surface: "full", Commander: true}
	r.Commanders = make([]int, len(p.decks))
	for s := 2; s < len(p.decks); s++ {
		r.MoreDecks = append(r.MoreDecks, "rc"+string(rune('0'+s)))
		r.MoreCards = append(r.MoreCards, expand(p.decks[s]))
	}
	return r
}

// play plays one seed in lockstep, gorge leading, and reads back the
// restricted mana the game spent by class and by carrier name.
func (p restrictedManaCommanderPod) play(t *testing.T, seed uint64, maxTurns int32) (Result, restrictedManaCounts) {
	t.Helper()
	r := p.repro(seed)
	m, err := r.Matchup(p.reg)
	if err != nil {
		t.Fatal(err)
	}
	tbl, _, err := p.set.MatchupN(p.decks)
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.MaxDecisions, rc.MaxTurns = 8000, maxTurns
	c := restrictedManaCounts{byName: map[string]int{}}
	rc.Finish = func(g *GorgeSide) { restrictedManaScan(g, &c) }
	k, err := SetSide(p.set, tbl, rc.Config)(m, seed)
	if err != nil {
		t.Fatalf("seed %d: %v", seed, err)
	}
	defer CloseSide(k)
	lead, err := NewGorgeSide(m, seed, rc.Config, nil)
	if err != nil {
		t.Fatal(err)
	}
	return LockstepLed(m, seed, lead, k, castFirstPolicy(seed), rc), c
}

// restrictedManaCommanderSeeds are the commander pod's seeds.
var restrictedManaCommanderSeeds = []uint64{1, 2, 3, 4, 5, 6}

// TestRestrictedManaCommanderLotusLockstep plays Commander pods over the
// restrictedmana fixture: no divergence and no kernel stall, and Jeweled
// Lotus's restricted unit really pays a commander cast (byName["Jeweled
// Lotus"] > 0), in a game whose repro is a Commander game.
func TestRestrictedManaCommanderLotusLockstep(t *testing.T) {
	p := newRestrictedManaCommanderPod(t, 2)
	r := p.repro(restrictedManaCommanderSeeds[0])
	if !r.Commander || len(r.Commanders) != len(p.decks) {
		t.Fatalf("pod repro: Commander %v, %d commander indices for %d decks: want a Commander game with one index a seat",
			r.Commander, len(r.Commanders), len(p.decks))
	}
	for s := range r.Commanders {
		if r.Commanders[s] != 0 {
			t.Fatalf("seat %d commander index %d: want deck index 0", s, r.Commanders[s])
		}
	}
	sum := restrictedManaCounts{byName: map[string]int{}}
	for _, seed := range restrictedManaCommanderSeeds {
		res, c := p.play(t, seed, 40)
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
		t.Logf("seed %d: %d decisions, over %v %s; spent fee/sac/life/tapX %v by %v (Jeweled Lotus %d)",
			seed, res.Decisions, res.Over, res.Stall, c.byClass, c.byName, c.byName["Jeweled Lotus"])
	}
	if sum.byName["Jeweled Lotus"] == 0 {
		t.Fatalf("restricted mana spent (fee, sacrifice, life, tapX) = %v by %v: want Jeweled Lotus's self-sacrifice unit to pay a commander cast", sum.byClass, sum.byName)
	}
}
