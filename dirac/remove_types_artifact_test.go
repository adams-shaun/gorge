package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestRemoveTypesArtifactStripsCorrelatedSubtype: an override RemoveTypes$
// naming the Artifact card type (Neurok Transmuter's "isn't an artifact")
// clears the type and takes the artifact subtypes with it (CR 205.1a), while
// the creature type and its subtypes stay. The subtype vocabulary is the
// table's, so the strip reads the live names without a pinned genPrep mask.
func TestRemoveTypesArtifactStripsCorrelatedSubtype(t *testing.T) {
	tbl := v.NewTable()
	tbl.Subtypes = []string{"Bear", "Equipment"}
	const equip = 1
	tbl.Cards[v.GrizzlyBears].Types = kop.TArtifact | kop.TCreature
	tbl.Cards[v.GrizzlyBears].Subtypes = kop.SubMask{}.With(equip)
	tbl.Overrides = append(tbl.Overrides, kop.Override{RemoveTypes: kop.TArtifact})
	sc := newScenTable(t, tbl, Config{})
	o := sc.perm(v.GrizzlyBears, 0)
	sl := sc.slot(o)
	if !sc.s.hasSubGen(sc.k, sl, equip) || sc.s.typesGen(sc.k, sl)&kop.TArtifact == 0 {
		t.Fatalf("precondition: the permanent is not an artifact Equipment: types %#x", sc.s.typesGen(sc.k, sl))
	}
	sc.s.applyOverride(sc.k, sl, 0, v.GrizzlyBears, false)
	if got := sc.s.typesGen(sc.k, sl); got&kop.TArtifact != 0 || got&kop.TCreature == 0 {
		t.Errorf("types after RemoveTypes Artifact = %#x, want Creature without Artifact", got)
	}
	if sc.s.hasSubGen(sc.k, sl, equip) {
		t.Error("the Equipment subtype survived the Artifact card type")
	}
}
