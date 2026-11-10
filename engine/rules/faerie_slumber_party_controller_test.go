package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestFaerieSlumberPartyCountsLastControllers drives the real corpus Faerie
// Slumber Party: "For each opponent who controlled a creature returned this
// way, create two 1/1 Faeries". The count reads each returned creature's LAST
// controller (CR 608.2h), not its owner, whose hand it is in by then.
func TestFaerieSlumberPartyCountsLastControllers(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	cases := []struct {
		name string
		// steal moves control of a named bear to a seat before casting.
		stealRuneclawToCaster, giveGrizzlyToOpp bool
		oppBoard                                 []string
		want                                     int
	}{
		{"each seat controls its own bear", false, false, []string{"Runeclaw Bear"}, 2},
		{"opponent controlled the caster's bear", true, true, []string{"Runeclaw Bear"}, 2},
		{"no opponent controlled a creature", true, false, []string{"Runeclaw Bear"}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e, cfg := miscHandsEngine(t, reg,
				[]string{"Faerie Slumber Party"}, nil,
				[]string{"Grizzly Bears"}, tc.oppBoard)
			grizzly := miscBoardObj(t, e, 0, "Grizzly Bears")
			runeclaw := miscBoardObj(t, e, 1, "Runeclaw Bear")
			wantCtrl0, wantCtrl1 := state.PlayerID(0), state.PlayerID(1)
			if tc.stealRuneclawToCaster {
				e.emit(events.Event{Kind: events.ControlChange, Obj: runeclaw, Player: 0})
				wantCtrl1 = 0
			}
			if tc.giveGrizzlyToOpp {
				e.emit(events.Event{Kind: events.ControlChange, Obj: grizzly, Player: 1})
				wantCtrl0 = 1
			}
			// PRECONDITION: the controllers the case set up, with owners unchanged.
			if g, r := e.G.Obj(grizzly), e.G.Obj(runeclaw); g.Controller != wantCtrl0 || r.Controller != wantCtrl1 ||
				g.Owner != 0 || r.Owner != 1 || g.Zone != state.ZBattlefield || r.Zone != state.ZBattlefield {
				t.Fatalf("setup: grizzly ctrl %d owner %d zone %v, runeclaw ctrl %d owner %d zone %v",
					g.Controller, g.Owner, g.Zone, r.Controller, r.Owner, r.Zone)
			}

			addMana(t, e, 0, "UUUUUU")
			party := miscHandObj(t, e, 0, "Faerie Slumber Party")
			submitChoices(t, e, miscCastOption(t, e, party))
			for i := 0; i < 60 && (stackHasObj(e, party) || e.G.Obj(party).Zone == state.ZHand); i++ {
				d := e.Pending()
				if d == nil {
					t.Fatalf("no decision pending while resolving (step %d)", i)
				}
				if d.Kind == decision.KPriority {
					passPriority(t, e)
					continue
				}
				var cs []int
				for j := 0; j < d.Min && j < len(d.Options); j++ {
					cs = append(cs, d.Options[j].Index)
				}
				submitChoices(t, e, cs...)
			}
			if stackHasObj(e, party) {
				t.Fatal("Faerie Slumber Party is still on the stack")
			}
			// The count is read after the move: both bears are in their OWNERS' hands.
			if g, r := e.G.Obj(grizzly), e.G.Obj(runeclaw); g.Zone != state.ZHand || g.Owner != 0 ||
				r.Zone != state.ZHand || r.Owner != 1 {
				t.Fatalf("after resolution: grizzly zone %v owner %d, runeclaw zone %v owner %d; want both in their owners' hands",
					g.Zone, g.Owner, r.Zone, r.Owner)
			}
			tokens := 0
			for _, id := range e.G.Zone(state.ZBattlefield, 0) {
				if o := e.G.Obj(id); o != nil && o.IsToken {
					tokens++
				}
			}
			if tokens != tc.want {
				t.Fatalf("seat 0 Faerie tokens = %d, want %d", tokens, tc.want)
			}
			replayCheck(t, e, cfg)
		})
	}
}
