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

// playExileFilterPop lowers an exile filter population whose Valid$ has no
// IsRemembered (Boiling Rock Rioter's Ally.YouOwn+ExiledWithSource, Krang &
// Shredder's Card.nonLand+ExiledWithSource, Goliath Daydreamer's
// Card.YouOwn+counters_GE1_DREAM, Kylox's Card.IsImprinted+YouOwn): the cards
// in every alive seat's exile, seat order then zone order, that match Valid$
// (gorge effPlay's public-zone walk, engine/effects/play.go), the ownership
// clause carried in the filter itself. The filter is read at the resolution
// by the kernel's castFreeFillGen (dirac/cast_freecast.go), for a paid play
// and a free one alike. ForgetPlayed$ and ReplaceGraveyard$ over such a
// population stay refused by name: no carrier reaches them in a lockstep.
// A Valid$ the filter compiler refuses keeps that refusal.
func (lc *lowerCtx) playExileFilterPop(valid, saExpr string, plain bool, emit func(lRef, int, string) error) error {
	if !plain {
		return refuse("Play ValidZone$ Exile without IsRemembered with ForgetPlayed$ or ReplaceGraveyard$")
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
