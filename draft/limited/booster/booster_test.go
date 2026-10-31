package booster_test

import (
	"math/rand/v2"
	"reflect"
	"strconv"
	"testing"

	"github.com/adams-shaun/gorge/draft/limited"
	"github.com/adams-shaun/gorge/draft/limited/booster"
	"github.com/adams-shaun/gorge/draft/limited/setdata"
)

// sheetCard builds one SheetCard with a distinct UUID and the given weight.
func sheetCard(name, number string, r limited.Rarity, weight int) setdata.SheetCard {
	return setdata.SheetCard{
		UUID:   "u" + number,
		Name:   name,
		Set:    "TST",
		Number: number,
		Rarity: r,
		Weight: weight,
	}
}

// v2spec assembles a valid v2 Spec from variants and sheets: the config's
// total weight is the variants' sum. The precondition assert makes a
// hand-built spec that fails Validate fail the test loudly, not silently.
func v2spec(packSize int, variants []setdata.Variant, sheets map[string]setdata.Sheet) setdata.Spec {
	total := 0
	for _, v := range variants {
		total += v.Weight
	}
	spec := setdata.Spec{
		Code: "TST", Name: "Test", PackSize: packSize,
		Source: "hand-built test spec",
		Config: setdata.Config{TotalWeight: total, Variants: variants, Sheets: sheets},
	}
	if err := spec.Validate(); err != nil {
		// Callers build specs that must validate; a broken one is a test bug.
		panic("v2spec: hand-built spec is invalid: " + err.Error())
	}
	return spec
}

func itoa(i int) string { return strconv.Itoa(i) }

// fiveColourSheet returns 20 W, 6 U, 6 B, 6 R and 2 G mono commons, named
// W1..W20, U1..U6, B1..B6, R1..R6, G1..G2, so a 7-common draw usually misses
// green and the colour balance has to trade a white for it.
func fiveColourSheet() setdata.Sheet {
	var cards []setdata.SheetCard
	for i := 1; i <= 20; i++ {
		cards = append(cards, sheetCard("W"+itoa(i), itoa(i), limited.Common, 1))
	}
	for _, col := range []byte{'U', 'B', 'R'} {
		for i := 1; i <= 6; i++ {
			cards = append(cards, sheetCard(string(col)+itoa(i), "1"+string(col)+itoa(i), limited.Common, 1))
		}
	}
	for i := 1; i <= 2; i++ {
		cards = append(cards, sheetCard("G"+itoa(i), "2G"+itoa(i), limited.Common, 1))
	}
	return setdata.Sheet{TotalWeight: len(cards), BalanceColors: true, Cards: cards}
}

// monoCount counts the slot cards that are mono-coloured of each WUBRG
// colour, using the test's own colourOf table.
func monoCount(colourOf booster.ColourOf, cards []limited.CardRef) [5]int {
	var out [5]int
	for _, c := range cards {
		cs := colourOf(c.Name)
		if len(cs) == 1 {
			switch cs[0] {
			case 'W':
				out[0]++
			case 'U':
				out[1]++
			case 'B':
				out[2]++
			case 'R':
				out[3]++
			case 'G':
				out[4]++
			}
		}
	}
	return out
}

// colourOfInitial maps a card name's first letter to its mono colour, the
// convention the five-colour test sheets use.
func colourOfInitial(name string) []byte {
	if len(name) == 0 {
		return nil
	}
	switch name[0] {
	case 'W', 'U', 'B', 'R', 'G':
		return []byte{name[0]}
	}
	return nil
}

// TestBoosterPackFillsContentsAndIDs opens a hand spec whose sheets are
// recognisable by rarity and checks the pack fills in sorted sheet order
// with sequential IDs from the caller's allocator.
func TestBoosterPackFillsContentsAndIDs(t *testing.T) {
	spec := v2spec(5,
		[]setdata.Variant{{Weight: 1, Contents: map[string]int{
			"common": 2, "land": 1, "rareMythic": 1, "uncommon": 1,
		}}},
		map[string]setdata.Sheet{
			"common": {TotalWeight: 3, Cards: []setdata.SheetCard{
				sheetCard("C1", "1", limited.Common, 1), sheetCard("C2", "2", limited.Common, 1), sheetCard("C3", "3", limited.Common, 1),
			}},
			"uncommon": {TotalWeight: 1, Cards: []setdata.SheetCard{sheetCard("U1", "10", limited.Uncommon, 1)}},
			"rareMythic": {TotalWeight: 5, Cards: []setdata.SheetCard{
				sheetCard("R1", "20", limited.Rare, 2), sheetCard("R2", "21", limited.Rare, 2), sheetCard("M1", "30", limited.Mythic, 1),
			}},
			"land": {TotalWeight: 1, Cards: []setdata.SheetCard{sheetCard("L1", "90", limited.Land, 1)}},
		})
	pack, err := booster.Open(spec, rand.New(rand.NewPCG(1, 2)), &booster.IDs{}, nil)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if len(pack) != 5 {
		t.Fatalf("pack has %d cards, want 5", len(pack))
	}
	// Sorted sheet order: common, common, land, rareMythic (R or M),
	// uncommon.
	wantRarity := []limited.Rarity{limited.Common, limited.Common, limited.Land, limited.Rare, limited.Uncommon}
	for i, r := range wantRarity {
		if i == 3 && pack[i].Rarity == limited.Mythic {
			r = limited.Mythic // the weighted rare/mythic draw may land either side
		}
		if pack[i].Rarity != r {
			t.Errorf("pack[%d] rarity = %c, want %c (sheets fill in sorted name order)", i, pack[i].Rarity, r)
		}
		if pack[i].Set != "TST" {
			t.Errorf("pack[%d].Set = %q, want the printing's set TST", i, pack[i].Set)
		}
		if wantID := "c" + itoa(i+1); pack[i].ID != wantID {
			t.Errorf("pack[%d].ID = %q, want %q (IDs assigned in fill order)", i, pack[i].ID, wantID)
		}
	}
}

// TestBoosterPackNeverRepeatsNames drives the resample rule from both
// directions: two draws from a two-card sheet come out distinct, and a
// third draw from the same two-card sheet is an error, not a duplicate.
func TestBoosterPackNeverRepeatsNames(t *testing.T) {
	sheets := map[string]setdata.Sheet{
		"common": {TotalWeight: 2, Cards: []setdata.SheetCard{
			sheetCard("Only1", "1", limited.Common, 1), sheetCard("Only2", "2", limited.Common, 1),
		}},
	}
	pack, err := booster.Open(v2spec(2, []setdata.Variant{{Weight: 1, Contents: map[string]int{"common": 2}}}, sheets),
		rand.New(rand.NewPCG(3, 4)), &booster.IDs{}, nil)
	if err != nil {
		t.Fatalf("Open two draws from a two-card sheet: %v", err)
	}
	if len(pack) != 2 || pack[0].Name == pack[1].Name {
		t.Fatalf("two draws gave %v: a pack must never repeat a name", pack)
	}
	if _, err := booster.Open(v2spec(3, []setdata.Variant{{Weight: 1, Contents: map[string]int{"common": 3}}}, sheets),
		rand.New(rand.NewPCG(3, 4)), &booster.IDs{}, nil); err == nil {
		t.Error("three draws from a two-card sheet succeeded: duplicates must fail Open, not pass silently")
	}
}

// TestBoosterDrawFollowsPerCardWeights pins the weighted draw: with two
// rares at weight 2 and one mythic at weight 1, the mythic takes the slot
// one time in five.
func TestBoosterDrawFollowsPerCardWeights(t *testing.T) {
	spec := v2spec(1,
		[]setdata.Variant{{Weight: 1, Contents: map[string]int{"rareMythic": 1}}},
		map[string]setdata.Sheet{
			"rareMythic": {TotalWeight: 5, Cards: []setdata.SheetCard{
				sheetCard("R1", "1", limited.Rare, 2), sheetCard("R2", "2", limited.Rare, 2), sheetCard("M1", "3", limited.Mythic, 1),
			}},
		})
	const packs = 20000
	mythics := 0
	for i := 0; i < packs; i++ {
		pack, err := booster.Open(spec, rand.New(rand.NewPCG(uint64(i), 7)), &booster.IDs{}, nil)
		if err != nil || len(pack) != 1 {
			t.Fatalf("Open pack %d: %v", i, err)
		}
		if pack[0].Rarity == limited.Mythic {
			mythics++
		}
	}
	share := float64(mythics) / packs // want 1*1 / (2*2 + 1*1) = 0.2
	if share < 0.18 || share > 0.22 {
		t.Errorf("mythic share of the weighted draw = %.4f over %d packs, want 0.2 ± 0.02", share, packs)
	}
}

// TestBoosterVariantWeightsPickTheRecipe pins the variant pick: a pack from
// the dominant variant's recipe has an extra common, the minority one an
// extra land; the shares follow the variant weights.
func TestBoosterVariantWeightsPickTheRecipe(t *testing.T) {
	sheets := map[string]setdata.Sheet{
		"common": {TotalWeight: 6, Cards: []setdata.SheetCard{
			sheetCard("C1", "1", limited.Common, 1), sheetCard("C2", "2", limited.Common, 1), sheetCard("C3", "3", limited.Common, 1),
			sheetCard("C4", "4", limited.Common, 1), sheetCard("C5", "5", limited.Common, 1), sheetCard("C6", "6", limited.Common, 1),
		}},
		"land": {TotalWeight: 2, Cards: []setdata.SheetCard{
			sheetCard("L1", "9", limited.Land, 1), sheetCard("L2", "10", limited.Land, 1),
		}},
	}
	spec := v2spec(4,
		[]setdata.Variant{
			{Weight: 3, Contents: map[string]int{"common": 3, "land": 1}},
			{Weight: 1, Contents: map[string]int{"common": 2, "land": 2}},
		}, sheets)
	const packs = 20000
	small := 0
	for i := 0; i < packs; i++ {
		pack, err := booster.Open(spec, rand.New(rand.NewPCG(uint64(i), 13)), &booster.IDs{}, nil)
		if err != nil || len(pack) != 4 {
			t.Fatalf("Open pack %d: %v", i, err)
		}
		commons := 0
		for _, c := range pack {
			if c.Rarity == limited.Common {
				commons++
			}
		}
		if commons == 2 {
			small++
		}
	}
	share := float64(small) / packs // the weight-1 variant's recipe
	if share < 0.22 || share > 0.28 {
		t.Errorf("minority variant share = %.4f over %d packs, want 0.25 ± 0.03", share, packs)
	}
}

// TestBoosterColourBalanceFillsMissingColours draws 7 commons from a sheet
// where green has only two cards, so most packs miss green; the colour
// balance must trade a white (the most-represented colour) for a green in
// every one of them, without ever emptying white or duplicating a name.
func TestBoosterColourBalanceFillsMissingColours(t *testing.T) {
	sheet := fiveColourSheet()
	spec := v2spec(7, []setdata.Variant{{Weight: 1, Contents: map[string]int{"common": 7}}}, map[string]setdata.Sheet{"common": sheet})
	colourOf := colourOfInitial
	// Precondition: green is genuinely scarce on the sheet, so an unbalanced
	// draw would miss it most of the time and the assertion below is live.
	var greenOnSheet int
	for _, sc := range sheet.Cards {
		if colourOf(sc.Name)[0] == 'G' {
			greenOnSheet++
		}
	}
	if greenOnSheet != 2 {
		t.Fatalf("precondition: sheet carries %d green commons, want 2", greenOnSheet)
	}
	const packs = 500
	greenPacks, whitePacks := 0, 0
	for i := 0; i < packs; i++ {
		ids := &booster.IDs{}
		pack, err := booster.Open(spec, rand.New(rand.NewPCG(uint64(i), 11)), ids, colourOf)
		if err != nil {
			t.Fatalf("Open pack %d: %v", i, err)
		}
		if len(pack) != 7 {
			t.Fatalf("pack %d has %d cards, want 7", i, len(pack))
		}
		names := map[string]bool{}
		for _, c := range pack {
			if names[c.Name] {
				t.Fatalf("pack %d repeats %q", i, c.Name)
			}
			names[c.Name] = true
			if names[c.ID] {
				t.Fatalf("pack %d reuses ID %q within itself", i, c.ID)
			}
			names[c.ID] = true
		}
		if ids.Count() != 7 {
			t.Errorf("pack %d allocated %d IDs, want 7 (balance must keep the drawn card's ID)", i, ids.Count())
		}
		mono := monoCount(colourOf, pack)
		if mono[4] > 0 {
			greenPacks++
		}
		if mono[0] > 0 {
			whitePacks++
		}
		for _, c := range pack {
			if c.Rarity != limited.Common {
				t.Errorf("pack %d carries %q at rarity %c off the common sheet", i, c.Name, c.Rarity)
			}
		}
	}
	if greenPacks != packs {
		t.Errorf("colour balance left green missing in %d of %d packs (white is plentiful, green is on the sheet: every pack must have one)", packs-greenPacks, packs)
	}
	if whitePacks != packs {
		t.Errorf("colour balance emptied white in %d of %d packs", packs-whitePacks, packs)
	}
}

// TestBoosterColourBalanceOnlyFromFiveCards pins the brief's threshold: a
// balance_colors sheet whose slot draws fewer than 5 cards is NOT balanced.
// The sheet is white-plentiful/blue-scarce and the slot draws 4: balanced,
// every all-white draw would trade a white for a blue and no pack would
// stay blueless; unbalanced, all-white packs occur and blue may stay
// missing. (At count 5 the same threshold forces every missing colour in —
// that direction is TestBoosterColourBalanceFillsMissingColours.)
func TestBoosterColourBalanceOnlyFromFiveCards(t *testing.T) {
	var cards []setdata.SheetCard
	for i := 1; i <= 20; i++ {
		cards = append(cards, sheetCard("W"+itoa(i), itoa(i), limited.Common, 1))
	}
	for i := 1; i <= 6; i++ {
		cards = append(cards, sheetCard("U"+itoa(i), "1U"+itoa(i), limited.Common, 1))
	}
	sheet := setdata.Sheet{TotalWeight: len(cards), BalanceColors: true, Cards: cards}
	spec := v2spec(4, []setdata.Variant{{Weight: 1, Contents: map[string]int{"common": 4}}}, map[string]setdata.Sheet{"common": sheet})
	missed := 0
	const packs = 200
	for i := 0; i < packs; i++ {
		pack, err := booster.Open(spec, rand.New(rand.NewPCG(uint64(i), 17)), &booster.IDs{}, colourOfInitial)
		if err != nil {
			t.Fatalf("Open pack %d: %v", i, err)
		}
		if monoCount(colourOfInitial, pack)[1] == 0 {
			missed++
		}
	}
	if missed == 0 {
		t.Fatal("every 4-card draw from the white/blue sheet hit blue: the precondition cannot show the threshold")
	}
	t.Logf("sub-five balance_colors slot left blue missing in %d of %d packs (allowed)", missed, packs)
}

// TestBoosterPodIsDeterministicAndSaltStable pins OpenPod: the same seed
// replays byte-identically, and growing the pod never shifts a pack another
// seat already holds.
func TestBoosterPodIsDeterministicAndSaltStable(t *testing.T) {
	// The pod needs enough distinct cards per sheet that 6 packs (3 seats ×
	// 2) can all draw pod-wide-unique names from each of its slots.
	sheets := map[string]setdata.Sheet{
		"common": fiveColourSheet(),
		"uncommon": {TotalWeight: 6, Cards: []setdata.SheetCard{
			sheetCard("UncA", "1", limited.Uncommon, 1), sheetCard("UncB", "2", limited.Uncommon, 1),
			sheetCard("UncC", "3", limited.Uncommon, 1), sheetCard("UncD", "4", limited.Uncommon, 1),
			sheetCard("UncE", "5", limited.Uncommon, 1), sheetCard("UncF", "6", limited.Uncommon, 1),
		}},
		"rareMythic": {TotalWeight: 7, Cards: []setdata.SheetCard{
			sheetCard("RareA", "7", limited.Rare, 2), sheetCard("RareB", "8", limited.Rare, 2),
			sheetCard("RareC", "9", limited.Rare, 1), sheetCard("RareD", "10", limited.Rare, 1),
			sheetCard("MythA", "11", limited.Mythic, 1),
		}},
		"land": {TotalWeight: 6, Cards: []setdata.SheetCard{
			sheetCard("LandA", "90", limited.Land, 1), sheetCard("LandB", "91", limited.Land, 1),
			sheetCard("LandC", "92", limited.Land, 1), sheetCard("LandD", "93", limited.Land, 1),
			sheetCard("LandE", "94", limited.Land, 1), sheetCard("LandF", "95", limited.Land, 1),
		}},
	}
	spec := v2spec(7, []setdata.Variant{{Weight: 1, Contents: map[string]int{
		"common": 4, "land": 1, "rareMythic": 1, "uncommon": 1,
	}}}, sheets)
	// Precondition: the common sheet balances colours, so the colour lookup
	// matters and the pod exercise covers it.
	if !sheets["common"].BalanceColors {
		t.Fatal("precondition: the pod spec's common sheet does not balance colours")
	}
	colourOf := colourOfInitial
	openPod := func(seats, packs int) [][][]limited.CardRef {
		t.Helper()
		pod, err := booster.OpenPod(spec, 42, seats, packs, &booster.IDs{}, colourOf)
		if err != nil {
			t.Fatalf("OpenPod(%d seats, %d packs): %v", seats, packs, err)
		}
		return pod
	}
	if !reflect.DeepEqual(openPod(3, 2), openPod(3, 2)) {
		t.Error("the same seed opened two different pods")
	}
	// Precondition: the compared pods have the shapes the stability checks
	// index into.
	if got := openPod(3, 2); len(got) != 3 || len(got[0]) != 2 {
		t.Fatalf("pod shape = %dx%d, want 3x2", len(got), len(got[0]))
	}
	if !reflect.DeepEqual(openPod(3, 2)[1][0], openPod(2, 2)[1][0]) {
		t.Error("pack (seat 1, pack 0) changed when the seat count grew 2→3")
	}
	if !reflect.DeepEqual(openPod(3, 2)[0][1], openPod(3, 3)[0][1]) {
		t.Error("pack (seat 0, pack 1) changed when the pack count grew 2→3")
	}
	// One event-wide allocator: the pod's IDs are unique across every seat
	// and pack, in seat-major order.
	ids := &booster.IDs{}
	pod, err := booster.OpenPod(spec, 42, 3, 2, ids, colourOf)
	if err != nil {
		t.Fatalf("OpenPod: %v", err)
	}
	if ids.Count() != 3*2*7 {
		t.Errorf("allocator handed out %d IDs, want %d", ids.Count(), 3*2*7)
	}
	seen := map[string]bool{}
	for _, seat := range pod {
		for _, pack := range seat {
			for _, c := range pack {
				if seen[c.ID] {
					t.Errorf("ID %q appears twice across the pod", c.ID)
				}
				seen[c.ID] = true
			}
		}
	}
	if pod[0][0][0].ID != "c1" {
		t.Errorf("seat-major order broken: first pod card has ID %q, want c1", pod[0][0][0].ID)
	}
	if pod[2][1][6].ID != "c42" {
		t.Errorf("seat-major order broken: last pod card has ID %q, want c42", pod[2][1][6].ID)
	}
}

// TestBoosterRefusesMissingInputs checks the guard rails: no allocator, no
// seeded rng, or a balancing sheet without a ColourOf all fail Open.
func TestBoosterRefusesMissingInputs(t *testing.T) {
	sheets := map[string]setdata.Sheet{
		"common": {TotalWeight: 1, BalanceColors: true, Cards: []setdata.SheetCard{sheetCard("C1", "1", limited.Common, 1)}},
	}
	spec := v2spec(1, []setdata.Variant{{Weight: 1, Contents: map[string]int{"common": 1}}}, sheets)
	rng := rand.New(rand.NewPCG(1, 1))
	if _, err := booster.Open(spec, rng, nil, nil); err == nil {
		t.Error("Open accepted a nil ID allocator")
	}
	if _, err := booster.Open(spec, nil, &booster.IDs{}, nil); err == nil {
		t.Error("Open accepted a nil rng")
	}
	if _, err := booster.Open(spec, rng, &booster.IDs{}, nil); err == nil {
		t.Error("Open accepted a balancing sheet without a ColourOf")
	}
	if _, err := booster.OpenPod(spec, 1, 0, 1, &booster.IDs{}, nil); err == nil {
		t.Error("OpenPod accepted zero seats")
	}
}
