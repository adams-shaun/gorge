package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestMotherLodeLowering pins Hit the Mother Lode's lowering: Discover 10
// (RememberDiscovered$ True) with a Treasure tail — TokenAmount$
// Remembered$CardManaCost/NMinus.10 gated on the discovered card's mana
// value — then Cleanup$ ClearRemembered. The kernel parks the resolution at
// the free cast's election and runs the tail after the casts are announced
// (act_discoverfreecast.go), so the tail lowers as any program tail.
func TestMotherLodeLowering(t *testing.T) {
	all, bs, _, err := buildPool(lowerNamedPool(t, []string{"Hit the Mother Lode"}))
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	for i, c := range all {
		if c.Name != "Hit the Mother Lode" {
			continue
		}
		seen++
		if rs := lowerRefusals(c); len(rs) != 0 {
			t.Fatalf("Hit the Mother Lode refused: %v", rs)
		}
		var spell *kop.Ability
		for j := range bs[i].def.Abilities {
			if len(bs[i].def.Abilities[j].Prog) > 0 {
				if spell != nil {
					t.Fatalf("more than one ability with a program")
				}
				spell = &bs[i].def.Abilities[j]
			}
		}
		if spell == nil {
			t.Fatalf("Hit the Mother Lode built without a program")
		}
		var foundCastFree, foundHand bool
		for _, in := range spell.Prog {
			switch in.Op {
			case kop.OpDiscover:
				if in.C == 1 {
					foundHand = true // the uncast card to hand
				}
			case kop.OpCastFree:
				foundCastFree = true
			}
		}
		if !foundCastFree || !foundHand {
			t.Fatalf("built program lacks the free cast (%v) or the hand tail (%v)", foundCastFree, foundHand)
		}
		// The token amount reads the discovered card's mana value against
		// 10 (the shortfall, TokenAmount$ Remembered$CardManaCost/NMinus.10):
		// the built value must read the remembered card's MV, not a constant.
		tok := false
		for _, in := range spell.Prog {
			if in.Op != kop.OpCreateToken {
				continue
			}
			tok = true
			if in.A.K != kop.VExpr {
				t.Fatalf("CreateToken amount value kind %d n %d, want VExpr (the MV expression)", in.A.K, in.A.N)
			}
		}
		if !tok {
			t.Fatalf("built program lacks the CreateToken")
		}
	}
	if seen != 1 {
		t.Fatalf("lowered %d Hit the Mother Lode cards, want 1", seen)
	}
}
