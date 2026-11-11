package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestGrantedExtortTriggersPerInstance drives the granted-keyword path with a
// synthetic lord: "Other creatures you control have extort" (Pontiff of
// Blight's static). The lord itself has no Extort, so the only trigger is the
// granted one on the bear; casting a spell must pose the {W/B} election and,
// on pay, drain each opponent and gain the controller that much (CR 702.101a).
//
// Before grantedKeywordTrigger learned Extort the grant reached the layer
// system (HasKeyword is true) but no trigger was queued, so no KModes ask
// appeared and the bear dealt no drain.
func TestGrantedExtortTriggersPerInstance(t *testing.T) {
	bolt := card(t, "Name:Bolt\nManaCost:R\nTypes:Instant\nA:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 1\nOracle:x\n")
	e := handEngineTokens(t, bolt)
	lord := onBoard(t, e, 0, "Name:Extort Lord\nManaCost:B\nTypes:Creature\nPT:1/1\nS:Mode$ Continuous | Affected$ Creature.Other+YouCtrl | AddKeyword$ Extort | Description$ Other creatures you control have extort.\nOracle:x\n")
	bear := onBoard(t, e, 0, "Name:Grizzly Bears\nManaCost:1 G\nTypes:Creature\nPT:2/2\nOracle:x\n")
	// Preconditions: the keyword is granted to the OTHER creature only, and
	// the lone granted instance is what the cast will fire.
	if !e.HasKeyword(bear, "Extort") {
		t.Fatal("precondition: the lord did not grant the bear Extort")
	}
	if e.HasKeyword(lord, "Extort") {
		t.Fatal("precondition: the lord has Extort itself, so the trigger would not be the granted path")
	}
	e.G.Players[0].Pool[state.MR] = 1
	e.G.Players[0].Pool[state.MW] = 1
	e.pending = nil
	e.Advance()
	d := e.Pending()
	if d == nil {
		t.Fatal("no pending decision")
	}
	var ci = -1
	for _, o := range d.Options {
		if o.Kind == "cast" {
			ci = o.Index
		}
	}
	if ci < 0 {
		t.Fatalf("no cast option: %+v", d.Options)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{ci}}); err != nil {
		t.Fatalf("submit cast: %v", err)
	}
	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("expected a target decision for Bolt, got %+v", d)
	}
	ti := -1
	for _, o := range d.Options {
		if o.Kind == "player" && o.Player == 1 {
			ti = o.Index
		}
	}
	if ti < 0 {
		t.Fatalf("no opponent target option: %+v", d.Options)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{ti}}); err != nil {
		t.Fatalf("submit target: %v", err)
	}
	d = e.Pending()
	for d != nil && d.Kind == decision.KPriority {
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{0}}); err != nil {
			t.Fatal(err)
		}
		d = e.Pending()
	}
	if d == nil || d.Kind != decision.KModes {
		t.Fatalf("expected the granted Extort KModes ask, got %+v", d)
	}
	if d.Player != 0 {
		t.Fatalf("Extort ask player = %d, want the granted creature's controller 0", d.Player)
	}
	life0, life1 := e.G.Players[0].Life, e.G.Players[1].Life
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{0}}); err != nil {
		t.Fatalf("submit extort pay: %v", err)
	}
	passUntilStackEmpty(t, e, 40)
	// Bolt deals 1 to the opponent; the granted Extort drains 1 and gains it.
	if e.G.Players[1].Life != life1-2 {
		t.Fatalf("opponent life %d after the granted Extort, want %d", e.G.Players[1].Life, life1-2)
	}
	if e.G.Players[0].Life != life0+1 {
		t.Fatalf("caster life %d after the granted Extort, want %d", e.G.Players[0].Life, life0+1)
	}
}
