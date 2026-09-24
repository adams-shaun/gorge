package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/state"
)

// The paired riders (Dazzling Sphinx/Chaos Wand) remember the whole revealed
// prefix, including the found card, without retaining a trigger capture.
// RememberRevealed$ alone still appends to the pre-existing Remembered set.
func TestDigUntilRememberFoundAndRevealedPreserveRevealedPrefix(t *testing.T) {
	for _, tc := range []struct {
		name, riders string
	}{
		{"both", "RememberFound$ True | RememberRevealed$ True"},
		{"revealed only", "RememberRevealed$ True"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, ids := digUntilFixture(t)
			trigger := h.g.Zone(state.ZBattlefield, 0)[0]
			ctx := &Ctx{Controller: 0, Remembered: []state.Target{{Obj: trigger}}, Captured: []state.Target{{Obj: trigger}}}
			lib := h.g.Zone(state.ZLibrary, 0)
			battlefield := h.g.Zone(state.ZBattlefield, 0)
			if trigger == ids[0] || trigger == ids[1] || len(battlefield) != 1 || battlefield[0] != trigger ||
				len(lib) < 2 || lib[0] != ids[0] || lib[1] != ids[1] ||
				h.g.Obj(ids[0]).Zone != state.ZLibrary || h.g.Obj(ids[1]).Zone != state.ZLibrary ||
				MatchesSpecCtx(h.g, "Aura", ids[0], ctx.SpecContext(0)) || !MatchesSpecCtx(h.g, "Aura", ids[1], ctx.SpecContext(0)) ||
				len(ctx.Remembered) != 1 || ctx.Remembered[0].Obj != trigger || len(ctx.Captured) != 1 || ctx.Captured[0].Obj != trigger {
				t.Fatalf("invalid trigger/reveal fixture: trigger=%d library=%v remembered=%v captured=%v", trigger, lib, ctx.Remembered, ctx.Captured)
			}
			Resolve(h, ctx, sa(t, "SP$ DigUntil | Valid$ Aura | FoundDestination$ Exile | RevealedDestination$ Exile | "+tc.riders))
			if h.g.Obj(ids[1]).Zone != state.ZExile || h.g.Obj(ids[0]).Zone != state.ZExile {
				t.Fatalf("revealed prefix did not move to exile: land=%s aura=%s", h.g.Obj(ids[0]).Zone, h.g.Obj(ids[1]).Zone)
			}
			want := []state.ObjID{ids[0], ids[1]}
			if tc.name == "revealed only" {
				want = append([]state.ObjID{trigger}, want...)
			}
			if len(ctx.Remembered) != len(want) {
				t.Fatalf("Remembered = %v, want ids %v", ctx.Remembered, want)
			}
			for i, id := range want {
				if ctx.Remembered[i] != (state.Target{Obj: id}) {
					t.Fatalf("Remembered[%d] = %+v, want object %d (all=%v)", i, ctx.Remembered[i], id, ctx.Remembered)
				}
			}
			if len(ctx.Captured) != 1 || ctx.Captured[0] != (state.Target{Obj: trigger}) {
				t.Fatalf("Captured = %v, want trigger %d", ctx.Captured, trigger)
			}
		})
	}
}
