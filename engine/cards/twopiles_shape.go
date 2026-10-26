package cards

// TwoPilesShapeSupported reports whether a TwoPiles ability is a shape the
// resolver (effects.effTwoPiles) can carry out. Coverage and resolution share
// it so a loud "unimplemented TwoPiles shape" Note path can never be counted
// as supported (Face.Primitives marks the rest api:TwoPiles.OtherShape).
//
// Supported: a ChosenPile$ (and UnchosenPile$, when present) naming an SVar of
// the face; no DefinedPiles$/LeftRightPile$; FaceDown$ absent or One; and a
// card set that is either DefinedCards$ Remembered/Targeted or the battlefield
// shape (TwoPilesBattlefieldShape). Parameter values are already trimmed by
// the script parser.
func TwoPilesShapeSupported(sa *SA, f *Face) bool {
	if sa == nil || sa.API != "TwoPiles" {
		return true
	}
	if sa.ParamStr(PKDefinedPiles) != "" || sa.ParamStr(PKLeftRightPile) != "" {
		return false
	}
	switch sa.ParamStr(PKFaceDown) {
	case "", "One", "one", "ONE":
	default:
		return false
	}
	chosen := sa.ParamStr(PKChosenPile)
	if chosen == "" || ResolveSVar(f.SVars, chosen) == nil {
		return false
	}
	if u := sa.ParamStr(PKUnchosenPile); u != "" && ResolveSVar(f.SVars, u) == nil {
		return false
	}
	switch sa.ParamStr(PKDefinedCards) {
	case "Remembered", "Targeted":
		return true
	case "":
		return TwoPilesBattlefieldShape(sa)
	}
	return false
}

// TwoPilesBattlefieldShape is the "separate all <permanents> target player
// controls into two piles" card set: no DefinedCards$, Zone$ Battlefield, a
// plain card-type ValidCards$ (no qualifier, so none is read relative to a
// Remembered referent), the pool player named by a ValidTgts$ target (no
// Defined$) or Defined$ You/TriggeredPlayer, and no RememberChosen$ (the
// chosen pile is applied through ChosenPile$ only).
func TwoPilesBattlefieldShape(sa *SA) bool {
	if sa == nil || sa.ParamStr(PKDefinedCards) != "" || sa.ParamStr(PKZone) != "Battlefield" ||
		sa.ParamStr(PKRememberChosen) != "" {
		return false
	}
	switch sa.ParamStr(PKValidCards) {
	case "Permanent", "Creature", "Land", "Artifact", "Enchantment", "Planeswalker":
	default:
		return false
	}
	switch sa.ParamStr(PKDefined) {
	case "":
		if sa.ParamStr(PKValidTgts) == "" {
			return false
		}
	case "You", "TriggeredPlayer":
	default:
		return false
	}
	for _, k := range []ParamKey{PKSeparator, PKChooser} {
		switch sa.ParamStr(k) {
		case "", "You", "Opponent", "TriggeredPlayer":
		default:
			return false
		}
	}
	return true
}
