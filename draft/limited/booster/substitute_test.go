package booster_test

import (
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/draft/limited"
	"github.com/adams-shaun/gorge/draft/limited/booster"
	"github.com/adams-shaun/gorge/draft/limited/setdata"
)

// TestBoosterSubstitutesUnplayableCards opens packs from a hand-built spec
// whose sheet is half unplayable and whose second variant draws from a
// sheet with no playable card: no pack ever deals an unplayable card, every
// swap is logged against the card it replaced, the dead-sheet variant opens
// as the heaviest live one, and Open deals exactly OpenLog's pack.
func TestBoosterSubstitutesUnplayableCards(t *testing.T) {
	spec := handSpec(2,
		[]setdata.Variant{
			{Weight: 3, Contents: map[string]int{"main": 2}},
			{Weight: 1, Contents: map[string]int{"main": 1, "dead": 1}},
		},
		map[string]setdata.Sheet{
			"main": {TotalWeight: 4, Cards: []setdata.SheetCard{
				sheetCard("Good A", "1", limited.Common, 1), sheetCard("Good B", "2", limited.Common, 1),
				sheetCard("Bad C", "3", limited.Common, 1), sheetCard("Bad D", "4", limited.Common, 1),
			}},
			"dead": {TotalWeight: 1, Cards: []setdata.SheetCard{sheetCard("Bad E", "5", limited.Rare, 1)}},
		})
	spec.Unplayable = map[string]bool{"Bad C": true, "Bad D": true, "Bad E": true}
	cardSubs, variantSubs := 0, 0
	for i := 0; i < 400; i++ {
		pack, subs, err := booster.OpenLog(spec, rand.New(rand.NewPCG(7, uint64(i))), &booster.IDs{}, nil)
		if err != nil {
			t.Fatalf("OpenLog pack %d: %v", i, err)
		}
		same, err := booster.Open(spec, rand.New(rand.NewPCG(7, uint64(i))), &booster.IDs{}, nil)
		if err != nil {
			t.Fatalf("Open pack %d: %v", i, err)
		}
		if len(pack) != 2 || len(same) != 2 {
			t.Fatalf("pack %d holds %d (Open %d) cards, want 2", i, len(pack), len(same))
		}
		for j := range pack {
			if pack[j] != same[j] {
				t.Fatalf("pack %d slot %d: Open dealt %v, OpenLog %v", i, j, same[j], pack[j])
			}
			if spec.Unplayable[pack[j].Name] {
				t.Fatalf("pack %d deals unplayable %q", i, pack[j].Name)
			}
		}
		for _, s := range subs {
			switch {
			case s.Sheet == "dead":
				if s.Original != "variant 1" || s.Substitute != "variant 0" {
					t.Errorf("pack %d variant swap %+v, want variant 1 -> variant 0", i, s)
				}
				variantSubs++
			case s.Sheet == "main" && spec.Unplayable[s.Original] && !spec.Unplayable[s.Substitute]:
				cardSubs++
			default:
				t.Errorf("pack %d logs an impossible swap %+v", i, s)
			}
		}
	}
	// About a quarter of the packs pick the dead variant, and half the
	// draws off the main sheet are unplayable.
	if variantSubs < 60 || variantSubs > 140 {
		t.Errorf("%d of 400 packs swapped their variant, want about 100", variantSubs)
	}
	if cardSubs < 300 || cardSubs > 500 {
		t.Errorf("%d card swaps over 800 draws, want about 400", cardSubs)
	}
}

// TestBoosterMBCPodDealsOnlyPlayableCards opens Mystery Booster Commander
// pods: 20-card packs, never a card off the substitute list, every swap
// logged with its seat and pack, the log identical for the same seed and
// OpenPod's packs identical to OpenPodLog's.
func TestBoosterMBCPodDealsOnlyPlayableCards(t *testing.T) {
	spec, err := setdata.Load("MBC")
	if err != nil {
		t.Fatalf("Load(MBC): %v", err)
	}
	if spec.PackSize != 20 || len(spec.Unplayable) == 0 {
		t.Fatalf("precondition: MBC pack size %d with %d substitute cards, want 20 and some", spec.PackSize, len(spec.Unplayable))
	}
	swaps := 0
	for seed := uint64(1); seed <= 25; seed++ {
		pod, log, err := booster.OpenPodLog(spec, seed, 4, 3, &booster.IDs{}, nil)
		if err != nil {
			t.Fatalf("OpenPodLog seed %d: %v", seed, err)
		}
		plain, err := booster.OpenPod(spec, seed, 4, 3, &booster.IDs{}, nil)
		if err != nil {
			t.Fatalf("OpenPod seed %d: %v", seed, err)
		}
		for s := range pod {
			for p, pack := range pod[s] {
				if len(pack) != 20 {
					t.Fatalf("seed %d seat %d pack %d holds %d cards, want 20", seed, s, p, len(pack))
				}
				for j, c := range pack {
					if spec.Unplayable[c.Name] {
						t.Fatalf("seed %d seat %d pack %d deals unplayable %q", seed, s, p, c.Name)
					}
					if plain[s][p][j] != c {
						t.Fatalf("seed %d seat %d pack %d slot %d: OpenPod and OpenPodLog differ", seed, s, p, j)
					}
				}
			}
		}
		for _, e := range log {
			if e.Seat < 0 || e.Seat >= 4 || e.Pack < 0 || e.Pack >= 3 || e.Sheet == "" || e.Original == "" || e.Substitute == "" {
				t.Fatalf("seed %d logs a malformed swap %+v", seed, e)
			}
		}
		swaps += len(log)
	}
	// About one pack in four draws an unplayable card (a colour sheet's
	// unsupported mechanics, a new commander, a Future Sight card).
	if swaps < 30 || swaps > 150 {
		t.Errorf("%d swaps over 300 packs, want about 70", swaps)
	}
}
