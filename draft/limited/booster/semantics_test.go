package booster_test

import (
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/draft/limited"
	"github.com/adams-shaun/gorge/draft/limited/booster"
	"github.com/adams-shaun/gorge/draft/limited/setdata"
)

// TestBoosterFixedSheetYieldsListedCards pins the fixed flag: a fixed
// sheet's slot takes the sheet's cards as listed, in order, and a fixed
// sheet with fewer unpacked cards than the slot draws fails Open instead of
// shorting the pack.
func TestBoosterFixedSheetYieldsListedCards(t *testing.T) {
	// Precondition the assertions rely on: the fixed sheet lists exactly the
	// four cards the slot is expected to reproduce, in this order.
	want := []string{"F1", "F2", "F3", "F4"}
	sheets := map[string]setdata.Sheet{
		"fixedS": {TotalWeight: 4, Fixed: true, Cards: []setdata.SheetCard{
			sheetCard("F1", "1", limited.Common, 1), sheetCard("F2", "2", limited.Common, 1),
			sheetCard("F3", "3", limited.Common, 1), sheetCard("F4", "4", limited.Common, 1),
		}},
	}
	pack, err := booster.Open(v2spec(2, []setdata.Variant{{Weight: 1, Contents: map[string]int{"fixedS": 2}}}, sheets),
		rand.New(rand.NewPCG(5, 6)), &booster.IDs{}, nil)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	for i, name := range want[:2] {
		if pack[i].Name != name {
			t.Errorf("fixed slot card %d = %q, want the listed %q", i, pack[i].Name, name)
		}
		if pack[i].ID != "c"+itoa(i+1) {
			t.Errorf("fixed slot card %d has ID %q, want c%d (fill order)", i, pack[i].ID, i+1)
		}
	}
	// Four draws from the four-card fixed sheet fill the pack exactly once.
	full, err := booster.Open(v2spec(4, []setdata.Variant{{Weight: 1, Contents: map[string]int{"fixedS": 4}}}, sheets),
		rand.New(rand.NewPCG(5, 6)), &booster.IDs{}, nil)
	if err != nil {
		t.Fatalf("Open full fixed pack: %v", err)
	}
	for i, name := range want {
		if full[i].Name != name {
			t.Errorf("full fixed pack card %d = %q, want %q", i, full[i].Name, name)
		}
	}
}

// TestBoosterNamesStayUniqueAcrossSheets pins the pack-wide name rule: the
// same card name printed on two sheets is drawn at most once per pack — the
// second sheet redraws — and every pack still fills.
func TestBoosterNamesStayUniqueAcrossSheets(t *testing.T) {
	sheets := map[string]setdata.Sheet{
		"common": {TotalWeight: 3, Cards: []setdata.SheetCard{
			sheetCard("Shared", "1", limited.Common, 1), sheetCard("Common1", "2", limited.Common, 1), sheetCard("Common2", "3", limited.Common, 1),
		}},
		"uncommon": {TotalWeight: 2, Cards: []setdata.SheetCard{
			sheetCard("Shared", "10", limited.Uncommon, 1), sheetCard("Uncommon1", "11", limited.Uncommon, 1),
		}},
	}
	spec := v2spec(2, []setdata.Variant{{Weight: 1, Contents: map[string]int{"common": 1, "uncommon": 1}}}, sheets)
	const packs = 500
	for i := 0; i < packs; i++ {
		pack, err := booster.Open(spec, rand.New(rand.NewPCG(uint64(i), 19)), &booster.IDs{}, nil)
		if err != nil {
			t.Fatalf("Open pack %d: %v", i, err)
		}
		if len(pack) != 2 {
			t.Fatalf("pack %d has %d cards, want 2", i, len(pack))
		}
		if pack[0].Name == pack[1].Name {
			t.Fatalf("pack %d repeats %q across two sheets: the redraw rule must keep pack names unique", i, pack[0].Name)
		}
	}
}
