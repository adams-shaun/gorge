package rules

// CopyPermanent's Controller$ TargetedPlayer/Targeted referent names the
// PLAYER the resolving ability targeted, not the controller of the copied
// object (CR 111.2: the player who creates a token owns and controls it).
// These two leaves pin the class the regression shipped in: the copy source
// and the copy's controller are different referents.
//
//   - Wedding Ring: Defined$ Self + ValidTgts$ Opponent. The copied object is
//     the Ring (seat 0's); the targeted opponent (seat 1) creates the token.
//   - Lucy MacLean, Positively Armed: Defined$ TriggeredCardLKICopy +
//     ValidTgts$ Player.!TriggeredCardController. The copied token is seat 0's;
//     the targeted player (seat 1) creates the copy.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestWeddingRingTokenGoesToTargetedOpponent casts Wedding Ring from hand and
// resolves its "if it was cast, target opponent creates a token that's a copy
// of it" trigger with seat 1 as the target.
func TestWeddingRingTokenGoesToTargetedOpponent(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	ring := lookup(t, reg, "Wedding Ring")
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{ring}, nil)

	ringID := moveByName(t, e, 0, "Wedding Ring", state.ZHand)
	if o := e.G.Obj(ringID); o == nil || o.Zone != state.ZHand {
		t.Fatalf("precondition: Wedding Ring %d is not in hand: %+v", ringID, o)
	}
	if n := countTokensNamedOnSeat(t, e, 1, "Wedding Ring"); n != 0 {
		t.Fatalf("precondition: seat 1 already has %d Wedding Ring tokens", n)
	}

	addMana(t, e, 0, "WWWW")
	castSpellOption(t, e, "Wedding Ring")

	// The cast spell resolves, then its enters trigger asks for the opponent.
	d := drainToTargetAsk(t, e, 60)
	if d.Kind != decision.KTarget {
		t.Fatalf("Wedding Ring's trigger did not ask for its opponent target: %+v", d)
	}
	target := -1
	for _, o := range d.Options {
		if o.Kind == "player" && o.Player == 1 {
			target = o.Index
		}
	}
	if target < 0 {
		t.Fatalf("Wedding Ring's target ask does not offer seat 1: %+v", d.Options)
	}
	submitChoices(t, e, target)
	passUntilStackEmpty(t, e, 60)

	// The created token must be seat 1's, both owned and controlled by it.
	var token state.ObjID
	for _, id := range e.G.Zone(state.ZBattlefield, 1) {
		o := e.G.Obj(id)
		if o != nil && o.IsToken && o.Face() != nil && o.Face().Name == "Wedding Ring" {
			token = id
		}
	}
	if token == 0 {
		t.Fatalf("seat 1 controls no Wedding Ring token after the ETB; p0 battlefield=%v p1 battlefield=%v",
			e.G.Zone(state.ZBattlefield, 0), e.G.Zone(state.ZBattlefield, 1))
	}
	o := e.G.Obj(token)
	if o.Owner != 1 || o.Controller != 1 {
		t.Fatalf("Wedding Ring token %d owner/controller = %d/%d, want 1/1", token, o.Owner, o.Controller)
	}
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		p := e.G.Obj(id)
		if p != nil && p.IsToken && p.Face() != nil && p.Face().Name == "Wedding Ring" {
			t.Fatalf("seat 0 created Wedding Ring token %d, want the copy only on seat 1", id)
		}
	}
	replayCheck(t, e, cfg)
}

// TestLucyMacLeanCopyGoesToTargetedPlayer puts a token under seat 0, lets
// Lucy's token-entry trigger target seat 1, and asserts the copy is seat 1's --
// not the copied token's controller's.
func TestLucyMacLeanCopyGoesToTargetedPlayer(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	lucy := lookup(t, reg, "Lucy MacLean, Positively Armed")
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{lucy}, nil)

	lucyID := moveByName(t, e, 0, "Lucy MacLean, Positively Armed", state.ZBattlefield)
	if o := e.G.Obj(lucyID); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Lucy %d is not on the battlefield: %+v", lucyID, o)
	}

	// A real token entry fires Lucy's ChangesZone trigger.
	e.emit(events.Event{Kind: events.TokenCreate, Player: 0, Text: "c_a_treasure_sac"})
	var original state.ObjID
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		o := e.G.Obj(id)
		if o != nil && o.IsToken {
			original = id
		}
	}
	if original == 0 {
		t.Fatal("precondition: the Treasure token was not minted")
	}
	if len(e.pendingTriggers) == 0 {
		t.Fatalf("precondition: Lucy's token-entry trigger did not queue (log %+v)", e.L.Events)
	}
	e.putTriggersOnStack()

	for i := 0; i < 60; i++ {
		if len(e.G.Stack) == 0 && len(e.pendingTriggers) == 0 {
			break
		}
		d := e.Pending()
		if d == nil {
			e.Advance()
			continue
		}
		switch d.Kind {
		case decision.KTriggerOptional:
			// The optional trigger's placement and resolution elections: yes.
			submitChoices(t, e, 0)
		case decision.KTriggerOrder:
			answerTriggerOrders(t, e)
		case decision.KTarget:
			idx := -1
			for _, o := range d.Options {
				if o.Kind == "player" && o.Player == 1 {
					idx = o.Index
				}
			}
			if idx < 0 {
				t.Fatalf("Lucy's target ask does not offer seat 1: %+v", d.Options)
			}
			submitChoices(t, e, idx)
		case decision.KPriority:
			idx := -1
			for _, o := range d.Options {
				if o.Kind == "pass" {
					idx = o.Index
				}
			}
			if idx < 0 {
				t.Fatalf("priority decision with no pass option: %+v", d)
			}
			submitChoices(t, e, idx)
		default:
			t.Fatalf("unexpected decision while resolving Lucy's trigger: %+v", d)
		}
	}
	if len(e.G.Stack) != 0 {
		t.Fatalf("Lucy's trigger did not resolve within the budget (stack %d)", len(e.G.Stack))
	}

	// The copy is a new token on seat 1; the original Treasure stays seat 0's.
	var copyID state.ObjID
	for _, id := range e.G.Zone(state.ZBattlefield, 1) {
		o := e.G.Obj(id)
		if o != nil && o.IsToken {
			copyID = id
		}
	}
	if copyID == 0 {
		t.Fatalf("seat 1 controls no copied token; p0 battlefield=%v p1 battlefield=%v",
			e.G.Zone(state.ZBattlefield, 0), e.G.Zone(state.ZBattlefield, 1))
	}
	c := e.G.Obj(copyID)
	if c.Owner != 1 || c.Controller != 1 {
		t.Fatalf("Lucy copy token %d owner/controller = %d/%d, want 1/1", copyID, c.Owner, c.Controller)
	}
	if !c.IsCopy || c.Card != e.G.Obj(original).Card {
		t.Fatalf("Lucy's token %d is not a copy of the original token %d: copy=%+v original=%+v",
			copyID, original, c, e.G.Obj(original))
	}
	if got := countTokensNamedOnSeat(t, e, 0, "Treasure Token"); got != 1 {
		t.Fatalf("seat 0 has %d Treasure Tokens, want only the original 1", got)
	}
	replayCheck(t, e, cfg)
}
