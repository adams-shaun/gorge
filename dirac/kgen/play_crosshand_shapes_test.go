package kgen

import (
	"strings"
	"testing"
)

// TestPlayCrossHandTargetedPlayerShapes pins the two api:Play Valid$ Card.TargetedPlayerCtrl
// spellings whose population reaches across to the targeted player: their
// hand (Mindclaw Shaman's revealed hand) and their graveyard (Sorcerous
// Squall's FishyCast, offering the instant or sorcery milled from that
// graveyard with the ReplaceGraveyard$ Exile rider). Both lower to a
// CastFree over the ability's single player target slot; the kernel reads
// the population off the slot's player (dirac/cast_freetgtplayer.go).

// playCrossHandCast is the named card's CastFree over a player target slot.
func playCrossHandCast(t *testing.T, name string) *lInstr {
	t.Helper()
	for _, c := range lowerNamed(t, []string{name}) {
		if c.Name != name {
			continue
		}
		if len(c.Refusals) != 0 {
			t.Fatalf("%s refusals = %v, want none", name, c.Refusals)
		}
		for i := range c.Abilities {
			for j := range c.Abilities[i].Prog {
				in := &c.Abilities[i].Prog[j]
				if in.Op == "CastFree" && in.Obj.Kind == rTarget {
					return in
				}
			}
		}
		t.Fatalf("%s: no CastFree over a player target slot: the spelling did not lower", name)
	}
	t.Fatalf("%s did not lower", name)
	return nil
}

func TestPlayCrossHandTargetedPlayerShapes(t *testing.T) {
	hand := playCrossHandCast(t, "Mindclaw Shaman")
	if hand.Obj.N != 0 {
		t.Errorf("Mindclaw Shaman CastFree target slot = %d, want 0", hand.Obj.N)
	}
	if hand.C&7 != zHand {
		t.Errorf("Mindclaw Shaman CastFree zone = %d, want Hand (%d)", hand.C&7, zHand)
	}

	gy := playCrossHandCast(t, "Sorcerous Squall")
	if gy.Obj.N != 0 {
		t.Errorf("Sorcerous Squall CastFree target slot = %d, want 0", gy.Obj.N)
	}
	if gy.C&7 != zGraveyard {
		t.Errorf("Sorcerous Squall CastFree zone = %d, want Graveyard (%d)", gy.C&7, zGraveyard)
	}
	for _, flag := range []string{"One", "ReplaceGY"} {
		if !strings.Contains(gy.CS, flag) {
			t.Errorf("Sorcerous Squall CastFree CS = %q, want the %s flag", gy.CS, flag)
		}
	}
	if gy.F == nil {
		t.Error("Sorcerous Squall CastFree has no ValidSA$ Instant,Sorcery filter")
	}
}
