package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// castTargetsFor chooses the cast option for id out of the current priority
// decision and advances the cast through its put-on-stack, returning the
// target decision's options WITHOUT answering them (an empty slice when the
// cast completed with no target decision, which is what a no-legal-target
// fizzle -- a target-hungry spell whose only legal target is protected --
// does).
func castTargetsFor(t *testing.T, e *Engine, id state.ObjID) []decision.Option {
	t.Helper()
	d := e.Pending()
	if d == nil {
		t.Fatal("no decision pending")
	}
	idx := -1
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == id {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("no cast option for %d: %+v", id, d.Options)
	}
	submitChoices(t, e, idx)
	if d := e.Pending(); d != nil && d.Kind == decision.KTarget {
		return d.Options
	}
	return nil
}

// TestProtectionFromBlueBlocksTargetingBlockingAndDamage is Task 15's end-to-
// end cycle on one protected permanent (Goblin Piledriver's shape, in
// miniature): a blue source's damage to it is prevented while a red one's
// lands; a blue creature cannot block it; a blue spell offers it no target
// while a red one does; and the game still replays exactly. The direct
// e.damaging/e.canBlock reads mirror the engine-internal hooks the brief
// exposes; the cast path goes through real decisions so the protection
// check in askTarget is exercised through play, not just unit-called.
func TestProtectionFromBlueBlocksTargetingBlockingAndDamage(t *testing.T) {
	pile := "Name:Piledriver\nManaCost:1 R\nTypes:Creature Goblin Warrior\nPT:1/2\nK:Protection from blue\nOracle:x\n"
	shock := "Name:BlueShock\nManaCost:U\nTypes:Instant\nA:SP$ DealDamage | ValidTgts$ Creature | NumDmg$ 2\nOracle:x\n"
	redShock := "Name:RedShock\nManaCost:R\nTypes:Instant\nA:SP$ DealDamage | ValidTgts$ Creature | NumDmg$ 2\nOracle:x\n"
	e, cfg, pd := newFixtureDeck(t, 71, pile, shock, redShock)
	e.emit(events.Event{Kind: events.MoveZone, Obj: pd, From: state.ZHand, To: state.ZBattlefield})

	// Damage: a blue source's damage is prevented; a red source's is not.
	blueGuy := putToken(t, e, 1, "Name:Merfolk\nManaCost:U\nTypes:Creature Merfolk\nPT:2/2\nOracle:x\n", state.ZBattlefield)
	e.damaging = blueGuy
	e.emit(events.Event{Kind: events.Damage, Obj: pd, Amount: 2})
	e.damaging = 0
	if e.G.Obj(pd).Damage != 0 {
		t.Fatal("blue damage was not prevented")
	}
	redGuy := putToken(t, e, 1, "Name:Goblin\nManaCost:R\nTypes:Creature Goblin\nPT:1/1\nOracle:x\n", state.ZBattlefield)
	e.damaging = redGuy
	e.emit(events.Event{Kind: events.Damage, Obj: pd, Amount: 1})
	e.damaging = 0
	if e.G.Obj(pd).Damage != 1 {
		t.Fatalf("red damage was prevented too: Damage=%d, want 1", e.G.Obj(pd).Damage)
	}

	// Blocking: the blue creature cannot block it; a red one can.
	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{pd}})
	if e.canBlock(blueGuy, pd) {
		t.Fatal("a blue creature blocked a creature with protection from blue")
	}
	if !e.canBlock(redGuy, pd) {
		t.Fatal("a red creature should be able to block")
	}

	// Targeting: a blue spell cannot target it, a red one can.
	shockID := addToHand(t, e, 0, shock)
	redShockID := addToHand(t, e, 0, redShock)
	addMana(t, e, 0, "UR")
	blueOpts := castTargetsFor(t, e, shockID)
	// The blue spell offers the OTHER (unprotected) creatures on the
	// battlefield, but the protection-from-blue piledriver must never be one.
	for _, o := range blueOpts {
		if o.Obj == pd {
			t.Fatalf("blue spell offered the protection-from-blue creature as a target: %+v", blueOpts)
		}
	}
	// Answer blueShock's pending target decision (to a non-protected
	// creature -- blueOpts never contained pd) so priority returns and the
	// red spell below can be cast on a clear stack.
	if d := e.Pending(); d != nil && d.Kind == decision.KTarget {
		submitChoices(t, e, 0)
	}
	redOpts := castTargetsFor(t, e, redShockID)
	var sawPD bool
	for _, o := range redOpts {
		if o.Obj == pd {
			sawPD = true
		}
	}
	if !sawPD {
		t.Fatalf("red spell omitted the piledriver: %+v", redOpts)
	}
	replayCheck(t, e, cfg)
}

// TestGrantedProtectionCountsAndDevoidIsColourless checks the two subtleties
// of the protectedFrom predicate: protection GRANTED by a resolved effect
// counts the same as printed (a transient DB$ Pump grant, not just a printed
// K: line), and a Devoid permanent is colourless, so "Protection from red"
// does not protect from a colourless Devoid source even though protection
// from red normally protects from anything red.
func TestGrantedProtectionCountsAndDevoidIsColourless(t *testing.T) {
	e, _, bear := newFixtureDeck(t, 72, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	e.emit(events.Event{Kind: events.MoveZone, Obj: bear, From: state.ZHand, To: state.ZBattlefield})
	effects.Resolve(e, &effects.Ctx{Source: bear, Controller: 0, Targets: []state.Target{{Obj: bear}}},
		&cards.SA{Kind: "DB", API: "Pump", Params: map[string]string{"KW": "Protection from red"}})
	red := putToken(t, e, 1, "Name:Goblin\nManaCost:R\nTypes:Creature Goblin\nPT:1/1\nOracle:x\n", state.ZBattlefield)
	devoid := putToken(t, e, 1, "Name:Breaker\nManaCost:6 G\nTypes:Creature Eldrazi\nPT:5/7\nK:Devoid\nOracle:x\n", state.ZBattlefield)
	if !e.protectedFrom(bear, red) || e.protectedFrom(bear, devoid) {
		t.Fatal("granted protection from red must apply to a red source and not to a devoid one")
	}
}
