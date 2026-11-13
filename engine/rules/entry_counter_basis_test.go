package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The two corpus fixtures pin CR 614.12's judgment basis for a FOREIGN
// enters-tapped replacement: Authority of the Consuls' `ValidCard$
// Creature.OppCtrl` must judge the entrant as it would exist on the
// battlefield, its intrinsic entry counters already folded, not as the
// counterless stack card. Overlord of the Balemurk impended enters with TIME
// counters, so it is NOT a creature as it enters and Authority must not tap
// it; Kaito, Bane of Nightmares enters with loyalty counters that turn its own
// PlayerTurn static on, so it IS a creature as it enters and Authority must
// tap it. Without the entry-counter basis both judgments read the stack card
// and come out the other way.

func authorityOnSeat1(t *testing.T, e *Engine, authority *cards.Card) {
	t.Helper()
	aid := moveSeededCard(t, e, 1, authority, state.ZBattlefield)
	if o := e.G.Obj(aid); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Authority of the Consuls is not on seat 1's battlefield: %+v", o)
	}
}

func castAndEnterCorpusCard(t *testing.T, e *Engine, id state.ObjID, mode, mana string, drain int) {
	t.Helper()
	if o := e.G.Obj(id); o == nil || o.Zone != state.ZHand {
		t.Fatalf("precondition: card is not in seat 0's hand: %+v", o)
	}
	addMana(t, e, 0, mana)
	submitChoices(t, e, castModeOption(t, e, id, mode))
	// Drain the resolution and whatever entry triggers follow it, declining
	// any optional pick (Overlord's graveyard return) so the entry state is
	// what the test reads.
	for i := 0; i < drain && len(e.G.Stack) > 0; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatal("no decision while the cast resolves")
		}
		if d.ResumeKind == "hidden_pick_confirm" {
			foundNo := false
			for _, option := range d.Options {
				if option.Kind == "no" {
					submitChoices(t, e, option.Index)
					foundNo = true
					break
				}
			}
			if !foundNo {
				t.Fatalf("optional entry trigger has no decline option: %+v", d.Options)
			}
			continue
		}
		passPriorityOnce(t, e)
	}
	if len(e.G.Stack) != 0 {
		t.Fatalf("entry trigger stack did not drain: %d objects remain", len(e.G.Stack))
	}
}

// TestOpponentEntersTappedSkipsImpendingOverlord: an impended Overlord enters
// with its five TIME counters and is not a creature as it enters (CR 614.12),
// so the opponent's Authority of the Consuls does not tap it. Without the
// entry-counter basis the fallback judges the stack card, still a creature
// with no TIME counters, and it enters tapped.
func TestOpponentEntersTappedSkipsImpendingOverlord(t *testing.T) {
	t.Parallel()
	overlord := corpusAlternativeCard(t, "Overlord of the Balemurk")
	authority := corpusAlternativeCard(t, "Authority of the Consuls")
	e, cfg := tokenReplGameSeats(t, 9879, []*cards.Card{overlord}, []*cards.Card{authority})
	authorityOnSeat1(t, e, authority)
	id := findCardObj(t, e, 0, "Overlord of the Balemurk", state.ZHand)
	face := e.G.Obj(id).Face()
	if got, ok := face.KeywordParam("Impending"); !ok || got != "5:1 B" {
		t.Fatalf("precondition: corpus Impending param = %q (present %v), want %q", got, ok, "5:1 B")
	}
	if got := e.G.Players[0].Pool.Total(); got != 0 {
		t.Fatalf("precondition: seat 0 starts with %d mana", got)
	}
	castAndEnterCorpusCard(t, e, id, "impended", "1B", 40)

	o := e.G.Obj(id)
	if o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Overlord did not enter the battlefield: %+v", o)
	}
	if o.CastFlags&state.FlagImpending == 0 {
		t.Fatal("precondition: Overlord lacks paid-Impending provenance")
	}
	if got := o.Counter("TIME"); got != 5 {
		t.Fatalf("precondition: Impending 5 entered with %d TIME counters, want 5", got)
	}
	if o.Tapped {
		t.Fatal("impended Overlord enters untapped: Authority of the Consuls must not judge a dormant permanent a creature (CR 614.12)")
	}
	replayCheck(t, e, cfg)
}

// TestOpponentEntersTappedTapsKaitoOnHisTurn: Kaito enters with four loyalty
// counters on his controller's turn, which turns his own static on, so he IS a
// creature as he enters and the opponent's Authority of the Consuls taps him.
// Without the entry-counter basis the fallback judges the counterless stack
// card, whose static cannot apply, and he enters untapped.
func TestOpponentEntersTappedTapsKaitoOnHisTurn(t *testing.T) {
	t.Parallel()
	kaito := corpusAlternativeCard(t, "Kaito, Bane of Nightmares")
	authority := corpusAlternativeCard(t, "Authority of the Consuls")
	e, cfg := tokenReplGameSeats(t, 9881, []*cards.Card{kaito}, []*cards.Card{authority})
	authorityOnSeat1(t, e, authority)
	id := findCardObj(t, e, 0, "Kaito, Bane of Nightmares", state.ZHand)
	if got := e.G.Players[0].Pool.Total(); got != 0 {
		t.Fatalf("precondition: seat 0 starts with %d mana", got)
	}
	castAndEnterCorpusCard(t, e, id, "", "CCUB", 40)

	o := e.G.Obj(id)
	if o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Kaito did not enter the battlefield: %+v", o)
	}
	if got := o.Counter("LOYALTY"); got != 4 {
		t.Fatalf("precondition: Kaito entered with %d LOYALTY counters, want 4", got)
	}
	if !o.Tapped {
		t.Fatal("Kaito enters untapped: his loyalty-counter static makes him a creature as he enters, so Authority of the Consuls applies (CR 614.12)")
	}
	replayCheck(t, e, cfg)
}
