package kgen

import "testing"

// affectedZoneNonBattlefieldForms: a Continuous static whose AffectedZone$ is
// off the battlefield. A type grant (Leyline of Transformation's All-zone
// chosen subtype, Ashes of the Fallen's graveyard one) rides
// kop.StatOffZoneGrant, never a StatContinuous (the battlefield readers match
// a filter without reading its zone), and Six's graveyard retrace rides
// kop.StatRetraceGrant. Aminatou's hand miracle grant rides kop.StatMiracleGrant.
var affectedZoneNonBattlefieldForms = []formCase{
	{card: "Leyline of Transformation", form: "AffectedZone$ All AddType$ ChosenType", lir: "addchosen"},
	{card: "Ashes of the Fallen", form: "AffectedZone$ Graveyard AddType$ ChosenType", lir: "addchosen"},
	{card: "Aminatou, Veil Piercer", form: "AffectedZone$ Hand AddKeyword$ Miracle:ManaCost:4", lir: "miraclecost="},
	{card: "Six", form: "AffectedZone$ Graveyard AddKeyword$ Retrace", lir: "kw=[Retrace]"},
}

func TestAffectedZoneForms(t *testing.T) { checkForms(t, affectedZoneNonBattlefieldForms) }
