package kgen

import (
	"strings"
	"testing"
)

// TestXCostSubtypeFlashLowers: the two spell-side CastWithFlash ValidSA$
// tails gorge reads (rules/statics_cast.go spellMatchesValidSA) lower with no
// refusal. Silver Scrutiny's `Spell.XCostLE3` self grant is the SelfFlash
// static carrying the X bound (FlashXMax = 1+3, kop.Static.A); Blur of
// Heroism's `Spell.Hero` battlefield grant folds the subtype into its card
// filter. Any other ValidSA$ word still refuses by name.
func TestXCostSubtypeFlashLowers(t *testing.T) {
	scrutiny := lowerCorpusCard(t, "Silver Scrutiny")
	if len(scrutiny.Refusals) != 0 {
		t.Fatalf("Silver Scrutiny refused: %v", scrutiny.Refusals)
	}
	if !scrutiny.SelfFlash || len(scrutiny.Statics) != 1 || scrutiny.Statics[0].Kind != "CastWithFlash" || scrutiny.Statics[0].FlashXMax != 4 {
		t.Fatalf("Silver Scrutiny: selfflash=%v statics %+v, want one CastWithFlash with FlashXMax 4", scrutiny.SelfFlash, scrutiny.Statics)
	}
	blur := lowerCorpusCard(t, "Blur of Heroism")
	if len(blur.Refusals) != 0 {
		t.Fatalf("Blur of Heroism refused: %v", blur.Refusals)
	}
	var flash *lStatic
	for i := range blur.Statics {
		if blur.Statics[i].Kind == "CastWithFlash" {
			flash = &blur.Statics[i]
		}
	}
	if flash == nil || flash.F == nil || flash.FlashXMax != 0 || blur.SelfFlash {
		t.Fatalf("Blur of Heroism: statics %+v, want a battlefield CastWithFlash with a filter", blur.Statics)
	}
	var b strings.Builder
	dumpLIR(&b, blur)
	if !strings.Contains(b.String(), "Hero") {
		t.Errorf("Blur of Heroism's flash filter does not read Hero:\n%s", b.String())
	}
	// a word the filter lowerer cannot read, and an X bound on a battlefield
	// grant (the kernel caps only the SelfFlash carrier's own X), refuse
	for _, tail := range []string{"Spell.Wizard+YouOwn", "Spell.XCostLEX", "Spell.Bogus", "Spell.XCostLE3"} {
		src := "Name:Probe\nManaCost:2 W\nTypes:Enchantment\nS:Mode$ CastWithFlash | ValidCard$ Card | ValidSA$ " + tail + " | Caster$ You\nOracle:x\n"
		if got := strings.Join(lowerScript(t, "Probe", src), " | "); !strings.Contains(got, "CastWithFlash shape") {
			t.Errorf("ValidSA$ %s: refusals %q, want CastWithFlash shape", tail, got)
		}
	}
}
