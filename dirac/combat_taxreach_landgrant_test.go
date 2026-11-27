package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestTaxReachLandGrantLiveFaceDownSetSub: a FaceDownOnly FaceDownSetType$
// record grants its basic land type through SetSub (Yedora's Forest), and
// prepLandGrant turns blgOn on for it; landGrantLive must agree, or the tax
// reach misses the face-down Forest's intrinsic {T}: Add {G}.
func TestTaxReachLandGrantLiveFaceDownSetSub(t *testing.T) {
	forest := kop.SubMask{}.With(0)
	applier := []kop.Ability{{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatOverride, Override: 1}}}
	build := func(ov kop.Override, abs []kop.Ability) (*kop.Table, genPrep) {
		tbl := &kop.Table{
			Subtypes:  []string{"Forest"},
			Overrides: []kop.Override{ov},
			Cards:     []kop.CardDef{{Abilities: abs}},
		}
		var g genPrep
		prepLandGrant(tbl, &prep{cards: make([]pcard, 1)}, &g)
		return tbl, g
	}

	tbl, g := build(kop.Override{HasSub: true, FaceDownOnly: true, SetSub: forest}, applier)
	if !g.blgOn {
		t.Fatal("precondition: prepLandGrant left blgOn off for the FaceDownOnly SetSub record")
	}
	r := tableReach(tbl)
	if !r.override[0] {
		t.Fatal("precondition: the card's override is not reachable")
	}
	if !r.landGrantLive(tbl, &g) {
		t.Error("landGrantLive false for a Yedora-only table whose blgOn is set")
	}

	// no basic-land grant: a reachable record that sets no basic subtype
	tbl, g = build(kop.Override{HasSub: true, FaceDownOnly: true}, applier)
	g.blgOn = true // force the guard open: the scans alone must say false
	r = tableReach(tbl)
	if r.landGrantLive(tbl, &g) {
		t.Error("landGrantLive true for a table with no basic-land grant")
	}
}
