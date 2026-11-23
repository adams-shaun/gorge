package kgen

import "testing"

var filterHeadsForms = []formCase{
	{card: "Night Dealings", form: "ValidTarget$ Player.Other (an opponent, as Player.Opponent; the THEFT counter stops the build, not the LIR)", lir: "players=2"},
	{card: "Chandra's Flame Wave", form: "ValidPlayers$ Targeted + Creature.TargetedPlayerCtrl (the target player damaged after the creatures)", lir: "DamageAll"},
	{card: "Chandra, Bold Pyromancer", form: "ValidTgts$ Player scopes Creature,Planeswalker to the target's permanents", lir: "Scope"},
	{card: "Cerebral Eruption", form: "ValidPlayers$ Targeted over a PeekAndReveal Opponent slot (no scope)", lir: "DamageAll"},
	{card: "Crown of Doom", form: "Player.!CardOwner needs the owner read (kept refused)", refused: `unknown filter head "Player"`},
	{card: "Bifurcate", form: "Targeted.Permanent+sameName (the target's name read, FwSameName over RefTargets0123)", lir: "Search obj=You A=1 C=1 F={types=0x9f cmp(204 0 0  ) prog[w8(0 0  )]}"},
}

func TestFilterHeadsForms(t *testing.T) {
	checkForms(t, filterHeadsForms)
}
