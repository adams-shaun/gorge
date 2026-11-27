package kgen

import "testing"

// facedownfamilyForms: the face-down family this change lowers. Yedora, Grave
// Gardener refuses its FaceDownSetType$ return (a face-down Forest's
// intrinsic mana ability is unmodelled; a follow-up ticket tracks it); Ugin,
// the Ineffable and Boltbender still refuse on their own shapes (the Effect's
// imprint/look lifetime and the any-number stack retarget) and are separate
// follow-ups, so they are not pinned here.
var facedownfamilyForms = []formCase{
	{card: "Ashcloud Phoenix", form: "ChangeZone FaceDown$ True to the battlefield", lir: "Return obj=TriggeredNew"},
	{card: "Deathmist Raptor", form: "Mode$ TurnFaceUp TriggerZones$ Graveyard", lir: "trigger TurnFaceUp"},
	{card: "Printlifter Ooze", form: "Token WithCountersType$ P1P1 WithCountersAmount$ X", lir: "TokenCounters A=Count"},
	{card: "Obscuring Aether", form: "SetState Mode$ TurnFaceDown + ReduceCost Spell.isCastFaceDown", lir: "TurnFaceDown obj=Self"},
	{card: "Necropotence", form: "ChangeZone Defined$ TopOfLibrary to Exile with ExileFaceDown$", lir: "ExileFaceDown"},
}

func TestDemoFacedownShapes(t *testing.T) { checkForms(t, facedownfamilyForms) }
