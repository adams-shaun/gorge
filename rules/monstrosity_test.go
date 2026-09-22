// The Monstrosity task (kw-monstrosity): the `A:AB$ PutCounter | Cost$ ...
// | Monstrosity$ N` activation, the events.AlterAttribute "Monstrous"
// designation it grants, the CR 701.33a one-shot offer gate and the
// Mode$ BecomeMonstrous trigger -- each pinned end to end on a REAL corpus
// card (never a committed .txt).
package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// monstrosityEngine advances a two-seat game to seat 0's turn-1 Main1 with
// the named corpus creature (seeded as a REAL deck card, so every object the
// assertions touch exists at genesis and a log-only replay can rebuild it)
// on seat 0's battlefield and a pool of `pool` mana symbols (each byte one
// of WUBRGC), and returns the engine, the config (for replayCheck) and the
// creature's id.
func monstrosityEngine(t *testing.T, name, pool string) (*Engine, Config, state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	c := corpusCardByName(t, name)
	cfg := seatZeroStart(Config{Seed: 7, Names: []string{"a", "b"}, Tokens: reg.Tokens,
		Decks: [][]*cards.Card{append([]*cards.Card{c}, mountainDeck(t, 39)...), mountainDeck(t, 40)}})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	id := moveByName(t, e, 0, name, state.ZBattlefield)
	if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("test precondition: %s on %v, want battlefield", name, o)
	}
	addMana(t, e, 0, pool)
	return e, cfg, id
}

// TestStormbreathDragonMonstrosityEndToEnd pins the whole feature on the real
// card: activating Monstrosity 3 puts three +1/+1 counters, marks the
// permanent monstrous, refuses a second activation (CR 701.33a's one-shot
// rule), and the BecomeMonstrous trigger deals damage to each opponent equal
// to the number of cards in that player's hand.
func TestStormbreathDragonMonstrosityEndToEnd(t *testing.T) {
	e, cfg, dragon := monstrosityEngine(t, "Stormbreath Dragon", "CCCCCRR")

	// Preconditions the assertions below depend on.
	if e.G.Obj(dragon).Monstrous {
		t.Fatal("test precondition: dragon already monstrous before the activation")
	}
	if e.G.Obj(dragon).Counter("P1P1") != 0 {
		t.Fatal("test precondition: dragon already carries +1/+1 counters")
	}
	opponentHand := len(e.G.Zone(state.ZHand, 1))
	if opponentHand <= 0 {
		t.Fatal("test precondition: opponent hand is empty, damage assertion would be vacuous")
	}
	// A logged draw widens the hand so the "equal to the cards in that
	// player's hand" dependence is measured, not assumed from the opening
	// deal.
	extra := e.G.Zone(state.ZLibrary, 1)[0]
	e.emit(events.Event{Kind: events.MoveZone, Obj: extra, From: state.ZLibrary, To: state.ZHand, Secret: true})
	opponentHand++

	opt, ok := findAbilityOption(e, dragon, 0)
	if !ok || opt.Kind != "ability" {
		t.Fatalf("Monstrosity activation not offered on a non-monstrous dragon: %+v", e.Pending().Options)
	}
	submitChoices(t, e, opt.Index)
	if got := e.G.Players[0].Pool.Total(); got != 0 {
		t.Fatalf("pool after paying {5}{R}{R} = %d, want 0", got)
	}
	passUntilStackEmpty(t, e, 50)

	d := e.G.Obj(dragon)
	if got := d.Counter("P1P1"); got != 3 {
		t.Fatalf("+1/+1 counters after Monstrosity 3 = %d, want 3", got)
	}
	if !d.Monstrous {
		t.Fatal("dragon not marked monstrous after the activation")
	}

	// The drain resolved both the ability and the BecomeMonstrous trigger
	// it queued (the trigger is placed when the empty stack re-grants
	// priority); the log carries the push.
	pushed := false
	for _, ev := range e.L.Events {
		if ev.Kind == events.TriggerPush && ev.Obj == dragon {
			pushed = true
		}
	}
	if !pushed {
		t.Fatal("BecomeMonstrous trigger never pushed")
	}
	if got := e.G.Players[1].Life; got != int32(20-opponentHand) {
		t.Fatalf("opponent life after the trigger = %d, want %d (20 minus %d hand cards)",
			got, 20-opponentHand, opponentHand)
	}

	// The one-shot rule: a monstrous dragon's activation is no longer offered.
	e.pending = nil
	e.priorityRound()
	if _, ok := findAbilityOption(e, dragon, 0); ok {
		t.Fatalf("Monstrosity activation re-offered on a monstrous dragon: %+v", e.Pending().Options)
	}
	replayCheck(t, e, cfg)
}

// TestDomesticatedHydraMonstrosityXAndStatic pins the X-cost form and the
// IsMonstrous filter predicate on the real card: Monstrosity X with X = 2
// puts two counters, and the "as long as CARDNAME is monstrous, it has
// trample" static turns on exactly when the designation lands.
func TestDomesticatedHydraMonstrosityXAndStatic(t *testing.T) {
	e, cfg, hydra := monstrosityEngine(t, "Domesticated Hydra", "CCGGG")

	if e.HasKeyword(hydra, "Trample") {
		t.Fatal("test precondition: hydra already has trample before it is monstrous")
	}

	opt, ok := findAbilityOption(e, hydra, 0)
	if !ok || opt.Kind != "ability" {
		t.Fatalf("Monstrosity X activation not offered: %+v", e.Pending().Options)
	}
	submitChoices(t, e, opt.Index)

	// The X announcement the {X}{G}{G}{G} cost poses at activation.
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || len(d.Options) == 0 || d.Options[0].Kind != "x" {
		t.Fatalf("want the X announce decision for Monstrosity X, got %+v", d)
	}
	two := -1
	for _, o := range d.Options {
		if o.Label == "X = 2" {
			two = o.Index
		}
	}
	if two < 0 {
		t.Fatalf("no X = 2 option: %+v", d)
	}
	submitChoices(t, e, two)
	passUntilStackEmpty(t, e, 50)

	if got := e.G.Obj(hydra).Counter("P1P1"); got != 2 {
		t.Fatalf("+1/+1 counters after Monstrosity X=2 = %d, want 2", got)
	}
	if !e.G.Obj(hydra).Monstrous {
		t.Fatal("hydra not marked monstrous after the activation")
	}
	if !e.HasKeyword(hydra, "Trample") {
		t.Fatal("IsMonstrous static did not turn on after the designation")
	}
	replayCheck(t, e, cfg)
}

// TestMonstrousDesignationClearsOnBattlefieldExit pins the Move departure
// fold: the designation ends the moment the permanent leaves the battlefield
// (a later entry is a new permanent and never inherits one), which is also
// what un-offers a returned activation and un-lights the IsMonstrous
// statics.
func TestMonstrousDesignationClearsOnBattlefieldExit(t *testing.T) {
	e, cfg, dragon := monstrosityEngine(t, "Stormbreath Dragon", "CCCCCRR")

	opt, ok := findAbilityOption(e, dragon, 0)
	if !ok {
		t.Fatalf("Monstrosity activation not offered: %+v", e.Pending().Options)
	}
	submitChoices(t, e, opt.Index)
	passUntilStackEmpty(t, e, 50)
	if !e.G.Obj(dragon).Monstrous {
		t.Fatal("test precondition: dragon should be monstrous after the activation")
	}

	e.emit(events.Event{Kind: events.MoveZone, Obj: dragon, From: state.ZBattlefield, To: state.ZGraveyard})
	if o := e.G.Obj(dragon); o.Monstrous {
		t.Fatal("monstrous designation survived a battlefield exit")
	}
	replayCheck(t, e, cfg)
}
