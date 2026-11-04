package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// Peace in Our Time's DBEffect remembers `RememberObjects$ TriggeredPlayer`:
// the player its trigger names. The capture used to yield nothing, so the
// registered CantAttack (ValidCard$ Creature.RememberedPlayerCtrl) bound nobody.
func TestEffectRememberedPlayersTriggeredPlayer(t *testing.T) {
	h := newHost(t, 2)
	src := h.g.AddObject(mkCard(t, "Name:Peace\nTypes:Enchantment\nOracle:x\n"), 0)
	s := sa(t, "DB$ Effect | RememberObjects$ TriggeredPlayer")

	c := &Ctx{Source: src.ID, Controller: 0}
	c.TriggerPlayer = state.Target{IsPlayer: true, Player: 1}
	got := effectRememberedPlayers(h, c, s)
	if len(got) != 1 || got[0] != 1 {
		t.Fatalf("TriggeredPlayer = %v, want [1] (the trigger's player, not the controller)", got)
	}

	if got := effectRememberedPlayers(h, &Ctx{Source: src.ID, Controller: 0}, s); len(got) != 0 {
		t.Fatalf("TriggeredPlayer with no trigger player = %v, want none", got)
	}
}
