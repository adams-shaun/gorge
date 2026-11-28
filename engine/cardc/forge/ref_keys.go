package forge

import "github.com/adams-shaun/gorge/engine/cardc/ir"

// RefKey is a parameter whose value is a reference, with the subject it
// wants. SubjNone means the effect decides: read it as an object, then, on a
// wrong-subject refusal, as a player.
type RefKey struct {
	Key     string
	Subject ir.Subject
}

// RefKeys are the parameters whose value is a reference, sorted. They are the
// corpus keys starting "Defined" except NonRefDefinedKeys.
var RefKeys = []RefKey{
	{"Defined", ir.SubjNone},
	{"DefinedAssembler", ir.SubjObject},
	{"DefinedAttacker", ir.SubjObject},
	{"DefinedBlocker", ir.SubjObject},
	{"DefinedCards", ir.SubjObject},
	{"DefinedContraption", ir.SubjObject},
	{"DefinedDamagers", ir.SubjObject},
	{"DefinedExiler", ir.SubjPlayer},
	{"DefinedLandwalk", ir.SubjObject},
	{"DefinedMagnet", ir.SubjObject},
	{"DefinedPiles", ir.SubjObject},
	{"DefinedPlayer", ir.SubjPlayer},
	{"DefinedTarget", ir.SubjNone},
}

// NonRefDefinedKeys are the corpus keys starting "Defined" whose value is a
// name, a description or a keyword, not a reference. Sorted.
var NonRefDefinedKeys = []string{"DefinedDesc", "DefinedKW", "DefinedName"}
