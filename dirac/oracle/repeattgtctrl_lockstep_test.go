package oracle

import (
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestRepeatTargetedControllerLockstep: RepeatEach RepeatPlayers$
// TargetedController visits only the controllers of the targeted permanents,
// in AliveFrom(controller) order (gorge repeatPlayersTargeted). Seat 2 casts
// Disorienting Choice in a four-seat pod where one opponent has nothing to
// target: the loop asks the other two, in turn order from seat 2, and the
// kernel stays in lockstep. The same loop over every opponent diverges.
func TestRepeatTargetedControllerLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	for _, c := range []struct {
		name string
		bare state.PlayerID
		want []state.PlayerID
	}{
		{"seat 0 untargeted", 0, []state.PlayerID{3, 1}}, // seat order would be 1, 3
		{"seat 3 untargeted", 3, []state.PlayerID{0, 1}}, // the next seat is skipped
	} {
		t.Run(strings.ReplaceAll(c.name, " ", ""), func(t *testing.T) {
			play := dcFourSeat(t, nil, c.bare)
			for seed := uint64(1); seed <= 4; seed++ {
				res, asked := play(seed)
				if res.Div != nil {
					t.Fatalf("seed %d: divergence at %d: %s %s %v %s", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Div.Detail)
				}
				if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
					t.Fatalf("seed %d: kernel stopped: %s", seed, res.Stall)
				}
				if !slices.Equal(asked, c.want) {
					t.Fatalf("seed %d: Disorienting Choice asked %v, want %v", seed, asked, c.want)
				}
			}
			every := dcFourSeat(t, func(tbl *kop.Table) {
				n := mutateCard(t, tbl, "Disorienting Choice", func(_ *kop.Ability, in *kop.Instr) {
					if in.Op == kop.OpRepeatEachPlayer && in.Obj == kop.RefTgtControllers {
						in.Obj = kop.RefEachOpponent
					}
				})
				if n == 0 {
					t.Fatal("Disorienting Choice: no RepeatEachPlayer over RefTgtControllers in the table")
				}
			}, c.bare)
			for seed := uint64(1); seed <= 4; seed++ {
				if res, _ := every(seed); res.Div != nil {
					return
				}
			}
			t.Error("Disorienting Choice's loop over every opponent changed no game")
		})
	}
}
