package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// This file pins two instances of CR 702.140e ("A mutated permanent has all
// abilities of each card and token that represents it") the attack and block
// keyword walks missed: myriad printed beneath the top (CR 702.116a), and the
// same afflict printed on the top and beneath it, whose instances trigger
// separately (CR 702.130b).

// mutatedStackedPile casts a Mutate creature printing top's extra lines onto a
// creature printing under's lines, and returns the pile beside a seat-1 bear.
func mutatedStackedPile(t *testing.T, seed uint64, top, under string) (*Engine, Config, state.ObjID, state.ObjID) {
	t.Helper()
	bearer := card(t, "Name:Stacked Bearer\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\n"+under+"Oracle:x\n")
	crown := card(t, "Name:Stacked Crown\nManaCost:2 G\nTypes:Creature Beast\nPT:3/3\n"+
		"K:Mutate:1 G\n"+top+"Oracle:x\n")
	blocker := card(t, mutateBearSrc)
	e, cfg := tokenReplGameSeats(t, seed, []*cards.Card{crown, bearer}, []*cards.Card{blocker})
	blk := moveSeededCard(t, e, 1, blocker, state.ZBattlefield)
	crownID := moveSeededCard(t, e, 0, crown, state.ZHand)
	pile := moveSeededCard(t, e, 0, bearer, state.ZBattlefield)
	addMana(t, e, 0, "GG") // the mutate cost {1}{G}
	mutateCastOnto(t, e, mutatedCastOption(t, e, crownID), pile, true)
	mutateDrain(t, e, 40)
	o := e.G.Obj(pile)
	if o == nil || o.Face() == nil || o.Face().Name != "Stacked Crown" || len(o.MergedCards) != 1 {
		t.Fatalf("precondition: want the Crown on top of the Bearer, got %+v", o)
	}
	if n := len(e.pendingTriggers); n != 0 || len(e.G.Stack) != 0 {
		t.Fatalf("precondition: %d pending triggers and %d stack entries after the merge", n, len(e.G.Stack))
	}
	return e, cfg, pile, blk
}

// emitAttack emits the engine's own declaration of pile against seat 1
// (Player is the defender). It writes no field itself, so the log alone
// replays the board.
func emitAttack(e *Engine, pile state.ObjID) {
	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{pile}})
}

// TestMutatedPileUnderCardMyriadTriggersOnce: myriad printed beneath the top
// queues one trigger as the pile attacks, minted from the under card's own
// compiled body; the top-printed pile is the control, and a pile printing it on
// both cards queues one each (CR 702.116b).
func TestMutatedPileUnderCardMyriadTriggersOnce(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, top, under string
		want             int
	}{
		{"top", "K:Myriad\n", "", 1},
		{"under", "", "K:Myriad\n", 1},
		{"top and under", "K:Myriad\n", "K:Myriad\n", 2},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e, cfg, pile, _ := mutatedStackedPile(t, 461+uint64(i), tc.top, tc.under)
			o := e.G.Obj(pile)
			if got := o.Face().HasKeyword("Myriad"); got != (tc.top != "") {
				t.Fatalf("precondition: top card prints myriad = %v: %q", got, o.Face().Keywords)
			}
			if !slices.Contains(e.Derived(pile).Keywords, "Myriad") {
				t.Fatalf("precondition: the pile does not derive myriad: %q", e.Derived(pile).Keywords)
			}
			from := len(e.L.Events)
			emitAttack(e, pile)
			if got := queuedSince(e, pile, from); got != tc.want {
				t.Fatalf("myriad queued %d triggers on the attack, want %d", got, tc.want)
			}
			e.pending = nil
			if e.putTriggersOnStack() {
				d := e.Pending() // two instances: their controller orders them
				if d.Kind != decision.KTriggerOrder || len(d.Options) != tc.want {
					t.Fatalf("placing the triggers asked %+v", d)
				}
				submitChoices(t, e, d.Options[0].Index, d.Options[1].Index)
			}
			if len(e.G.Stack) != tc.want {
				t.Fatalf("stack depth %d after the attack, want %d myriad triggers", len(e.G.Stack), tc.want)
			}
			for _, id := range e.G.Stack {
				if a := e.G.Obj(id); a == nil || a.Source != pile || a.Ability == nil || a.Ability.API != "Myriad" {
					t.Fatalf("a pushed ability is not the pile's DB$ Myriad: %+v", a)
				}
			}
			replayCheck(t, e, cfg)
		})
	}
}

// TestMutatedPileIdenticalAfflictTriggersTwice: the same afflict on the top and
// beneath it is two instances, each draining the attacked player (CR
// 702.130b). One printed instance on either card stays one.
func TestMutatedPileIdenticalAfflictTriggersTwice(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, top, under string
		want             int
		loss             int32
	}{
		{"top and under", "K:Afflict:1\n", "K:Afflict:1\n", 2, 2},
		{"top only", "K:Afflict:1\n", "", 1, 1},
		{"under only", "", "K:Afflict:1\n", 1, 1},
		{"different amounts", "K:Afflict:2\n", "K:Afflict:1\n", 2, 3},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e, cfg, pile, blk := mutatedStackedPile(t, 471+uint64(i), tc.top, tc.under)
			o := e.G.Obj(pile)
			if _, got := o.Face().KeywordParam("Afflict"); got != (tc.top != "") {
				t.Fatalf("precondition: top card prints afflict = %v: %q", got, o.Face().Keywords)
			}
			if b := e.G.Obj(blk); b == nil || b.Zone != state.ZBattlefield || b.Controller != 1 {
				t.Fatalf("precondition: no seat-1 blocker: %+v", b)
			}
			emitAttack(e, pile)
			if n := len(e.pendingTriggers); n != 0 || len(e.G.Stack) != 0 {
				t.Fatalf("precondition: the attack left %d pending triggers", n)
			}
			from := len(e.L.Events)
			e.emit(events.Event{Kind: events.DeclareBlockers, Player: 1, Pairs: [][2]state.ObjID{{pile, blk}}})
			if got := queuedSince(e, pile, from); got != tc.want {
				t.Fatalf("afflict queued %d triggers on becoming blocked, want %d", got, tc.want)
			}
			life := e.G.Players[1].Life
			e.pending = nil
			if !e.putTriggersOnStack() {
				e.grantPriority() // two instances ask their order instead
			}
			mutateDrain(t, e, 40)
			if got := life - e.G.Players[1].Life; got != tc.loss {
				t.Fatalf("defender lost %d life, want %d", got, tc.loss)
			}
			replayCheck(t, e, cfg)
		})
	}
}
