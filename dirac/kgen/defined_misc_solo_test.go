package kgen

import (
	"strings"
	"testing"
)

// TestLowerDefinedMiscSolo covers the two Defined$ shapes this change reads:
// CopyPermanent's `Defined$ ExiledWith` (The Cloning of Shredder's chapters
// II and III) and Animate's `Defined$ Self.<qualifier>` (Iron Man Armor's
// "if this Equipment isn't a creature"). Shredder lowers whole; Iron Man's
// refusal moves past Defined$ to the staticAbilities$ grant lt-low-animate-
// grant-static owns. Each out-of-shape variant still refuses by name.
func TestLowerDefinedMiscSolo(t *testing.T) {
	all := lowerNamed(t, []string{"The Cloning of Shredder", "Iron Man Armor"})
	byName := map[string]*lCard{}
	for _, c := range all {
		byName[c.Name] = c
	}
	t.Run("The Cloning of Shredder", func(t *testing.T) {
		c := byName["The Cloning of Shredder"]
		if c == nil {
			t.Fatal("card was not lowered")
		}
		if r := lowerRefusals(c); len(r) != 0 {
			t.Fatalf("refused: %v", r)
		}
		if !c.ExiledWith {
			t.Error("the card does not record its exiled-with link (ExiledWith false)")
		}
		copies := 0
		for _, ab := range c.Abilities {
			for i := range ab.Prog {
				if ab.Prog[i].Op == "CopyExiledWith" {
					copies++
					if ab.Prog[i].F == nil {
						t.Error("CopyExiledWith has no exiled-with filter")
					}
					if ab.Prog[i].C == 0 {
						t.Error("CopyExiledWith lost the NonLegendary/AddTypes exception")
					}
				}
			}
		}
		if copies != 2 {
			t.Fatalf("%d CopyExiledWith ops, want one for each of chapters II and III", copies)
		}
	})
	t.Run("Iron Man Armor", func(t *testing.T) {
		c := byName["Iron Man Armor"]
		if c == nil {
			t.Fatal("card was not lowered")
		}
		var static, defined bool
		for _, r := range c.Refusals {
			static = static || strings.Contains(r.Reason, "Animate staticAbilities$") && strings.Contains(r.Reason, "sets no P/T")
			defined = defined || strings.Contains(r.Reason, "Defined$")
		}
		if !static {
			t.Errorf("refusal did not move to Animate staticAbilities$ (refusals %v)", c.Refusals)
		}
		if defined {
			t.Errorf("still refuses on Defined$ (refusals %v)", c.Refusals)
		}
	})
	t.Run("out of shape variants", func(t *testing.T) {
		// A Choices$ pool asks the controller to pick one of the exiled set;
		// the op copies all of it, so the shape stays unread and refuses.
		got := keclSynth(t, "Exiled Copy Pool", "Name:Exiled Copy Pool\nManaCost:1\nTypes:Sorcery\n"+
			"A:SP$ CopyPermanent | Defined$ ExiledWith | Choices$ Card\n")
		wantReason(t, got, "unread params")
		wantReason(t, got, "Choices")
		// A compound Self qualifier is not the one base the guard reads.
		got = keclSynth(t, "Self Compound", "Name:Self Compound\nManaCost:1\nTypes:Creature Human\nPT:1/1\n"+
			"A:AB$ Animate | Cost$ 1 | Defined$ Self.nonCreature & Creature | Power$ 1 | Toughness$ 1\n")
		wantReason(t, got, `Defined$ "Self.nonCreature & Creature"`)
	})
}

// wantReason requires one refusal to contain want.
func wantReason(t *testing.T, rs []refusal, want string) {
	t.Helper()
	if !strings.Contains(keclReason(rs), want) {
		t.Errorf("refusals %q lack %q", keclReason(rs), want)
	}
}
