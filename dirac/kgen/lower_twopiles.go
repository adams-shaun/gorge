package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// twoPilesRemembered lowers the split of the remembered cards (gorge
// effTwoPiles' DefinedCards$ Remembered arm, CR 700.3): one player separates
// the cards an upstream PeekAndReveal remembered into two piles, the other
// picks one; the chosen pile goes to the hand, the other to the graveyard.
// OpTwoPiles, its B.N the kop.Tp* flags.
//
// The players are read as gorge's twoPilesPlayer reads them: the chooser is
// Chooser$, else Defined$; the separator is Separator$, else the chooser.
// You separating for an opponent's pick is TpOppChooses (Curator of
// Destinies, Steam Augury); an opponent separating for your pick is
// TpOppSeparates (Fact or Fiction, Sphinx of Uthuun). Zone$ and FaceDown$ One
// are not read by gorge once DefinedCards$ names the set.
//
// ChosenPile$ moves its pile from the library to the hand with ChangeZone
// Defined$ Remembered (the pile in its own order: pile A as picked, pile B as
// revealed). UnchosenPile$ is that move to the graveyard (TpUnchosenPileOrder)
// or Curator's ChangeZoneAll ChangeType$ Card.IsRemembered sweep (the
// library's order). Jace, Architect of Thought's pile on the bottom of the
// library refuses: gorge's sweep puts it there in the library's order, while
// CR 401.4 lets the owner arrange it.
func (lc *lowerCtx) twoPilesRemembered(ps *params) error {
	for _, k := range []string{"LeftRightPile", "DefinedPiles", "RememberChosen", "ValidCards", "KeepRemembered"} {
		if _, ok := ps.get(k); ok {
			return refuse("TwoPiles %s$", k)
		}
	}
	if dc := ps.str("DefinedCards"); dc != "Remembered" {
		return refuse("TwoPiles DefinedCards$ %q", dc)
	}
	if _, ok := ps.get("Zone"); ok && ps.str("Zone") != "Library" {
		return refuse("TwoPiles Zone$ %q", ps.str("Zone"))
	}
	if _, ok := ps.get("FaceDown"); ok && ps.str("FaceDown") != "One" {
		return refuse("TwoPiles FaceDown$ %q", ps.str("FaceDown"))
	}
	// Defined$ is read only for an absent Chooser$ (gorge's chooseSpec).
	chooser, defined := ps.str("Chooser"), ps.str("Defined")
	if chooser == "" {
		chooser = defined
	}
	sep := ps.str("Separator")
	if sep == "" {
		sep = chooser
	}
	flags := 0
	switch {
	case sep == "You" && chooser == "Opponent":
		flags |= kop.TpOppChooses
	case sep == "Opponent" && chooser == "You":
		flags |= kop.TpOppSeparates
	default:
		return refuse("TwoPiles Separator$ %q, chooser %q", sep, chooser)
	}
	// Both accepted shapes name an opponent as the separator or the chooser,
	// and CR 801.5b makes that opponent the spell controller's CHOICE at
	// three or more seats (gorge twoPilesAskOpponent). The kernel asks it for
	// either shape (act_twopiles.go pilesOppAsk) and keeps the answer live
	// across the split, so the shape is seat-exact and needs no two-seat
	// marker.
	chosen := cards.ResolveSVar(lc.face.SVars, ps.str("ChosenPile"))
	if !pileBody(chosen, "ChangeZone", "Defined", "Remembered", "Destination", "Hand") {
		return refuse("TwoPiles pile actions")
	}
	unchosen := cards.ResolveSVar(lc.face.SVars, ps.str("UnchosenPile"))
	switch {
	case pileBody(unchosen, "ChangeZone", "Defined", "Remembered", "Destination", "Graveyard"):
		flags |= kop.TpUnchosenPileOrder
	case pileBody(unchosen, "ChangeZoneAll", "ChangeType", "Card.IsRemembered", "Destination", "Graveyard"):
	case pileBody(unchosen, "ChangeZoneAll", "ChangeType", "Card.IsRemembered", "Destination", "Library", "LibraryPosition", "-1"):
		return refuse("TwoPiles UnchosenPile$ on the library's bottom: gorge's sweep keeps the library's order, CR 401.4 lets the owner arrange it")
	default:
		return refuse("TwoPiles pile actions")
	}
	in := lInstr{Op: "TwoPiles", Obj: lRef{Kind: rYou}, C: zHand,
		CS: "unchosen=Graveyard|separator=" + sep + "|chooser=" + chooser}
	if flags != 0 {
		in.B = cst(flags)
	}
	lc.emit(in)
	return nil
}

// twoPilesEach lowers the battlefield TwoPiles inside a RepeatEach player body
// whose separator is the repeated player and whose chooser is the opponent
// that body's ChoosePlayerEach ask chose (Bend or Break: Defined$ Remembered,
// Separator$ Remembered, Chooser$ ChosenPlayer; gorge effTwoPiles' battlefield
// arm over the remembered player). They separate their nontoken lands
// (ValidCards$ Land.!token+RememberedPlayerCtrl, kop CtrlEachPl) into two
// piles, the chosen opponent picks one (ChGenPileChoose), and the chosen pile
// is destroyed while the other is tapped: ChosenPile$ DB$ DestroyAll with
// ValidCards$ exactly Land.IsRemembered, UnchosenPile$ DB$ TapAll with the
// same (gorge twoPilesRunPile's fresh Remembered = pile). Anything else
// refuses by name. OpExt ExtTwoPilesEach.
func (lc *lowerCtx) twoPilesEach(ps *params) error {
	if lc.repeatPl == 0 {
		return refuse("TwoPiles battlefield Defined$ Remembered outside a RepeatEach player body")
	}
	for _, k := range []string{"LeftRightPile", "DefinedPiles", "RememberChosen", "DefinedCards", "FaceDown", "KeepRemembered"} {
		if _, ok := ps.get(k); ok {
			return refuse("TwoPiles %s$", k)
		}
	}
	if v := ps.str("Zone"); v != "Battlefield" {
		return refuse("TwoPiles Zone$ %q", v)
	}
	if v := ps.str("Defined"); v != "Remembered" {
		return refuse("TwoPiles Defined$ %q", v)
	}
	if v := ps.str("Separator"); v != "Remembered" {
		return refuse("TwoPiles Separator$ %q", v)
	}
	if v := ps.str("Chooser"); v != "ChosenPlayer" {
		return refuse("TwoPiles Chooser$ %q", v)
	}
	vc := ps.str("ValidCards")
	if vc != "Land.!token+RememberedPlayerCtrl" {
		return refuse("TwoPiles battlefield ValidCards$ %q", vc)
	}
	if !pileAction(cards.ResolveSVar(lc.face.SVars, ps.str("ChosenPile")), "DestroyAll") {
		return refuse("TwoPiles ChosenPile$ %q", ps.str("ChosenPile"))
	}
	if !pileAction(cards.ResolveSVar(lc.face.SVars, ps.str("UnchosenPile")), "TapAll") {
		return refuse("TwoPiles UnchosenPile$ %q", ps.str("UnchosenPile"))
	}
	// the chain's rider, prompt and AI metadata only
	ps.consume("Zone", "Defined", "Separator", "Chooser", "ValidCards", "ChosenPile", "UnchosenPile", "SubAbility", "ChoiceTitle", "AILogic")
	if left := ps.unread(); len(left) > 0 {
		return refuse("TwoPiles battlefield unread params %v", left)
	}
	f, err := lc.filter(vc)
	if err != nil {
		return err
	}
	lc.emit(lInstr{Op: "TwoPilesEach", F: f, Note: "the repeated player's nontoken lands split into two piles; the chosen opponent picks one; the chosen pile destroyed, the other tapped"})
	return nil
}

// pileAction reports whether a ChosenPile$/UnchosenPile$ body is exactly the
// api over the chosen pile's members: DB$ DestroyAll or TapAll with
// ValidCards$ Land.IsRemembered (gorge twoPilesRunPile's fresh Remembered =
// pile) and no other rules parameter, no sub-ability.
func pileAction(sa *cards.SA, api string) bool {
	if sa == nil || sa.API != api || sa.Sub != nil {
		return false
	}
	n := 0
	for k, v := range sa.Params {
		if ignoredParams[k] {
			continue
		}
		if k == "ValidCards" && strings.TrimSpace(v) == "Land.IsRemembered" {
			n++
			continue
		}
		return false
	}
	return n == 1
}

// pileBody reports whether a ChosenPile$/UnchosenPile$ body is exactly the
// api move from the library that kv names (key, value pairs beside Origin$
// Library): no other rules parameter, no sub-ability.
func pileBody(sa *cards.SA, api string, kv ...string) bool {
	if sa == nil || sa.API != api || sa.Sub != nil {
		return false
	}
	want := map[string]string{"Origin": "Library"}
	for i := 0; i+1 < len(kv); i += 2 {
		want[kv[i]] = kv[i+1]
	}
	n := 0
	for k, v := range sa.Params {
		if ignoredParams[k] {
			continue
		}
		if w, ok := want[k]; !ok || w != v {
			return false
		}
		n++
	}
	return n == len(want)
}
