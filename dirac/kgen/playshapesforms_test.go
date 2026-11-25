package kgen

import "testing"

// playshapesformsForms: the Play refusals of the coverage-gap class
// (`Play without WithoutManaCost`, `Play without ValidSA`, `Play without
// Optional`), checked as staticConditionForms are. The paid plays (no
// WithoutManaCost$ True) lower onto CastFree's Paid bit. A mandatory Play
// passes playMandatoryOK; Jeleva pins it where the corpus still refuses it,
// and dirac/cast_mandatorydone_test.go pins the offer's absent Done. The
// absent ValidSA$ over a LAND-CAPABLE population (Vault 112: a library dig
// can remember a land, which gorge offers and plays as a land, CR 305)
// lowers onto CastFree's Lands bit (dirac/cast_freeland.go).
var playshapesformsForms = []formCase{
	{card: "Chandra, Acolyte of Flame", form: "targeted paid cast from graveyard (ValidSA$ Spell)", lir: "CastFree"},
	{card: "Horde of Notions", form: "targeted free cast from graveyard (no ValidSA$, a landless population)", lir: "CastFree"},
	{card: "Vaan, Street Thief", form: "remembered paid cast from exile (no WithoutManaCost$)", lir: "CastFree"},
	{card: "Jeleva, Nephalia's Scourge", form: "mandatory Play (no Optional$) past the Optional gate", refused: "Play ValidZone$ Exile without IsRemembered"},
	{card: "Vault 112: Sadistic Simulation", form: "remembered free play from exile (no ValidSA$, a land-capable population)", lir: "CastFree"},
}

func TestPlayShapesForms(t *testing.T) { checkForms(t, playshapesformsForms) }
