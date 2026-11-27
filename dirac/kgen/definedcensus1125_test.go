package kgen

import "testing"

// definedCensus1125Forms: the Defined$ values the 11-25 census lowered
// (lower_definedcensus1125.go), each beside a shape of the same value that
// stays refused, and the census values gorge's effects Defined resolver does
// not read (engine/effects/testdata/defined-selector-unknown.txt) or reads in
// a way the kernel cannot name exactly, which stay refused by name.
var definedCensus1125Forms = []formCase{
	// gorge context.go definedSpec, the bare TriggeredDefender arm:
	// c.DefendingPlayer, as definedSpecTriggeredDefendingPlayer reads it
	{card: "Unquenchable Fury", form: "Defined$ TriggeredDefender on the granted own-attacker Attacks trigger", lir: "DealDamage obj=Defending A=HandSizeOf(Defending)"},
	{card: "Calculating Lich", form: "Defined$ TriggeredDefender on another attacker's Attacks trigger", refused: `Defined$ "TriggeredDefender" outside an Attacks trigger of the controller's own attacker`},
	// gorge context.go definedSpecTargetedCard: refTargetUnion's objects
	{card: "Will of the Sultai", form: "Defined$ TargetedCard over a one-slot creature chain", lir: "GrantKeyword obj=Target1 CS=Trample"},
	// gorge context.go definedZoneValidTargets' ValidExile arm: the resolving
	// controller's exile zone, in zone order, each match copied. A ValidExile
	// read outside CopyPermanent (Court of Locthwain's Effect) still refuses.
	{card: "Altaïr Ibn-La'Ahad", form: "CopyPermanent Defined$ ValidExile Card.Creature+YouOwn+counters_GE1_MEMORY", lir: "CopyAllExile CS=Tapped|Attacking|ExileAtCombat"},
	{card: "Court of Locthwain", form: "Effect Defined$ ValidExile outside CopyPermanent", refused: `Defined$ "ValidExile`},
	// gorge context.go: " & " joins independent selectors (knownDefinedTargets,
	// DefinedRef's RefCompound arm), each member keeping its own reading:
	// definedSpecTriggeredAttacker + definedSpecSelf for Alaborn Zealot's
	// Destroy, definedSpecYou + definedSpecTriggeredTarget for Angel of
	// Destiny's GainLife. The effect runs once per member.
	// Each effect runs once per member, so each member gets its own entry: a
	// lowering that dropped one would fail its entry. (The card appears
	// twice; checkForms keys by name and re-checks the same lowered card.)
	{card: "Alaborn Zealot", form: "Destroy Defined$ TriggeredAttackerLKICopy & Self (compound subjects, first member)", lir: "Destroy obj=Triggered"},
	{card: "Alaborn Zealot", form: "Destroy Defined$ TriggeredAttackerLKICopy & Self (compound subjects, second member)", lir: "Destroy obj=Self"},
	{card: "Angel of Destiny", form: "GainLife Defined$ You & TriggeredTarget (compound players, first member)", lir: "GainLife obj=You"},
	{card: "Angel of Destiny", form: "GainLife Defined$ You & TriggeredTarget (compound players, second member)", lir: "GainLife obj=TriggeredPlayer A=TriggerAmount"},
	// gorge effects/player_filter.go playerStateQualifierAttackedBySourceThisTurn
	// (the source object's AttackedPlayersThisTurn ledger folded by
	// DeclareAttackers), read through the same player-set grammar definedSpec
	// uses.
	{card: "Angel of Destiny", form: "LosesGame Defined$ Player.attackedBySourceThisTurn", lir: "LoseGame obj=AttackedBySource"},
	{card: "Cait, Cage Brawler", form: "Discard Defined$ TriggeredDefendingPlayer & You (compound outside an effect subject's grammar)", refused: `Defined$ "TriggeredDefendingPlayer & You"`},
	// gorge does not read these (defined-selector-unknown.txt)
	{card: "Prismari Command", form: "Draw Defined$ ThisTargetedPlayer (gorge unread)", refused: `Defined$ "ThisTargetedPlayer"`},
	{card: "Glorious End", form: "LosesGame Defined$ SourceController (gorge unread)", refused: `Defined$ "SourceController"`},
	// gorge reads these, but not as any kernel ref: ParentTargetedController
	// is the live controller (definedSpecParentTargetedController, no CR
	// 608.2h look-back) where RefTargetController reads the last known one;
	// CorrectedSelf names a returned source Self does not (selfDefinedTarget)
	{card: "Unlicensed Disintegration", form: "Defined$ ParentTargetedController (live, not last-known, controller)", refused: `Defined$ "ParentTargetedController"`},
	{card: "Fickle Efreet", form: "Defined$ CorrectedSelf (a returned source)", refused: `Defined$ "CorrectedSelf"`},
}

func TestDefinedCensus1125(t *testing.T) { checkForms(t, definedCensus1125Forms) }
