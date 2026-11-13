package kgen

import (
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
// You separating for an opponent's pick is the zero shape (Curator of
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
	case sep == "Opponent" && chooser == "You":
		flags |= kop.TpOppSeparates
	default:
		return refuse("TwoPiles Separator$ %q, chooser %q", sep, chooser)
	}
	// Both accepted shapes name an opponent as the separator or the chooser,
	// and CR 801.5b makes that opponent the spell controller's CHOICE at
	// three or more seats (gorge twoPilesAskOpponent). dirac reads it as the
	// next live seat, so this lowering holds only for two players -- the
	// two-seat marker that 3+-seat games refuse the carrier on.
	lc.twoSeat("TwoPiles: an opponent separates or chooses, read as the next live seat")
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
