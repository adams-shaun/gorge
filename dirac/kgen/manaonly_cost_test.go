package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestQuirionChosenSecondLowers: Quirion Elves' second "{T}: Add one mana of
// the chosen colour" lowers beside its fixed {G} ability; the kernel reads the
// source's chosen colour from the permanent (kop.ChosenSecondManaAb,
// pcard.manaChosen) and keeps the unit in the fixed-production census pass
// (pcard.manaFixedChosen).
func TestQuirionChosenSecondLowers(t *testing.T) {
	lw := lowerNamedPool(t, []string{"Quirion Elves"})
	_, bs, _, err := buildPool(lw)
	if err != nil {
		t.Fatal(err)
	}
	var def *kop.CardDef
	for i := range bs {
		if bs[i].def.Name == "Quirion Elves" {
			def = &bs[i].def
		}
	}
	if def == nil {
		t.Fatal("Quirion Elves has no built card def")
	}
	var mana []int
	for j := range def.Abilities {
		if def.Abilities[j].Kind == kop.AbMana {
			mana = append(mana, j)
		}
	}
	if len(mana) != 2 {
		t.Fatalf("Quirion Elves lowered %d mana abilities, want 2", len(mana))
	}
	if a := &def.Abilities[mana[0]]; a.AFlags != 0 || a.ManaColors != kop.ColG {
		t.Fatalf("first ability %+v, want a plain {G} tap", a)
	}
	if !kop.ChosenSecondManaAb(def, mana[0], mana[1]) {
		t.Fatalf("second ability %+v is not the chosen-colour tap", &def.Abilities[mana[1]])
	}
}

// TestManaOnlyCostLowers: a mana ability whose cost is mana alone (Nomadic
// Elf's {1}{G}, Orochi Leafcaller's {G}, Agent of Stromgald's {R}, Bog
// Initiate's {1}, Energy Refractor's {2}) lowers as a repeatable first-ability
// filter (kop.FeeOnlyManaAb, dirac mana_filterrepeat.go); a variant with a
// SubAbility$ stays refused.
func TestManaOnlyCostLowers(t *testing.T) {
	names := []string{"Nomadic Elf", "Orochi Leafcaller", "Agent of Stromgald", "Bog Initiate",
		"Energy Refractor", "Prismite", "Crossroads Candleguide", "Signpost Scarecrow", "Urn of Godfire", "Llanowar Envoy"}
	for _, c := range lowerPoolBuilt(t, names) {
		if len(c.refusals) != 0 {
			t.Errorf("%s refusals %q, want none", c.name, c.refusals)
		}
	}
	_, bs, _, err := buildPool(lowerNamedPool(t, names))
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	for i := range bs {
		def := &bs[i].def
		if len(def.Abilities) == 0 {
			continue
		}
		seen++
		a := &def.Abilities[0]
		if a.Kind != kop.AbMana || !kop.FeeOnlyManaAb(a) || !kop.ManaAbilityHonoured(def, 0) {
			t.Errorf("%s ability 0 %+v: want a fee-only honoured mana ability", def.Name, a)
		}
	}
	if seen != len(names) {
		t.Fatalf("built %d of %d cards", seen, len(names))
	}
	got := stationProbeRefusals(t, "A:AB$ Mana | Cost$ 1 | Produced$ W | SubAbility$ DBPump | SpellDescription$ Probe.",
		"SVar:DBPump:DB$ Pump | Defined$ Self | NumAtt$ 1 | SpellDescription$ Probe pumps.")
	if !strings.Contains(got, "rider program") {
		t.Fatalf("refusals %q, want the SubAbility$ variant refused", got)
	}
}

// TestChosenSecondBesideMoreThanOneRefused: the pair shape is the bounded
// one; a third mana ability beside the chosen one stays refused by name.
func TestChosenSecondBesideMoreThanOneRefused(t *testing.T) {
	const src = "Name:Probe Elf\nManaCost:1 G\nTypes:Creature Elf\nPT:1/1\n" +
		"A:AB$ Mana | Cost$ T | Produced$ G | SpellDescription$ Probe green.\n" +
		"A:AB$ Mana | Cost$ T | Produced$ Chosen | SpellDescription$ Probe chosen.\n" +
		"A:AB$ Mana | Cost$ T | Produced$ R | SpellDescription$ Probe red.\n" +
		"K:ETBReplacement:Other:ProbeChooseColor\n" +
		"SVar:ProbeChooseColor:DB$ ChooseColor | Defined$ You | SpellDescription$ Probe chooses.\n"
	got := stationProbeRefusals(t, src)
	if !strings.Contains(got, "3 mana abilities:") {
		t.Fatalf("refusals %q, want the three-ability refusal", got)
	}
}
