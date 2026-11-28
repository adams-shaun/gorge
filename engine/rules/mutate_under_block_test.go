package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// This file pins the combat-declaration half of CR 702.140e ("A mutated
// permanent has all abilities of each card and token that represents it"): a
// block-mode trigger printed on a card beneath the top fires from the pile,
// once, pushed as that under card's own trigger (MergedTriggerPush).

// mergedQueuedSince counts the triggers sourced at id queued since log position
// from that carry an under card's face identity, and all triggers sourced at id.
func mergedQueuedSince(e *Engine, id state.ObjID, from int) (merged, all int) {
	for _, pt := range e.pendingTriggers {
		if pt.Source == id {
			all++
			if pt.Merged > 0 {
				merged++
			}
		}
	}
	for _, ev := range e.L.Events[from:] {
		switch ev.Kind {
		case events.TriggerPush, events.KeywordTriggerPush, events.MergedTriggerPush, events.GrantTriggerPush:
			if ev.Obj == id {
				all++
				if ev.Kind == events.MergedTriggerPush {
					merged++
				}
			}
		}
	}
	return merged, all
}

const underGainLifeSVar = "SVar:TrigGain:DB$ GainLife | Defined$ You | LifeAmount$ 1\n"

// TestMutatedPileUnderCardBlockTriggersFireOnce: an under card's
// AttackerBlocked, Blocks and AttackerUnblocked lines each queue exactly one
// instance from the pile, under the under card's own face.
func TestMutatedPileUnderCardBlockTriggersFireOnce(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, line string
		fire       func(t *testing.T, e *Engine, pile state.ObjID)
	}{
		{"AttackerBlocked", "T:Mode$ AttackerBlocked | ValidCard$ Card.Self | Execute$ TrigGain | TriggerDescription$ Whenever CARDNAME becomes blocked, you gain 1 life.\n",
			func(t *testing.T, e *Engine, pile state.ObjID) {
				blocker := onBoard(t, e, 1, mutateBearSrc)
				declareAttackersOnly(t, e, 0, 1, pile)
				e.pendingTriggers = nil
				e.emit(events.Event{Kind: events.DeclareBlockers, Player: 1, Pairs: [][2]state.ObjID{{pile, blocker}}})
			}},
		{"Blocks", "T:Mode$ Blocks | ValidCard$ Card.Self | Execute$ TrigGain | TriggerDescription$ Whenever CARDNAME blocks, you gain 1 life.\n",
			func(t *testing.T, e *Engine, pile state.ObjID) {
				attacker := onBoard(t, e, 1, mutateBearSrc)
				e.emit(events.Event{Kind: events.DeclareBlockers, Player: 0, Pairs: [][2]state.ObjID{{attacker, pile}}})
			}},
		{"AttackerUnblocked", "T:Mode$ AttackerUnblocked | ValidCard$ Card.Self | Execute$ TrigGain | TriggerDescription$ Whenever CARDNAME attacks and isn't blocked, you gain 1 life.\n",
			func(t *testing.T, e *Engine, pile state.ObjID) {
				declareAttackersOnly(t, e, 0, 1, pile)
				e.pendingTriggers = nil
				e.checkAttackerUnblockedTriggers()
			}},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e, _, pile := mutatedKeywordPile(t, 471+uint64(i), tc.line+underGainLifeSVar)
			o := e.G.Obj(pile)
			if len(o.Face().Triggers) != 0 {
				t.Fatalf("precondition: the top card prints %d triggers, want none", len(o.Face().Triggers))
			}
			if uf := o.MergedFaceAt(0); uf == nil || len(uf.Triggers) != 1 {
				t.Fatalf("precondition: the under card does not print the %s line: %+v", tc.name, uf)
			}
			from := len(e.L.Events)
			tc.fire(t, e, pile)
			merged, all := mergedQueuedSince(e, pile, from)
			if all != 1 || merged != 1 {
				t.Fatalf("the under card's %s line queued %d triggers (%d as the under card's), want exactly 1 as the under card's", tc.name, all, merged)
			}
		})
	}
}
