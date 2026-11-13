package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// The as-enters chosen opponent (CR 614.12; The Rack, Black Vise, Xantcha):
// K:ETBReplacement:Other:<SVar> with a lone DB$ ChoosePlayer | Defined$ You
// | Choices$ Player.Opponent. gorge effChoosePlayer asks a KChoose over the
// living opponents and records the answer on the entering object
// (Object.Chosen, cleared when it leaves the battlefield). In two players
// (twoSeat-at-caller: etbChosenOpp) the ask has one option, answered in place, so the enters program is
// kop.ExtChosenOpp: the kernel stores the one opponent in the permanent's
// chosen byte. The card's other abilities read it live: a Phase trigger's
// ValidPlayer$ Player.Chosen (kop.WhoseChosenPl), Defined$ ChosenPlayer /
// Player.Chosen (kop.RefSrcChosenPl) and Count$ValidHand Card.ChosenCtrl
// (the chosen player's hand cards).

// whoChosenPl is the trigger Whose of Player.Chosen. build_ab.go casts the
// value to kop.Whose, so it must equal kop.WhoseChosenPl
// (TestWhoChosenPlMatchesKop).
const whoChosenPl uint8 = 201

// entersChoosingOpp reports whether face f chooses an opponent as it enters
// (the lone ChoosePlayer above) and nothing else on it writes or clears a
// chosen player, so the entry's answer is the one every reader sees.
func entersChoosingOpp(f *cards.Face) bool {
	if f == nil {
		return false
	}
	n := 0
	for i := range f.Repls {
		r := &f.Repls[i]
		if r.ParamStr(cards.PKKeyword) != "ETBReplacement" || r.With == nil || r.With.API != "ChoosePlayer" {
			continue
		}
		if !etbChooseOppShape(r.With) {
			return false
		}
		n++
	}
	if n != 1 {
		return false
	}
	writers := 0
	for _, body := range f.SVars {
		if strings.Contains(body, "ChoosePlayer") {
			writers++
		}
		if strings.Contains(body, "ClearChosenPlayer") {
			return false
		}
	}
	return writers == 1
}

// etbChooseOppShape: a lone ChoosePlayer by You over the opponents.
func etbChooseOppShape(sa *cards.SA) bool {
	if sa == nil || sa.API != "ChoosePlayer" || sa.Sub != nil {
		return false
	}
	switch sa.Params["Choices"] {
	case "Player.Opponent", "Opponent":
	default:
		return false
	}
	d, ok := sa.Params["Defined"]
	return !ok || strings.TrimSpace(d) == "You"
}

// etbChosenOpp lowers the ETB replacement's ChoosePlayer node to
// ExtChosenOpp: the entering permanent's own, unconditional, sole choice.
func (lc *lowerCtx) etbChosenOpp(ps *params, node, sa *cards.SA, other bool, cond *lCond) error {
	if other || node != sa || cond.Kind != cNone || len(lc.ab.Prog) != 0 || !etbChooseOppShape(sa) || !entersChoosingOpp(lc.face) {
		return refuse("ETB replacement api ChoosePlayer")
	}
	lc.twoSeat("as-enters ChoosePlayer among the opponents answered as the one opponent")
	ps.consume("Choices", "Defined", "ChoiceTitle", "AILogic", "SpellDescription")
	lc.emit(lInstr{Op: "ChosenOpp", Note: "the one opponent is the chosen player"})
	return nil
}

// srcChosenDefined maps ChosenPlayer / Player.Chosen to the source's
// as-enters chosen player on a card that chooses one (entersChoosingOpp).
func (lc *lowerCtx) srcChosenDefined(v string) (lRef, bool) {
	if (v == "ChosenPlayer" || v == "Player.Chosen") && !lc.replEnter && entersChoosingOpp(lc.face) {
		return lRef{Kind: rSrcChosenPl}, true
	}
	return lRef{}, false
}
