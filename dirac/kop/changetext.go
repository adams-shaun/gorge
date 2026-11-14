package kop

// ExtChangeText (CR 612 text-changing effect; gorge api:ChangeText, Mind
// Bend): the resolving controller picks a "from" word and a "to" word (one
// combined two-pick ask, either order: gorge's changetext KChoose, options
// changetext_from / changetext_to) and the change is applied to the
// battlefield permanent Obj. C = ChangeTextColour (the five colour words, as
// ChangeColorWord$ Choose Choose) or ChangeTextLand (the five basic land
// types, ChangeTypeWord$ ChooseBasicLandType ChooseBasicLandType).
//
// The kernel holds no per-permanent text: a change is modelled only where it
// is a no-op, i.e. the target card's face does not read the "from" word
// (CardDef.TextWords bit clear). A target that does read it stalls the game
// (dirac StallTextChange), never answering from the unchanged face.
// dirac/act_changetext.go.
const ExtChangeText ExtOp = 261

// ExtChangeText's C: which word list the two picks range over.
const (
	ChangeTextColour int16 = 1
	ChangeTextLand   int16 = 2
)

func init() { extOpNames[ExtChangeText] = "ChangeText" }
