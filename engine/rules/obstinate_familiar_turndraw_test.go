package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// The draw step's turn-based draw parked on Obstinate Familiar's optional
// draw replacement (replChoiceDraw): whichever way the active player answers,
// the step then grants THEM priority, exactly once, after the draw
// (CR 117.3a, CR 504.2; replacement_turndraw.go). Before the fix the answer
// left the upkeep's pass state in place and seat 1 received priority with
// seat 0's pass already counted.
func TestTurnDrawOptionalReplacementGrantsActivePriority(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		kind  string
		draws int
	}{
		{name: "apply", kind: "apply", draws: 0},
		{name: "decline", kind: "decline", draws: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			reg := testutil.CorpusRegistry(t)
			fam, ok := reg.Lookup("Obstinate Familiar")
			if !ok {
				t.Fatal("Obstinate Familiar missing from corpus")
			}
			deck := append([]*cards.Card{fam}, mountainDeck(t, 39)...)
			cfg := seatZeroStart(Config{Seed: 244, Names: []string{"a", "b"},
				Decks:  [][]*cards.Card{deck, mountainDeck(t, 40)},
				Tokens: map[string]*cards.Card{}})
			e := New(cfg)
			e.Advance()
			var fid state.ObjID
			for _, z := range []state.Zone{state.ZHand, state.ZLibrary} {
				for _, id := range e.G.Zone(z, 0) {
					if o := e.G.Obj(id); o.Face() != nil && o.Face().Name == "Obstinate Familiar" {
						fid = id
					}
				}
			}
			if fid == 0 {
				t.Fatal("Obstinate Familiar was not dealt")
			}
			e.emit(events.Event{Kind: events.MoveZone, Obj: fid, From: e.G.Obj(fid).Zone, To: state.ZBattlefield})
			if o := e.G.Obj(fid); o == nil || o.Zone != state.ZBattlefield {
				t.Fatal("precondition: Obstinate Familiar is not on the battlefield")
			}
			driveToStep(t, e, 3, 0, state.StepUpkeep)
			since := len(e.L.Events)
			driveToStep(t, e, 3, 0, state.StepDraw)
			d := e.Pending()
			if d == nil || d.Kind != decision.KReplacement || d.Player != 0 {
				t.Fatalf("turn draw did not park on seat 0's replacement ask: %+v", d)
			}
			drawAt := -1
			for i := since; i < len(e.L.Events); i++ {
				if e.L.Events[i].Kind == events.StepChange && e.L.Events[i].Step == state.StepDraw {
					drawAt = i
					break
				}
			}
			if drawAt < 0 {
				t.Fatal("no StepChange{StepDraw} after the upkeep")
			}
			// precondition: the upkeep left the pass state the bug inherited
			// (seat 1 holding priority, one pass counted), so the assertion
			// below can tell a fresh grant from the stale holder
			if e.G.Priority == e.G.Active && e.G.Passes == 0 {
				t.Fatalf("precondition: priority %d passes %d already reads as a fresh grant", e.G.Priority, e.G.Passes)
			}
			pick := -1
			for _, o := range d.Options {
				if o.Kind == tc.kind {
					pick = o.Index
				}
			}
			if pick < 0 {
				t.Fatalf("no %q option in %+v", tc.kind, d.Options)
			}
			if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{pick}}); err != nil {
				t.Fatalf("submit %s: %v", tc.kind, err)
			}
			draws := 0
			for _, ev := range e.L.Events[drawAt+1:] {
				if ev.Kind == events.Draw && ev.Player == 0 {
					draws++
				}
			}
			if draws != tc.draws {
				t.Fatalf("draw-step draws = %d, want %d", draws, tc.draws)
			}
			pd := e.Pending()
			if pd == nil || pd.Kind != decision.KPriority || pd.Player != 0 {
				t.Fatalf("post-draw pending = %+v, want seat 0's priority ask (CR 117.3a)", pd)
			}
			if n := drawStepPriorityRounds(t, e, drawAt, events.DecisionMade); n != 1 {
				t.Fatalf("draw-step priority rounds = %d, want exactly 1", n)
			}
			replayCheck(t, e, cfg)
		})
	}
}
