package kgen

import "strings"

// playSelfExileArm reports whether a Play takes the Card.Self exile-grant arm
// (and its refusal): a paid Play (no WithoutManaCost$ True) other than a
// filter population over exile cast as the card itself (not a copy), which
// lowers through the population switch instead. gorge's beginPlay begins the ordinary cast paying its
// printed cost (CR 601.2); the kernel's freeQ drain pays it through the manual
// mana window and reverses an unpayable cast (CR 733.1, turn_gen.go), the
// route Vaan, Street Thief's paid remembered Play already rides. Nathan Drake,
// Treasure Hunter and Sanwell, Avenger Ace (remembered exile) and Boiling Rock
// Rioter (exiled with the source). A paid copy cast or a paid hand, graveyard
// or library population keeps its refusal.
func playSelfExileArm(paid bool, valid, vzone string, copyCard bool) bool {
	return paid && (vzone != "Exile" || valid == "Card.Self" || copyCard)
}

// playExileFilterPop lowers an exile filter population keyed on
// ExiledWithSource (Boiling Rock Rioter's Ally.YouOwn+ExiledWithSource): the
// cards in every alive seat's exile, seat order then zone order, that match
// Valid$ (gorge effPlay's public-zone walk, engine/effects/play.go), the
// ownership clause carried in the filter itself. Read for the paid play only
// (paidPlain: paid, no ForgetPlayed$ or ReplaceGraveyard$),
// the shape the lockstep proves: a free one (Jeleva, Nephalia's Scourge) and
// a Valid$ with neither IsRemembered nor ExiledWithSource (Goliath
// Daydreamer's counters_GE1_DREAM) keep play's refusal.
func (lc *lowerCtx) playExileFilterPop(valid, saExpr string, paidPlain bool, emit func(lRef, int, string) error) error {
	if !paidPlain || !strings.Contains(valid, "ExiledWithSource") {
		return refuse("Play ValidZone$ Exile without IsRemembered")
	}
	expr, err := andFilterExpr(valid, saExpr)
	if err != nil {
		return err
	}
	return emit(lRef{Kind: rNone}, zExile, expr)
}

// playTypeDotType reports whether a Valid$ part is a dotted pair of plain type
// words (Artifact.Creature): both capitalised letters only. The filter reader
// validates the words themselves and refuses an unknown one by name.
func playTypeDotType(part string) bool {
	a, b, ok := strings.Cut(part, ".")
	return ok && playTypeWord(a) && playTypeWord(b)
}

func playTypeWord(w string) bool {
	if w == "" || w[0] < 'A' || w[0] > 'Z' {
		return false
	}
	for _, r := range w {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z') {
			return false
		}
	}
	return true
}
