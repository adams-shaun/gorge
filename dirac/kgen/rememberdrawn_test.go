package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestRememberDrawnLowers: RememberDrawn$ on a Draw is a remembered-set
// writer, so its same-chain Remembered readers lower: Rise of the
// Deathbringer's AllReplaced (every card the draw and a replacement body
// moved) and Transcendent Archaic's True beside an OptionalDecider$.
func TestRememberDrawnLowers(t *testing.T) {
	names := []string{"Rise of the Deathbringer", "Transcendent Archaic"}
	lw := lowerNamedPool(t, names)
	all, bs, _, err := buildPool(lw)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]int{}
	for i, c := range all {
		if !c.Token {
			byName[c.Name] = i
		}
	}
	for _, name := range names {
		i, ok := byName[name]
		if !ok {
			t.Fatalf("%s: not lowered", name)
		}
		if r := lowerRefusals(all[i]); len(r) > 0 {
			t.Errorf("%s: refused: %v", name, r)
		}
	}

	// Rise of the Deathbringer remembers every card its replaced draw moved.
	rise := bs[byName["Rise of the Deathbringer"]].def
	var riseDraw bool
	for _, a := range rise.Abilities {
		for _, in := range a.Prog {
			if in.Op != kop.OpDraw {
				continue
			}
			riseDraw = true
			if in.C&kop.MvRemember == 0 || in.C&kop.DrawRememberAll == 0 {
				t.Errorf("Rise of the Deathbringer: Draw C %#x, want MvRemember|DrawRememberAll", in.C)
			}
		}
	}
	if !riseDraw {
		t.Error("Rise of the Deathbringer: no Draw instruction")
	}

	// Transcendent Archaic asks before its remembering Draw (a declined draw
	// remembers nothing), and the draw remembers only what it moved.
	arch := bs[byName["Transcendent Archaic"]].def
	var may, draw bool
	for _, a := range arch.Abilities {
		for _, in := range a.Prog {
			switch {
			case in.Op == kop.OpMayAsk && in.C == kop.MayGuardDraw:
				may = true
			case in.Op == kop.OpDraw:
				draw = true
				if in.C&kop.MvRemember == 0 || in.C&kop.DrawRememberAll != 0 {
					t.Errorf("Transcendent Archaic: Draw C %#x, want MvRemember only", in.C)
				}
			}
		}
	}
	if !may || !draw {
		t.Errorf("Transcendent Archaic: may-ask %v draw %v, want both", may, draw)
	}
}
