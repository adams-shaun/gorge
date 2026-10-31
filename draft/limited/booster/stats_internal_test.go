// The in-package statistics the public pack output cannot attribute: which
// variant a pack came from, and the sheet-level weighted draws the
// allow_duplicates flag governs. Everything here reads setdata specs (the
// embedded ones and hand-built ones) — no corpus, no engine.
package booster

import (
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/draft/limited"
	"github.com/adams-shaun/gorge/draft/limited/setdata"
)

// TestVariantSharesFollowWeights opens 10,000 FDN packs per collation from
// one fixed seed and asserts each variant takes its weight's share of the
// packs within ±0.01 (the openPack variant index is what Open's public
// output cannot carry). The special-guest variants' combined share — the
// 1.5% the published product quotes — is asserted on the same run.
func TestVariantSharesFollowWeights(t *testing.T) {
	for _, c := range []setdata.Collation{setdata.Play, setdata.Arena} {
		spec, err := setdata.LoadCollation("FDN", c)
		if err != nil {
			t.Fatalf("LoadCollation(FDN, %s): %v", c, err)
		}
		const packs = 10000
		const seed = 0xFD00F00D
		counts := make([]int, len(spec.Config.Variants))
		guests := 0
		for i := 0; i < packs; i++ {
			_, v, err := openPack(spec, rand.New(rand.NewPCG(seed, uint64(i))), &IDs{}, nil)
			if err != nil {
				t.Fatalf("%s: openPack %d: %v", c, i, err)
			}
			counts[v]++
			if _, ok := spec.Config.Variants[v].Contents["specialGuest"]; ok {
				guests++
			}
		}
		guestWeight, total := 0, spec.Config.TotalWeight
		for _, v := range spec.Config.Variants {
			if _, ok := v.Contents["specialGuest"]; ok {
				guestWeight += v.Weight
			}
		}
		for i, v := range spec.Config.Variants {
			want := float64(v.Weight) / float64(total)
			got := float64(counts[i]) / packs
			if diff := got - want; diff > 0.01 || diff < -0.01 {
				t.Errorf("%s: variant %d (weight %d) share = %.4f, want %.4f ± 0.01", c, i, v.Weight, got, want)
			}
		}
		guestShare := float64(guests) / packs
		wantGuest := float64(guestWeight) / float64(total)
		if diff := guestShare - wantGuest; diff > 0.004 || diff < -0.004 {
			t.Errorf("%s: special-guest pack share = %.4f, want %.4f ± 0.004", c, guestShare, wantGuest)
		}
	}
}

// card builds a test SheetCard with a distinct UUID.
func card(name, number string, r limited.Rarity, weight int) setdata.SheetCard {
	return setdata.SheetCard{UUID: "u" + number, Name: name, Set: "TST", Number: number, Rarity: r, Weight: weight}
}

// TestWeightedDrawHonoursAllowDuplicates pins the flag at the mechanism it
// governs: on a one-card sheet the second draw fails when the sheet draws
// without replacement, and succeeds (the same card again) when the sheet
// allows duplicates. The pack-level name rule is separate and always on.
func TestWeightedDrawHonoursAllowDuplicates(t *testing.T) {
	sheet := setdata.Sheet{TotalWeight: 1, Cards: []setdata.SheetCard{card("Only", "1", limited.Common, 1)}}
	// Mark only the UUID as drawn (the name stays unpacked), isolating the
	// UUID half of the replacement rule from the name rule.
	st := &drawState{inPack: map[string]bool{}, drawn: map[string]bool{}}
	st.drawn[sheet.Cards[0].UUID] = true
	if _, err := drawWeighted(rand.New(rand.NewPCG(1, 2)), sheet, st); err == nil {
		t.Error("a sheet without allow_duplicates yielded its already-drawn card")
	}
	dup := sheet
	dup.AllowDuplicates = true
	if _, err := drawWeighted(rand.New(rand.NewPCG(1, 2)), dup, st); err != nil {
		t.Errorf("an allow_duplicates sheet refused its already-drawn card: %v", err)
	}
}

// TestRareMythicWeightedShareMatchesCardWeights is the
// rare-vs-mythic check: 100,000 draws from the FDN arena rareMythic sheet,
// whose 60 rares at weight 2 and 20 mythics at weight 1 give a mythic share
// of 20/140 ≈ 0.1429, land within ±0.015 of that.
func TestRareMythicWeightedShareMatchesCardWeights(t *testing.T) {
	spec, err := setdata.LoadCollation("FDN", setdata.Arena)
	if err != nil {
		t.Fatalf("LoadCollation(FDN, play-arena): %v", err)
	}
	sheet := spec.Config.Sheets["rareMythic"]
	rareW, mythicW, cards := 0, 0, 0
	for _, sc := range sheet.Cards {
		switch sc.Rarity {
		case limited.Rare:
			rareW += sc.Weight
			cards++
		case limited.Mythic:
			mythicW += sc.Weight
			cards++
		}
	}
	if cards != 80 || rareW != 120 || mythicW != 20 {
		t.Fatalf("precondition: FDN arena rareMythic sheet is %d cards at %d/%d rare/mythic weight, want 80 at 120/20", cards, rareW, mythicW)
	}
	const draws = 100000
	mythics := 0
	for i := 0; i < draws; i++ {
		st := &drawState{inPack: map[string]bool{}, drawn: map[string]bool{}}
		sc, err := drawWeighted(rand.New(rand.NewPCG(0xFEED, uint64(i))), sheet, st)
		if err != nil {
			t.Fatalf("draw %d: %v", i, err)
		}
		if sc.Rarity == limited.Mythic {
			mythics++
		}
	}
	share := float64(mythics) / draws
	want := float64(mythicW) / float64(rareW+mythicW)
	t.Logf("mythic share of the arena rareMythic draws: %.4f, want %.4f", share, want)
	if share < want-0.015 || share > want+0.015 {
		t.Errorf("mythic share of the rareMythic slot = %.4f, want %.4f ± 0.015", share, want)
	}
}
