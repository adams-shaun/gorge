package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestTargetControllerRefsAnySeats: a target's controller or a targeted
// player's own life-lost tally is the target's, not the caster's one
// opponent (CR 115.1, 601.2c, 119.3). The four carriers lower seat-exactly:
// Astarion and Thought-Stalker Warlock read ExLifeLostThisTurn with the
// target-controller ref; Rules of Acquisition's Draw reads the remembered
// target's controller; Legions to Ashes' same-name token clause reads target
// 0's controller through the kernel's CtrlTgt role.
func TestTargetControllerRefsAnySeats(t *testing.T) {
	names := []string{
		"Astarion, the Decadent",
		"Thought-Stalker Warlock",
		"Rules of Acquisition",
		"Legions to Ashes",
	}
	lw := lowerNamedPool(t, names)
	all, _, pt, err := buildPool(lw)
	if err != nil {
		t.Fatal(err)
	}
	cards := map[string]*lCard{}
	for _, c := range all {
		if c.Token {
			continue
		}
		cards[c.Name] = c
	}
	seen := 0
	for _, n := range names {
		c, ok := cards[n]
		if !ok {
			t.Fatalf("%s was not built", n)
		}
		seen++
		if len(c.Refusals) != 0 {
			t.Errorf("%s refused: %v", n, c.Refusals)
		}
		if c.TwoSeat {
			t.Errorf("%s is two-seat only: %v", n, c.TwoSeatWhy)
		}
		if len(c.TwoSeatWhy) != 0 {
			t.Errorf("%s names a two-seat reason: %v", n, c.TwoSeatWhy)
		}
	}
	if seen != len(names) {
		t.Fatalf("checked %d of %d cards", seen, len(names))
	}

	// Astarion and Thought-Stalker Warlock: ExLifeLostThisTurn reads the
	// target's player, never the controller's ref-less form or the opponent
	// sum.
	life := 0
	for _, e := range pt.exprs {
		if e.Kind != kop.ExLifeLostThisTurn {
			continue
		}
		life++
		if e.Ref != kop.RefTargetController {
			t.Errorf("ExLifeLostThisTurn has ref %v, want RefTargetController", e.Ref)
		}
	}
	if life == 0 {
		t.Fatal("no ExLifeLostThisTurn expression: the targeted-player life-lost lowering never ran")
	}

	// Rules of Acquisition: the third rule's Draw reads the Pump's
	// remembered target controller (the other mode's Draw is You), and it
	// runs before the GainControl so that controller is still the target's.
	draws, tgt, drawAt, gainAt := 0, 0, -1, -1
	for _, a := range cards["Rules of Acquisition"].Abilities {
		for i, in := range a.Prog {
			switch {
			case in.Op == "Draw":
				draws++
				if in.Obj.Kind == rTargetController {
					tgt++
					if drawAt < 0 {
						drawAt = i
					}
				}
			case in.Op == "GainControl" && gainAt < 0:
				gainAt = i
			}
		}
	}
	if draws == 0 {
		t.Fatal("Rules of Acquisition has no Draw instruction: the card was not lowered")
	}
	if tgt == 0 {
		t.Errorf("Rules of Acquisition's %d Draws: none reads the remembered target controller", draws)
	}
	if drawAt >= 0 && gainAt >= 0 && drawAt > gainAt {
		t.Errorf("Rules of Acquisition draws for the remembered controller at %d after the GainControl at %d: the ref reads the new controller", drawAt, gainAt)
	}

	// Legions to Ashes: the same-name token clause reads target 0's
	// controller (CtrlTgt), not OppCtrl.
	tgtCtrl := 0
	for _, f := range pt.filters {
		if f.Pred != kop.PredSameNameT0 || f.CtrlTgt == 0 {
			continue
		}
		tgtCtrl++
		if f.Ctrl != kop.CtrlAny || f.CtrlTgt != 1 {
			t.Errorf("Legions to Ashes same-name filter Ctrl %v CtrlTgt %d, want CtrlAny/1 (target 0's controller)", f.Ctrl, f.CtrlTgt)
		}
	}
	if tgtCtrl == 0 {
		t.Fatal("no PredSameNameT0 filter reads CtrlTgt: the token clause still reads OppCtrl")
	}
}
