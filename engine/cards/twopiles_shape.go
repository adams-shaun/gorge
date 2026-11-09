package cards

import "strings"

// TwoPilesShapeSupported reports whether a TwoPiles ability is a shape the
// resolver (effects.effTwoPiles) can carry out. Coverage and resolution share
// it so a loud "unimplemented TwoPiles shape" Note path can never be counted
// as supported (Face.Primitives marks the rest api:TwoPiles.OtherShape).
//
// Supported: a ChosenPile$ (and UnchosenPile$, when present) naming an SVar of
// the face; no LeftRightPile$; FaceDown$ absent or One; and either DefinedPiles$
// naming two Remembered/Imprinted/Targeted piles (Hostile Negotiations), or a
// card set that is DefinedCards$ Remembered/Targeted or the battlefield
// shape (TwoPilesBattlefieldShape). Parameter values are already trimmed by
// the script parser.
func TwoPilesShapeSupported(sa *SA, f *Face) bool {
	if sa == nil || sa.API != "TwoPiles" {
		return true
	}
	if sa.ParamStr(PKLeftRightPile) != "" {
		return false
	}
	definedPiles := sa.ParamStr(PKDefinedPiles) != ""
	if definedPiles {
		// The Zone$/ValidCards$/DefinedCards$ card set is not consulted:
		// Forge ignores them once DefinedPiles$ names the two piles.
		a, b, ok := TwoPilesDefinedPiles(sa)
		if !ok || !twoPilesPileSelector(a) || !twoPilesPileSelector(b) {
			return false
		}
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
	if definedPiles {
		return true
	}
	switch sa.ParamStr(PKDefinedCards) {
	case "Remembered", "Targeted":
		return true
	case "":
		return TwoPilesBattlefieldShape(sa)
	}
	return false
}

// TwoPilesDefinedPiles is DefinedPiles$ split on the FIRST comma (Forge's
// split(",", 2)): the two pile selectors, trimmed. ok is false when the value
// is absent or does not name two non-empty selectors. The one home of the
// split: the resolver reads it through here.
func TwoPilesDefinedPiles(sa *SA) (a, b string, ok bool) {
	v := sa.ParamStr(PKDefinedPiles)
	a, b, found := strings.Cut(v, ",")
	a, b = strings.TrimSpace(a), strings.TrimSpace(b)
	return a, b, found && a != "" && b != ""
}

// twoPilesPileSelector is a DefinedPiles$ selector the resolver evaluates:
// the three the corpus carriers use (Remembered, Imprinted) plus Targeted.
func twoPilesPileSelector(s string) bool {
	return s == "Remembered" || s == "Imprinted" || s == "Targeted"
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
