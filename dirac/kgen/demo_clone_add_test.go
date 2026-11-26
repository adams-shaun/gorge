package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestDemoCloneAddShapes: the three Clone forms beyond the battlefield lower
// with no refusal, each as the representation its shape carries
// (lower_clonecard.go): Saheeli's AddTypes$ a type exception on the copy,
// Shifting Woodland's TgtZone$ Graveyard target a CloneTmplCard template,
// Curie's Defined$ Exiled a CloneTmplSac template with a CloneGrant record
// that grants her damage trigger, and that trigger's draw reads
// kop.ExCardBasePower.
func TestDemoCloneAddShapes(t *testing.T) {
	names := []string{"Curie, Emergent Intelligence", "Saheeli, Sublime Artificer", "Shifting Woodland"}
	lw := lowerNamedPool(t, names)
	if len(lw.cards) != len(names) {
		t.Fatalf("lowered %d cards, want %d", len(lw.cards), len(names))
	}
	for _, c := range lw.cards {
		for _, r := range c.Refusals {
			t.Errorf("%s refused: %s", c.Name, r.Reason)
		}
	}
	_, bs, pt, err := buildPool(lw)
	if err != nil {
		t.Fatal(err)
	}
	flags := map[string]int{}
	basePower := false
	for _, b := range bs {
		for _, r := range b.refusals {
			t.Errorf("%s build refusal: %v", b.def.Name, r)
		}
		for _, ab := range b.def.Abilities {
			for _, in := range ab.Prog {
				if in.Op == kop.OpExt && in.Ext() == kop.ExtCloneCopy {
					flags[b.def.Name] |= int(in.B.N)
				}
			}
		}
	}
	want := map[string]int{
		"Saheeli, Sublime Artificer":   kop.CloneEOT | 1<<kop.CloneTmplShift, // the template is target slot 1
		"Shifting Woodland":            kop.CloneEOT | kop.CloneTmplCard,
		"Curie, Emergent Intelligence": kop.CloneTmplSac | kop.CloneGrant,
	}
	for n, w := range want {
		if flags[n] != w {
			t.Errorf("%s: ExtCloneCopy flags %#x, want %#x", n, flags[n], w)
		}
	}
	addTypes, grant := false, false
	for i := range pt.overrides {
		addTypes = addTypes || pt.overrides[i].AddTypes != 0
		grant = grant || pt.overrides[i].Grant != [2]uint8{}
	}
	for _, x := range pt.exprs {
		basePower = basePower || x.Kind == kop.ExCardBasePower
	}
	if !addTypes {
		t.Error("Saheeli: no built override adds a type")
	}
	if !grant {
		t.Error("Curie: no built override grants her trigger")
	}
	if !basePower {
		t.Error("Curie: no built expression reads kop.ExCardBasePower")
	}
}
