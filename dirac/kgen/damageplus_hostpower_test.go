package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestDamagePlusHostPowerLowers: Hawkeye, Young Avenger's noncombat
// plus-X damage replacement, X the host's power, lowers as a DamagePlus
// with the host-power flag, the opponent and opponent-permanent recipients
// and the noncombat restriction; the counter-kind form stays off.
func TestDamagePlusHostPowerLowers(t *testing.T) {
	cs := lowerNamed(t, []string{"Hawkeye, Young Avenger"})
	if len(cs) != 1 || cs[0].Name != "Hawkeye, Young Avenger" {
		t.Fatalf("lowered %d cards, want Hawkeye, Young Avenger", len(cs))
	}
	c := cs[0]
	for _, r := range c.Refusals {
		t.Errorf("refused: %s", r.Reason)
	}
	want := kop.ReplDmgPowHost | kop.ReplDmgOppPlayer | kop.ReplDmgOppPerm | kop.ReplDmgNonCombat
	seen := 0
	for _, r := range c.Repls {
		if r.Kind != "DamagePlus" {
			continue
		}
		seen++
		if r.Bits != want {
			t.Errorf("DamagePlus bits %#x, want %#x", r.Bits, want)
		}
		if r.Ctr != "" || r.Bits&kop.ReplDmgCtrHost != 0 {
			t.Errorf("host power lowered as a counter read (Ctr %q, bits %#x)", r.Ctr, r.Bits)
		}
	}
	if seen != 1 {
		t.Fatalf("%d DamagePlus replacements, want 1", seen)
	}
}
