package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// convokeMarshalSrc is the fixture commander: a decision-free 1/1 with a
// plain {4} cost and Convoke (CR 702.51). No coloured pips and no abilities
// keep every cast decision-free except the Convoke announcement itself, so
// the tax arithmetic below is the only thing in play.
const convokeMarshalSrc = `Name:Convoke Marshal
ManaCost:4
Types:Legendary Creature Human Soldier
PT:1/1
K:Convoke
Oracle:x
`

// sparedRecruitSrc is a colourless, ability-free 1/1: an untapped creature
// that can Convoke {1} of a total cost and nothing else.
const sparedRecruitSrc = `Name:Spared Recruit
ManaCost:1
Types:Creature Human
PT:1/1
Oracle:x
`

// convokeMarshalGameSeats drives a Commander game where seat 0 leads with the
// Convoke Marshal and seat 1 with the decision-free Beatstick, to turn 1
// Main1 of seat 0 (the only point a sorcery-speed command-zone cast is
// offered), and returns the engine with seat 0's commander extracted.
func convokeMarshalGame(t *testing.T, seed uint64) (*Engine, state.ObjID) {
	t.Helper()
	e, _ := commanderGame(t, seed, FormatCommander, 0, [][]string{
		{convokeMarshalSrc},
		{commanderBeatstickSrc},
	})
	e.Advance()
	driveToStep(t, e, 1, 0, state.StepMain1)
	return e, e.G.Players[0].Commanders[0]
}

// marshalFirstCastCycle drives the marshal's first command-zone cast through
// the real legal-action path with the pool paying the whole {4} (the Convoke
// ask is DECLINED -- Min 0 makes the empty announcement legal), then returns
// it to the command zone the way castCommanderAndReturn does. Unlike that
// helper it can cross the Convoke ask: passUntilStackEmpty fails closed on a
// non-priority decision it does not know.
func marshalFirstCastCycle(t *testing.T, e *Engine, id state.ObjID) {
	t.Helper()
	opt := commanderCastOption(e, id)
	if opt == nil {
		t.Fatalf("precondition: first untaxed cast not offered: %+v", e.Pending().Options)
	}
	submitChoices(t, e, opt.Index)
	for n := 0; n < 30; n++ {
		d := e.Pending()
		if d == nil {
			break
		}
		if d.Kind == decision.KChoose {
			// The Convoke announcement: the pool pays the whole printed cost,
			// so the empty (decline) answer must be legal -- assert that, do
			// not assume it.
			if d.Min > 0 {
				t.Fatalf("precondition: Convoke ask demands Min %d with a fully funded pool: %+v", d.Min, d)
			}
			if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{}}); err != nil {
				t.Fatalf("decline the Convoke ask: %v", err)
			}
			continue
		}
		if d.Kind != decision.KPriority {
			t.Fatalf("non-priority decision while draining: %+v", d)
		}
		if len(e.G.Stack) == 0 {
			// The marshal resolved; stop before the passes run the turn out
			// (passUntilStackEmpty's own stop condition).
			break
		}
		idx := -1
		for _, o := range d.Options {
			if o.Kind == "pass" {
				idx = o.Index
			}
		}
		if idx < 0 {
			t.Fatalf("priority decision with no pass: %+v", d.Options)
		}
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{idx}}); err != nil {
			t.Fatalf("pass: %v", err)
		}
	}
	if len(e.G.Stack) != 0 {
		t.Fatalf("stack not drained: %+v", e.G.Stack)
	}
	onField := false
	for _, oid := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(oid); o != nil && o.Face() != nil && o.Face().Name == "Convoke Marshal" {
			onField = true
		}
	}
	if !onField {
		t.Fatalf("precondition: the marshal never reached the battlefield")
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZBattlefield, To: state.ZCommand})
	e.pending = nil
	e.Advance()
}

// addRecruits puts `n` untapped Spared Recruits onto p's battlefield.
func addRecruits(t *testing.T, e *Engine, p state.PlayerID, n int) []state.ObjID {
	t.Helper()
	var ids []state.ObjID
	for i := 0; i < n; i++ {
		o := e.G.AddObject(card(t, sparedRecruitSrc), p)
		o.Zone = state.ZBattlefield
		ids = append(ids, o.ID)
	}
	e.G.SetZone(state.ZBattlefield, p, ids)
	return ids
}

// TestCommanderConvokePaysTax is the CR 702.51a / 903.8 money test: a
// commander cast before from the command zone carries its {2} tax, and
// Convoke's untapped creatures may pay the TOTAL cost -- the tax included,
// not just the printed cost. With lands (here: the floated pool) for the
// printed {4} only and two spare untapped creatures, the taxed second cast
// is payable {4}-from-the-pool + {2}-from-the-creatures, and gorge must
// offer it. Before the fix the offer gate credited the creatures against
// the printed generic only and the tax was added afterwards, so the credit
// never reached the tax and no cast option appeared.
func TestCommanderConvokePaysTax(t *testing.T) {
	t.Parallel()
	e, cmd := convokeMarshalGame(t, 41)
	fundAndRefresh(t, e, 4)
	marshalFirstCastCycle(t, e, cmd)
	if got := e.G.Players[0].CmdCasts[0]; got != 1 {
		t.Fatalf("CmdCasts[0] after the first cast = %d, want 1", got)
	}
	if tax := e.commanderTaxAmount(0, cmd); tax != 2 {
		t.Fatalf("precondition: second-cast tax = %d, want 2", tax)
	}
	// Recruits in hand for the tax: one spare untapped creature is short of
	// the {2}, two cover it. No mana can come from anywhere but the pool.
	recruits := addRecruits(t, e, 0, 2)
	if n := len(e.G.Zone(state.ZBattlefield, 0)); n != 2 {
		t.Fatalf("precondition: %d permanents on the battlefield, want 2", n)
	}
	for _, oid := range recruits {
		if o := e.G.Obj(oid); o == nil || o.Tapped {
			t.Fatalf("precondition: recruit %d is not an untapped credit", oid)
		}
	}
	e.pending = nil
	e.Advance()
	// The negative control that makes the positive assertion below able to
	// fail for the RIGHT reason: the creatures alone (pool drained to zero)
	// cannot pay the {6} total, so the offer must stay withheld.
	e.G.Players[0].Pool = state.Mana{}
	e.pending = nil
	e.Advance()
	if c := commanderCastOption(e, cmd); c != nil {
		t.Fatalf("creatures-only cast offered though the creatures cover just {2} of the {6}: %+v", e.Pending().Options)
	}
	// The ticket's state: the pool pays the printed {4}, the two creatures
	// pay the {2} tax. The offer gate must price the credit over the taxed
	// total (CR 702.51a) and offer the cast.
	e.G.Players[0].Pool[state.MC] = 4
	e.pending = nil
	e.Advance()
	if d := e.Pending(); d == nil || d.Kind != decision.KPriority || d.Player != 0 {
		t.Fatalf("precondition: not seat 0's priority at the assertion: %+v", d)
	}
	if c := commanderCastOption(e, cmd); c == nil {
		t.Fatalf("the taxed convoke cast whose creatures pay the tax is not offered: pool {4} + 2 untapped creatures, tax {2}; options %+v", e.Pending().Options)
	}
}
