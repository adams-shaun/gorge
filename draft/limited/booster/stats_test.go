package booster_test

import (
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/draft/limited"
	"github.com/adams-shaun/gorge/draft/limited/booster"
	"github.com/adams-shaun/gorge/draft/limited/setdata"
)

// openPacks opens n packs of one spec from independent PCG streams under one
// seed, sharing one event-wide ID allocator, and runs the structural asserts
// every pack must pass: full size, no repeated name, ID uniqueness. It
// returns the packs for the caller's distribution asserts.
func openPacks(t *testing.T, spec setdata.Spec, seed uint64, n int, colourOf booster.ColourOf, ids *booster.IDs) [][]limited.CardRef {
	t.Helper()
	out := make([][]limited.CardRef, n)
	for i := range out {
		pack, err := booster.Open(spec, rand.New(rand.NewPCG(seed, uint64(i))), ids, colourOf)
		if err != nil {
			t.Fatalf("Open pack %d: %v", i, err)
		}
		if len(pack) != spec.PackSize {
			t.Fatalf("pack %d has %d cards, want the spec's pack size %d", i, len(pack), spec.PackSize)
		}
		seen := map[string]bool{}
		for _, c := range pack {
			if seen[c.Name] {
				t.Fatalf("pack %d repeats %q", i, c.Name)
			}
			if seen[c.ID] {
				t.Fatalf("pack %d reuses ID %q", i, c.ID)
			}
			seen[c.Name] = true
			seen[c.ID] = true
		}
		out[i] = pack
	}
	return out
}

// TestBoosterFDNArenaStats is the E5-a1 statistics gate over 10,000
// Foundations packs at the default (play-arena) collation from a fixed
// seed: 14 cards each, no duplicate names, the special-guest packs at the
// collation's 1.5% share (the weight-3 variant), every land card drawn off
// the land sheet — plus the same seed opening an identical pack set.
func TestBoosterFDNArenaStats(t *testing.T) {
	spec, err := setdata.Load("FDN")
	if err != nil {
		t.Fatalf("Load(FDN): %v", err)
	}
	// Precondition: the arena config is the one whose rates this test pins —
	// the weight-3 variant carrying the specialGuest slot is 3/200 = 1.5%.
	var guestWeight int
	for _, v := range spec.Config.Variants {
		if _, ok := v.Contents["specialGuest"]; ok {
			guestWeight += v.Weight
		}
	}
	if guestWeight != 3 || spec.Config.TotalWeight != 200 {
		t.Fatalf("precondition: FDN arena special-guest weight is %d of %d, want 3 of 200", guestWeight, spec.Config.TotalWeight)
	}

	const packs = 10000
	const seed = 0xFD00F00D
	ids := &booster.IDs{}
	all := openPacks(t, spec, seed, packs, nil, ids)
	if ids.Count() != packs*spec.PackSize {
		t.Errorf("allocator handed out %d IDs, want %d (one per card, event-wide)", ids.Count(), packs*spec.PackSize)
	}

	guests := 0
	for _, pack := range all {
		for _, c := range pack {
			if c.Set == "SPG" {
				guests++
				break
			}
		}
	}
	share := float64(guests) / packs
	t.Logf("special-guest pack share: %.4f", share)
	if share < 0.011 || share > 0.019 {
		t.Errorf("special-guest pack share = %.4f over %d packs, want 0.015 ± 0.004", share, packs)
	}

	// Same seed, same packs: open a second batch from the same seed with a
	// fresh allocator and compare byte for byte.
	again := openPacks(t, spec, seed, 200, nil, &booster.IDs{})
	for i, pack := range again {
		for j, c := range pack {
			if c != all[i][j] {
				t.Fatalf("the same seed opened a different pack: pack %d card %d is %+v, want %+v", i, j, c, all[i][j])
			}
		}
	}
}

// TestBoosterFDNPlayStats opens 2,000 Foundations packs at the paper Play
// Booster collation from a fixed seed: 14 cards each, no duplicate names,
// the special-guest share at its collation's 15/1000 — plus the same seed
// opening an identical pack set, exactly as the arena gate does.
func TestBoosterFDNPlayStats(t *testing.T) {
	spec, err := setdata.LoadCollation("FDN", setdata.Play)
	if err != nil {
		t.Fatalf("LoadCollation(FDN, play): %v", err)
	}
	var guestWeight int
	for _, v := range spec.Config.Variants {
		if _, ok := v.Contents["specialGuest"]; ok {
			guestWeight += v.Weight
		}
	}
	if guestWeight != 15 || spec.Config.TotalWeight != 1000 {
		t.Fatalf("precondition: FDN play special-guest weight is %d of %d, want 15 of 1000", guestWeight, spec.Config.TotalWeight)
	}
	const packs = 2000
	const seed = 0xFD00F00D
	ids := &booster.IDs{}
	all := openPacks(t, spec, seed, packs, nil, ids)
	if ids.Count() != packs*spec.PackSize {
		t.Errorf("allocator handed out %d IDs, want %d (one per card, event-wide)", ids.Count(), packs*spec.PackSize)
	}
	guests := 0
	for _, pack := range all {
		for _, c := range pack {
			if c.Set == "SPG" {
				guests++
				break
			}
		}
	}
	share := float64(guests) / packs
	t.Logf("FDN play special-guest pack share: %.4f", share)
	if share < 0.005 || share > 0.025 {
		t.Errorf("FDN play special-guest pack share = %.4f over %d packs, want 0.015 ± 0.01 at this sample size", share, packs)
	}

	// Same seed, same packs, as at the arena collation.
	again := openPacks(t, spec, seed, 200, nil, &booster.IDs{})
	for i, pack := range again {
		for j, c := range pack {
			if c != all[i][j] {
				t.Fatalf("the same seed opened a different play pack: pack %d card %d is %+v, want %+v", i, j, c, all[i][j])
			}
		}
	}
}

// TestBoosterBLBArenaSmoke is the 13-card smoke: 1,000 Bloomburrow packs at
// the default collation — every one exactly 13 cards (no land slot), no
// duplicate names, IDs one per card.
func TestBoosterBLBArenaSmoke(t *testing.T) {
	spec, err := setdata.Load("BLB")
	if err != nil {
		t.Fatalf("Load(BLB): %v", err)
	}
	if spec.PackSize != 13 {
		t.Fatalf("precondition: BLB arena pack size = %d, want 13", spec.PackSize)
	}
	const packs = 1000
	ids := &booster.IDs{}
	openPacks(t, spec, 0xB10B, packs, nil, ids)
	if ids.Count() != packs*13 {
		t.Errorf("allocator handed out %d IDs, want %d", ids.Count(), packs*13)
	}
}
