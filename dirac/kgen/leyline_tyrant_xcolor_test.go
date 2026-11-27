package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestLeylineTyrantXColorLowers pins the kgen side of a printed trigger's own
// coloured-X cost: Leyline Tyrant's "you may pay any amount of {R}" is Cost$ X
// with XColor$ R, and the PayManaX cost must carry the colour (Forge's
// SpellAbility.getXColor, CR 107.3) instead of a generic X.
func TestLeylineTyrantXColorLowers(t *testing.T) {
	c := lowerCorpusCard(t, "Leyline Tyrant")
	if len(c.Refusals) != 0 {
		t.Fatalf("Leyline Tyrant refuses: %v", c.Refusals)
	}
	var found bool
	for _, ab := range c.Abilities {
		for _, in := range ab.Prog {
			if in.Op != "IfPaid" || in.CS != "ManaX" {
				continue
			}
			found = true
			col, ok := kop.PayXColor(int16(in.B.N))
			if !ok || col != uint8(kop.R) {
				t.Errorf("ManaX cost B = %d, want the red X marker (R = %d)", in.B.N, kop.R)
			}
		}
	}
	if !found {
		t.Fatalf("no IfPaid ManaX instruction in %+v", c.Abilities)
	}
}

// TestLeylineTyrantXColorGenericUnchanged: only an XColor$ names a colour; a
// plain trigger-own {X} cost keeps the generic X marker (Shanna, Purifying
// Blade) so the ordinary pay-{X} shape is untouched.
func TestLeylineTyrantXColorGenericUnchanged(t *testing.T) {
	c := lowerCorpusCard(t, "Shanna, Purifying Blade")
	if len(c.Refusals) != 0 {
		t.Fatalf("generic trigger X refuses: %v", c.Refusals)
	}
	var found bool
	for _, ab := range c.Abilities {
		for _, in := range ab.Prog {
			if in.Op == "IfPaid" && in.CS == "ManaX" {
				found = true
				if in.B.N != 0 {
					t.Errorf("generic ManaX cost B = %d, want 0", in.B.N)
				}
			}
		}
	}
	if !found {
		t.Fatalf("no IfPaid ManaX instruction in %+v", c.Abilities)
	}
}
