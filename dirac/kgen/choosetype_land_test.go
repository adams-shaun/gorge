package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestChooseTypeLandAsk pins Traveler's Cloak's two halves: the as-enters
// `Type$ Land` ask lowers to the enters program's OpChooseType with
// B = kop.ChooseTypeLand (the ask over gorge's TypeChoiceLabels("Land")), and
// the static `AddKeyword$ Landwalk:ChosenType` lowers to the static's land
// filter carrying the ChosenType read the kernel resolves against the host.
func TestChooseTypeLandAsk(t *testing.T) {
	const cloak = "Name:Cloak Probe\nManaCost:2 U\nTypes:Enchantment Aura\nK:Enchant:Creature\n" +
		"K:ETBReplacement:Other:PickLandKind\n" +
		"SVar:PickLandKind:DB$ ChooseType | Defined$ You | Type$ Land | SpellDescription$ pick a land type\n" +
		"S:Mode$ Continuous | AffectedDefined$ Enchanted | Affected$ Creature | AddKeyword$ Landwalk:ChosenType | Description$ x\n"
	lc := lowerSynth(t, cloak)
	if r := lc.Refusals; len(r) != 0 {
		t.Fatalf("the Cloak probe refuses: %q", keclReason(r))
	}
	ins := chooseTypeInstrs(lc)
	if len(ins) != 1 || ins[0].B.N != kop.ChooseTypeLand || ins[0].C != 0 {
		t.Fatalf("as-enters Land lowers to %+v, want one ChooseType with B = ChooseTypeLand", ins)
	}
	if len(lc.Statics) != 1 || lc.Statics[0].Landwalk == nil {
		t.Fatalf("the grant lowers to %+v, want one static with a landwalk filter", lc.Statics)
	}
	var b strings.Builder
	dumpLIR(&b, lc)
	if !strings.Contains(b.String(), "landwalk={types=0x1 ChosenType}") {
		t.Errorf("the landwalk filter is not the Land.ChosenType read:\n%s", b.String())
	}

	// The controls: the other Type$ spellings keep their own marker, so the
	// assertion above can fail.
	for typ, want := range map[string]int{"Creature": 0, "Basic Land": kop.ChooseTypeBasicLand} {
		got := chooseTypeInstrs(lowerSynth(t, strings.Replace(cloak, "Type$ Land", "Type$ "+typ, 1)))
		if len(got) != 1 || got[0].B.N != want {
			t.Errorf("Type$ %s lowers to %+v, want B = %d", typ, got, want)
		}
	}
	// A spelling the 18-label ask cannot answer refuses, never lowers silently.
	for name, from := range map[string]string{
		"AtRandom":     "Type$ Land | AtRandom$ True",
		"InvalidTypes": "Type$ Land | InvalidTypes$ Swamp",
		"ValidTypes":   "Type$ Land | ValidTypes$ Swamp,Island",
		"Secretly":     "Type$ Land | Secretly$ True",
		"Defined":      "Type$ Land | Defined$ Player.Opponent",
	} {
		src := strings.Replace(cloak, "Defined$ You | Type$ Land", from, 1)
		if name == "Defined" {
			src = strings.Replace(cloak, "Defined$ You | Type$ Land", "Defined$ Player.Opponent | Type$ Land", 1)
		}
		if len(lowerSynth(t, src).Refusals) == 0 {
			t.Errorf("as-enters Land with %s lowers, want a refusal", name)
		}
	}
	// The grant's bare ChosenType spec is read against the static's host; a
	// Pump's KW$ list has no host choice to read, so it stays refused.
	const pump = "Name:Pump Probe\nManaCost:U\nTypes:Instant\n" +
		"A:SP$ Pump | ValidTgts$ Creature | KW$ Landwalk:ChosenType | SpellDescription$ x\n"
	if len(lowerSynth(t, pump).Refusals) == 0 {
		t.Errorf("a Pump's Landwalk:ChosenType lowers, want a refusal")
	}
}
