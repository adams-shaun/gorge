package kgen

import "testing"

// affectedZoneNonBattlefieldForms: a Continuous static whose AffectedZone$ is
// off the battlefield. A type grant (Leyline of Transformation's All-zone
// chosen subtype, Ashes of the Fallen's graveyard one) rides
// kop.StatOffZoneGrant, never a StatContinuous (the battlefield readers match
// a filter without reading its zone), and Six's graveyard retrace rides
// kop.StatRetraceGrant. Aminatou's hand miracle grant keeps the refusal:
// gorge offers the granted miracle trigger, which the kernel does not.
var affectedZoneNonBattlefieldForms = []formCase{
	{card: "Leyline of Transformation", form: "AffectedZone$ All AddType$ ChosenType", lir: "addchosen"},
	{card: "Ashes of the Fallen", form: "AffectedZone$ Graveyard AddType$ ChosenType", lir: "addchosen"},
	{card: "Aminatou, Veil Piercer", form: "AffectedZone$ Hand AddKeyword$ Miracle (the granted miracle offer is unmodelled)", refused: `AffectedZone "Hand" without MayPlay`},
	{card: "Six", form: "AffectedZone$ Graveyard AddKeyword$ Retrace", lir: "kw=[Retrace]"},
}

func TestAffectedZoneForms(t *testing.T) { checkForms(t, affectedZoneNonBattlefieldForms) }
