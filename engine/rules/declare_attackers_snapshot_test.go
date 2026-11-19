package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestDeclareAttackersSnapshotPreDeclaration pins the engine's declaration
// instant at the pending attackers decision: before the intent is submitted
// the chosen creature is neither attacking nor tapped, and the submit makes
// it both in one turn-based action (CR 508.1, 508.1f, 508.1k).
//
// This is the convention the farm's `xengine fshadow` divergence
// `snapshot|attackers|permanents|?Lotus Cobra` turns on. The forge-shadow-1
// recorder writes a step's snapshot just before the step's first ask
// (ShadowLog.ask calls flushSnap), but RecordingController.declareAttackers
// runs the declaration before it calls log.ask, so Forge's
// `tN:declare-attackers` snapshot is post-declaration while gorge is still at
// its pending KAttackers decision. The two snapshots read different instants;
// gorge's state here is the rules-correct pre-declaration one, so a fix
// belongs in the recorder (flush before declaring) or in the follower, never
// in the engine.
func TestDeclareAttackersSnapshotPreDeclaration(t *testing.T) {
	t.Parallel()
	const lotusCobra = "Name:Lotus Cobra\nManaCost:1 G\nTypes:Creature Snake\nPT:2/1\nOracle:x\n"

	names := []string{"a", "b", "c", "d"}
	decks := make([][]*cards.Card, len(names))
	for i := range decks {
		decks[i] = mountainDeck(t, 40)
	}
	e := New(seatZeroStart(Config{Seed: 42, Names: names, Decks: decks}))
	// Seat 1 is the active player of the class's pod (Lotus Cobra's
	// controller), and its declare-attackers step is the checkpoint the
	// Forge log carries: tN:declare-attackers.
	e.G.Active = 1
	e.G.Step = state.StepDeclareAttackers
	cobra := onBoardReady(t, e, 1, lotusCobra)

	e.askAttackers()
	d := e.Pending()
	if d == nil || d.Kind != decision.KAttackers {
		t.Fatalf("pending = %+v, want a KAttackers decision (the state Forge's pre-ask snapshot is compared against)", d)
	}

	// Precondition: the creature is a legal attacker, so the pending
	// decision really offers it. Without this the pre-declaration assertion
	// below would pass vacuously.
	offered := false
	for _, o := range d.Options {
		if o.Obj == cobra {
			offered = true
			break
		}
	}
	if !offered {
		t.Fatalf("Lotus Cobra %d is not offered as an attacker: %+v", cobra, d.Options)
	}

	if o := e.G.Obj(cobra); o.IsAttacking || o.Tapped {
		t.Fatalf("before the declaration: IsAttacking=%v Tapped=%v, want false/false (CR 508.1: declaring is the turn-based action the pending decision answers)", o.IsAttacking, o.Tapped)
	}

	// One option per living defender (CR 506.2); pick the first, then make
	// the declaration by hand because submitAttackersOnly expects exactly
	// one option per requested creature.
	choice := -1
	for _, o := range d.Options {
		if o.Obj == cobra {
			choice = o.Index
			break
		}
	}
	if choice < 0 {
		t.Fatalf("no attack option for Lotus Cobra %d", cobra)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{choice}}); err != nil {
		t.Fatalf("submit the attack: %v", err)
	}
	o := e.G.Obj(cobra)
	if !o.IsAttacking {
		t.Fatal("after the declaration: IsAttacking=false, want true (CR 508.1k)")
	}
	if !o.Tapped {
		t.Fatal("after the declaration: Tapped=false, want true (CR 508.1f)")
	}
}
