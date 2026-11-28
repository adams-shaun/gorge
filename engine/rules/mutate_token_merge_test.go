package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestMutateTokenTopMerge pins CR 729.2 with CR 400.7 on a mutate target that
// is a creature TOKEN (legal: the mutate target is Creature.nonHuman+YouOwn).
// Casting the mutating card "on top" puts a real card over the token's card,
// so the merged permanent is a card permanent, not a token; the token's own
// card demotes to an under-component that stays a TOKEN object, and a token
// that leaves the battlefield ceases to exist instead of following the pile
// into the graveyard (whereas the mutating card, being a card, is the one
// that survives the pile's death as a graveyard card).
func TestMutateTokenTopMerge(t *testing.T) {
	t.Parallel()
	e, cfg := tokenReplGame(t, 402, card(t, mutateTesterSrc))
	mut := moveSeededCard(t, e, 0, card(t, mutateTesterSrc), state.ZHand)
	bear := putToken(t, e, 0, mutateBearSrc, state.ZBattlefield)
	addMana(t, e, 0, "GG") // the mutate cost {1}{G}
	mutateCastOnto(t, e, mutatedCastOption(t, e, mut), bear, true)

	pile := e.G.Obj(bear)
	if pile == nil || pile.Zone != state.ZBattlefield || pile.Face() == nil ||
		pile.Face().Name != "Apex Tester" {
		t.Fatalf("mutated pile = %+v, want the Tester on top of the token", pile)
	}
	// CR 729.2: a merged permanent is a token only when its topmost component
	// is one -- the top card here is the real mutating card, so the pile is a
	// card permanent (and neither a token nor a copy).
	if pile.IsToken {
		t.Fatal("the pile is still a token after a real card was mutated onto it (CR 729.2)")
	}
	if pile.IsCopy {
		t.Fatal("the pile still carries IsCopy after a real card became its top component")
	}
	if len(pile.MergedCards) != 1 {
		t.Fatalf("merged under-cards = %+v, want the demoted token card", pile.MergedCards)
	}
	under := pile.MergedCards[0].Obj
	comp := e.G.Obj(under)
	if comp == nil || comp.Zone != state.ZCeased {
		t.Fatalf("demoted component %d = %+v, want it parked in ceased", under, comp)
	}
	if !comp.IsToken {
		t.Fatalf("the demoted component lost its token status: it was the token's own card")
	}

	// The pile dies: the mutating CARD goes to the graveyard as the pile's
	// last-known card, and the token component ceases with the pile instead
	// of being moved to the graveyard as a card.
	e.emit(events.Sacrifice(bear))
	// BEFORE any state-based check: the Move fold must never move the token
	// component at all -- it stays parked. Asserting here is what pins the
	// fold: a later SBA pass (ceaseDeadTokens) would cease a graveyard token
	// anyway and hide a component that was wrongly moved and then removed.
	if c := e.G.Obj(under); c == nil || c.Zone != state.ZCeased {
		t.Fatalf("token component before the state-based check = %+v, want it still parked in ceased", e.G.Obj(under))
	}
	e.checkStateBased()
	if p := e.G.Obj(bear); p == nil || p.Zone != state.ZGraveyard ||
		p.Card == nil || p.Face() == nil || p.Face().Name != "Apex Tester" {
		t.Fatalf("pile after dying = %+v, want the Tester's card in the graveyard", e.G.Obj(bear))
	}
	if c := e.G.Obj(under); c == nil || c.Zone != state.ZCeased {
		t.Fatalf("token component after the pile died = %+v, want it ceased (CR 400.7)", e.G.Obj(under))
	}
	grave := e.G.Zone(state.ZGraveyard, 0)
	inGrave, inPileGrave := false, false
	for _, id := range grave {
		if id == under {
			inGrave = true
		}
		if id == bear {
			inPileGrave = true
		}
	}
	if inGrave {
		t.Fatalf("the token component %d is on the graveyard list -- a token ceases, it does not move", under)
	}
	if !inPileGrave {
		t.Fatalf("the mutated card is not on the graveyard list %v", grave)
	}
	replayCheck(t, e, cfg)
}
