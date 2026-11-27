package kgen

import "testing"

// facedownfamilyForms: the face-down family this change lowers. Yedora, Grave
// Gardener folds its FaceDownSetType$ return into an override applied while
// the returned permanent is face down. Ugin, the Ineffable and Boltbender
// still refuse on their own shapes (the Effect's imprint/look lifetime and
// the any-number stack retarget) and are separate follow-ups, so they are not
// pinned here.
var facedownfamilyForms = []formCase{
	{card: "Yedora, Grave Gardener", form: "ChangeZone FaceDown$ True with FaceDownSetType$ Land & Forest", lir: "the face-down type set applies to the returned object while it is face down"},
	{card: "Ashcloud Phoenix", form: "ChangeZone FaceDown$ True to the battlefield", lir: "Return obj=TriggeredNew"},
	{card: "Deathmist Raptor", form: "Mode$ TurnFaceUp TriggerZones$ Graveyard", lir: "trigger TurnFaceUp"},
	{card: "Printlifter Ooze", form: "Token WithCountersType$ P1P1 WithCountersAmount$ X", lir: "TokenCounters A=Count"},
	{card: "Obscuring Aether", form: "SetState Mode$ TurnFaceDown + ReduceCost Spell.isCastFaceDown", lir: "TurnFaceDown obj=Self"},
	{card: "Necropotence", form: "ChangeZone Defined$ TopOfLibrary to Exile with ExileFaceDown$", lir: "ExileFaceDown"},
}

func TestDemoFacedownShapes(t *testing.T) { checkForms(t, facedownfamilyForms) }
