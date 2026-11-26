package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Cursed Mirror: an ETBReplacement Clone whose copy lasts until end of turn
// (an optional as-enters Copy replacement whose Clone chooses a creature,
// adds haste and lasts until end of turn). The as-enters copy is
// duration-bounded: the entering permanent's copy ends at the end step
// (gorge edh_etbclone_riders_test.go TestEDHETBCloneRidersCursedMirrorHastyUntilEOT).
func TestCursedMirrorLowering(t *testing.T) {
	c := lowerNamedCard(t, "Cursed Mirror")
	if len(c.Refusals) != 0 {
		t.Fatalf("Cursed Mirror refusals = %v, want none", c.Refusals)
	}
	var found *lInstr
	for i := range c.Repls {
		for j := range c.Repls[i].Prog {
			if c.Repls[i].Prog[j].Op == "BecomeCopy" {
				found = &c.Repls[i].Prog[j]
				break
			}
		}
	}
	if found == nil {
		t.Fatalf("Cursed Mirror repls = %+v, want a BecomeCopy", c.Repls)
	}
	if found.B.Kind != vConst || found.B.N&int(kop.BecomeCopyEOT) == 0 {
		t.Errorf("BecomeCopy B = %+v, want the EOT bit (BecomeCopyEOT)", found.B)
	}
	if found.C&1 == 0 {
		t.Errorf("BecomeCopy C = %d, want bit 0 (the Optional$ ask)", found.C)
	}
}
