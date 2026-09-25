package rules

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// This file covers CR 702.168c: a PERMANENT's promised gift is a "when this
// permanent enters" triggered ability, not part of the spell's own
// resolution. Every permanent GiftAbility carrier in the pinned corpus is
// driven through its real cast so the gift is observed as a respondable
// stack object placed AFTER entry, ordered against the card's own printed
// ETB where it has one.

// giftPermanentCarrier is one permanent face carrying SVar:GiftAbility.
type giftPermanentCarrier struct {
	name string
	// ownETB is true when the card carries its own printed "when this
	// enters" trigger that a promised cast also queues, so the gift trigger
	// must be ordered against it. For Scrapshooter and Starforged Sword that
	// printed ETB is itself gated on Card.Self+PromisedGift.
	ownETB bool
	// giftCount reports the observable gift outcome on the promised
	// opponent (hand size for a draw; a named token's count for a token).
	giftDraws       bool
	giftTokenName   string
	giftTokenSeat   state.PlayerID
	needOwnCreature bool // printed ETB target: a creature seat 0 controls
	needOppArtifact bool // printed ETB target: an artifact/enchantment seat 1 controls
	isAura          bool // the cast itself asks for an aura target
}

// giftPermanentCarriers is measured at FORGE_REF: /usr/bin/grep -rln "K:Gift"
// .cards/cardsfolder | while read f; do /usr/bin/grep -qE
// '^Types:.*(Creature|Enchantment|Artifact)' "$f" && echo "$f"; done
// yields exactly these four (the filing report named only Kitnap and
// Octomancer, which is short by two).
var giftPermanentCarriers = []giftPermanentCarrier{
	{name: "Kitnap", ownETB: true, giftDraws: true, isAura: true},
	{name: "Octomancer", ownETB: false, giftTokenName: "Octopus Token", giftTokenSeat: 1},
	{name: "Scrapshooter", ownETB: true, giftDraws: true, needOppArtifact: true},
	{name: "Starforged Sword", ownETB: true, giftTokenName: "Fish Token", giftTokenSeat: 1, needOwnCreature: true},
}

// TestGiftPermanentQueuesRespondableEntryTriggers drives each permanent Gift
// carrier through its real cast and asserts the gift is a real triggered
// ability placed on the stack after entry: at the trigger-order point (or
// the first stack placement, for a carrier with no printed ETB) the permanent
// is already on the battlefield and NO gift has resolved yet; only after the
// triggers are placed does the gift body run.
func TestGiftPermanentQueuesRespondableEntryTriggers(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	for i, c := range giftPermanentCarriers {
		t.Run(c.name, func(t *testing.T) {
			card := mustCorpusCard(t, reg, c.name)
			e, cfg := tokenReplGame(t, uint64(900+i), card)
			id := moveSeededCard(t, e, 0, card, state.ZHand)

			// Board pieces the cast's aura target and the printed ETB's
			// target filter need to be satisfiable.
			bear := putToken(t, e, 0, giftBearSrc, state.ZBattlefield)
			var oppArtifact state.ObjID
			if c.needOppArtifact {
				oppArtifact = putToken(t, e, 1,
					"Name:Gift Test Relic\\nTypes:Artifact\\nOracle:x\\n", state.ZBattlefield)
			}

			oppHand := len(e.G.Zone(state.ZHand, 1))
			addMana(t, e, 0, "UUUUGGG") // covers the most expensive carrier
			submitChoices(t, e, castCardOption(t, e, id).Index)
			answerGift(t, e, true)
			// Precondition: the promise really was recorded on the spell.
			if d := e.Pending(); d == nil {
				t.Fatal("no decision after the gift promise")
			}
			if c.isAura {
				d := e.Pending()
				if d.Kind != decision.KTarget {
					t.Fatalf("aura cast target ask = %+v, want KTarget", d)
				}
				ti := targetOptionIndex(d, bear)
				if ti < 0 {
					t.Fatalf("aura target ask does not offer the bear: %+v", d.Options)
				}
				submitChoices(t, e, ti)
			}

			// Locate the moment the promised permanent's triggers are being
			// ordered/placed. The order ask happens only when the card has a
			// printed ETB; otherwise the drain pushes the gift trigger
			// directly. Either way the permanent must already be on the
			// battlefield and the gift must NOT have resolved yet.
			sawOrder := false
			orderOpts := 0
			var orderObjs []state.ObjID
			for step := 0; step < 30; step++ {
				d := e.Pending()
				if d == nil {
					// Stack drained between decisions; stop.
					break
				}
				// The gift trigger has been placed and has since left the stack:
				// the entry interaction is over. Stop before a priority pass can
				// roll the turn forward into another player's draw step.
				if len(e.G.Stack) == 0 && countGiveGift(e) > 0 {
					break
				}
				if d.Kind == decision.KTriggerOrder {
					sawOrder = true
					orderOpts = len(d.Options)
					for _, o := range d.Options {
						orderObjs = append(orderObjs, o.Obj)
					}
					// Precondition for every assertion below: the permanent
					// has entered and no gift has fired yet.
					if z := e.G.Obj(id).Zone; z != state.ZBattlefield {
						t.Fatalf("permanent zone = %v at the trigger-order point, want battlefield", z)
					}
					if n := countGiveGift(e); n != 0 {
						t.Fatalf("GiveGift emitted %d times before the entry triggers were placed; the gift resolved during spell resolution", n)
					}
					choices := make([]int, 0, len(d.Options))
					for _, o := range d.Options {
						choices = append(choices, o.Index)
					}
					if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: choices}); err != nil {
						t.Fatalf("submit trigger order: %v", err)
					}
					continue
				}
				if d.Kind == decision.KPriority || d.Kind == decision.KTriggerOptional {
					// Before entry this is just the spell's own priority window:
					// pass and keep going. Once the permanent has entered, the
					// gift trigger must be a stack object, never an inline
					// resolution during the spell.
					if z := e.G.Obj(id).Zone; z == state.ZBattlefield && countGiveGift(e) == 0 && !giftTriggerOnStack(e, id) {
						t.Fatalf("permanent entered but no gift trigger on the stack; stack=%v", e.G.Stack)
					}
					if err := answerPriorityOrTarget(t, e, bear, oppArtifact); err != nil {
						t.Fatalf("%v", err)
					}
					continue
				}
				// A printed/gift trigger's target ask.
				if err := answerPriorityOrTarget(t, e, bear, oppArtifact); err != nil {
					t.Fatalf("%v", err)
				}
			}

			// Postconditions the fix exists to guarantee.
			if z := e.G.Obj(id).Zone; z != state.ZBattlefield {
				t.Fatalf("permanent zone = %v, want battlefield", z)
			}
			if c.ownETB {
				if !sawOrder {
					t.Fatal("no trigger-order ask: the gift was not queued alongside the card's printed ETB")
				}
				if orderOpts != 2 {
					t.Fatalf("trigger-order options = %d, want 2 (gift plus printed ETB)", orderOpts)
				}
				if orderObjs[0] != orderObjs[1] {
					t.Fatalf("trigger-order options name sources %v, want both the entering permanent %d", orderObjs, id)
				}
			}
			if n := countGiveGift(e); n != 1 {
				t.Fatalf("GiveGift markers = %d, want 1 (emitted at the gift trigger)", n)
			}
			moveAt, giftAt := -1, -1
			for i, ev := range e.L.Events {
				if ev.Kind == events.MoveZone && ev.Obj == id && ev.To == state.ZBattlefield {
					moveAt = i
				}
				if ev.Kind == events.GiveGift && ev.Obj == id {
					giftAt = i
				}
			}
			if moveAt < 0 || giftAt <= moveAt {
				t.Fatalf("GiveGift index %d must follow battlefield entry index %d", giftAt, moveAt)
			}
			if c.giftDraws {
				if got := len(e.G.Zone(state.ZHand, 1)); got != oppHand+1 {
					t.Fatalf("promised opponent hand = %d, want %d (the gift drew them a card)", got, oppHand+1)
				}
			} else if c.giftTokenName != "" {
				if got := countTokensNamedOnSeat(t, e, c.giftTokenSeat, c.giftTokenName); got != 1 {
					t.Fatalf("promised opponent %s count = %d, want 1", c.giftTokenName, got)
				}
			}
			replayCheck(t, e, cfg)
		})
	}
}

// giftTriggerOnStack reports whether id's GiftAbility SVar is currently a
// stack object (the KeywordTriggerPush body resolved from the card's own
// SVar). The mint rewrites the body's Defined$/TokenOwner$ Promised
// referents to PromisedSnapshot (the resolution-time read of the promise
// snapshotted into the payload), so only the params that survive the rewrite
// are compared.
func giftTriggerOnStack(e *Engine, id state.ObjID) bool {
	giftSA := cards.ResolveSVar(e.G.Obj(id).Face().SVars, "GiftAbility")
	if giftSA == nil {
		return false
	}
	for _, sid := range e.G.Stack {
		o := e.G.Obj(sid)
		if o == nil || o.Source != id || o.Ability == nil {
			continue
		}
		if o.Ability.API == giftSA.API && o.Ability.Params["TokenScript"] == giftSA.Params["TokenScript"] {
			return true
		}
	}
	return false
}

// answerPriorityOrTarget answers one entry-drain decision for this test:
// priority passes, a target/choose ask takes `want` when offered (else the
// first option). It returns an error instead of fataling so the caller can
// attach context.
func answerPriorityOrTarget(t *testing.T, e *Engine, wants ...state.ObjID) error {
	t.Helper()
	d := e.Pending()
	if d == nil {
		return nil
	}
	switch d.Kind {
	case decision.KPriority:
		for _, o := range d.Options {
			if o.Kind == "pass" {
				return e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{o.Index}})
			}
		}
		return fmt.Errorf("priority decision with no pass option: %+v", d)
	case decision.KTarget, decision.KChoose:
		for _, w := range wants {
			if w == 0 {
				continue
			}
			if i := targetOptionIndex(d, w); i >= 0 {
				return e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{i}})
			}
		}
		if len(d.Options) == 0 {
			return fmt.Errorf("target decision with no option: %+v", d)
		}
		return e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{d.Options[0].Index}})
	default:
		if len(d.Options) == 0 {
			return fmt.Errorf("decision with no option: %+v", d)
		}
		return e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{d.Options[0].Index}})
	}
}

// submitPresentedOrder answers a KTriggerOrder decision by submitting the
// options in the presented order (controller's choice, made deterministically
// so replay reproduces it).
func submitPresentedOrder(t *testing.T, e *Engine) {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KTriggerOrder {
		t.Fatalf("trigger order decision = %+v, want KTriggerOrder", d)
	}
	choices := make([]int, 0, len(d.Options))
	for _, o := range d.Options {
		choices = append(choices, o.Index)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: choices}); err != nil {
		t.Fatalf("submit trigger order: %v", err)
	}
}

// TestGiftPermanentPromiseDeliversAfterSourceLeavesTheBattlefield is the
// respondable interaction CR 702.168c creates: the gift is a triggered
// ability with a response window, so the opponent can remove the source in
// response to its own entry triggers. The ability resolves independently of
// its source (CR 112.7a) and must still deliver -- the promised receiver was
// snapshotted into the trigger at queue time, not re-read from the
// permanent's live (cleared on leaving) promise.
func TestGiftPermanentPromiseDeliversAfterSourceLeavesTheBattlefield(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	kitnap := mustCorpusCard(t, reg, "Kitnap")
	demystify := mustCorpusCard(t, reg, "Demystify")
	e, cfg := tokenReplGameSeats(t, 4102, []*cards.Card{kitnap}, []*cards.Card{demystify})
	oppHand := len(e.G.Zone(state.ZHand, 1))
	kitID := moveSeededCard(t, e, 0, kitnap, state.ZHand)
	demID := moveSeededCard(t, e, 1, demystify, state.ZHand)
	bear := putToken(t, e, 0, giftBearSrc, state.ZBattlefield)

	addMana(t, e, 0, "UUUU") // Kitnap is 2UU
	addMana(t, e, 1, "W")    // Demystify
	submitChoices(t, e, castCardOption(t, e, kitID).Index)
	answerGift(t, e, true)
	// Aura target: the bear. From here the spell resolves, enters, and both
	// ETBs (printed tap/stun and gift) are ordered and placed.
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("aura cast target ask = %+v, want KTarget", d)
	}
	ti := targetOptionIndex(d, bear)
	if ti < 0 {
		t.Fatalf("aura target ask does not offer the bear: %+v", d.Options)
	}
	submitChoices(t, e, ti)

	// Destroy Kitnap in the response window: at seat 1's first priority
	// AFTER the entry triggers are placed, cast Demystify on it. The cast's
	// own preconditions are asserted right there, so the test can never pass
	// with the window missing.
	castRemoval := false
	for step := 0; step < 80; step++ {
		d := e.Pending()
		if d == nil {
			break
		}
		// The removal landed and both entry triggers have resolved: stop
		// before a priority pass can roll the turn forward into another
		// player's draw step (which would change hand sizes out from under
		// the assertion).
		if castRemoval && len(e.G.Stack) == 0 && !giftTriggerOnStack(e, kitID) {
			break
		}
		switch d.Kind {
		case decision.KTriggerOrder:
			submitPresentedOrder(t, e)
		case decision.KPriority:
			if !castRemoval && d.Player == 1 && e.G.Obj(kitID).Zone == state.ZBattlefield &&
				giftTriggerOnStack(e, kitID) {
				submitChoices(t, e, castCardOption(t, e, demID).Index)
				castRemoval = true
				continue
			}
			passPriorityOnce(t, e)
		default:
			// Demystify's enchantment target (Kitnap) when offered; anything
			// else through the shared answerer.
			if i := targetOptionIndex(d, kitID); i >= 0 {
				submitChoices(t, e, i)
				continue
			}
			if err := answerPriorityOrTarget(t, e, bear); err != nil {
				t.Fatalf("step %d: %v", step, err)
			}
		}
	}
	if !castRemoval {
		t.Fatal("never cast the removal: no seat-1 priority had the gift trigger as a stack object while Kitnap was on the battlefield")
	}
	// Precondition: the removal really landed before the gift resolved.
	if z := e.G.Obj(kitID).Zone; z != state.ZGraveyard {
		t.Fatalf("Kitnap zone = %v, want graveyard (the removal must land before the gift trigger resolves)", z)
	}
	if n := countGiveGift(e); n != 1 {
		t.Fatalf("GiveGift markers = %d, want 1", n)
	}
	if got := len(e.G.Zone(state.ZHand, 1)); got != oppHand+1 {
		t.Fatalf("promised opponent hand = %d, want %d: the gift must deliver even though Kitnap left the battlefield before its gift trigger resolved", got, oppHand+1)
	}
	replayCheck(t, e, cfg)
}

// TestGiftPermanentPromiseGoesToTheChosenSeatAtThreeSeats measures the
// promise mechanics at three seats: the election names the chosen opponent
// (player ids, not "the opponent"), the snapshot carries exactly that seat,
// and only that seat draws.
func TestGiftPermanentPromiseGoesToTheChosenSeatAtThreeSeats(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	kitnap := mustCorpusCard(t, reg, "Kitnap")
	e, cfg := willThreeSeatGame(t, 4103, kitnap)
	kitID := moveSeededCard(t, e, 0, kitnap, state.ZHand)
	bear := putToken(t, e, 0, giftBearSrc, state.ZBattlefield)

	hands := [3]int{}
	for i := range hands {
		hands[i] = len(e.G.Zone(state.ZHand, state.PlayerID(i)))
	}
	addMana(t, e, 0, "UUUU")
	submitChoices(t, e, castCardOption(t, e, kitID).Index)
	// The election offers a decline plus one promise option per alive
	// opponent; promise seat 1 specifically and assert it was offered.
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("gift ask = %+v, want KChoose", d)
	}
	pi := -1
	for _, o := range d.Options {
		if o.Kind == "gift_promise" && o.Player == 1 {
			pi = o.Index
		}
	}
	if pi < 0 {
		t.Fatalf("gift ask offers no promise option naming seat 1: %+v", d.Options)
	}
	submitChoices(t, e, pi)
	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("aura cast target ask = %+v, want KTarget", d)
	}
	ti := targetOptionIndex(d, bear)
	if ti < 0 {
		t.Fatalf("aura target ask does not offer the bear: %+v", d.Options)
	}
	submitChoices(t, e, ti)

	for step := 0; step < 80; step++ {
		d := e.Pending()
		if d == nil {
			break
		}
		// The gift has resolved and the stack is empty: stop before a
		// priority pass can roll the turn forward into another player's
		// draw step or a cleanup discard (which would change hand sizes out
		// from under the assertion).
		if len(e.G.Stack) == 0 && countGiveGift(e) > 0 {
			break
		}
		switch d.Kind {
		case decision.KTriggerOrder:
			submitPresentedOrder(t, e)
		case decision.KPriority:
			passPriorityOnce(t, e)
		default:
			if err := answerPriorityOrTarget(t, e, bear); err != nil {
				t.Fatalf("step %d: %v", step, err)
			}
		}
	}
	if n := countGiveGift(e); n != 1 {
		t.Fatalf("GiveGift markers = %d, want 1", n)
	}
	if got := len(e.G.Zone(state.ZHand, 1)); got != hands[1]+1 {
		t.Fatalf("promised seat 1 hand = %d, want %d", got, hands[1]+1)
	}
	if got := len(e.G.Zone(state.ZHand, 2)); got != hands[2] {
		t.Fatalf("unpromised seat 2 hand = %d, want %d (unchanged): the gift went to the chosen seat only", got, hands[2])
	}
	replayCheck(t, e, cfg)
}
