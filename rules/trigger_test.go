package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

const etbDrawSrc = `Name:Scribe
ManaCost:1 U
Types:Creature Human
PT:1/1
T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ TrigDraw | TriggerDescription$ draw
SVar:TrigDraw:DB$ Draw | NumCards$ 1 | Defined$ You
Oracle:x
`

func TestEnterTheBattlefieldTriggerGoesOnTheStackAndResolves(t *testing.T) {
	e := layerEngine(t)
	before := len(e.G.Zone(state.ZHand, 0))
	id := onBoard(t, e, 0, etbDrawSrc)
	// onBoard bypasses events, so fire the zone change explicitly.
	e.G.SetZone(state.ZBattlefield, 0, nil)
	e.G.Obj(id).Zone = state.ZHand
	e.G.SetZone(state.ZHand, 0, append(e.G.Zone(state.ZHand, 0), id))
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZBattlefield})

	e.putTriggersOnStack()
	if len(e.G.Stack) != 1 {
		t.Fatalf("stack = %v, want the triggered ability", e.G.Stack)
	}
	e.resolveTop()
	if got := len(e.G.Zone(state.ZHand, 0)); got != before+1 {
		t.Fatalf("hand = %d, want %d after the ETB draw", got, before+1)
	}
}

func TestTriggerDoesNotFireForOtherObjects(t *testing.T) {
	e := layerEngine(t)
	watcher := onBoard(t, e, 0, etbDrawSrc)
	_ = watcher
	other := e.G.AddObject(card(t, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"), 0)
	other.Zone = state.ZHand
	e.G.SetZone(state.ZHand, 0, []state.ObjID{other.ID})
	e.emit(events.Event{Kind: events.MoveZone, Obj: other.ID,
		From: state.ZHand, To: state.ZBattlefield})
	e.putTriggersOnStack()
	if len(e.G.Stack) != 0 {
		t.Fatalf("Card.Self trigger fired for a different object: %v", e.G.Stack)
	}
}

func TestTriggerZonesGateFiring(t *testing.T) {
	// A graveyard-only trigger must not fire while the card is on the
	// battlefield.
	src := `Name:Ghoul
ManaCost:B
Types:Creature Zombie
PT:1/1
T:Mode$ Phase | Phase$ Upkeep | TriggerZones$ Graveyard | Execute$ TrigLose | TriggerDescription$ x
SVar:TrigLose:DB$ LoseLife | LifeAmount$ 1 | Defined$ You
Oracle:x
`
	e := layerEngine(t)
	id := onBoard(t, e, 0, src)
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepUpkeep})
	e.putTriggersOnStack()
	if len(e.G.Stack) != 0 {
		t.Fatal("a graveyard trigger fired from the battlefield")
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: id,
		From: state.ZBattlefield, To: state.ZGraveyard})
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepUpkeep})
	e.putTriggersOnStack()
	if len(e.G.Stack) != 1 {
		t.Fatal("the graveyard trigger did not fire from the graveyard")
	}
}

func TestSimultaneousTriggersStackInAPNAPOrder(t *testing.T) {
	e := layerEngine(t)
	// Two upkeep triggers, one per seat. APNAP means the active player's goes
	// on the stack first and therefore resolves last.
	src := `Name:Bell
ManaCost:W
Types:Enchantment
T:Mode$ Phase | Phase$ Upkeep | Execute$ TrigGain | TriggerDescription$ x
SVar:TrigGain:DB$ GainLife | LifeAmount$ 1 | Defined$ You
Oracle:x
`
	a := onBoard(t, e, 0, src)
	b := onBoard(t, e, 1, src)
	e.G.Active = 0
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepUpkeep})
	e.putTriggersOnStack()
	if len(e.G.Stack) != 2 {
		t.Fatalf("stack = %v, want two triggers", e.G.Stack)
	}
	if e.G.Obj(e.G.Stack[0]).Source != a || e.G.Obj(e.G.Stack[1]).Source != b {
		t.Fatal("triggers were not stacked in APNAP order")
	}
}

func TestReplacementRedirectsTheEvent(t *testing.T) {
	src := `Name:Phoenix
ManaCost:2 R
Types:Creature Phoenix
PT:2/2
R:Event$ Moved | Destination$ Graveyard | ValidCard$ Card.Self | ReplaceWith$ RepExile | Description$ x
SVar:RepExile:DB$ ChangeZone | Origin$ Battlefield | Destination$ Exile | Defined$ Self
Oracle:x
`
	e := layerEngine(t)
	id := onBoard(t, e, 0, src)
	e.emit(events.Event{Kind: events.MoveZone, Obj: id,
		From: state.ZBattlefield, To: state.ZGraveyard})
	if e.G.Obj(id).Zone != state.ZExile {
		t.Fatalf("zone = %s, want exile", e.G.Obj(id).Zone)
	}
	if len(e.G.Zone(state.ZGraveyard, 0)) != 0 {
		t.Fatal("the replaced move still reached the graveyard")
	}
}

func TestReplacementAppliesOnlyOncePerEvent(t *testing.T) {
	// A replacement that re-emits a matching event must not re-trigger itself
	// into an infinite loop.
	src := `Name:Loop
ManaCost:1
Types:Artifact
R:Event$ Moved | Destination$ Graveyard | ValidCard$ Card.Self | ReplaceWith$ RepAgain | Description$ x
SVar:RepAgain:DB$ ChangeZone | Origin$ Battlefield | Destination$ Graveyard | Defined$ Self
Oracle:x
`
	e := layerEngine(t)
	id := onBoard(t, e, 0, src)
	done := make(chan struct{})
	go func() {
		e.emit(events.Event{Kind: events.MoveZone, Obj: id,
			From: state.ZBattlefield, To: state.ZGraveyard})
		close(done)
	}()
	<-done
	if e.G.Obj(id).Zone != state.ZGraveyard {
		t.Fatalf("zone = %s", e.G.Obj(id).Zone)
	}
}

// --- Additional coverage beyond the brief's mandatory six, for the five
// trigger modes those tests don't otherwise exercise, and for the totality
// concerns (cascade bound, SVar threading, F3 compliance) the brief calls
// out by name. ---

func TestSpellCastTriggerMatchesValidCardAndPlayer(t *testing.T) {
	src := `Name:Watcher
ManaCost:1 U
Types:Creature Wizard
PT:1/1
T:Mode$ SpellCast | ValidCard$ Instant | ValidActivatingPlayer$ Opponent | Execute$ TrigDraw | TriggerDescription$ x
SVar:TrigDraw:DB$ Draw | NumCards$ 1 | Defined$ You
Oracle:x
`
	e := layerEngine(t)
	onBoard(t, e, 0, src)
	bolt := card(t, "Name:Shock\nManaCost:R\nTypes:Instant\nA:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 1\nOracle:x\n")
	spell := e.G.AddObject(bolt, 1)
	spell.Zone = state.ZHand
	e.G.SetZone(state.ZHand, 1, []state.ObjID{spell.ID})
	e.emit(events.Event{Kind: events.PutOnStack, Obj: spell.ID, Player: 1,
		From: state.ZHand, To: state.ZStack})
	e.putTriggersOnStack()
	if len(e.G.Stack) != 2 { // the spell itself, plus the trigger
		t.Fatalf("stack = %v, want the spell plus one trigger", e.G.Stack)
	}
}

func TestSpellCastTriggerIgnoresATriggeredAbilityEnteringTheStack(t *testing.T) {
	// A SpellCast trigger with a bare "Any" filter must not fire for a
	// triggered ability object (no Face -- Ruling F3) entering the stack;
	// only an actual spell (a card) counts as "casting a spell".
	src := `Name:Watcher
ManaCost:1 U
Types:Creature Wizard
PT:1/1
T:Mode$ SpellCast | ValidCard$ Any | Execute$ TrigDraw | TriggerDescription$ x
SVar:TrigDraw:DB$ Draw | NumCards$ 1 | Defined$ You
Oracle:x
`
	src2 := `Name:Herald
ManaCost:W
Types:Creature Human
PT:1/1
T:Mode$ Phase | Phase$ Upkeep | Execute$ TrigGain | TriggerDescription$ x
SVar:TrigGain:DB$ GainLife | LifeAmount$ 1 | Defined$ You
Oracle:x
`
	e := layerEngine(t)
	onBoard(t, e, 0, src)
	onBoard(t, e, 0, src2)
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepUpkeep})
	e.putTriggersOnStack()
	if len(e.G.Stack) != 1 {
		t.Fatalf("stack = %v, want only the Phase trigger", e.G.Stack)
	}
	if e.G.Obj(e.G.Stack[0]).Ability == nil {
		t.Fatal("expected the stack object to be an ability")
	}
}

func TestAttacksTriggerFiresForTheDeclaredAttacker(t *testing.T) {
	src := `Name:Raider
ManaCost:1 R
Types:Creature Goblin
PT:2/2
T:Mode$ Attacks | ValidCard$ Card.Self | Execute$ TrigPump | TriggerDescription$ x
SVar:TrigPump:DB$ Pump | Defined$ Self | NumAtt$ +1
Oracle:x
`
	e := layerEngine(t)
	id := onBoard(t, e, 0, src)
	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 0, IDs: []state.ObjID{id}})
	e.putTriggersOnStack()
	if len(e.G.Stack) != 1 {
		t.Fatalf("stack = %v, want the attack trigger", e.G.Stack)
	}
}

func TestAttacksTriggerDoesNotFireForANonAttacker(t *testing.T) {
	src := `Name:Raider
ManaCost:1 R
Types:Creature Goblin
PT:2/2
T:Mode$ Attacks | ValidCard$ Card.Self | Execute$ TrigPump | TriggerDescription$ x
SVar:TrigPump:DB$ Pump | Defined$ Self | NumAtt$ +1
Oracle:x
`
	e := layerEngine(t)
	id := onBoard(t, e, 0, src)
	other := onBoard(t, e, 0, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	_ = id
	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 0, IDs: []state.ObjID{other}})
	e.putTriggersOnStack()
	if len(e.G.Stack) != 0 {
		t.Fatalf("stack = %v, want no trigger for a non-attacker", e.G.Stack)
	}
}

func TestDamageDoneTriggerMatchesValidTarget(t *testing.T) {
	src := `Name:Vampire
ManaCost:1 B
Types:Creature Vampire
PT:2/2
T:Mode$ DamageDone | ValidTarget$ Player | Execute$ TrigGain | TriggerDescription$ x
SVar:TrigGain:DB$ GainLife | LifeAmount$ 1 | Defined$ You
Oracle:x
`
	e := layerEngine(t)
	onBoard(t, e, 0, src)
	e.emit(events.Event{Kind: events.Damage, Player: 1, Amount: 2})
	e.putTriggersOnStack()
	if len(e.G.Stack) != 1 {
		t.Fatalf("stack = %v, want the damage trigger", e.G.Stack)
	}
}

func TestDamageDealtOnceFiresAtMostOncePerTurn(t *testing.T) {
	src := `Name:Vampire
ManaCost:1 B
Types:Creature Vampire
PT:2/2
T:Mode$ DamageDealtOnce | ValidTarget$ Player | Execute$ TrigGain | TriggerDescription$ x
SVar:TrigGain:DB$ GainLife | LifeAmount$ 1 | Defined$ You
Oracle:x
`
	e := layerEngine(t)
	onBoard(t, e, 0, src)
	e.emit(events.Event{Kind: events.Damage, Player: 1, Amount: 2})
	e.emit(events.Event{Kind: events.Damage, Player: 1, Amount: 2})
	e.putTriggersOnStack()
	if len(e.G.Stack) != 1 {
		t.Fatalf("stack = %v, want exactly one trigger for two damage events in the same turn", e.G.Stack)
	}
	e.resolveTop() // clear the stack so the next check counts only new triggers.
	// Advancing to the next turn resets the gate.
	e.G.Turn++
	e.emit(events.Event{Kind: events.Damage, Player: 1, Amount: 2})
	e.putTriggersOnStack()
	if len(e.G.Stack) != 1 {
		t.Fatalf("stack = %v, want the trigger to fire again next turn", e.G.Stack)
	}
}

func TestBecomesTargetTriggerFiresWhenTargeted(t *testing.T) {
	src := `Name:Ward
ManaCost:1 W
Types:Creature Soldier
PT:1/3
T:Mode$ BecomesTarget | ValidTarget$ Card.Self | Execute$ TrigGain | TriggerDescription$ x
SVar:TrigGain:DB$ GainLife | LifeAmount$ 1 | Defined$ You
Oracle:x
`
	e := layerEngine(t)
	id := onBoard(t, e, 0, src)
	e.emit(events.Event{Kind: events.TargetsChosen, Obj: 999, IDs: []state.ObjID{id}})
	e.putTriggersOnStack()
	if len(e.G.Stack) != 1 {
		t.Fatalf("stack = %v, want the becomes-target trigger", e.G.Stack)
	}
}

func TestLandPlayedTriggerFiresOnlyForLands(t *testing.T) {
	src := `Name:Ranger
ManaCost:1 G
Types:Creature Human
PT:2/2
T:Mode$ LandPlayed | ValidCard$ Land | Execute$ TrigGain | TriggerDescription$ x
SVar:TrigGain:DB$ GainLife | LifeAmount$ 1 | Defined$ You
Oracle:x
`
	e := layerEngine(t)
	onBoard(t, e, 0, src)
	land := e.G.AddObject(card(t, "Name:Forest\nTypes:Basic Land Forest\nOracle:x\n"), 0)
	land.Zone = state.ZHand
	e.G.SetZone(state.ZHand, 0, []state.ObjID{land.ID})
	e.emit(events.Event{Kind: events.MoveZone, Obj: land.ID, From: state.ZHand, To: state.ZBattlefield})
	e.emit(events.Event{Kind: events.LandPlayed, Player: 0})
	e.putTriggersOnStack()
	if len(e.G.Stack) != 1 {
		t.Fatalf("stack = %v, want the land-played trigger", e.G.Stack)
	}
}

func TestLandPlayedTriggerIgnoresTheSeparateLandPlayedEvent(t *testing.T) {
	// The bookkeeping LandPlayed event carries no Obj, so it must never by
	// itself satisfy a LandPlayed trigger's ValidCard$.
	src := `Name:Ranger
ManaCost:1 G
Types:Creature Human
PT:2/2
T:Mode$ LandPlayed | ValidCard$ Land | Execute$ TrigGain | TriggerDescription$ x
SVar:TrigGain:DB$ GainLife | LifeAmount$ 1 | Defined$ You
Oracle:x
`
	e := layerEngine(t)
	onBoard(t, e, 0, src)
	e.emit(events.Event{Kind: events.LandPlayed, Player: 0})
	e.putTriggersOnStack()
	if len(e.G.Stack) != 0 {
		t.Fatalf("stack = %v, want no trigger from the bookkeeping event alone", e.G.Stack)
	}
}

func TestTriggerCascadeIsBounded(t *testing.T) {
	// A trigger that fires in response to its own effect must not queue
	// forever: maxTriggerFires caps how many times one (source, index) pair
	// can enqueue, even across many repeats of the same event.
	src := `Name:Looper
ManaCost:1 B
Types:Creature Horror
PT:1/1
T:Mode$ DamageDone | ValidSource$ Card.Self | Execute$ TrigGain | TriggerDescription$ x
SVar:TrigGain:DB$ GainLife | LifeAmount$ 1 | Defined$ You
Oracle:x
`
	e := layerEngine(t)
	id := onBoard(t, e, 0, src)
	// Put the object itself on top of the stack so damageSource() names it.
	e.G.Stack = []state.ObjID{id}
	for i := 0; i < maxTriggerFires+50; i++ {
		e.emit(events.Event{Kind: events.Damage, Player: 1, Amount: 1})
	}
	if got := len(e.pendingTriggers); got != maxTriggerFires {
		t.Fatalf("pendingTriggers = %d, want the cascade capped at %d", got, maxTriggerFires)
	}
}

func TestDiesTriggerFiresWithDefaultTriggerZones(t *testing.T) {
	// A "dies" trigger's own source leaves the battlefield as part of the
	// very event it must react to: by the time checkTriggers runs, the
	// object's current zone is already the graveyard, so a naive zoneGate
	// checking only the post-move zone would never see it "on the
	// battlefield" and would never fire. zoneGate must also accept the zone
	// the object was just in for the specific event that moved it.
	src := `Name:Imp
ManaCost:B
Types:Creature Imp
PT:1/1
T:Mode$ ChangesZone | Origin$ Battlefield | Destination$ Graveyard | ValidCard$ Card.Self | Execute$ TrigLose | TriggerDescription$ x
SVar:TrigLose:DB$ LoseLife | LifeAmount$ 1 | Defined$ You
Oracle:x
`
	e := layerEngine(t)
	id := onBoard(t, e, 0, src)
	e.emit(events.Event{Kind: events.MoveZone, Obj: id,
		From: state.ZBattlefield, To: state.ZGraveyard})
	e.putTriggersOnStack()
	if len(e.G.Stack) != 1 {
		t.Fatalf("stack = %v, want the dies trigger", e.G.Stack)
	}
}

func TestReplacementDoesNotFireForAnUnrelatedDestination(t *testing.T) {
	src := `Name:Phoenix
ManaCost:2 R
Types:Creature Phoenix
PT:2/2
R:Event$ Moved | Destination$ Graveyard | ValidCard$ Card.Self | ReplaceWith$ RepExile | Description$ x
SVar:RepExile:DB$ ChangeZone | Origin$ Battlefield | Destination$ Exile | Defined$ Self
Oracle:x
`
	e := layerEngine(t)
	id := onBoard(t, e, 0, src)
	e.emit(events.Event{Kind: events.MoveZone, Obj: id,
		From: state.ZBattlefield, To: state.ZHand})
	if e.G.Obj(id).Zone != state.ZHand {
		t.Fatalf("zone = %s, want hand: an unrelated destination must not be replaced", e.G.Obj(id).Zone)
	}
}
