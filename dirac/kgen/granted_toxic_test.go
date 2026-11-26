package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestGrantedToxicShapes: the six cards that grant Toxic N rather than print
// it lower with no refusal (CR 702.164c, dirac/kw_toxicgrant.go), each as the
// representation its grant shape carries: a Pump / PumpAll KW$ term is an
// ExtKwMark KwMarkToxic with N in C's high byte, a Continuous AddKeyword$
// term kop.Static.Toxic, an Animate Keywords$ term kop.Override.Toxic.
func TestGrantedToxicShapes(t *testing.T) {
	names := []string{"Aspirant's Ascent", "Plague Nurse", "Monument to Perfection", "Karumonix, the Rat King", "Necrogen Communion", "Prosthetic Injector"}
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
	marks := map[string]int{}
	statics := map[string]uint8{}
	for _, b := range bs {
		for _, r := range b.refusals {
			t.Errorf("%s build refusal: %v", b.def.Name, r)
		}
		for _, ab := range b.def.Abilities {
			if ab.Kind == kop.AbStatic && ab.Static.Toxic != 0 {
				statics[b.def.Name] += ab.Static.Toxic
			}
			for _, in := range ab.Prog {
				if in.Op == kop.OpExt && in.Ext() == kop.ExtKwMark && in.C&kop.KwMarkToxic != 0 {
					marks[b.def.Name] += int(in.C) >> kop.KwMarkToxicShift
				}
			}
		}
	}
	wantMarks := map[string]int{"Aspirant's Ascent": 1, "Plague Nurse": 1}
	wantStatics := map[string]uint8{"Karumonix, the Rat King": 1, "Necrogen Communion": 2, "Prosthetic Injector": 1}
	for n, w := range wantMarks {
		if marks[n] != w {
			t.Errorf("%s: KwMarkToxic amount %d, want %d", n, marks[n], w)
		}
	}
	for n, w := range wantStatics {
		if statics[n] != w {
			t.Errorf("%s: Static.Toxic %d, want %d", n, statics[n], w)
		}
	}
	monument := false
	for i := range pt.overrides {
		if pt.overrides[i].Toxic == 9 {
			monument = true
		}
	}
	if !monument {
		t.Error("Monument to Perfection: no built override carries Toxic 9")
	}
}
