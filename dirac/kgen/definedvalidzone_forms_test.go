package kgen

import "testing"

// definedValidZoneForms: a ChangeZone whose Defined$ is the zone set
// "ValidGraveyard <filter>" out of the graveyard lowers as the ChangeZoneAll
// sweep of <filter> (lower_zone.go definedValidGraveyard), and a
// CopyPermanent whose Defined$ is "ValidExile <filter>" lowers as the
// controller's-exile copy sweep (lower_zone.go copyPermanentValidExile, gorge
// definedZoneValidTargets' ValidExile arm); the other Valid* selectors and a
// sweep param the ChangeZoneAll lowering does not read keep refusing.
var definedValidZoneForms = []formCase{
	{card: "Faith's Reward", form: "SP$ ChangeZone Defined$ ValidGraveyard Permanent.YouOwn+ThisTurnEnteredFrom_Battlefield", lir: "ChangeZoneAll C=1 CS=from=Graveyard F={types=0x9f owner=1"},
	{card: "No Rest for the Wicked", form: "AB$ ChangeZone to hand, Defined$ ValidGraveyard Creature.YouOwn+...", lir: "ChangeZoneAll C=2 CS=from=Graveyard F={types=0x2 owner=1"},
	{card: "Pyrrhic Revival", form: "Defined$ ValidGraveyard Creature with WithCountersType$", lir: "ChangeZoneAll A=1 C=1 CS=from=Graveyard|counters=M1M1 F={types=0x2}"},
	{card: "Second Sunrise", form: "Defined$ ValidGraveyard over four type alternatives", lir: "ChangeZoneAll C=1 CS=from=Graveyard F={types=0x4"},
	{card: "Retether", form: "ValidGraveyard sweep with AttachedTo$", refused: "unread params [AttachedTo]"},
	{card: "Altaïr Ibn-La'Ahad", form: "CopyPermanent Defined$ ValidExile over the controller's exile", lir: "CopyAllExile CS=Tapped|Attacking|ExileAtCombat"},
	{card: "Court of Locthwain", form: "Effect Defined$ ValidExile outside CopyPermanent", refused: `Defined$ "ValidExile`},
}

func TestDefinedValidZoneForms(t *testing.T) { checkForms(t, definedValidZoneForms) }
