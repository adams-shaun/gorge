package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Sevinne's Reclamation's Optional$ Parent copy lowers to an OpMayAsk under
// MayGuardCopyParent right over the copy; Chain of Acid's (Controller$
// beside Optional$) asks the target's controller (TestCopySpellSlice).
func TestCopyParentOptional(t *testing.T) {
	found := 0
	for _, c := range lowerNamed(t, []string{"Sevinne's Reclamation", "Chain of Acid"}) {
		switch c.Name {
		case "Sevinne's Reclamation":
			found++
			if len(c.Refusals) != 0 {
				t.Fatalf("refused: %+v", c.Refusals)
			}
			prog := c.Abilities[0].Prog
			ok := false
			for i := 0; i+1 < len(prog); i++ {
				if prog[i].Op == "MayAsk" && prog[i].C == kop.MayGuardCopyParent && prog[i].Skip == 1 && prog[i+1].Op == "CopySpell" {
					ok = true
				}
			}
			if !ok {
				t.Fatalf("no MayAsk(MayGuardCopyParent) over the copy: %+v", prog)
			}
		case "Chain of Acid":
			found++
			if len(c.Refusals) != 0 {
				t.Fatalf("Chain of Acid refused: %+v", c.Refusals)
			}
		}
	}
	if found != 2 {
		t.Fatalf("lowered %d of 2 cards", found)
	}
}
