package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// hostedCmdrProjectionRepros are four-seat hosted-cmdr pod recordings (kfuzz
// -kernel hosted-cmdr -seats 4 -policy heuristic -seed 1) that each diverged at
// one object or seat projection. The recordings keep their own decks, so a
// later slice swap cannot change the game they replay.
var hostedCmdrProjectionRepros = []struct {
	name string
	at   int // the decision the recording diverged at before the fix
	// reached reports that gorge's finished game made the play the fix covers.
	reached func(g *GorgeSide) bool
}{
	// CR 702.168b, 708.2: Yedora's face-down return is no disguise cast, so no ward.
	{"face_down_return_has_no_disguise_ward", 1861, faceDownReturnOf("Experiment Twelve")},
	// CR 608.2c, 119.3: each repeated opponent's own life lost, not the sum.
	{"repeated_player_life_lost_is_their_own", 1573, endStepLifeLossOf("Archfiend of Despair")},
	// CR 111.8, 108.2b: gorge was wrong; the shuffled-in token never returns.
	{"warped_token_stays_out_of_the_dig", 2358, tokenShuffledAway},
	// CR 614.12, 708.2a: gorge was wrong; a face-down land is no creature.
	{"face_down_land_return_takes_no_creature_counter", 1259, faceDownReturnOf("Temple of the False God")},
	// CR 305.6: the same pod later, where that land funds a filter's fee.
	{"face_down_forest_funds_a_filter_fee", 1850, faceDownReturnOf("Temple of the False God")},
}

// hostedCmdrCardName is the printed name of gorge's object id.
func hostedCmdrCardName(g *GorgeSide, id state.ObjID) string {
	if o := g.E.G.Obj(id); o != nil && o.Card != nil && len(o.Card.Faces) > 0 {
		return o.Card.Faces[0].Name
	}
	return ""
}

// faceDownReturnOf: card left a graveyard for the battlefield face down.
func faceDownReturnOf(card string) func(g *GorgeSide) bool {
	return func(g *GorgeSide) bool {
		for i := range g.E.L.Events {
			e := &g.E.L.Events[i]
			if e.Kind == events.MoveZone && e.From == state.ZGraveyard && e.To == state.ZBattlefield &&
				strings.HasPrefix(e.Counter, "entered_face_down") && hostedCmdrCardName(g, e.Obj) == card {
				return true
			}
		}
		return false
	}
}

// endStepLifeLossOf: card's triggered ability resolved and some player lost life.
func endStepLifeLossOf(card string) func(g *GorgeSide) bool {
	return func(g *GorgeSide) bool {
		armed := false
		for i := range g.E.L.Events {
			e := &g.E.L.Events[i]
			switch {
			case e.Kind == events.TriggerPush && hostedCmdrCardName(g, e.Obj) == card:
				armed = true
			case armed && e.Kind == events.LifeChange && e.Amount < 0:
				return true
			}
		}
		return false
	}
}

// tokenShuffledAway: a token left the battlefield for a library.
func tokenShuffledAway(g *GorgeSide) bool {
	for i := range g.E.L.Events {
		e := &g.E.L.Events[i]
		if e.Kind == events.MoveZone && e.From == state.ZBattlefield && e.To == state.ZLibrary {
			if o := g.E.G.Obj(e.Obj); o != nil && o.IsToken {
				return true
			}
		}
	}
	return false
}

// TestHostedCmdrPodProjectionLockstep: each recording replays past its former
// divergence with every projection equal, and gorge made the play it covers.
func TestHostedCmdrPodProjectionLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	for _, c := range hostedCmdrProjectionRepros {
		t.Run(c.name, func(t *testing.T) {
			reached := false
			res := hostedCmdrPodReplay(t, c.name, func(g *GorgeSide) { reached = c.reached(g) })
			if res.Div != nil {
				t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
			}
			if !reached {
				t.Fatalf("gorge never made the play this recording covers in %d decisions", res.Decisions)
			}
			if strings.HasPrefix(res.Stall, KernelStallPrefix) || res.Decisions < c.at {
				t.Fatalf("stall %q after %d decisions, want a clean game to %d", res.Stall, res.Decisions, c.at)
			}
		})
	}
}
