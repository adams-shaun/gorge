package setdata_test

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/draft/limited"
	"github.com/adams-shaun/gorge/draft/limited/setdata"
)

// mustLoadCollation loads and validates one embedded spec, failing the test
// on either.
func mustLoadCollation(t *testing.T, code string, c setdata.Collation) setdata.Spec {
	t.Helper()
	spec, err := setdata.LoadCollation(code, c)
	if err != nil {
		t.Fatalf("LoadCollation(%s, %s): %v", code, c, err)
	}
	if err := spec.Validate(); err != nil {
		t.Fatalf("LoadCollation(%s, %s): embedded spec fails its own validator: %v", code, c, err)
	}
	return spec
}

// TestSetdataCodesAreExactlyTheEmbeddedSets pins the loader's universe: exactly
// the four embedded sets, sorted, and unknown or hostile codes refuse.
func TestSetdataCodesAreExactlyTheEmbeddedSets(t *testing.T) {
	codes := setdata.Codes()
	if len(codes) != 4 || codes[0] != "BLB" || codes[1] != "FDN" || codes[2] != "MBC" || codes[3] != "TRK" {
		t.Fatalf("setdata.Codes() = %v, want [BLB FDN MBC TRK]", codes)
	}
	if _, err := setdata.Load("NOPE"); err == nil {
		t.Error("Load(NOP) succeeded for an unembedded set")
	}
	if _, err := setdata.Load(""); err == nil {
		t.Error("Load(\"\") succeeded")
	}
	if _, err := setdata.Load("../mtgjson/FDN"); err == nil {
		t.Error("Load accepted a path-traversal code")
	}
}

// TestSetdataCollationsArePlayAndArena pins the collation surface of the
// play-booster sets (MBC carries only its draft booster,
// TestMBCDraftBoosterShape) and refuses an unknown collation with the
// carried ones in the message.
func TestSetdataCollationsArePlayAndArena(t *testing.T) {
	for _, code := range setdata.Codes() {
		if code == "MBC" {
			continue
		}
		got := setdata.Collations(code)
		if len(got) != 2 || got[0] != setdata.Play || got[1] != setdata.Arena {
			t.Errorf("Collations(%s) = %v, want [play play-arena]", code, got)
		}
	}
	if got := setdata.Collations("NOPE"); got != nil {
		t.Errorf("Collations(NOP) = %v, want nil", got)
	}
	if _, err := setdata.LoadCollation("FDN", "vintage"); err == nil {
		t.Fatal("LoadCollation(FDN, vintage) succeeded for an unembedded collation")
	} else if want := `no "vintage" collation`; !strings.Contains(err.Error(), want) {
		t.Errorf("unknown-collation error %q lacks %q", err.Error(), want)
	}
}

// TestSetdataDefaultCollationIsArena pins the default: Load serves the
// play-arena collation, the one the 17Lands ratings are measured on. FDN
// arena carries the arena-only rareMythic sheet and neither foil sheet; BLB
// arena packs are 13 cards — nothing may assume 14.
func TestSetdataDefaultCollationIsArena(t *testing.T) {
	if setdata.DefaultCollation != setdata.Arena {
		t.Fatalf("DefaultCollation = %q, want play-arena", setdata.DefaultCollation)
	}
	fdn := mustLoadCollation(t, "FDN", setdata.Arena)
	def, err := setdata.Load("FDN")
	if err != nil {
		t.Fatalf("Load(FDN): %v", err)
	}
	if def.PackSize != fdn.PackSize || def.Name != fdn.Name || len(def.Config.Variants) != len(fdn.Config.Variants) {
		t.Fatalf("Load(FDN) does not serve the play-arena spec: %+v vs %+v", def, fdn)
	}
	if _, ok := fdn.Config.Sheets["rareMythic"]; !ok {
		t.Error("FDN arena lacks the rareMythic sheet")
	}
	for _, arenaOnlyAbsence := range []string{"foil", "foilLand", "rareMythicWithShowcase"} {
		if _, ok := fdn.Config.Sheets[arenaOnlyAbsence]; ok {
			t.Errorf("FDN arena carries the paper-only %q sheet", arenaOnlyAbsence)
		}
	}
	if fdn.PackSize != 14 {
		t.Errorf("FDN arena pack size = %d, want 14", fdn.PackSize)
	}
	blb := mustLoadCollation(t, "BLB", setdata.Arena)
	if blb.PackSize != 13 {
		t.Errorf("BLB arena pack size = %d, want 13 (no land slot)", blb.PackSize)
	}
}

// TestSetdataPlayCollation pins the paper Play Booster shape of both sets:
// two foil-bearing variants of the 788+12+197+3 recipe, a specialGuest
// sheet, and FDN's foilReplacement-free paper sheet set.
func TestSetdataPlayCollation(t *testing.T) {
	fdn := mustLoadCollation(t, "FDN", setdata.Play)
	if fdn.PackSize != 14 {
		t.Errorf("FDN play pack size = %d, want 14", fdn.PackSize)
	}
	if len(fdn.Config.Variants) != 4 {
		t.Fatalf("FDN play has %d variants, want 4", len(fdn.Config.Variants))
	}
	weights := map[int]bool{}
	total := 0
	for _, v := range fdn.Config.Variants {
		weights[v.Weight] = true
		total += v.Weight
	}
	for _, w := range []int{788, 12, 197, 3} {
		if !weights[w] {
			t.Errorf("FDN play lacks a variant of weight %d (has %v)", w, weights)
		}
	}
	if total != fdn.Config.TotalWeight {
		t.Errorf("FDN play variant weights sum to %d, want total_weight %d", total, fdn.Config.TotalWeight)
	}
	for _, sheet := range []string{"common", "uncommon", "rareMythicWithShowcase", "specialGuest", "foil", "foilLand", "land", "wildcard"} {
		if _, ok := fdn.Config.Sheets[sheet]; !ok {
			t.Errorf("FDN play lacks sheet %q", sheet)
		}
	}
	if got := len(fdn.Config.Sheets["specialGuest"].Cards); got != 10 {
		t.Errorf("FDN play specialGuest sheet has %d cards, want 10", got)
	}
	for _, sc := range fdn.Config.Sheets["specialGuest"].Cards {
		if sc.Set != "SPG" {
			t.Errorf("special-guest card %q carries set %q, want SPG", sc.Name, sc.Set)
		}
	}
	blb := mustLoadCollation(t, "BLB", setdata.Play)
	if blb.PackSize != 14 || len(blb.Config.Variants) != 4 {
		t.Errorf("BLB play = pack %d with %d variants, want 14 and 4", blb.PackSize, len(blb.Config.Variants))
	}
}

// TestSetdataSheetWeightsAreCardSums spot-checks the derived numbers the
// sampler draws on: each sheet's total weight is the sum of its cards'
// weights, and the arena rareMythic sheet splits 120 rare : 20 mythic in
// card weights (the 1-mythic-in-7 arena rare slot).
func TestSetdataSheetWeightsAreCardSums(t *testing.T) {
	for _, code := range setdata.Codes() {
		for _, c := range setdata.Collations(code) {
			spec := mustLoadCollation(t, code, c)
			for name, sheet := range spec.Config.Sheets {
				total := 0
				for _, sc := range sheet.Cards {
					total += sc.Weight
				}
				if total != sheet.TotalWeight {
					t.Errorf("%s %s sheet %q weights sum to %d, want %d", code, c, name, total, sheet.TotalWeight)
				}
			}
		}
	}
	arena := mustLoadCollation(t, "FDN", setdata.Arena)
	rare, mythic := 0, 0
	for _, sc := range arena.Config.Sheets["rareMythic"].Cards {
		switch sc.Rarity {
		case limited.Rare:
			rare += sc.Weight
		case limited.Mythic:
			mythic += sc.Weight
		}
	}
	if rare != 120 || mythic != 20 {
		t.Errorf("FDN arena rareMythic weights = rare %d / mythic %d, want 120 / 20", rare, mythic)
	}
}

// goodSpec is a minimal valid spec the validator cases break in one
// place each.
func goodSpec() setdata.Spec {
	card := func(name, number string, r limited.Rarity, w int) setdata.SheetCard {
		return setdata.SheetCard{UUID: "u" + number, Name: name, Set: "TST", Number: number, Rarity: r, Weight: w}
	}
	return setdata.Spec{
		Code: "TST", Name: "Test", PackSize: 3,
		Source: "hand-built test spec",
		Config: setdata.Config{
			SourceSetCodes: []string{"TST"},
			TotalWeight:    10,
			Variants: []setdata.Variant{
				{Weight: 9, Contents: map[string]int{"common": 2, "rareMythic": 1}},
				{Weight: 1, Contents: map[string]int{"common": 1, "rareMythic": 1, "land": 1}},
			},
			Sheets: map[string]setdata.Sheet{
				"common": {TotalWeight: 3, Cards: []setdata.SheetCard{card("C1", "1", limited.Common, 1), card("C2", "2", limited.Common, 1), card("C3", "3", limited.Common, 1)}},
				"rareMythic": {TotalWeight: 6, Cards: []setdata.SheetCard{
					card("R1", "10", limited.Rare, 2), card("R2", "11", limited.Rare, 2), card("M1", "12", limited.Mythic, 2),
				}},
				"land": {TotalWeight: 1, Cards: []setdata.SheetCard{card("L1", "90", limited.Common, 1)}},
			},
		},
	}
}

// TestSetdataValidateAcceptsAndRejects exercises the validator's rules on
// broken copies of a known-good spec; the loaded spec passing Validate
// first is what makes the rejections meaningful.
func TestSetdataValidateAcceptsAndRejects(t *testing.T) {
	good := goodSpec()
	if err := good.Validate(); err != nil {
		t.Fatalf("precondition: hand-built spec is invalid: %v", err)
	}
	cases := map[string]func(s *setdata.Spec){
		"variant names a missing sheet":  func(s *setdata.Spec) { s.Config.Variants[0].Contents["unreal"] = 1 },
		"variant weights miss the total": func(s *setdata.Spec) { s.Config.Variants[0].Weight-- },
		"variant contents miss the pack": func(s *setdata.Spec) { s.Config.Variants[0].Contents["common"]-- },
		"variants disagree on pack size": func(s *setdata.Spec) { s.Config.Variants[1].Contents["common"]++; s.PackSize = 4 },
		"zero variant weight":            func(s *setdata.Spec) { s.Config.Variants[0].Weight = 0 },
		"empty sheet":                    func(s *setdata.Spec) { s.Config.Sheets["land"] = setdata.Sheet{} },
		"card weight not the sheet sum":  func(s *setdata.Spec) { sh := s.Config.Sheets["land"]; sh.TotalWeight = 5; s.Config.Sheets["land"] = sh },
		"zero card weight": func(s *setdata.Spec) {
			sh := s.Config.Sheets["land"]
			sh.Cards[0].Weight = 0
			s.Config.Sheets["land"] = sh
		},
		"invalid rarity letter": func(s *setdata.Spec) {
			sh := s.Config.Sheets["land"]
			sh.Cards[0].Rarity = 'X'
			s.Config.Sheets["land"] = sh
		},
		"zero pack size":   func(s *setdata.Spec) { s.PackSize = 0 },
		"missing set name": func(s *setdata.Spec) { s.Name = "" },
	}
	// Each case mutates a FRESH spec: goodSpec builds new maps and slices
	// per call, so no case corrupts another through shared headers.
	for name, breakIt := range cases {
		s := goodSpec()
		breakIt(&s)
		if err := s.Validate(); err == nil {
			t.Errorf("Validate accepted a spec with %s", name)
		}
	}
}

// TestTRKPlayBoosterShape pins the Star Trek play booster as the edition's
// slots lay it out: 14 cards (5 common, 1 common with a Stardates
// replacement, 1 common with a Klingon replacement, 3 uncommon, a rare or
// mythic, a wildcard, a foil wildcard, a land), the land slot split into a
// nonfoil and a foil variant, the same on both collations.
func TestTRKPlayBoosterShape(t *testing.T) {
	for _, c := range []setdata.Collation{setdata.Play, setdata.Arena} {
		spec := mustLoadCollation(t, "TRK", c)
		if spec.PackSize != 14 || len(spec.Config.Variants) != 2 {
			t.Fatalf("TRK %s = pack %d with %d variants, want 14 and 2", c, spec.PackSize, len(spec.Config.Variants))
		}
		base := map[string]int{"common": 5, "commonStardates": 1, "commonKlingon": 1, "uncommon": 3,
			"rareMythic": 1, "wildcard": 1, "foil": 1}
		for i, land := range []string{"land", "foilLand"} {
			v := spec.Config.Variants[i]
			for sheet, n := range base {
				if v.Contents[sheet] != n {
					t.Errorf("TRK %s variant %d draws %d from %s, want %d", c, i, v.Contents[sheet], sheet, n)
				}
			}
			if v.Contents[land] != 1 || len(v.Contents) != len(base)+1 {
				t.Errorf("TRK %s variant %d contents %v, want one %s and the base slots", c, i, v.Contents, land)
			}
		}
		if w := spec.Config.Variants; w[0].Weight != 800 || w[1].Weight != 200 {
			t.Errorf("TRK %s land variants weigh %d/%d, want 800/200", c, w[0].Weight, w[1].Weight)
		}
		for name, sh := range spec.Config.Sheets {
			if len(sh.Cards) == 0 {
				t.Errorf("TRK %s sheet %s is empty", c, name)
			}
			if wantFoil := name == "foil" || name == "foilLand"; sh.Foil != wantFoil {
				t.Errorf("TRK %s sheet %s foil = %v, want %v", c, name, sh.Foil, wantFoil)
			}
		}
		// The replacement sheets carry their share: Stardates cards are
		// about 4% of the Common-Stardates slot, all mythic and set SDS.
		sds := spec.Config.Sheets["commonStardates"]
		var sdsW int
		for _, sc := range sds.Cards {
			if sc.Set == "SDS" {
				sdsW += sc.Weight
				if sc.Rarity != limited.Mythic {
					t.Errorf("TRK %s Stardates card %q has rarity %c, want mythic", c, sc.Name, sc.Rarity)
				}
			}
		}
		if share := float64(sdsW) / float64(sds.TotalWeight); share < 0.039 || share > 0.041 {
			t.Errorf("TRK %s Stardates share = %.4f, want about 0.04", c, share)
		}
		// Rares and mythics only in the rare slot's base, mythic about 12.4%.
		var mythic int
		for _, sc := range spec.Config.Sheets["rareMythic"].Cards {
			if sc.Rarity != limited.Rare && sc.Rarity != limited.Mythic {
				t.Errorf("TRK %s rareMythic sheet holds %q of rarity %c", c, sc.Name, sc.Rarity)
			}
			if sc.Rarity == limited.Mythic {
				mythic += sc.Weight
			}
		}
		if share := float64(mythic) / float64(spec.Config.Sheets["rareMythic"].TotalWeight); share < 0.12 || share > 0.15 {
			t.Errorf("TRK %s mythic share of the rare slot = %.4f, want about 0.13", c, share)
		}
	}
}

// TestMBCDraftBoosterShape pins Mystery Booster Commander Edition as
// MTGJSON publishes it: one "draft" collation (so Load serves it), 20-card
// packs of 3 cards from each colour sheet, a commander, a new commander, a
// playtest card, a wildcard and a Future Sight card, the last swapped for a
// foil or an Alchemy acorn card and the new commander for a foil in the
// six variants, weighted to 400000.
func TestMBCDraftBoosterShape(t *testing.T) {
	if got := setdata.Collations("MBC"); len(got) != 1 || got[0] != setdata.Draft {
		t.Fatalf("Collations(MBC) = %v, want [draft]", got)
	}
	if got := setdata.DefaultCollationFor("MBC"); got != setdata.Draft {
		t.Errorf("DefaultCollationFor(MBC) = %q, want draft", got)
	}
	if got := setdata.DefaultCollationFor("FDN"); got != setdata.Arena {
		t.Errorf("DefaultCollationFor(FDN) = %q, want play-arena", got)
	}
	spec, err := setdata.Load("MBC")
	if err != nil {
		t.Fatalf("Load(MBC): %v", err)
	}
	if spec.Collation != setdata.Draft || spec.PackSize != 20 || len(spec.Config.Variants) != 6 || spec.Config.TotalWeight != 400000 {
		t.Fatalf("MBC = %s pack %d, %d variants, total %d; want draft, 20, 6, 400000",
			spec.Collation, spec.PackSize, len(spec.Config.Variants), spec.Config.TotalWeight)
	}
	type key struct{ fs, nc string }
	want := map[key]int{
		{"futureSight", "newCommander"}: 363093, {"futureSightFoil", "newCommander"}: 16764, {"alchemyAcorn", "newCommander"}: 1143,
		{"futureSight", "newCommanderFoil"}: 18107, {"futureSightFoil", "newCommanderFoil"}: 836, {"alchemyAcorn", "newCommanderFoil"}: 57,
	}
	for i, v := range spec.Config.Variants {
		for _, sheet := range []string{"white", "blue", "black", "red", "green"} {
			if v.Contents[sheet] != 3 {
				t.Errorf("MBC variant %d draws %d from %s, want 3", i, v.Contents[sheet], sheet)
			}
		}
		var k key
		for sheet, n := range v.Contents {
			switch sheet {
			case "futureSight", "futureSightFoil", "alchemyAcorn":
				k.fs = sheet
			case "newCommander", "newCommanderFoil":
				k.nc = sheet
			}
			if n != 1 && n != 3 {
				t.Errorf("MBC variant %d draws %d from %s", i, n, sheet)
			}
		}
		if v.Contents["commander"] != 1 || v.Contents["playtest"] != 1 || v.Contents["wildcard"] != 1 {
			t.Errorf("MBC variant %d contents %v lack a commander, playtest or wildcard slot", i, v.Contents)
		}
		if w, ok := want[k]; !ok || w != v.Weight {
			t.Errorf("MBC variant %d (%s, %s) weighs %d, want %d", i, k.fs, k.nc, v.Weight, w)
		}
		delete(want, k)
	}
	sizes := map[string]int{"white": 358, "blue": 358, "black": 358, "red": 358, "green": 358, "commander": 121,
		"newCommander": 60, "newCommanderFoil": 60, "playtest": 120, "wildcard": 111, "futureSight": 119,
		"futureSightFoil": 134, "alchemyAcorn": 10}
	for name, n := range sizes {
		sh, ok := spec.Config.Sheets[name]
		if !ok || len(sh.Cards) != n {
			t.Errorf("MBC sheet %s has %d cards, want %d", name, len(sh.Cards), n)
		}
		if wantFoil := name == "futureSightFoil" || name == "newCommanderFoil" || name == "alchemyAcorn"; sh.Foil != wantFoil {
			t.Errorf("MBC sheet %s foil = %v, want %v", name, sh.Foil, wantFoil)
		}
	}
	// The substitute list covers the whole Alchemy sheet (no acorn card is
	// playable) and no card of the new-commander sheet's playable majority.
	for _, sc := range spec.Config.Sheets["alchemyAcorn"].Cards {
		if !spec.Unplayable[sc.Name] {
			t.Errorf("alchemy acorn card %q is not on the substitute list", sc.Name)
		}
	}
	if len(spec.Unplayable) == 0 {
		t.Fatal("MBC carries no substitute list")
	}
	if fdn := mustLoadCollation(t, "FDN", setdata.Arena); fdn.Unplayable != nil {
		t.Errorf("FDN carries a substitute list of %d cards, want none", len(fdn.Unplayable))
	}
}
