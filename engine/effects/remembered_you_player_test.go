package effects

import "testing"

// Willie Lumpkin, Postman's DBEffect remembers `RememberObjects$ You`: the
// resolving ability's controller, whose Target$ Player.IsRemembered the
// EffectOwner$'s restriction names. The capture used to yield nothing.
func TestEffectRememberedPlayersYou(t *testing.T) {
	h := newHost(t, 3)
	src := h.g.AddObject(mkCard(t, "Name:Willie\nTypes:Creature\nPT:1/1\nOracle:x\n"), 2)

	s := sa(t, "DB$ Effect | RememberObjects$ You")
	got := effectRememberedPlayers(h, &Ctx{Source: src.ID, Controller: 2}, s)
	if len(got) != 1 || got[0] != 2 {
		t.Fatalf("You = %v, want [2] (the resolving controller)", got)
	}
	// The controller, not a trigger referent, is the capture.
	c := &Ctx{Source: src.ID, Controller: 2}
	c.TriggerTarget.IsPlayer, c.TriggerTarget.Player = true, 1
	if got := effectRememberedPlayers(h, c, s); len(got) != 1 || got[0] != 2 {
		t.Fatalf("You with a trigger target = %v, want [2]", got)
	}

	none := sa(t, "DB$ Effect | StaticAbilities$ X")
	if got := effectRememberedPlayers(h, &Ctx{Source: src.ID, Controller: 2}, none); len(got) != 0 {
		t.Fatalf("empty RememberObjects$ = %v, want none", got)
	}
}
