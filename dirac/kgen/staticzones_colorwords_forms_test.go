package kgen

import (
	"strings"
	"testing"
)

// Swirl the Mists' Continuous ChangeColorWordsTo$ static lowers as
// kop.StatColorWords (lower_colorwords.go) and its pool carries
// CardDef.TextWords, so the kernel's stall rule can read which faces name a
// colour word. TestStaticZonesColorWordsForms pins the lowering; the
// kernel half runs in lockstep (dirac/oracle colorwords_lockstep_test.go).
func TestStaticZonesColorWordsForms(t *testing.T) {
	c := lowerNamedCard(t, "Swirl the Mists")
	if len(c.Refusals) != 0 {
		t.Fatalf("Swirl the Mists refusals = %v, want none", c.Refusals)
	}
	var kinds []string
	for _, s := range c.Statics {
		kinds = append(kinds, s.Kind)
	}
	if !strings.Contains(strings.Join(kinds, ","), "ColorWords") {
		t.Errorf("Swirl the Mists statics = %v, want a ColorWords static", kinds)
	}
	if !c.HasChangeText {
		t.Error("Swirl the Mists lowers without HasChangeText: the pool's cards would carry no CardDef.TextWords, so the kernel could read no colour word")
	}
}
