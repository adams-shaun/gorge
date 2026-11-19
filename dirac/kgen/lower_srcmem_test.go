package kgen

import (
	"strings"
	"testing"
)

// TestSrcMemShapes: a card whose leaves or watch trigger reads what another
// ability of the card remembered lowers with CardDef.SrcMem (the kernel's
// per-source memory, dirac/zone_srcmem.go); a Remembered written outside the
// whitelisted writers stays refused; Hostage Taker now lowers through OpExileUntilLeaves remembering the exiled card (TestRemExileUntilRememberFourSeatLockstep), not through SrcMem.
func TestSrcMemShapes(t *testing.T) {
	for _, name := range []string{"Dance of Many", "Hypnox"} {
		lc := lowerCorpusCard(t, name)
		if r := probeRefusals(lc); r != "" {
			t.Errorf("%s refused: %s", name, r)
			continue
		}
		if !lc.SrcMem || lc.SmSeed == 0 {
			t.Errorf("%s: SrcMem %v SmSeed %#x, want SrcMem with a non-zero seed", name, lc.SrcMem, lc.SmSeed)
		}
		if lc.RemPersist {
			t.Errorf("%s: RemPersist set beside SrcMem", name)
		}
	}
	hostage := lowerCorpusCard(t, "Hostage Taker")
	if r := probeRefusals(hostage); r != "" {
		t.Errorf("Hostage Taker refused: %s (lowers since the RememberChanged exile-until-leaves writer)", r)
	}
	if hostage.SrcMem {
		t.Errorf("Hostage Taker carries SrcMem")
	}
	_ = strings.Contains
}
