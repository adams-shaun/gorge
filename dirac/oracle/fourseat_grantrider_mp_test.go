package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Path of Ancestry's spent-mana rider beside Chromatic Lantern's granted
// any-colour ability, in a four-seat edh pod (kfuzz -kernel edh -seats 4 -seed 2,
// pair 22, decision 3280). Rogue Refiner (a Human, beside a Human commander) is
// cast with mana from Chromatic Lantern, Path of Ancestry and Aetheric
// Amplifier while Aetherflux Conduit is out. gorge's witness activates the
// granted "Any" ability for the Path, and TriggersWhenSpent$ rides the printed
// ability's mana batch only (engine/rules/pay emitRestrictedManaSpend,
// CR 106.3 and 603.2), so the cast queues Conduit's trigger alone. The kernel's
// planner tapped the Path's own ability and queued its scry as well, which
// stacked the two triggers in one ask where gorge had one.
//
// The pin replays the recording in lockstep and requires Conduit's trigger on
// Rogue Refiner's cast (the interaction was reached). DebugGrantRiderMutate makes the
// kernel count the grant's tap for the rider again; the same game diverges.
func fourSeatGrantRiderPlay(t *testing.T) (res Result, conduitOnRefiner int) {
	t.Helper()
	res = fourSeatPodReproReplayWith(t, "path_grant_rider_four_seat_pod", func(g *GorgeSide) {
		for i := range g.E.L.Events {
			ev := &g.E.L.Events[i]
			if ev.Kind != events.TriggerPush {
				continue
			}
			o := g.E.G.Obj(ev.Obj)
			if o == nil || o.Face() == nil || o.Face().Name != "Aetherflux Conduit" || o.Zone != state.ZBattlefield {
				continue
			}
			for _, id := range ev.IDs {
				if c := g.E.G.Obj(id); c != nil && c.Face() != nil && c.Face().Name == "Rogue Refiner" {
					conduitOnRefiner++
				}
			}
		}
	})
	return res, conduitOnRefiner
}

func TestFourSeatGrantRiderLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	res, conduit := fourSeatGrantRiderPlay(t)
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if res.Decisions <= 3280 {
		t.Fatalf("%d decisions, the game must go past decision 3280", res.Decisions)
	}
	if conduit == 0 {
		t.Fatal("no Aetherflux Conduit trigger: the cast was never reached")
	}
}

func TestFourSeatGrantRiderMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugGrantRiderMutate = 1
	defer func() { dirac.DebugGrantRiderMutate = 0 }()
	res, _ := fourSeatGrantRiderPlay(t)
	if res.Div == nil {
		t.Fatal("the kernel counting a granted tap for the printed rider still agrees with gorge")
	}
}
