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
