package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// kremWrites sees a Draw / Surveil / SacrificeAll that
// carries the CS "Remember" as a write (the cross-ability remembered guard),
// build maps that CS to kop.MvRemember, and any other CS refuses.
func TestRememberWritesSeen(t *testing.T) {
	for _, op := range []string{"Draw", "Surveil", "SacrificeAll"} {
		if !kremWrites(lInstr{Op: op, CS: "Remember"}) {
			t.Errorf("%s with CS Remember is not a remembered-set write", op)
		}
		if kremWrites(lInstr{Op: op}) {
			t.Errorf("%s without Remember is a write", op)
		}
		var ki kop.Instr
		if err := remember3("Remember", &ki); err != nil || ki.C&kop.MvRemember == 0 {
			t.Errorf("%s: Remember -> C %d, %v", op, ki.C, err)
		}
	}
	var ki kop.Instr
	if err := remember3("Other", &ki); err == nil {
		t.Error("an unknown CS did not refuse")
	}
}
