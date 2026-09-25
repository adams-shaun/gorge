package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// TestNinjutsuAgainstSeatZeroDefender is the regression pin for the seat-0
// discriminator bug: K:Ninjutsu against player 0 must place the ninja tapped
// AND attacking seat 0. Before the fix, pendingCast.ninjutsuDefender used 0
// both for "no defender captured" and for "the defender is seat 0", so this
// shape (the activator is player 1, the defender is player 0) silently lost
// the CR 702.49b binding and the permanent entered tapped but not attacking.
// This is half of all two-seat games, not a multiplayer corner.
func TestNinjutsuAgainstSeatZeroDefender(t *testing.T) {
	reg := searchTestRegistry(t)
	ninja := searchCorpusCard(t, reg, "Walker of Secret Ways")
	if d := ninja.Link(); len(d) != 0 {
		t.Fatalf("link Walker of Secret Ways: %v", d)
	}
	if !ninja.Faces[0].HasKeyword("Ninjutsu") {
		t.Fatal("precondition: Walker of Secret Ways does not print Ninjutsu in the corpus")
	}

	// A game whose starting seat is player 1, so player 1 is active and
	// player 0 is the defender. seatZeroStart cannot express this: it hunts
	// for a starting seat 0. Hunt the complementary seed instead, and keep
	// the effective seed for the replay check.
	cfg := Config{Seed: 9601, Names: []string{"defender", "ninja"},
		Decks: [][]*cards.Card{nil, nil}, Tokens: map[string]*cards.Card{}}
	s0 := []*cards.Card{ninja, card(t, ninjutsuBearSrc)}
	for len(s0) < 40 {
		s0 = append(s0, mountainDeck(t, 1)...)
	}
	cfg.Decks = [][]*cards.Card{mountainDeck(t, 40), s0}
	// seatOneStart: hunt the smallest seed >= cfg.Seed whose toss starts
	// seat 1 (the mirror of seatZeroStart, which always hunts seat 0). The
	// effective seed is what the replay check below must be handed.
	for New(cfg).G.Active != 1 {
		cfg.Seed++
	}
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	if e.G.Active != 1 {
		t.Fatalf("precondition: seat 1 must be the starting seat, got %d", e.G.Active)
	}

	ninjaID := searchMoveByNameSeat(t, e, 1, "Walker of Secret Ways", state.ZHand)
	if o := e.G.Obj(ninjaID); o == nil || o.Zone != state.ZHand {
		t.Fatalf("precondition: Walker of Secret Ways not in hand: %+v", o)
	}

	// Attack with seat 1's Bear into seat 0, and reach seat 1's
	// declare-blockers step with the Bear unblocked.
	bear := putCreature(t, e, 1, ninjutsuBearSrc)
	if o := e.G.Obj(bear); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Bear not on the battlefield: %+v", o)
	}
	e.priorityRound()
	driveToStep(t, e, e.G.Turn, 1, state.StepDeclareAttackers)
	passToKind(t, e, decision.KAttackers)
	submitAttackersOnly(t, e, bear)
	driveToBlockersPriority(t, e, 1)
	if e.G.Step != state.StepDeclareBlockers || e.G.Active != 1 {
		t.Fatalf("expected seat 1's declare-blockers step, got step %s active %d", e.G.Step, e.G.Active)
	}
	if o := e.G.Obj(bear); o == nil || !o.IsAttacking || o.Attacking != 0 || len(o.BlockedBy) != 0 {
		t.Fatalf("precondition: Bear should be an UNBLOCKED attacker of seat 0, got %+v", o)
	}

	fundPoolSeat(t, e, 1, "CU") // Walker's ninjutsu cost is {1}{U}
	opt := abilityFor(t, e, 1, ninjaID)
	if opt == nil {
		t.Fatalf("ninjutsu ability not offered at seat 1's declare-blockers step: %+v", e.Pending())
	}
	submitChoices(t, e, opt.Index)

	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || len(d.Options) == 0 || d.Options[0].Kind != "returncost" {
		t.Fatalf("ninjutsu did not ask to return an unblocked attacker: %+v", d)
	}
	chosen := -1
	for _, o := range d.Options {
		if o.Obj == bear {
			chosen = o.Index
		}
	}
	if chosen < 0 {
		t.Fatalf("ninjutsu return ask did not offer the unblocked Bear: %+v", d.Options)
	}
	submitChoices(t, e, chosen)
	passUntilStackEmpty(t, e, 40)

	if o := e.G.Obj(bear); o == nil || o.Zone != state.ZHand {
		t.Fatalf("the returned Bear is in %v, want its owner's hand", o)
	}
	o := e.G.Obj(ninjaID)
	if o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("Walker of Secret Ways is in %v, want the battlefield", o)
	}
	if !o.Tapped {
		t.Error("Walker of Secret Ways entered untapped, want tapped (CR 702.49a)")
	}
	if !o.IsAttacking || o.Attacking != 0 {
		t.Errorf("Walker of Secret Ways attacking=%v defender=%d, want attacking seat 0",
			o.IsAttacking, o.Attacking)
	}
	replayCheck(t, e, cfg)
}

// fundPoolSeat is fundPool for a named seat: it emits ManaAdd events into
// seat p's pool without driving anywhere, then re-asks priority so the
// pending decision reflects the funded pool.
func fundPoolSeat(t *testing.T, e *Engine, p state.PlayerID, symbols string) {
	t.Helper()
	for _, r := range symbols {
		e.emit(events.Event{Kind: events.ManaAdd, Player: p, Counter: string(r), Amount: 1})
	}
	e.priorityRound()
}

// searchMoveByNameSeat is searchMoveByName for a named seat: it finds the
// card in seat p's own hand or library and logs a MoveZone to the requested
// zone. searchMoveByName is hardwired to seat 0, which is the seat the
// regression below deliberately does NOT use.
func searchMoveByNameSeat(t *testing.T, e *Engine, p state.PlayerID, name string, to state.Zone) state.ObjID {
	t.Helper()
	for _, z := range []state.Zone{state.ZHand, state.ZLibrary} {
		for _, id := range e.G.Zone(z, p) {
			o := e.G.Obj(id)
			if o != nil && o.Face() != nil && o.Face().Name == name {
				if z != to {
					e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: z, To: to})
				}
				e.pending = nil
				e.priorityRound()
				return id
			}
		}
	}
	t.Fatalf("corpus fixture %q absent from seat %d's hand/library", name, p)
	return 0
}

// ninjutsuBlockerSrc is the inline 0/4 blocker the real-block withholding
// test seeds into seat 1's deck. It needs no Haste: it blocks on the turn
// seat 0 attacks, and this is seat 1's own turn-order-agnostic defence.
const ninjutsuBlockerSrc = "Name:Wall Bear\nManaCost:G\nTypes:Creature Bear\nPT:0/4\nOracle:x\n"

// ninjutsuDeckWithBlocker is ninjutsuDeck with one blocker in seat 1's deck,
// so the attacker can be blocked through a real CR 509.1 KBlockers answer
// instead of a synthetic state.Object.BlockedBy write. The blocker is a
// card of seat 1's own deck (putCreature's moveSeeded path), so the game
// replays from the log.
func ninjutsuDeckWithBlocker(t *testing.T, seed uint64, ninja *cards.Card) (*Engine, Config) {
	t.Helper()
	s0 := []*cards.Card{ninja, card(t, ninjutsuBearSrc)}
	for len(s0) < 40 {
		s0 = append(s0, mountainDeck(t, 1)...)
	}
	s1 := []*cards.Card{card(t, ninjutsuBlockerSrc)}
	for len(s1) < 40 {
		s1 = append(s1, mountainDeck(t, 1)...)
	}
	cfg := seatZeroStart(Config{Seed: seed, Names: []string{"ninja", "defender"},
		Decks: [][]*cards.Card{s0, s1}, Tokens: map[string]*cards.Card{}})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	return e, cfg
}

// attackWithBearBlockedBySeatOne drives seat 0's Bear declared attacking
// seat 1 and has seat 1 block it with the inline Wall Bear -- a real KBlockers
// answer, not a direct BlockedBy write -- then crosses the combat priority
// window to seat 0's declare-blockers priority. It returns the attacker and
// blocker ids.
func attackWithBearBlockedBySeatOne(t *testing.T, e *Engine) (attacker, blocker state.ObjID) {
	t.Helper()
	bear := putCreature(t, e, 0, ninjutsuBearSrc)
	wall := putCreature(t, e, 1, ninjutsuBlockerSrc)
	if o := e.G.Obj(bear); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Bear not on the battlefield: %+v", o)
	}
	if o := e.G.Obj(wall); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Wall Bear not on the battlefield: %+v", o)
	}
	e.priorityRound() // moveSeeded clears the pending ask; re-pose priority
	driveToStep(t, e, e.G.Turn, 0, state.StepDeclareAttackers)
	passToKind(t, e, decision.KAttackers)
	submitAttackersOnly(t, e, bear)
	// Cross seat 0's declare-attackers priority window, then answer the real
	// CR 509.1 blockers ask with the Wall Bear.
	passToKind(t, e, decision.KBlockers)
	submitBlockersOnly(t, e, wall)
	driveToBlockersPriority(t, e, 0)
	if e.G.Step != state.StepDeclareBlockers || e.G.Active != 0 {
		t.Fatalf("expected seat 0's declare-blockers step, got step %s active %d", e.G.Step, e.G.Active)
	}
	if o := e.G.Obj(bear); o == nil || !o.IsAttacking || o.Attacking != 1 || len(o.BlockedBy) == 0 {
		t.Fatalf("precondition: Bear should be a BLOCKED attacker of seat 1, got %+v", o)
	}
	return bear, wall
}
