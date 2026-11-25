package kgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// manaConvertForms: the ManaConvert static shapes kgen lowers (CR 609.4b;
// gorge rules/mana_convert.go manaConversionParts). Oath of Nissa is the
// spell-scoped any-colour twin of Vizier's any-type one, its Filter the
// ValidCard$ planeswalker spec; Case File Auditor carries the same static
// over Case.YouCtrl and now refuses only on its CaseSolved trigger.
var manaConvertForms = []formCase{
	{card: "Oath of Nissa", form: "ManaConvert Planeswalker.YouCtrl AnyType->AnyColor (StatAnyColorSpells)", lir: "static ManaConvert F={types=0x10 ctrl=1}"},
	{card: "Vizier of the Menagerie", form: "ManaConvert Creature.YouCtrl AnyType->AnyType (StatAnyTypeCreatures)", lir: "static ManaConvert F=nil"},
	{card: "Chromatic Orrery", form: "ManaConvert AnyType->AnyColor, no ValidSA/ValidCard (StatAnyColorAll)", lir: "static ManaConvert F=nil"},
	{card: "Mycosynth Lattice", form: "ManaConvert with no ValidPlayer$ (and an AffectedZone$ All static) stays refused", refused: `AffectedZone "All" without MayPlay; S[2] Mode=ManaConvert: static mode ManaConvert`},
	{card: "Case File Auditor", form: "ManaConvert Case.YouCtrl lowers; the CaseSolved trigger still refuses", refused: "trigger mode CaseSolved"},
}

func TestManaConvertForms(t *testing.T) {
	checkForms(t, manaConvertForms)
}

// TestManaConvertAuditorOnlyRefusesTrigger: the Auditor's static lowers, so
// "static mode ManaConvert" is not among its refusals.
func TestManaConvertAuditorOnlyRefusesTrigger(t *testing.T) {
	cardsDir := ratchetCards(t)
	list := filepath.Join(t.TempDir(), "cards.txt")
	if err := os.WriteFile(list, []byte("Case File Auditor\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := loadCardList(list, cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range lowerPool(p).cards {
		if c.Name != "Case File Auditor" {
			continue
		}
		var got []string
		for _, r := range c.Refusals {
			got = append(got, r.Node+": "+r.Reason)
		}
		if len(got) == 0 {
			t.Fatal("Case File Auditor lowered with no refusal: its CaseSolved trigger should still refuse")
		}
		if j := strings.Join(got, "; "); strings.Contains(j, "static mode ManaConvert") {
			t.Errorf("static ManaConvert still refuses: %s", j)
		}
		return
	}
	t.Fatal("Case File Auditor not lowered")
}

// TestManaConvertBuildsKinds: the built static is StatAnyColorSpells with a
// Filter for Oath of Nissa (the ValidCard$ spec, a planeswalker), and
// StatAnyTypeCreatures with none for Vizier of the Menagerie, and the unscoped
// StatAnyColorAll with none for Chromatic Orrery.
func TestManaConvertBuildsKinds(t *testing.T) {
	lw := lowerNamedPool(t, []string{"Oath of Nissa", "Vizier of the Menagerie", "Chromatic Orrery"})
	all, bs, _, err := buildPool(lw)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]kop.StaticKind{"Oath of Nissa": kop.StatAnyColorSpells, "Vizier of the Menagerie": kop.StatAnyTypeCreatures, "Chromatic Orrery": kop.StatAnyColorAll}
	seen := map[string]int{}
	for i, c := range all {
		kind, ok := want[c.Name]
		if !ok {
			continue
		}
		for _, a := range bs[i].def.Abilities {
			if a.Kind != kop.AbStatic || a.Static.Kind != kind {
				continue
			}
			seen[c.Name]++
			if (a.Static.Filter != 0) != (kind == kop.StatAnyColorSpells) {
				t.Errorf("%s: static %d has Filter %d", c.Name, kind, a.Static.Filter)
			}
		}
	}
	for n := range want {
		if seen[n] != 1 {
			t.Errorf("%s: %d built statics of kind %d, want 1", n, seen[n], want[n])
		}
	}
}
