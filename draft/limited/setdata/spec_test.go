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

// TestSetdataCodesAreExactlyFDNAndBLB pins the loader's universe: exactly
// the two embedded sets, sorted, and unknown or hostile codes refuse.
func TestSetdataCodesAreExactlyFDNAndBLB(t *testing.T) {
	codes := setdata.Codes()
	if len(codes) != 2 || codes[0] != "BLB" || codes[1] != "FDN" {
		t.Fatalf("setdata.Codes() = %v, want [BLB FDN]", codes)
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

// TestSetdataCollationsArePlayAndArena pins the collation surface of both
// embedded sets and refuses an unknown collation with the carried ones in
// the message.
func TestSetdataCollationsArePlayAndArena(t *testing.T) {
	for _, code := range setdata.Codes() {
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

// goodSpec is a minimal valid v2 spec the validator cases break in one
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
