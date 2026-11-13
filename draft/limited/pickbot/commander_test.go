package pickbot_test

import (
	"fmt"
	"math/rand/v2"
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/draft/limited"
	"github.com/adams-shaun/gorge/draft/limited/build"
	"github.com/adams-shaun/gorge/draft/limited/draft"
	pb "github.com/adams-shaun/gorge/draft/limited/pickbot"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/deck"
)

// synthCommanderPacks cuts 4 seats × 3 rounds of synthetic 20-card packs
// from FDN's sheet cards: three commander-eligible cards and seventeen
// others per pack, drawn by a seeded rng (no set ships 20-card packs in the
// embedded set data yet).
func synthCommanderPacks(t *testing.T, reg *cards.Registry, seed uint64) [][][]limited.CardRef {
	t.Helper()
	var cmdrs, rest []string
	for _, n := range fdnMainSetNames(t) {
		c, ok := reg.Lookup(n)
		if !ok || len(c.Faces) == 0 || c.Faces[0].IsLand() && c.Faces[0].IsBasic() {
			continue
		}
		if deck.IsCommanderEligible(c) {
			cmdrs = append(cmdrs, n)
		} else {
			rest = append(rest, n)
		}
	}
	if len(cmdrs) < 3 {
		t.Fatalf("FDN has %d commander-eligible cards, want at least 3", len(cmdrs))
	}
	r := rand.New(rand.NewPCG(seed, 0xC0DE))
	id := 0
	next := func(name string) limited.CardRef {
		id++
		return limited.CardRef{ID: fmt.Sprintf("c%d", id), Name: name, Set: "FDN", Number: "1", Rarity: limited.Uncommon}
	}
	packs := make([][][]limited.CardRef, 4)
	for s := range packs {
		packs[s] = make([][]limited.CardRef, 3)
		for p := range packs[s] {
			for _, i := range r.Perm(len(cmdrs))[:3] {
				packs[s][p] = append(packs[s][p], next(cmdrs[i]))
			}
			for _, i := range r.Perm(len(rest))[:17] {
				packs[s][p] = append(packs[s][p], next(rest[i]))
			}
		}
	}
	return packs
}

// commanderDraft drafts one synthetic pod with the commander persona and
// builds every seat's deck.
func commanderDraft(t *testing.T, reg *cards.Registry, seed uint64, size int) ([][]limited.CardRef, []build.Deck) {
	t.Helper()
	pod, err := draft.NewPodPicks(seed, synthCommanderPacks(t, reg, seed), 2)
	if err != nil {
		t.Fatal(err)
	}
	d := &pb.CommanderDrafter{Lookup: lookupOf(reg), DeckSize: size}
	rngs := make([]*rand.Rand, 4)
	for s := range rngs {
		rngs[s] = rand.New(rand.NewPCG(seed, uint64(s)))
	}
	for !pod.Done() {
		for s := 0; s < 4; s++ {
			if v, ok := pod.Next(s); ok {
				if err := pod.Pick(s, v.PickNo, d.Pick(v, rngs[s])); err != nil {
					t.Fatalf("seat %d pick %d: %v", s, v.PickNo, err)
				}
			}
		}
	}
	b := &pb.CommanderBuilder{Lookup: lookupOf(reg), DeckSize: size}
	pools := make([][]limited.CardRef, 4)
	decks := make([]build.Deck, 4)
	for s := range pools {
		pools[s] = pod.Pool(s)
		decks[s] = b.Build("FDN", pools[s])
	}
	return pools, decks
}

// TestCommanderDraftBuildsLegalDecks drafts a seeded synthetic 20-card pod
// two picks per pass and checks every built deck: 60 cards, one commander or
// a Commander-draft pair, every card inside the commanders' colour identity,
// a 16..30 land base, and the same decks again from the same seed.
func TestCommanderDraftBuildsLegalDecks(t *testing.T) {
	reg := corpus(t)
	pools, decks := commanderDraft(t, reg, 3, 0)
	for s, d := range decks {
		if len(pools[s]) != 60 {
			t.Fatalf("seat %d drafted %d cards, want 60", s, len(pools[s]))
		}
		if err := build.ValidateCommander(pools[s], d); err != nil {
			t.Fatalf("seat %d: %v", s, err)
		}
		f := build.ToDeckFile(pools[s], d, "cmdr")
		var cmdrs []*cards.Card
		ident := uint8(0)
		for _, n := range f.Commanders {
			c := lookupCard(t, reg, n)
			cmdrs = append(cmdrs, c)
			ident |= c.ColourIdentity()
		}
		if len(cmdrs) == 2 && !deck.IsDraftCommanderPair(cmdrs[0], cmdrs[1]) {
			t.Fatalf("seat %d commanders %v are not a Commander-draft pair", s, f.Commanders)
		}
		if len(cmdrs) == 1 && !deck.IsCommanderEligible(cmdrs[0]) {
			t.Fatalf("seat %d commander %v is not eligible", s, f.Commanders)
		}
		lands := d.Total() - len(d.Main)
		for _, e := range f.Cards {
			c := lookupCard(t, reg, e.Name)
			if !c.Faces[0].IsBasic() && c.ColourIdentity()&^ident != 0 {
				t.Errorf("seat %d plays %s outside commanders %v", s, e.Name, f.Commanders)
			}
			if !c.Faces[0].IsBasic() && c.Faces[0].IsLand() {
				lands += e.Count
			}
		}
		if lands < 16 || lands > 30 {
			t.Errorf("seat %d deck has %d lands, want 16..30", s, lands)
		}
		t.Logf("seat %d: commanders %v, %d lands, basics %v", s, f.Commanders, lands, d.Basics)
	}
	_, again := commanderDraft(t, reg, 3, 0)
	if !reflect.DeepEqual(decks, again) {
		t.Fatal("the same seed built different decks")
	}
}

// TestCommanderBuilderPairsMonoColourCommanders: a pool whose two commanders
// are partnerless and mono-coloured (white and red) builds around both, the
// Commander-draft partner rule, and plays both colours.
func TestCommanderBuilderPairsMonoColourCommanders(t *testing.T) {
	reg := corpus(t)
	pool := []limited.CardRef{
		ref("w", "Isamaru, Hound of Konda", limited.Rare),
		ref("r", "Krenko, Mob Boss", limited.Rare),
	}
	var white, red int
	for _, n := range fdnMainSetNames(t) {
		c, ok := reg.Lookup(n)
		if !ok || c.Faces[0].IsLand() || deck.IsCommanderEligible(c) {
			continue
		}
		switch c.ColourIdentity() {
		case 1 << 0:
			if white < 20 {
				white++
				pool = append(pool, ref(fmt.Sprintf("w%d", white), n, limited.Common))
			}
		case 1 << 3:
			if red < 20 {
				red++
				pool = append(pool, ref(fmt.Sprintf("r%d", red), n, limited.Common))
			}
		}
	}
	d := (&pb.CommanderBuilder{Lookup: lookupOf(reg)}).Build("FDN", pool)
	if err := build.ValidateCommander(pool, d); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(d.Commanders, []string{"w", "r"}) {
		t.Fatalf("commanders = %v, want the white and red pair", d.Commanders)
	}
	if d.Basics[0] == 0 || d.Basics[3] == 0 || d.Basics[1]+d.Basics[2]+d.Basics[4] != 0 {
		t.Fatalf("basics = %v, want Plains and Mountains only", d.Basics)
	}
}

// TestCommanderBuilderDeckSize50 drafts the same seeded pod for a 50-card
// event: every deck is exactly 50 cards, commanders included, passes
// ValidateCommanderSize at 50, and its land base scales to about 14.
func TestCommanderBuilderDeckSize50(t *testing.T) {
	reg := corpus(t)
	pools, decks := commanderDraft(t, reg, 3, 50)
	for s, d := range decks {
		if d.Total() != 50 {
			t.Fatalf("seat %d deck = %d cards, want 50", s, d.Total())
		}
		if err := build.ValidateCommanderSize(pools[s], d, 50); err != nil {
			t.Fatalf("seat %d: %v", s, err)
		}
		lands := d.Total() - len(d.Main)
		for _, e := range build.ToDeckFile(pools[s], d, "cmdr").Cards {
			if c := lookupCard(t, reg, e.Name); !c.Faces[0].IsBasic() && c.Faces[0].IsLand() {
				lands += e.Count
			}
		}
		if lands < 13 || lands > 25 {
			t.Errorf("seat %d deck has %d lands, want 13..25", s, lands)
		}
		t.Logf("seat %d: commanders %v, %d lands, basics %v", s, d.Commanders, lands, d.Basics)
	}
}
