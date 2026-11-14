package kgen

import "strings"

// The mid-resolution ChoosePlayer (Gluntch, the Bestower: `DB$ ChoosePlayer |
// Defined$ You | Choices$ Player | RememberChosen$ True`, then a Draw, a Token
// and a PutCounter pick reading ChosenPlayer; gorge effChoosePlayer). The
// resolving controller chooses one living player (any seat count: the kernel's
// pool is the living players from the controller, kop.ExtChoosePlayerRes), the
// choice becomes the resolution's chosen player and joins its remembered
// players, which Choices$ Player.!IsRemembered leaves out of a later pool.
// ChosenPlayer reads kop.RefChosenPlayerRes in that ability afterwards.

// choosePlayerRes lowers the Choices$ Player / Player.!IsRemembered ChoosePlayer
// beside RememberChosen$ True; every other spelling refuses by name.
func (lc *lowerCtx) choosePlayerRes(ps *params) error {
	if lc.chosenOppAb != nil && lc.chosenOppAb == lc.ab {
		return refuse("ChoosePlayer Choices$ %q beside the one-opponent ChoosePlayer", ps.str("Choices"))
	}
	if v := ps.str("Defined"); v != "You" {
		return refuse("ChoosePlayer Choices$ %q with Defined$ %q", ps.str("Choices"), v)
	}
	// gorge appends the answer to the resolution's remembered list only under
	// RememberChosen$, which is what a Player.!IsRemembered pool reads
	if !ps.is("RememberChosen", "True") {
		return refuse("ChoosePlayer Choices$ %q without RememberChosen$ True", ps.str("Choices"))
	}
	if _, ok := ps.get("ValidTgts"); ok {
		return refuse("ChoosePlayer Choices$ %q with ValidTgts$", ps.str("Choices"))
	}
	fresh := 0
	if ps.str("Choices") == "Player.!IsRemembered" {
		fresh = 1 // kop.ChoosePlayerFresh
	}
	// prompt and AI only
	ps.consume("Defined", "Choices", "RememberChosen", "ChoiceTitle", "AILogic")
	lc.chosenResAb = lc.ab
	lc.emit(lInstr{Op: "ChoosePlayerRes", C: fresh, Note: "the controller chooses a living player"})
	return nil
}

// chosenPlayerResDefined maps ChosenPlayer / Player.Chosen to the resolution's
// chosen player once this ability's mid-resolution ChoosePlayer is lowered.
func (lc *lowerCtx) chosenPlayerResDefined(v string) (lRef, bool) {
	if lc.chosenResAb == nil || lc.chosenResAb != lc.ab {
		return lRef{}, false
	}
	if v == "ChosenPlayer" || v == "Player.Chosen" {
		return lRef{Kind: rChosenPlayerRes}, true
	}
	return lRef{}, false
}

// chosenPlayerCtrlFilter rewrites a Choices$ spec's "ControlledBy ChosenPlayer"
// to YouCtrl when the chooser is that same chosen player (PutCounter's
// `Choices$ Creature.ControlledBy ChosenPlayer | Chooser$ ChosenPlayer`): the
// kernel reads a pick's YouCtrl from the chooser's seat, so the two spellings
// name one set of permanents. Any other chooser leaves the word to refuse.
func chosenPlayerCtrlFilter(spec string) string {
	return strings.ReplaceAll(spec, "ControlledBy ChosenPlayer", "YouCtrl")
}
