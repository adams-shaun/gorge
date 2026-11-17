package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestDauthiVoidwalkerLowers: Dauthi Voidwalker's any-zone dies-to-exile
// replacement carries WithCountersType$ VOID on its ChangeZone body, so it
// lowers to one ReplExileOnLeave with C = 1 (every zone) and A = the counter
// (VOID in the low byte, 1 above it). Its activated ability lowers the
// filtered exile ChooseCard and the end-of-turn free play of the chosen card.
func TestDauthiVoidwalkerLowers(t *testing.T) {
	cs := lowerNamed(t, []string{"Dauthi Voidwalker"})
	if len(cs) != 1 {
		t.Fatalf("lowered %d cards, want 1", len(cs))
	}
	c := cs[0]
	for _, r := range c.Refusals {
		t.Errorf("Dauthi Voidwalker refused: %s", r.Reason)
	}
	if len(c.Repls) != 1 {
		t.Fatalf("repls = %+v, want one ExileOnLeave", c.Repls)
	}
	r := c.Repls[0]
	if r.Kind != "ExileOnLeave" {
		t.Fatalf("replacement kind = %q, want ExileOnLeave", r.Kind)
	}
	if !r.AnyZone {
		t.Error("the replacement is not any-zone; Dauthi exiles a card from anywhere")
	}
	if r.Ctr != "VOID" || r.CtrN != 1 {
		t.Errorf("replacement counter = %q x%d, want VOID x1", r.Ctr, r.CtrN)
	}

	for _, in := range effectProg(c) {
		switch in.Op {
		case "ChooseCard":
			if in.F == nil {
				t.Error("ChooseCard has no Choices$ filter")
			} else if in.C != zExile {
				t.Errorf("ChooseCard zone = %d, want Exile", in.C)
			}
		case "MayPlay":
			if in.Obj.Kind != rChosen || in.CS != "EOT" || !in.B.isConst() || in.B.N&kop.MayPlayFree == 0 {
				t.Errorf("MayPlay = %v, want the chosen card free to the end of the turn", in)
			}
		}
	}

	_, bs, _, err := buildPool(&lowered{cards: cs})
	if err != nil {
		t.Fatal(err)
	}
	if len(bs[0].refusals) != 0 {
		t.Fatalf("build refusals = %v", bs[0].refusals)
	}
	var repl *kop.Repl
	choose, mayplay := false, false
	for i := range bs[0].def.Abilities {
		ab := &bs[0].def.Abilities[i]
		if ab.Kind == kop.AbReplacement && ab.Repl.Kind == kop.ReplExileOnLeave {
			repl = &ab.Repl
		}
		for _, in := range ab.Prog {
			switch in.Op {
			case kop.OpChooseCard:
				choose = in.F != 0
			case kop.OpMayPlay:
				mayplay = in.Obj == kop.RefChosen && in.C == int16(kop.ZoneExile) &&
					in.B.N&kop.MayPlayFree != 0 && in.A.N == kop.DurEOT
			}
		}
	}
	if repl == nil {
		t.Fatal("no built ReplExileOnLeave")
	}
	if repl.C != 1 {
		t.Errorf("built any-zone flag C = %d, want 1", repl.C)
	}
	if kind, n := kop.CounterKind(uint8(repl.A)), uint8(uint16(repl.A)>>8); kind != kop.CtrVoid || n != 1 {
		t.Errorf("built counter = kind %d x%d, want VOID x1", kind, n)
	}
	if !choose {
		t.Error("Dauthi's activated ability does not lower to OpChooseCard over exile")
	}
	if !mayplay {
		t.Error("Dauthi's grant does not lower to a free OpMayPlay of the chosen card")
	}
}
