package cards

// MultiplePilesShape is the DB$ MultiplePiles shape the resolver
// (effects.effMultiplePiles) carries out: every player in Defined$ Player
// (APNAP order from the controller) separates the ValidCards$ permanents they
// control on the battlefield into Piles$ piles, then a pile is picked for each
// at random (RandomChosen$ True), and the ChosenPile$ SVar runs once over the
// union of those piles (Whims of the Fates). Anything else is outside it.
type MultiplePilesShape struct {
	Piles int    // Piles$, 2..MaxMultiplePiles
	Valid string // the plain card-type ValidCards$
	You   bool   // Defined$ You: only the controller separates
}

// MaxMultiplePiles bounds Piles$: the split asks scale with it.
const MaxMultiplePiles = 8

// ParseMultiplePiles reads a MultiplePiles ability's shape. ok is false for
// every parameter combination the resolver does not carry out. The one home
// of the read: the coverage predicate and the resolver both go through it.
func ParseMultiplePiles(sa *SA) (shape MultiplePilesShape, ok bool) {
	if sa == nil || sa.API != "MultiplePiles" || sa.Params["Zone"] != "Battlefield" ||
		sa.Params["RandomChosen"] != "True" || sa.Params["Separator"] != "" || sa.Params["Chooser"] != "" {
		return shape, false
	}
	switch sa.Params["Defined"] {
	case "Player":
	case "You":
		shape.You = true
	default:
		return shape, false
	}
	switch sa.Params["ValidCards"] {
	case "Permanent", "Creature", "Land", "Artifact", "Enchantment", "Planeswalker":
		shape.Valid = sa.Params["ValidCards"]
	default:
		return shape, false
	}
	v := sa.Params["Piles"]
	if len(v) != 1 || v[0] < '2' || v[0]-'0' > MaxMultiplePiles {
		return shape, false
	}
	shape.Piles = int(v[0] - '0')
	return shape, true
}

// MultiplePilesShapeSupported reports whether a MultiplePiles ability is the
// shape the resolver carries out, its ChosenPile$ naming an SVar of the face
// (Face.Primitives marks the rest api:MultiplePiles.OtherShape).
func MultiplePilesShapeSupported(sa *SA, f *Face) bool {
	if sa == nil || sa.API != "MultiplePiles" {
		return true
	}
	if _, ok := ParseMultiplePiles(sa); !ok {
		return false
	}
	chosen := sa.ParamStr(PKChosenPile)
	return chosen != "" && ResolveSVar(f.SVars, chosen) != nil
}
