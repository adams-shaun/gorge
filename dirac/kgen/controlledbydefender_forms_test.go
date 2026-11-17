package kgen

import "testing"

// controlledByDefenderForms: ControlledBy TriggeredDefendingPlayer / DefenderCtrl
// in a becomes-blocked or unblocked trigger's target filter (lower.go
// defenderTrigBody), the defending-controller role (ctrl=5, kop.F2DefendingCtrl)
// an Attacks trigger already lowered; an activated ability's stays refused
// (gorge binds no defending player there).
var controlledByDefenderForms = []formCase{
	{card: "Goblin Vandal", form: "AttackerUnblocked: target Artifact.ControlledBy TriggeredDefendingPlayer", lir: "F={types=0x4 ctrl=5}"},
	{card: "Rust Scarab", form: "AttackerBlocked: target Artifact/Enchantment.ControlledBy TriggeredDefendingPlayer", lir: "F={types=0x4 ctrl=5 | types=0x8 ctrl=5}"},
	{card: "Orcish Squatters", form: "AttackerUnblocked: target Land.DefenderCtrl", lir: "F={types=0x1 ctrl=5}"},
	{card: "Ancient Hellkite", form: "activated Creature.DefenderCtrl (no bound defender)", refused: "ControlledBy TriggeredDefendingPlayer outside"},
}

func TestControlledByDefendingPlayerForms(t *testing.T) { checkForms(t, controlledByDefenderForms) }
