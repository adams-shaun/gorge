package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// A spell cast with flashback (or aftermath, jump-start, harmonize) that
// ward counters is exiled, not put into the graveyard (CR 702.34a: exiled
// "instead of putting it anywhere else any time it would leave the stack"),
// as effCounter's default path already did (dirac lockstep pair 13142).
func TestWardCounterExilesAFlashbackSpell(t *testing.T) {
	for _, tc := range []struct {
		flags uint64
		want  state.Zone
	}{
		{0, state.ZGraveyard},
		{state.FlagFlashback, state.ZExile},
	} {
		h := &fakeHost{g: state.NewGame(names(2))}
		spell := h.g.AddObject(mkCard(t, "Name:Spell\nTypes:Instant\nOracle:x\n"), 1)
		spell.Zone = state.ZStack // fixture setup; effects never mutate game state directly.
		spell.CastFlags = tc.flags
		c := &Ctx{Controller: 0, UnlessPay: "decline"}
		c.TriggerStack = spell.ID
		effWard(h, c, &cards.SA{API: "Ward", Params: map[string]string{"UnlessCost": "2"}})
		var to state.Zone
		for _, ev := range h.log {
			if ev.Kind == events.MoveZone && ev.Obj == spell.ID {
				to = ev.To
			}
		}
		if to != tc.want {
			t.Errorf("cast flags %b: countered by ward to %s, want %s", tc.flags, to, tc.want)
		}
	}
}
