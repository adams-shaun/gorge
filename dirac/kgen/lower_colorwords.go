package kgen

// colorWordsStatic lowers Swirl the Mists' Continuous static: "All instances
// of color words in the text of spells and permanents are changed to the
// chosen color word" (S:Mode$ Continuous | Affected$ Spell,Permanent |
// AffectedZone$ Stack,Battlefield | ChangeColorWordsTo$ ChosenColor, CR
// 612.5). gorge realizes it as layer-3 text substitutions on every spell and
// permanent (rules colorwords_sync.go), which rewrite each face's parameters
// (a target spec's nonBlack reads nonRed).
//
// The kernel holds no per-object text, so the static rides kop.StatColorWords
// and is modelled where it is a no-op: a spell about to be cast, a spell on
// the stack or a permanent whose card reads (CardDef.TextWords) a colour word
// other than the source's chosen one stalls the game (StallTextChange,
// unmodelled) instead of answering from the unchanged face (dirac
// static_colorwords.go), the rule api:ChangeText lowers under (textreach.go).
// The pool therefore carries CardDef.TextWords (HasChangeText).
//
// Only the exact shape lowers: the chosen colour word as the replacement, over
// Spell,Permanent in Stack,Battlefield, with no condition and no key beside
// the description. Any other ChangeColorWordsTo$ spelling refuses by name.
func (lc *lowerCtx) colorWordsStatic(sp *params, st *lStatic, aff, az string) error {
	if v := sp.str("ChangeColorWordsTo"); v != "ChosenColor" {
		return refuse("ChangeColorWordsTo %q", v)
	}
	if aff != "Spell,Permanent" || az != "Stack,Battlefield" {
		return refuse("ChangeColorWordsTo over Affected %q in AffectedZone %q", aff, az)
	}
	if st.Cond.Kind != cNone {
		return refuse("ChangeColorWordsTo static with a condition")
	}
	for _, k := range sortedParamKeys(sp.m) {
		switch k {
		case "Mode", "Affected", "AffectedZone", "ChangeColorWordsTo", "Description", "Secondary":
		default:
			return refuse("ChangeColorWordsTo static beside %s$", k)
		}
	}
	sp.consume("Affected", "AffectedZone", "ChangeColorWordsTo")
	lc.out.HasChangeText = true // the pool's cards carry CardDef.TextWords (build.go)
	st.Kind = "ColorWords"
	return nil
}
