package cards

// TwoPilesShapeSupported reports whether a TwoPiles ability is a shape the
// resolver (effects.effTwoPiles) can carry out. Coverage and resolution share
// it so a loud "unimplemented TwoPiles shape" Note path can never be counted
// as supported (Face.Primitives marks the rest api:TwoPiles.OtherShape).
//
// Supported: a ChosenPile$ (and UnchosenPile$, when present) naming an SVar of
// the face (ChosenPile$ may be absent when RememberChosen$ True carries the
// chosen pile on to a later sub-ability: Make an Example); no LeftRightPile$;
// FaceDown$ absent, One or True (display only: who sees a pile is not
// modelled); and either DefinedPiles$ naming two Remembered/Imprinted/Targeted
// piles (Hostile Negotiations), or a card set that is a DefinedCards$ in
// TwoPilesDefinedCardsSupported or the battlefield shape
// (TwoPilesBattlefieldShape). Parameter values are already trimmed by the
// script parser.
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
	case "", "One", "one", "ONE", "True", "true", "TRUE":
	default:
		return false
	}
	chosen := sa.ParamStr(PKChosenPile)
	if chosen == "" {
		// No pile body: only the RememberChosen$ ride-along shape (the chosen
		// pile lands in the Remembered set for the next sub-ability), and not
		// with DefinedPiles$, whose resolver arm always runs a body.
		if definedPiles || !TwoPilesRemembersChosen(sa) {
			return false
		}
	} else if ResolveSVar(f.SVars, chosen) == nil {
		return false
	}
	if u := sa.ParamStr(PKUnchosenPile); u != "" && ResolveSVar(f.SVars, u) == nil {
		return false
	}
	if definedPiles {
		return true
	}
	if spec := sa.ParamStr(PKDefinedCards); spec != "" {
		return TwoPilesDefinedCardsSupported(spec)
	}
	return TwoPilesBattlefieldShape(sa) || TwoPilesGraveyardShape(sa)
}

// TwoPilesRemembersChosen is RememberChosen$ True: the chosen pile is added to
// the resolution's Remembered set.
func TwoPilesRemembersChosen(sa *SA) bool {
	return sa != nil && sa.ParamStr(PKRememberChosen) == "True"
}

// TwoPilesSet is the kind of card set a DefinedCards$ value names.
type TwoPilesSet uint8

// The DefinedCards$ card sets the resolver builds.
const (
	TwoPilesSetNone       TwoPilesSet = iota // not a set the resolver builds
	TwoPilesSetRemembered                    // the Remembered objects
	TwoPilesSetTargeted                      // the Targeted objects
	TwoPilesSetImprinted                     // the host's Imprinted objects
	TwoPilesSetTop                           // the top N of the controller's library
)

// TwoPilesDefinedCards classifies a DefinedCards$ value; n is the card count
// of a TwoPilesSetTop (Sauron's Ransom's Top_4_OfLibrary). The one home of the
// list: the resolver reads it through here.
func TwoPilesDefinedCards(spec string) (kind TwoPilesSet, n int) {
	switch spec {
	case "Remembered":
		return TwoPilesSetRemembered, 0
	case "Targeted":
		return TwoPilesSetTargeted, 0
	case "Imprinted":
		return TwoPilesSetImprinted, 0
	}
	if n, ok := TwoPilesTopOfLibrary(spec); ok {
		return TwoPilesSetTop, n
	}
	return TwoPilesSetNone, 0
}

// TwoPilesDefinedCardsSupported reports whether spec is a card set the
// resolver builds.
func TwoPilesDefinedCardsSupported(spec string) bool {
	kind, _ := TwoPilesDefinedCards(spec)
	return kind != TwoPilesSetNone
}

// TwoPilesTopOfLibrary parses a "Top_<N>_OfLibrary" DefinedCards$ value
// (N a positive decimal number).
func TwoPilesTopOfLibrary(spec string) (n int, ok bool) {
	const pre, suf = "Top_", "_OfLibrary"
	if len(spec) <= len(pre)+len(suf) || spec[:len(pre)] != pre || spec[len(spec)-len(suf):] != suf {
		return 0, false
	}
	digits := spec[len(pre) : len(spec)-len(suf)]
	for i := 0; i < len(digits); i++ {
		if digits[i] < '0' || digits[i] > '9' || n > 1000 {
			return 0, false
		}
		n = n*10 + int(digits[i]-'0')
	}
	return n, n > 0
}

// TwoPilesDefinedPiles is DefinedPiles$ split on the FIRST comma (Forge's
// split(",", 2)): the two pile selectors, trimmed. ok is false when the value
// is absent or does not name two non-empty selectors. The one home of the
// split: the resolver reads it through here.
func TwoPilesDefinedPiles(sa *SA) (a, b string, ok bool) {
	v := sa.ParamStr(PKDefinedPiles)
	cut := -1
	for i := 0; i < len(v); i++ {
		if v[i] == ',' {
			cut = i
			break
		}
	}
	if cut < 0 {
		return "", "", false
	}
	a, b = trimSpaces(v[:cut]), trimSpaces(v[cut+1:])
	return a, b, a != "" && b != ""
}

// trimSpaces trims ASCII blanks without the strings package (the cards
// package's strings.* call count is ratcheted down, not up).
func trimSpaces(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}

// twoPilesPileSelector is a DefinedPiles$ selector the resolver evaluates:
// the three the corpus carriers use (Remembered, Imprinted) plus Targeted.
func twoPilesPileSelector(s string) bool {
	return s == "Remembered" || s == "Imprinted" || s == "Targeted"
}

// TwoPilesBattlefieldValid is the battlefield shape's ValidCards$ with the
// redundant RememberedPlayerCtrl qualifier dropped when the pool player IS the
// Remembered player (Defined$ Remembered): the pool is already that player's
// permanents. Forge spells the qualifier with a dot on some carriers
// (Do or Die) and a plus on others (Bend or Break's
// Land.!token+RememberedPlayerCtrl); both are the same "this player" scope.
// The one home of that normalisation, shared with the resolver.
func TwoPilesBattlefieldValid(sa *SA) string {
	v := sa.ParamStr(PKValidCards)
	if sa.ParamStr(PKDefined) != "Remembered" {
		return v
	}
	const dot, plus = ".RememberedPlayerCtrl", "+RememberedPlayerCtrl"
	for _, q := range [...]string{dot, plus} {
		if len(v) > len(q) && v[len(v)-len(q):] == q {
			return v[:len(v)-len(q)]
		}
	}
	return v
}

// twoPilesPlainType reports whether spec is one plain card type, optionally
// followed by the per-object !token predicate (Bend or Break's Land.!token).
// The resolver's matcher evaluates the predicate itself, so admitting it here
// only widens the coverage classifier to a term the chooser cannot misread.
func twoPilesPlainType(spec string) bool {
	const notToken = ".!token"
	if len(spec) > len(notToken) && spec[len(spec)-len(notToken):] == notToken {
		spec = spec[:len(spec)-len(notToken)]
	}
	switch spec {
	case "Permanent", "Creature", "Land", "Artifact", "Enchantment", "Planeswalker":
		return true
	}
	return false
}

// twoPilesPilePlayers reports whether the Separator$/Chooser$ selectors are
// players the resolver evaluates. ChosenPlayer is the chosen-seat spelling
// Bend or Break uses; the rest are the pre-existing shapes.
func twoPilesPilePlayers(sa *SA) bool {
	for _, k := range []ParamKey{PKSeparator, PKChooser} {
		switch sa.ParamStr(k) {
		case "", "You", "Opponent", "TriggeredPlayer", "Remembered", "ChosenPlayer":
		default:
			return false
		}
	}
	return true
}

// TwoPilesBattlefieldShape is the "separate all <permanents> target player
// controls into two piles" card set: no DefinedCards$, Zone$ Battlefield, a
// plain card-type ValidCards$ (the one qualifier is RememberedPlayerCtrl with
// Defined$ Remembered: the pool is already that player's permanents), and the
// pool player named by a ValidTgts$ target (no Defined$) or Defined$
// You/TriggeredPlayer/Remembered.
func TwoPilesBattlefieldShape(sa *SA) bool {
	if sa == nil || sa.ParamStr(PKDefinedCards) != "" || sa.ParamStr(PKZone) != "Battlefield" {
		return false
	}
	defined := sa.ParamStr(PKDefined)
	if !twoPilesPlainType(TwoPilesBattlefieldValid(sa)) {
		return false
	}
	switch defined {
	case "":
		if sa.ParamStr(PKValidTgts) == "" {
			return false
		}
	case "You", "TriggeredPlayer", "Remembered":
	default:
		return false
	}
	return twoPilesPilePlayers(sa)
}

// TwoPilesGraveyardShape is the non-battlefield pool shape (Death or Glory):
// Zone$ Graveyard, no DefinedCards$ or ValidTgts$, the pool is Defined$ You,
// a plain card-type ValidCards$, and Separator$/Chooser$ selectors the
// resolver evaluates. Only the graveyard pool is admitted -- the resolver
// builds it from the pool player's zone -- so every other Zone$ keeps the
// api:TwoPiles.OtherShape marker.
func TwoPilesGraveyardShape(sa *SA) bool {
	if sa == nil || sa.ParamStr(PKDefinedCards) != "" || sa.ParamStr(PKZone) != "Graveyard" {
		return false
	}
	if sa.ParamStr(PKValidTgts) != "" || sa.ParamStr(PKDefined) != "You" {
		return false
	}
	if !twoPilesPlainType(TwoPilesBattlefieldValid(sa)) {
		return false
	}
	return twoPilesPilePlayers(sa)
}
