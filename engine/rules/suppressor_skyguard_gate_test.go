package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

// Suppressor Skyguard's intervening-if is `CheckDefinedPlayer$ Player.OpponentOf
// TriggeredAttackingPlayer+!Defending`: "that player has another opponent who
// isn't being attacked". Inline script (the 2026-11-20 corpus spelling).
const skyguardSrc = "Name:Skyguard Probe\nManaCost:2 W U\nTypes:Creature Human Knight\nPT:2/4\nK:Flying\n" +
	"T:Mode$ AttackersDeclared | TriggerZones$ Battlefield | AttackedTarget$ You | AttackingPlayer$ Player | CheckDefinedPlayer$ Player.OpponentOf TriggeredAttackingPlayer+!Defending | Execute$ SkyProbeFog | TriggerDescription$ Probe: fog.\n" +
	"SVar:SkyProbeFog:DB$ Effect | ReplacementEffects$ SkyProbeRepl | Duration$ UntilEndOfCombat\n" +
	"SVar:SkyProbeRepl:Event$ DamageDone | Prevent$ True | IsCombat$ True | ActiveZones$ Command | ValidTarget$ You | Description$ Probe: prevent.\n" +
	"Oracle:Probe.\n"

// skyguardPushes seats a Skyguard on seat 1, puts two ready attackers on seat
// 0 and declares each at the given defender seat, returning how many triggers
// the declaration pushed. `seats` is the table size; seat 2 is the unattacked
// third player when it exists.
func skyguardPushes(t *testing.T, seats int, defenders [2]int) int {
	t.Helper()
	var e *Engine
	if seats == 3 {
		e = threeSeatEngine(t)
	} else {
		e = layerEngine(t)
	}
	if len(e.G.Players) != seats {
		t.Fatalf("precondition: %d seats, want %d", len(e.G.Players), seats)
	}
	onBoardCard(t, e, 1, card(t, skyguardSrc))
	a := onBoardReady(t, e, 0, "Name:BearA\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	b := onBoardReady(t, e, 0, "Name:BearB\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	e.G.Active = 0
	e.G.Step = state.StepDeclareAttackers
	e.askAttackers()
	ai := batchAttackerOptionIndex(t, e, a, state.PlayerID(defenders[0]))
	bi := batchAttackerOptionIndex(t, e, b, state.PlayerID(defenders[1]))
	declareAttackersAt(t, e, [][2]int{{int(a), ai}, {int(b), bi}})
	return batchTriggerPushes(e)
}

func TestSuppressorSkyguardOpponentGate(t *testing.T) {
	// 3 seats, both attackers at the Skyguard's controller: seat 2 is another
	// opponent of the attacker that is not being attacked -> the gate holds.
	if got := skyguardPushes(t, 3, [2]int{1, 1}); got != 1 {
		t.Errorf("3 seats, only seat 1 attacked: Skyguard pushed %d triggers, want 1", got)
	}
	// 3 seats, the attack splits across seats 1 and 2: the attacker has no
	// unattacked opponent -> the gate fails.
	if got := skyguardPushes(t, 3, [2]int{1, 2}); got != 0 {
		t.Errorf("3 seats, seats 1 and 2 both attacked: Skyguard pushed %d triggers, want 0", got)
	}
	// 2 seats: the attacker's only opponent is the attacked seat -> fails.
	if got := skyguardPushes(t, 2, [2]int{1, 1}); got != 0 {
		t.Errorf("2 seats: Skyguard pushed %d triggers, want 0", got)
	}
}

// TestSuppressorSkyguardGateNeedsTheAttackRoles: with no trigger context (a
// replacement's gate, or a hand-built caller) the attacking-player role is
// unbound, so the spelling holds for nobody even though every seat but one is
// an "opponent" -- it never widens to "any other seat".
func TestSuppressorSkyguardGateNeedsTheAttackRoles(t *testing.T) {
	e := threeSeatEngine(t)
	src := onBoardCard(t, e, 1, card(t, skyguardSrc))
	const spec = "Player.OpponentOf TriggeredAttackingPlayer+!Defending"
	if e.checkDefinedPlayerHolds(spec, src, 1, nil) {
		t.Fatal("no attack roles bound: the gate must fail closed")
	}
	atk := state.Target{Player: 0, IsPlayer: true}
	tc := &effects.TriggerContext{AttackingPlayer: atk}
	if e.checkDefinedPlayerHolds(spec, src, 1, tc) {
		t.Fatal("attacking player bound but no defending player: `!Defending` must not invert the absence into a match")
	}
	tc.DefendingPlayer = state.Target{Player: 1, IsPlayer: true}
	if !e.checkDefinedPlayerHolds(spec, src, 1, tc) {
		t.Fatal("seat 0 attacks seat 1 on a three-seat table: seat 2 is an unattacked opponent, the gate must hold")
	}
}
