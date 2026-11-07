package kgen

import (
	"strings"
	"testing"
)

// TestShareLandTypeLowers: Myriad Landscape's two-land search lowers with its
// shared-type rider read (the kernel's SrShareLandType), and the rider over a
// filter that admits a nonbasic land, or with another value, refuses by name.
func TestShareLandTypeLowers(t *testing.T) {
	cs := lowerNamed(t, []string{"Myriad Landscape"})
	if len(cs) != 1 || cs[0].Name != "Myriad Landscape" {
		t.Fatalf("lowered %d cards, want Myriad Landscape", len(cs))
	}
	for _, r := range cs[0].Refusals {
		t.Errorf("refused: %s", r.Reason)
	}
	var sb strings.Builder
	dumpLIR(&sb, cs[0])
	if !strings.Contains(sb.String(), "ShareLandType") {
		t.Errorf("lowered IR lacks the ShareLandType flag:\n%s", sb.String())
	}
	for _, c := range []struct{ name, root, want string }{
		{"nonbasic filter", "SP$ ChangeZone | Cost$ B | Origin$ Library | Destination$ Battlefield | ChangeType$ Land | ChangeNum$ 2 | ShareLandType$ True", "admits a nonbasic card"},
		{"False", "SP$ ChangeZone | Cost$ B | Origin$ Library | Destination$ Battlefield | ChangeType$ Land.Basic | ChangeNum$ 2 | ShareLandType$ False", `ShareLandType$ "False"`},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, _ := czLowered(t, c.root)
			kdefWant(t, got, c.want)
		})
	}
}
