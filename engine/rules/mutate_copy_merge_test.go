package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestMutateCopyMerge pins CR 730.2d (a merged permanent is a token only if
// its topmost component is one) for a mutating SOURCE that is a spell copy
// (Double Major copying a mutate spell): the copy becomes a token (CR 707.10f,
// 608.3f), so a pile it tops is a token, a component it is stays a token, and
// the pile's death never leaves a card made from the copy.
func TestMutateCopyMerge(t *testing.T) {
	t.Parallel()

	// castAndCopy casts the Tester mutating onto bear, copies the spell on
	// the stack, and resolves the copy (the top stack object) only.
	castAndCopy := func(t *testing.T, e *Engine, mut, bear state.ObjID, onTop bool) state.ObjID {
		t.Helper()
		addMana(t, e, 0, "GG") // the mutate cost {1}{G}
		mutateCastOntoUndrained(t, e, mutatedCastOption(t, e, mut), bear, onTop)
		e.emit(events.Event{Kind: events.StackCopy, Obj: mut, Player: 0})
		cp := e.G.Stack[len(e.G.Stack)-1]
		if c := e.G.Obj(cp); cp == mut || c == nil || !c.IsCopy || c.CastFlags&state.FlagMutated == 0 {
			t.Fatalf("precondition: stack top %d = %+v, want a copy of the mutating spell", cp, e.G.Obj(cp))
		}
		for i := 0; i < 10 && len(e.G.Obj(bear).MergedCards) == 0; i++ {
			passPriorityOnce(t, e)
		}
		if len(e.G.Obj(bear).MergedCards) != 1 || len(e.G.Stack) != 1 || e.G.Stack[0] != mut {
			t.Fatalf("precondition: want only the copy merged and the original still on the stack: merged %+v stack %v",
				e.G.Obj(bear).MergedCards, e.G.Stack)
		}
		return cp
	}
	// graveNames counts seat 0's graveyard objects by face name.
	graveNames := func(e *Engine) map[string]int {
		out := map[string]int{}
		for _, id := range e.G.Zone(state.ZGraveyard, 0) {
			out[e.G.Obj(id).Face().Name]++
		}
		return out
	}

	t.Run("copy on top of a token", func(t *testing.T) {
		e, cfg := tokenReplGame(t, 402, card(t, mutateTesterSrc))
		mut := moveSeededCard(t, e, 0, card(t, mutateTesterSrc), state.ZHand)
		bear := putToken(t, e, 0, mutateBearSrc, state.ZBattlefield)
		castAndCopy(t, e, mut, bear, true)

		pile := e.G.Obj(bear)
		if pile.Face().Name != "Apex Tester" {
			t.Fatalf("precondition: pile top %q, want the copy of the Tester", pile.Face().Name)
		}
		if !pile.IsToken || pile.IsCopy {
			t.Fatalf("pile token=%v copy=%v with a spell copy on top, want a token (CR 730.2d, 608.3f)", pile.IsToken, pile.IsCopy)
		}
		if c := e.G.Obj(pile.MergedCards[0].Obj); !c.IsToken {
			t.Fatalf("the demoted token component %d lost its token status", c.ID)
		}

		// The original merges on top: a real card, so the pile is a card
		// permanent holding two token components.
		passUntilStackEmpty(t, e, 40)
		pile = e.G.Obj(bear)
		if pile.IsToken || pile.IsCopy || len(pile.MergedCards) != 2 {
			t.Fatalf("pile token=%v copy=%v merged %+v after the card went on top, want a card pile of 3", pile.IsToken, pile.IsCopy, pile.MergedCards)
		}
		for _, m := range pile.MergedCards {
			if c := e.G.Obj(m.Obj); !c.IsToken || c.IsCopy {
				t.Fatalf("component %d token=%v copy=%v, want a token", c.ID, c.IsToken, c.IsCopy)
			}
		}
		e.emit(events.Sacrifice(bear))
		e.checkStateBased()
		if got := graveNames(e); got["Apex Tester"] != 1 || got["Mutate Bear"] != 0 {
			t.Fatalf("graveyard %v, want exactly one Apex Tester card and no token", got)
		}
		replayCheck(t, e, cfg)
	})

	t.Run("copy on top of a card", func(t *testing.T) {
		e, cfg := tokenReplGame(t, 402, card(t, mutateTesterSrc), card(t, mutateBearSrc))
		mut := moveSeededCard(t, e, 0, card(t, mutateTesterSrc), state.ZHand)
		bear := moveSeededCard(t, e, 0, card(t, mutateBearSrc), state.ZBattlefield)
		if e.G.Obj(bear).IsToken {
			t.Fatal("precondition: the target must be a real card")
		}
		castAndCopy(t, e, mut, bear, true)

		pile := e.G.Obj(bear)
		if !pile.IsToken || pile.IsCopy {
			t.Fatalf("pile token=%v copy=%v with a spell copy on top, want a token (CR 730.2d)", pile.IsToken, pile.IsCopy)
		}
		under := pile.MergedCards[0].Obj
		if c := e.G.Obj(under); c.IsToken {
			t.Fatalf("the demoted component %d is a real card, want it not a token", under)
		}
		// The pile dies with the copy on top: the real card reaches the
		// graveyard and nothing is made from the copy.
		e.emit(events.Sacrifice(bear))
		e.checkStateBased()
		if got := graveNames(e); got["Mutate Bear"] != 1 || got["Apex Tester"] != 0 {
			t.Fatalf("graveyard %v, want the bear card alone", got)
		}
		if c := e.G.Obj(under); c.Zone != state.ZGraveyard {
			t.Fatalf("the real card component is in zone %v, want the graveyard", c.Zone)
		}
		replayCheck(t, e, cfg)
	})

	t.Run("copy under a card", func(t *testing.T) {
		e, cfg := tokenReplGame(t, 402, card(t, mutateTesterSrc), card(t, mutateBearSrc))
		mut := moveSeededCard(t, e, 0, card(t, mutateTesterSrc), state.ZHand)
		bear := moveSeededCard(t, e, 0, card(t, mutateBearSrc), state.ZBattlefield)
		cp := castAndCopy(t, e, mut, bear, false)

		pile := e.G.Obj(bear)
		if pile.IsToken || pile.MergedCards[0].Obj != cp {
			t.Fatalf("pile token=%v merged %+v, want a card pile with the copy %d under", pile.IsToken, pile.MergedCards, cp)
		}
		if c := e.G.Obj(cp); !c.IsToken || c.IsCopy {
			t.Fatalf("the copy component token=%v copy=%v, want a token (CR 608.3f)", c.IsToken, c.IsCopy)
		}
		e.emit(events.Sacrifice(bear))
		// Before any state-based check: the Move fold must not move it.
		if c := e.G.Obj(cp); c.Zone != state.ZCeased {
			t.Fatalf("the copy component moved to zone %v, want it still parked in ceased", c.Zone)
		}
		e.checkStateBased()
		if got := graveNames(e); got["Mutate Bear"] != 1 || got["Apex Tester"] != 0 {
			t.Fatalf("graveyard %v, want the bear card alone", got)
		}
		replayCheck(t, e, cfg)
	})
}
