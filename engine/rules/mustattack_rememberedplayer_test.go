package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// ds2MustAttackBoard is a three-seat board for the RememberedPlayerCtrl
// requirement: seat 1 is the active attacker and owns the creature the
// requirement is scoped to, and seat 2 owns an untouched control creature
// (which the engine never offers as an attacker, because CR 508.1 asks only
// the active player to declare attackers -- that is exactly what makes it a
// control for "the requirement selected the remembered player's creature").
// Seat 2 is also the only living opponent of the active seat, so it is a
// legitimate named defender for the attack-duty test below.
//
// The registrations below bind seat 1 as their remembered PLAYER, the way
// Rowan Kenrith's `AB$ Effect | ValidTgts$ Player | RememberObjects$ Targeted
// | StaticAbilities$ MustAttack` and Gideon Jura's DelayedTrigger Effect
// capture it (engine/effects/effect.go's `case cards.StaticMustAttack` fills
// state.ContinuousEffect.RememberedPlayers through effectRememberedPlayers).
func ds2MustAttackBoard(t *testing.T) (e *Engine, victim, control state.ObjID) {
	t.Helper()
	e = threeSeatEngine(t)
	e.G.Active = 1
	e.G.Step = state.StepDeclareAttackers
	victim = onBoardReady(t, e, 1, "Name:Balduvian Bears\nManaCost:1 G\nTypes:Creature\nPT:2/2\nOracle:x\n")
	control = onBoardReady(t, e, 2, "Name:Grizzly Bears\nManaCost:1 G\nTypes:Creature\nPT:2/2\nOracle:x\n")
	// Preconditions the assertions depend on: both are genuine battlefield
	// creatures of their seats, and the scoped one really can attack, so a
	// "not Required" verdict below can never be the artifact of an
	// unable-to-attack creature.
	for _, c := range []struct {
		id   state.ObjID
		seat state.PlayerID
	}{{victim, 1}, {control, 2}} {
		o := e.G.Obj(c.id)
		if o == nil || o.Zone != state.ZBattlefield || o.Controller != c.seat || !e.IsCreature(c.id) {
			t.Fatalf("precondition: %+v must be a battlefield creature controlled by seat %d", o, c.seat)
		}
	}
	if !e.canAttack(victim) || !e.canAttackPair(victim, 2) {
		t.Fatal("precondition: the remembered player's creature must be able to attack seat 2")
	}
	return e, victim, control
}

// ds2RowanRegistration is the Effect registration shape both card scripts
// produce: Restriction MustAttack, its ValidCreature$ scoped through
// Creature.RememberedPlayerCtrl, and the captured player on the registration.
func ds2RowanRegistration(source state.ObjID, params map[string]string, remembered state.PlayerID) ContinuousEffect {
	return ContinuousEffect{
		Source: source, Controller: 0, UntilEOT: true,
		Restriction:       "MustAttack",
		RestrictParams:    params,
		RememberedPlayers: []state.PlayerID{remembered},
	}
}

// TestMustAttackRememberedPlayerCtrlBindsTheCapturedPlayer is the Rowan
// Kenrith half (CR 508.1d): an Effect-registered
// `Mode$ MustAttack | ValidCreature$ Creature.RememberedPlayerCtrl` with no
// MustAttack$ reference requires exactly the creatures the captured player
// controls, so the captured (active) player's creature must be declared and a
// different player's creature is untouched. Before the fix
// mustAttackLineSelects passed nil for the remembered players, the predicate
// failed closed, and the line contributed nothing.
func TestMustAttackRememberedPlayerCtrlBindsTheCapturedPlayer(t *testing.T) {
	t.Parallel()
	e, victim, control := ds2MustAttackBoard(t)
	if e.attackRequirements(victim).Any() || e.attackRequirements(control).Any() {
		t.Fatal("precondition: a bare board must carry no attack requirement")
	}
	e.AddContinuous(ds2RowanRegistration(victim,
		map[string]string{"Mode": "MustAttack", "ValidCreature": "Creature.RememberedPlayerCtrl"}, 1))

	if got := e.attackRequirements(victim); !got.Broad || len(got.Named) != 0 {
		t.Fatalf("the remembered player's creature must carry a BROAD requirement, got %+v", got)
	}
	if !e.mustAttackRequired(victim) {
		t.Fatal("the remembered player's creature must be Required by the ValidCreature$ Creature.RememberedPlayerCtrl Effect")
	}
	if got := e.attackRequirements(control); got.Any() {
		t.Fatalf("a creature of a different player must carry no requirement, got %+v", got)
	}
	if e.mustAttackRequired(control) {
		t.Fatal("a creature of a different player must not be Required")
	}
	// The KAttackers decision carries the same verdict, so the seat is really
	// told to declare it (this is what the XMage scenario submits against).
	e.askAttackers()
	d := e.Pending()
	if d == nil || d.Kind != decision.KAttackers {
		t.Fatalf("precondition: no declare-attackers decision pending after askAttackers (%+v)", d)
	}
	required := map[state.ObjID]bool{}
	for _, o := range d.Options {
		if o.Required {
			required[o.Obj] = true
		}
	}
	if len(required) != 1 || !required[victim] {
		t.Fatalf("required attacker objects = %v, want exactly the remembered player's creature %d", required, victim)
	}
}

// TestMustAttackRememberedPlayerCtrlIsPlayerScoped pins the *scoping* half of
// the fix (CR 508.1d): binding the registration's captured players must
// select the creatures that player controls and nothing else, so a
// registration that captured a DIFFERENT player leaves the creature alone.
// Before the fix the captured players were dropped (nil), so the first half
// was empty and this pair of boards behaved identically.
func TestMustAttackRememberedPlayerCtrlIsPlayerScoped(t *testing.T) {
	t.Parallel()
	params := map[string]string{"Mode": "MustAttack", "ValidCreature": "Creature.RememberedPlayerCtrl"}
	// The captured player is the creature's controller: selected.
	e, victim, _ := ds2MustAttackBoard(t)
	e.AddContinuous(ds2RowanRegistration(victim, params, 1))
	if !e.mustAttackRequired(victim) {
		t.Fatal("precondition: a registration capturing the creature's controller must select it")
	}

	// The captured player is somebody else: not selected, so the line
	// contributes nothing and the creature is free.
	e2, victim2, _ := ds2MustAttackBoard(t)
	e2.AddContinuous(ds2RowanRegistration(victim2, params, 2))
	if got := e2.attackRequirements(victim2); got.Any() {
		t.Fatalf("a registration capturing another player must not select this creature, got %+v", got)
	}
	if e2.mustAttackRequired(victim2) {
		t.Fatal("a registration capturing another player must not make this creature Required")
	}
}
