package pickbot_test

import (
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/draft/limited"
	"github.com/adams-shaun/gorge/draft/limited/build"
	pb "github.com/adams-shaun/gorge/draft/limited/pickbot"
	"github.com/adams-shaun/gorge/draft/limited/setdata"
)

// builderPoolNames is a fixed 42-card FDN pool, all real cards resolved
// from the corpus: 22 white/green creatures, 15 white/green non-creatures
// rated high enough by 17Lands that the raw best-23 would carry only 13
// creatures (so the builder's creature preference is load-bearing), 5 cards
// of mana value 6+, 2 blue and 3 colourless. White-green is meant to win
// the pair scan.
var builderPoolNames = []string{
	// creatures
	"Spinner of Souls", "Scavenging Ooze", "Arahbo, the First Fang",
	"Giada, Font of Hope", "Exemplar of Light", "Skyknight Squire",
	"Cat Collector", "Sun-Blessed Healer", "Dazzling Angel", "Healer's Hawk",
	"Helpful Hunter", "Guarded Heir", "Herald of Eternal Dawn",
	"Ghalta, Primal Hunger", "Quilled Greatwurm", "Nessian Hornbeetle",
	"Reclamation Sage", "Crystal Barricade", "Savannah Lions",
	"Dauntless Veteran", "Wary Thespian", "Cathar Commando",
	// non-creatures
	"Celestial Armor", "Sylvan Scavenging", "Vivien Reid", "Day of Judgment",
	"Felling Blow", "Valkyrie's Call", "Luminous Rebuke",
	"Ajani, Caller of the Pride", "Banishing Light", "Fleeting Flight",
	"Joust Through", "Bite Down", "Make Your Move", "Overrun",
	"Preposterous Proportions",
	// colourless
	"Gleaming Barrier", "Adventuring Gear", "Burnished Hart",
	// blue (off-colour)
	"Elementalist Adept", "Brineborn Cutthroat",
}

// builderPool resolves the named pool into CardRefs with the setdata
// rarities, failing loudly on any name the corpus or the set lacks.
func builderPool(t *testing.T) []limited.CardRef {
	t.Helper()
	reg := corpus(t)
	spec, err := setdata.Load("FDN")
	if err != nil {
		t.Fatalf("setdata FDN: %v", err)
	}
	rarity := sheetRarities(t, spec)
	pool := make([]limited.CardRef, 0, len(builderPoolNames))
	for i, n := range builderPoolNames {
		lookupCard(t, reg, n)
		r, ok := rarity[n]
		if !ok {
			t.Fatalf("%q is not on any FDN sheet", n)
		}
		pool = append(pool, limited.CardRef{
			ID: "c" + itoa(i), Name: n, Set: "FDN", Number: "1", Rarity: r,
		})
	}
	if len(pool) != 42 {
		t.Fatalf("built a %d-card pool, want 42", len(pool))
	}
	return pool
}

// TestBuilderBest23TwoColourDeck builds from the fixed FDN pool and asserts
// the spec's contract: a valid 40-card deck, two-coloured (white-green
// here), at least DefaultTuning.MinCreatures creatures (the pool carries 22
// in-pair creatures, so the preference is fully satisfiable), at most 3
// cards of mana value 6+, and deterministic across calls.
func TestBuilderBest23TwoColourDeck(t *testing.T) {
	reg := corpus(t)
	pool := builderPool(t)
	b := &pb.Best23{Lookup: lookupOf(reg)}

	deck := b.Build("FDN", pool)
	if err := build.Validate(pool, deck); err != nil {
		t.Fatalf("built deck invalid: %v", err)
	}
	if total := deck.Total(); total != 40 {
		t.Fatalf("built a %d-card deck, want 40", total)
	}
	basics := 0
	for _, n := range deck.Basics {
		basics += n
	}
	if basics != 17 {
		t.Fatalf("built deck runs %d basics, want 17", basics)
	}
	if len(deck.Main) != 23 {
		t.Fatalf("built deck maindecks %d cards, want 23", len(deck.Main))
	}

	// Every maindeck card is white, green or colourless — the winning
	// colour pair — and the counts the spec caps hold.
	creatures, highMV := 0, 0
	for _, id := range deck.Main {
		name := poolName(t, pool, id)
		card := lookupCard(t, reg, name)
		mask := card.ColourIdentity()
		if mask&^(1<<0|1<<4) != 0 {
			t.Fatalf("maindeck %q has colour mask %04b, outside white-green", name, mask)
		}
		f := card.Faces[0]
		if f.IsCreature() {
			creatures++
		}
		if f.Cmc() >= 6 {
			highMV++
		}
	}
	if min := pb.DefaultTuning.MinCreatures; creatures < min {
		t.Fatalf("built deck runs %d creatures, want at least %d", creatures, min)
	}
	if highMV > 3 {
		t.Fatalf("built deck runs %d cards of mana value 6+, want at most 3", highMV)
	}

	// Deterministic: a second build is card-for-card equal.
	again := b.Build("FDN", pool)
	if len(again.Main) != len(deck.Main) {
		t.Fatalf("rebuild maindecked %d cards, want %d", len(again.Main), len(deck.Main))
	}
	for i := range deck.Main {
		if again.Main[i] != deck.Main[i] || again.Basics != deck.Basics {
			t.Fatalf("rebuild differs at card %d: %q vs %q", i, again.Main[i], deck.Main[i])
		}
	}
}

// capPoolNames is a 24-card pool for the mana-value cap: four giant
// creatures of mana value 6+ that the heuristic (set ZZZ, no rating file)
// rates above everything else, plus 20 cheap cards. Uncapped, the best 23
// carries all four giants; capped, exactly three.
var capPoolNames = []string{
	"Quilled Greatwurm", "Quakestrider Ceratops", "Ghalta, Primal Hunger",
	"Herald of Eternal Dawn",
	"Healer's Hawk", "Llanowar Elves", "Cathar Commando", "Gnarlid Colony",
	"Dwynen's Elite", "Ambush Wolf", "Wary Thespian", "Beast-Kin Ranger",
	"Ajani's Pridemate", "Dazzling Angel", "Mischievous Pup", "Prideful Parent",
	"Fleeting Flight", "Joust Through", "Bite Down", "Snakeskin Veil",
	"Adventuring Gear", "Fishing Pole", "Gleaming Barrier", "Campus Guide",
}

// TestBuilderCapsHighManaValue pins the builder's mana-value cap on the
// heuristic path: with four giants of mana value 6+ the deck plays exactly
// three, and stays a valid 40-card two-colour deck satisfying the creature
// preference.
func TestBuilderCapsHighManaValue(t *testing.T) {
	reg := corpus(t)
	pool := make([]limited.CardRef, len(capPoolNames))
	// The giants carry their real rarities: the heuristic's rarity term is
	// what lifts them above the cheap pool, so a common-defaulted pool
	// would leave the cap unexercised.
	giantRarity := map[string]limited.Rarity{
		"Quilled Greatwurm":      limited.Mythic,
		"Quakestrider Ceratops":  limited.Uncommon,
		"Ghalta, Primal Hunger":  limited.Rare,
		"Herald of Eternal Dawn": limited.Mythic,
	}
	for i, n := range capPoolNames {
		lookupCard(t, reg, n)
		pool[i] = limited.CardRef{ID: "z" + itoa(i), Name: n, Rarity: giantRarity[n]}
	}
	if len(pool) != 24 {
		t.Fatalf("built a %d-card pool, want 24", len(pool))
	}
	b := &pb.Best23{Lookup: lookupOf(reg)}
	deck := b.Build("ZZZ", pool)
	if err := build.Validate(pool, deck); err != nil {
		t.Fatalf("built deck invalid: %v", err)
	}
	if total := deck.Total(); total != 40 {
		t.Fatalf("built a %d-card deck, want 40", total)
	}
	highMV, creatures := 0, 0
	for _, id := range deck.Main {
		card := lookupCard(t, reg, poolName(t, pool, id))
		mask := card.ColourIdentity()
		if mask&^(1<<0|1<<4) != 0 {
			t.Fatalf("maindeck %q has colour mask %04b, outside white-green", card.Faces[0].Name, mask)
		}
		if card.Faces[0].Cmc() >= 6 {
			highMV++
		}
		if card.Faces[0].IsCreature() {
			creatures++
		}
	}
	if highMV != 3 {
		t.Fatalf("built deck runs %d cards of mana value 6+, want exactly 3 (the cap)", highMV)
	}
	if min := pb.DefaultTuning.MinCreatures; creatures < min {
		t.Fatalf("built deck runs %d creatures, want at least %d", creatures, min)
	}
	_ = rand.NewPCG // referenced only to keep the rng import honest for later tests
}

// TestBuilderRegistryEntries pins the registry: the two spec names are
// registered with both halves, sorted, and the registered drafter answers a
// pick through the same rated path the direct type does.
func TestBuilderRegistryEntries(t *testing.T) {
	names := pb.Names()
	if len(names) != 2 || names[0] != "best23" || names[1] != "rating-colour" {
		t.Fatalf("registry names = %v, want [best23 rating-colour]", names)
	}
	d, b, ok := pb.Lookup("rating-colour")
	if !ok || d == nil || b == nil {
		t.Fatalf("registry entry rating-colour: drafter %v builder %v ok %v", d, b, ok)
	}
	if _, _, ok := pb.Lookup("best23"); !ok {
		t.Fatal("registry has no best23 entry")
	}
	if _, _, ok := pb.Lookup("nope"); ok {
		t.Fatal("an unknown name resolved in the registry")
	}
	// The registered drafter answers an FDN pick without any corpus: the
	// rated path is name-keyed.
	pack := []limited.CardRef{
		ref("s1", "Thousand-Year Storm", limited.Rare),
		ref("l1", "Liliana, Dreadhorde General", limited.Rare),
	}
	if got := d.Pick(pb.PickView{Set: "FDN", Round: 1, PickNo: 1, Pack: pack}, rand.New(rand.NewPCG(1, 2))); got != "l1" {
		t.Fatalf("registry drafter pick = %q, want l1", got)
	}
}

func poolName(t *testing.T, pool []limited.CardRef, id string) string {
	t.Helper()
	for _, c := range pool {
		if c.ID == id {
			return c.Name
		}
	}
	t.Fatalf("deck maindecks %q, which is not in the pool", id)
	return ""
}
