package kgen

import "testing"

// globalLandTypeSetterForms: the global CR 305.7 land-type setter
// (RemoveLandTypes$ over every nonbasic land) lowers to the same
// SetLandSub + LoseAbilities override the attached form records.
var globalLandTypeSetterForms = []formCase{
	{card: "Blood Moon", form: "Land.nonBasic is a Mountain", lir: "override{addsub=Mountain setlandsub loseabilities}"},
	{card: "Magus of the Moon", form: "Land.nonBasic is a Mountain (creature carrier)", lir: "override{addsub=Mountain setlandsub loseabilities}"},
	{card: "Harbinger of the Seas", form: "Land.nonBasic is an Island", lir: "override{addsub=Island setlandsub loseabilities}"},
	{card: "Conversion", form: "Mountain (the subtype) are Plains", lir: "override{addsub=Plains setlandsub loseabilities}"},
	{card: "Glaciers", form: "Mountain (the subtype) are Plains", lir: "override{addsub=Plains setlandsub loseabilities}"},
	{card: "Zhao, the Moon Slayer", form: "Land.nonBasic is a Mountain while Zhao has a conqueror counter", lir: "override{addsub=Mountain setlandsub loseabilities}"},
}

func TestGlobalLandTypeSetterForms(t *testing.T) {
	checkForms(t, globalLandTypeSetterForms)
}
