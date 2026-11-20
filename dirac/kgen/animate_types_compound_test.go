package kgen

import (
	"strings"
	"testing"
)

// TestAnimateTypesCompoundWords pins the Animate Types$ word split: a space
// separates types like a comma (gorge animateGrant, combatfx.go's
// strings.Fields over the comma-normalised value), so Subversive Acolyte's
// "Types$ Human Cleric" lowers two subtypes, Human and Cleric -- never one
// subtype word "Human Cleric", a type no object in Magic has and the kernel
// table would carry beside gorge's separate words (the lockstep divergence
// this pool's first Acolyte charm exposed: the kernel's projection showed a
// subtype the gorge side could not name).
func TestAnimateTypesCompoundWords(t *testing.T) {
	const src = "Name:Compound Types Probe\nManaCost:1 B\nTypes:Creature Human\nPT:2/3\n" +
		"A:AB$ Animate | Cost$ 3 | Defined$ Self | Types$ Human Cleric | Duration$ Permanent | RemoveCreatureTypes$ True | SpellDescription$ becomes two kinds.\n" +
		"A:AB$ Animate | Cost$ 3 PayLife<1> | Defined$ Self | Types$ Phyrexian | Duration$ Permanent | RemoveCreatureTypes$ True | SpellDescription$ becomes another kind.\n"
	lc := lowerSynth(t, src)
	var reasons []string
	for _, r := range lc.Refusals {
		reasons = append(reasons, r.Reason)
	}
	if len(reasons) != 0 {
		t.Fatalf("refused: %s", strings.Join(reasons, "; "))
	}
	var animates []*lOverride
	for _, o := range lc.overrides {
		if !o.RemoveCreatureSub {
			continue
		}
		animates = append(animates, o)
		for _, w := range append(append([]string{}, o.AddSubtypes...), o.SetSubtypes...) {
			if strings.Contains(w, " ") {
				t.Errorf("override carries a compound subtype word %q", w)
			}
		}
	}
	if len(animates) != 2 {
		t.Fatalf("%d animate records, want 2", len(animates))
	}
	got := map[string]bool{}
	for _, o := range animates {
		for _, w := range o.AddSubtypes {
			got[w] = true
		}
	}
	for _, w := range []string{"Human", "Cleric", "Phyrexian"} {
		if !got[w] {
			t.Errorf("the animate grants lack the separate word %q (got %v)", w, got)
		}
	}
	if got["Human Cleric"] {
		t.Errorf("the compound word \"Human Cleric\" is still one subtype")
	}

	// RemoveCardTypes$ keeps the subtypes correlated with the card types the
	// split words add: "Creature Human Cleric" adds the Creature card type
	// and its own subtypes, so the creature set is replaced -- nothing kept
	// (the compound word "Human Cleric" would correlate nothing and wrongly
	// keep the creature subtypes).
	const keept = "Name:Compound Keep Probe\nManaCost:2 G\nTypes:Enchantment\n" +
		"A:AB$ Animate | Cost$ 2 | ValidTgts$ Land | Types$ Creature Human Cleric | RemoveCardTypes$ True | Duration$ Permanent | SpellDescription$ x.\n"
	lkc := lowerSynth(t, keept)
	for _, r := range lkc.Refusals {
		t.Fatalf("keep probe refused: %s", r.Reason)
	}
	var kept *lOverride
	for _, o := range lkc.overrides {
		if o.HasSetTypes {
			kept = o
		}
	}
	if kept == nil {
		t.Fatalf("the RemoveCardTypes record is missing")
	}
	if kept.Kept.Creature() {
		t.Errorf("RemoveCardTypes keeps the creature subtypes: the Types$ words did not split")
	}
}
