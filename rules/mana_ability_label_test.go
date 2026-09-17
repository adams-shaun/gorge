package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/state"
)

// The stage-1 "choose a mana ability" labels were built straight from the raw
// Produced$ script token (fb-e079def5): a Talisman-of-Indulgence-shaped wheel
// showed the jargon "Add Combo B R", which no player can read and which
// defeats the web wheel's mana-pip styling (it tints a list only when EVERY
// label is a single "Add <C>"). manaAbilityLabel renders them for humans; the
// direct unit table lives in TestManaAbilityLabelShapes below, and these two
// engine tests pin the reported flow end to end.

// TestTalismanOfIndulgenceStageOneLabelsAreHuman pins the reported card's
// flow end to end: Talisman of Indulgence carries TWO mana abilities ({T}:
// Add {C}, and {T}: Add {B} or {R} plus the 1-damage SubAbility), so its
// stage-1 wheel offers both, labelled "Add C" and "Add B or R" -- no raw
// "Combo" or "Any" token reaches a label.
func TestTalismanOfIndulgenceStageOneLabelsAreHuman(t *testing.T) {
	const talisman = "Name:Talisman of Indulgence\nTypes:Artifact\n" +
		"A:AB$ Mana | Cost$ T | Produced$ C\n" +
		"A:AB$ Mana | Cost$ T | Produced$ Combo B R | SubAbility$ DBPain\n" +
		"SVar:DBPain:DB$ DealDamage | NumDmg$ 1 | Defined$ You\nOracle:x\n"
	e, cfg, id := manaSourceEngine(t, talisman)
	activateMana(t, e, id)
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || len(d.Options) != 2 {
		t.Fatalf("stage-1 decision = %+v, want a 2-option ability choose", d)
	}
	labels := map[string]bool{}
	for _, o := range d.Options {
		if o.Obj != id || o.Kind != "mana" {
			t.Fatalf("stage-1 option %+v is not a mana option for %d", o, id)
		}
		labels[o.Label] = true
	}
	if !labels["Add C"] || !labels["Add B or R"] || len(labels) != 2 {
		t.Fatalf("stage-1 labels = %v, want exactly \"Add C\" and \"Add B or R\"", labels)
	}
	// The stage-2 colour wheel still offers the resolved single colours the
	// web wheel tints.
	submitChoices(t, e, func() int {
		for _, o := range d.Options {
			if o.Label == "Add B or R" {
				return o.Index
			}
		}
		t.Fatalf("no Add B or R option: %+v", d.Options)
		return -1
	}())
	d = e.Pending()
	if d == nil || len(d.Options) != 2 {
		t.Fatalf("stage-2 colour decision = %+v, want B/R only", d)
	}
	for _, want := range []string{"Add B", "Add R"} {
		manaOption(t, d, want[len("Add "):])
	}
	submitChoices(t, e, manaOption(t, d, "R"))
	if got := e.G.Players[0].Pool[state.MR]; got != 1 {
		t.Fatalf("pool after choosing R = %d red, want 1", got)
	}
	replayCheck(t, e, cfg)
}

// TestManaProducedAnyStageOneLabel pins the Any shape's stage-1 label on a
// two-ability source (a one-ability Any source never opens the wheel -- it
// goes straight to the five-colour ask).
func TestManaProducedAnyStageOneLabel(t *testing.T) {
	const src = "Name:Any Source\nTypes:Artifact\n" +
		"A:AB$ Mana | Cost$ T | Produced$ C\n" +
		"A:AB$ Mana | Cost$ T | Produced$ Any\nOracle:x\n"
	e, _, id := manaSourceEngine(t, src)
	activateMana(t, e, id)
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || len(d.Options) != 2 {
		t.Fatalf("stage-1 decision = %+v, want a 2-option ability choose", d)
	}
	for _, o := range d.Options {
		if o.Label == "Add Any" {
			t.Fatalf("raw Any token leaked into a stage-1 label: %+v", d.Options)
		}
		if o.Label == "Add any color" {
			continue
		}
		if o.Label != "Add C" {
			t.Fatalf("unexpected stage-1 label %q: %+v", o.Label, d.Options)
		}
	}
}

// TestManaAbilityLabelShapes is the direct unit table over the formatter,
// including the shapes no engine fixture reaches (a three-colour combo, a
// doubled pip, Chosen).
func TestManaAbilityLabelShapes(t *testing.T) {
	cases := []struct{ produced, want string }{
		{"Combo B R", "Add B or R"},
		{"Combo R G", "Add R or G"},
		{"Combo W U", "Add W or U"},
		{"Combo U B R", "Add U, B or R"},
		{"Combo R G W", "Add R, G or W"},
		{"Combo R G ", "Add R or G"}, // trailing space trimmed
		{" Any ", "Add any color"},
		{"Combo Any", "Add any color"},
		{"Chosen", "Add chosen color"},
		{"C", "Add C"},
		{"G", "Add G"},
		{"RR", "Add RR"},
	}
	for _, c := range cases {
		ma := &cards.SA{Kind: "AB", API: "Mana", Params: map[string]string{"Produced": c.produced}}
		if got := manaAbilityLabel(ma); got != c.want {
			t.Errorf("manaAbilityLabel(Produced$ %q) = %q, want %q", c.produced, got, c.want)
		}
	}
	// The formatter and the resolution's colour classifier must agree on
	// which Combo shapes are a real colour list: everything the resolver
	// would ask about renders as a colour list, never as raw jargon.
	if cols, ok := effects.ComboColours("Combo B R"); !ok || len(cols) != 2 {
		t.Fatalf("ComboColours(Combo B R) = %v, %v", cols, ok)
	}
}
