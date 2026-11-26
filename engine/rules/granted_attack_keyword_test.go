package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestGrantedAnnihilatorSacrifices: a Continuous static's AddKeyword$
// Annihilator:2 (Idol of False Gods' self-grant, Eldrazi Conscription's aura
// grant) gives the recipient the printed keyword's rules text (CR 702.86a,
// 613.1f): "whenever this creature attacks, defending player sacrifices two
// permanents". The granted walk must queue the printed expansion's trigger
// (engine/cards/kw_annihilator.go) and the defending player must sacrifice two.
// A creature the static does not reach queues nothing.
func TestGrantedAnnihilatorSacrifices(t *testing.T) {
	t.Parallel()
	e := combatEngine(t)
	onBoard(t, e, 0, "Name:Granter\nManaCost:2\nTypes:Artifact\n"+
		"S:Mode$ Continuous | Affected$ Creature.YouCtrl+namedBear | AddKeyword$ Annihilator:2 | Description$ x\nOracle:x\n")
	bear := onBoard(t, e, 0, "Name:Bear\nManaCost:1 G\nTypes:Creature\nPT:2/2\nOracle:x\n")
	other := onBoard(t, e, 0, "Name:Wolf\nManaCost:1 G\nTypes:Creature\nPT:2/2\nOracle:x\n")
	e.G.Obj(bear).SummonSick, e.G.Obj(other).SummonSick = false, false
	p1 := onBoard(t, e, 1, "Name:One\nTypes:Artifact\nOracle:x\n")
	p2 := onBoard(t, e, 1, "Name:Two\nTypes:Artifact\nOracle:x\n")
	p3 := onBoard(t, e, 1, "Name:Three\nTypes:Artifact\nOracle:x\n")
	if !e.HasKeyword(bear, "Annihilator") || e.HasKeyword(other, "Annihilator") {
		t.Fatalf("precondition: the grant reaches the bear only: bear %v wolf %v", e.Derived(bear).Keywords, e.Derived(other).Keywords)
	}
	if len(e.G.Obj(bear).Face().Triggers) != 0 {
		t.Fatal("precondition: the bear prints a trigger")
	}
	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{bear, other}})
	if len(e.pendingTriggers) != 1 {
		t.Fatalf("granted annihilator queued %d triggers, want 1", len(e.pendingTriggers))
	}
	e.putTriggersOnStack()
	e.resolveTop()
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.Player != 1 {
		t.Fatalf("granted annihilator did not ask the defending player to sacrifice: %+v", d)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: 1, Choices: []int{0, 2}}); err != nil {
		t.Fatal(err)
	}
	gone := 0
	for _, id := range []state.ObjID{p1, p2, p3} {
		if e.G.Obj(id).Zone == state.ZGraveyard {
			gone++
		}
	}
	if gone != 2 {
		t.Fatalf("defending player sacrificed %d permanents, want 2", gone)
	}
}

// TestGrantedAnnihilatorVariableFailsClosed: Ulamog, the Defiler's
// AddKeyword$ Annihilator:X reads an SVar the keyword line does not carry;
// the synthesizer builds nothing for it rather than a wrong count.
func TestGrantedAnnihilatorVariableFailsClosed(t *testing.T) {
	t.Parallel()
	if grantedKeywordTrigger("Annihilator:X") != nil || grantedKeywordTrigger("Annihilator") != nil {
		t.Fatal("a non-literal or missing annihilator count synthesized a trigger")
	}
	if grantedKeywordTrigger("Annihilator:2") == nil {
		t.Fatal("Annihilator:2 synthesized no trigger")
	}
}

// TestGrantedBattleCryPumpsOtherAttackers: a Pump KW$ Battle cry (Reckless
// Pyrosurfer's landfall self-grant, CR 702.91a) gives the creature the
// printed keyword's trigger (engine/cards/kw_battle_cry.go): each OTHER
// attacking creature gets +1/+0, the source itself does not.
func TestGrantedBattleCryPumpsOtherAttackers(t *testing.T) {
	t.Parallel()
	e := combatEngine(t)
	surfer := onBoard(t, e, 0, "Name:Surfer\nManaCost:1 R\nTypes:Creature\nPT:2/2\n"+
		"A:AB$ Pump | Cost$ 0 | Defined$ Self | KW$ Battle cry | SpellDescription$ x\nOracle:x\n")
	wolf := onBoard(t, e, 0, "Name:Wolf\nManaCost:1 G\nTypes:Creature\nPT:2/2\nOracle:x\n")
	e.G.Obj(surfer).SummonSick, e.G.Obj(wolf).SummonSick = false, false
	f := e.G.Obj(surfer).Face()
	e.resolveAbility(surfer, 0, nil, f.Abilities[0], f.SVars)
	if !e.HasKeyword(surfer, "Battle cry") {
		t.Fatalf("precondition: the pump did not grant Battle cry: %v", e.Derived(surfer).Keywords)
	}
	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{surfer, wolf}})
	if len(e.pendingTriggers) != 1 {
		t.Fatalf("granted battle cry queued %d triggers, want 1", len(e.pendingTriggers))
	}
	e.putTriggersOnStack()
	e.resolveTop()
	if got := e.Derived(wolf).Power; got != 3 {
		t.Fatalf("other attacker power = %d, want 3", got)
	}
	if got := e.Derived(surfer).Power; got != 2 {
		t.Fatalf("battle cry source power = %d, want 2 (it pumps only the others)", got)
	}
}
