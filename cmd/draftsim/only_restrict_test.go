package main

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/draft/limited"
	"github.com/adams-shaun/gorge/draft/limited/booster"
	"github.com/adams-shaun/gorge/draft/limited/setdata"
)

// TestOnlyRestrictedPacksOpen: packs opened from a setdata.Restrict spec
// deal only allowed names, and the variant that draws from the emptied
// mythic sheet opens as another.
func TestOnlyRestrictedPacksOpen(t *testing.T) {
	card := func(name string, r limited.Rarity, w int) setdata.SheetCard {
		return setdata.SheetCard{UUID: name, Name: name, Rarity: r, Weight: w}
	}
	spec := setdata.Spec{Code: "TST", Config: setdata.Config{
		TotalWeight: 3,
		Variants: []setdata.Variant{
			{Weight: 2, Contents: map[string]int{"common": 2, "rare": 1}},
			{Weight: 1, Contents: map[string]int{"common": 2, "mythic": 1}},
		},
		Sheets: map[string]setdata.Sheet{
			"common": {TotalWeight: 6, Cards: []setdata.SheetCard{card("c1", 'C', 1), card("c2", 'C', 2), card("c3", 'C', 3)}},
			"rare":   {TotalWeight: 2, Cards: []setdata.SheetCard{card("r1", 'R', 1), card("r2", 'R', 1)}},
			"mythic": {TotalWeight: 1, Cards: []setdata.SheetCard{card("m1", 'M', 1)}},
			"bonus":  {TotalWeight: 9, Cards: []setdata.SheetCard{card("r3", 'R', 9), card("c4", 'C', 9)}},
		},
	}}
	got, _, err := setdata.Restrict(spec, onlyList{"c1": true, "c3": true, "r3": true, "c4": true}.allows)
	if err != nil {
		t.Fatal(err)
	}
	swaps := 0
	for seed := uint64(0); seed < 30; seed++ {
		pack, subs, err := booster.OpenLog(got, rand.New(rand.NewPCG(seed, 0)), &booster.IDs{}, nil)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		swaps += len(subs)
		var b []string
		for _, c := range pack {
			b = append(b, c.Name)
		}
		if p := strings.Join(b, ","); len(pack) != 3 || !strings.HasSuffix(p, ",r3") || strings.Contains(p, "c2") {
			t.Fatalf("seed %d pack %s, want two of c1/c3 and r3", seed, p)
		}
	}
	if swaps == 0 {
		t.Error("no pack opened the mythic variant as another")
	}
}
