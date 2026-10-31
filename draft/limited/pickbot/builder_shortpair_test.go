package pickbot_test

import (
	"testing"

	"github.com/adams-shaun/gorge/draft/limited"
	"github.com/adams-shaun/gorge/draft/limited/build"
	pb "github.com/adams-shaun/gorge/draft/limited/pickbot"
	"github.com/adams-shaun/gorge/draft/limited/setdata"
)

// shortPairPoolNames is a 34-card FDN pool built to expose the thin-pair
// padding bug: 26 white/green cards (17Lands FDN.json GIH win rates
// 0.5202-0.5676, the mid tier) that fill white-green's 23 maindeck slots
// several times over, and 8 bombs rated 0.5817-0.6038 spread over blue,
// black and red (three per colour, so no pair containing white or green
// holds more than 3 of them). Scored on in-pair cards with a short-pair
// penalty, white-green's full 23 beat every padded pair; scored the old
// way, with the pool's best remaining cards credited to every short pair,
// white-blue's bombs plus fill outscores white-green and wins the scan.
var shortPairPoolNames = []string{
	// white/green, 0.52-0.57
	"Guarded Heir", "Angel of Finality", "Savannah Lions", "Eager Trufflesnout",
	"Fleeting Flight", "Nessian Hornbeetle", "Joust Through",
	"Loot, Exuberant Explorer", "Ajani's Pridemate", "Youthful Valkyrie",
	"Squad Rallier", "Cathar Commando", "Bite Down", "Anthem of Champions",
	"Affectionate Indrik", "Wary Thespian", "Ambush Wolf", "Make Your Move",
	"Mossborn Hydra", "Prideful Parent", "Treetop Snarespinner",
	"Snakeskin Veil", "Vanguard Seraph", "Giant Growth", "Dwynen's Elite",
	"Cackling Prowler",
	// off-colour bombs, 0.58+
	"Kiora, the Rising Tide", "Mischievous Mystic", "Micromancer",
	"Vampire Gourmand", "Infernal Vessel", "Tragic Banshee",
	"Searslicer Goblin", "Fiery Annihilation",
}

// shortPairPool resolves the named pool into CardRefs with the setdata FDN
// rarities, failing loudly on any name the corpus or the set lacks.
func shortPairPool(t *testing.T) []limited.CardRef {
	t.Helper()
	reg := corpus(t)
	spec, err := setdata.Load("FDN")
	if err != nil {
		t.Fatalf("setdata FDN: %v", err)
	}
	rarity := sheetRarities(t, spec)
	pool := make([]limited.CardRef, 0, len(shortPairPoolNames))
	for i, n := range shortPairPoolNames {
		card := lookupCard(t, reg, n)
		r, ok := rarity[n]
		if !ok {
			t.Fatalf("%q is not on any FDN sheet", n)
		}
		pool = append(pool, limited.CardRef{
			ID: "sp" + itoa(i), Name: n, Set: "FDN", Number: "1", Rarity: r,
		})
		// Precondition: the 26 white/green cards are white, green or both
		// and the 8 bombs are mono-blue, mono-black or mono-red, so every
		// pair other than white-green is the thin one and holds at most 3
		// bombs.
		mask := card.ColourIdentity()
		if i < 26 && mask&^(1<<0|1<<4) != 0 {
			t.Fatalf("%q has colour mask %04b, outside white-green", n, mask)
		}
		if i >= 26 && (mask == 0 || mask&(mask-1) != 0) {
			t.Fatalf("%q has colour mask %04b, not a single off colour", n, mask)
		}
	}
	if len(pool) != 34 {
		t.Fatalf("built a %d-card pool, want 34", len(pool))
	}
	return pool
}

// TestBuilderBest23IgnoresThinPairPaddedWithBombs pins the spec §4.6 step 1
// contract: the pair scan scores a colour pair on its in-pair cards, so the
// deck's maindeck stays inside the pair it chose even when other colours
// hold higher-rated cards. On a pool whose white-green cards are all mid
// tier and whose eight bombs live off-pair, the build maindecks at most one
// card outside white-green.
func TestBuilderBest23IgnoresThinPairPaddedWithBombs(t *testing.T) {
	reg := corpus(t)
	pool := shortPairPool(t)
	b := &pb.Best23{Lookup: lookupOf(reg)}

	deck := b.Build("FDN", pool)
	if err := build.Validate(pool, deck); err != nil {
		t.Fatalf("built deck invalid: %v", err)
	}
	if total := deck.Total(); total != 40 {
		t.Fatalf("built a %d-card deck, want 40", total)
	}
	if len(deck.Main) != 23 {
		t.Fatalf("built deck maindecks %d cards, want 23", len(deck.Main))
	}

	outside := 0
	for _, id := range deck.Main {
		name := poolName(t, pool, id)
		card := lookupCard(t, reg, name)
		if mask := card.ColourIdentity(); mask&^(1<<0|1<<4) != 0 {
			outside++
			t.Logf("maindeck %q has colour mask %04b, outside white-green", name, mask)
		}
	}
	if outside > 1 {
		t.Fatalf("built deck maindecks %d cards outside white-green, want at most 1", outside)
	}

	// Deterministic: a second build is card-for-card equal.
	again := b.Build("FDN", pool)
	for i := range deck.Main {
		if again.Main[i] != deck.Main[i] {
			t.Fatalf("rebuild differs at card %d: %q vs %q", i, again.Main[i], deck.Main[i])
		}
	}
}
