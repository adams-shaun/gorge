package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestRaiseSacOfferMatchesTheTargetAsk pins that an activation the offer walk
// admits is not reversed at the target ask (CR 601.2c, 602.2b): Brutal
// Suppression's "Sacrifice a land" per copy may sacrifice a land that is also
// tapped for the mana (costs are paid in any order, CR 601.2h), so N copies
// need max(mana lands, N) lands, not 2N; and Oppressive Rays' {3} on a
// {T}-only ability is payable from three untapped lands.
func TestRaiseSacOfferMatchesTheTargetAsk(t *testing.T) {
	t.Parallel()
	type row struct {
		name    string
		bearer  string
		copies  int
		lands   int
		rays    bool
		offered bool
	}
	rows := []row{
		// Brutal Suppression, {3} Rebel Informer: max(3, N) lands.
		{"informer N=1 3 lands", "Rebel Informer", 1, 3, false, true},
		{"informer N=1 2 lands", "Rebel Informer", 1, 2, false, false},
		{"informer N=2 3 lands", "Rebel Informer", 2, 3, false, true},
		{"informer N=2 2 lands", "Rebel Informer", 2, 2, false, false},
		{"informer N=3 3 lands", "Rebel Informer", 3, 3, false, true},
		{"informer N=4 4 lands", "Rebel Informer", 4, 4, false, true},
		{"informer N=4 3 lands", "Rebel Informer", 4, 3, false, false},
		// Brutal Suppression, {T}-only Blightspeaker: N lands.
		{"blightspeaker N=1 1 land", "Blightspeaker", 1, 1, false, true},
		{"blightspeaker N=2 2 lands", "Blightspeaker", 2, 2, false, true},
		{"blightspeaker N=2 1 land", "Blightspeaker", 2, 1, false, false},
		// Oppressive Rays: {3} added to a {T}-only ability.
		{"rays doomsayers 3 lands", "Errant Doomsayers", 0, 3, true, true},
		{"rays doomsayers 2 lands", "Errant Doomsayers", 0, 2, true, false},
		{"rays doomsayers 8 lands", "Errant Doomsayers", 0, 8, true, true},
		{"rays blightspeaker 5 lands", "Blightspeaker", 0, 5, true, true},
		{"rays mercenary 3 lands", "Mercenary Informer", 0, 3, true, false},
		{"rays mercenary 6 lands", "Mercenary Informer", 0, 6, true, true},
	}
	for _, r := range rows {
		r := r
		t.Run(r.name, func(t *testing.T) {
			t.Parallel()
			fixtures := []string{r.bearer}
			if r.copies > 0 {
				for i := 0; i < r.copies; i++ {
					fixtures = append(fixtures, "Brutal Suppression")
				}
			}
			e, _ := raiseEngine(t, fixtures, nil)
			for i := 0; i < r.copies; i++ {
				blightMove(t, e, 0, "Brutal Suppression", state.ZBattlefield)
			}
			bearer := blightMove(t, e, 0, r.bearer, state.ZBattlefield)
			e.G.Obj(bearer).SummonSick = false
			if r.rays {
				raysOn(t, e, 0, bearer)
			}
			basics := []string{"Plains", "Island", "Swamp", "Mountain", "Forest"}
			var lands []state.ObjID
			for i := 0; i < r.lands; i++ {
				lands = append(lands, blightMove(t, e, 0, basics[i%len(basics)], state.ZBattlefield))
			}
			idx := 0
			if r.bearer == "Blightspeaker" {
				idx = 0 // {T}: target player loses 1 life
			}
			opt, ok := findAbilityOption(e, bearer, idx)
			if ok != r.offered {
				t.Fatalf("ability offered = %v, want %v (%d lands, %d copies)", ok, r.offered, r.lands, r.copies)
			}
			if !ok {
				return
			}
			submitChoices(t, e, opt.Index)
			raiseDrive(t, e, func(d *decision.Decision) []int { return nil })
			if len(e.G.Stack) != 1 {
				t.Fatalf("offered activation left %d stack objects, want 1 (reversed at the target ask)", len(e.G.Stack))
			}
			sacked := 0
			for _, l := range lands {
				if raiseZone(e, l) == state.ZGraveyard {
					sacked++
				}
			}
			if sacked != r.copies {
				t.Fatalf("%d lands sacrificed, want %d", sacked, r.copies)
			}
		})
	}
}
