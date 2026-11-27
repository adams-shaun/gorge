package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// api_planechase.go lowers the planechase verbs the kernel models (dirac/
// zone_planechase.go). A plane's own chaos ability is a trigger (lower_trigger.go);
// DB$ ChaosEnsues is the marker that erupts the resolving controller's
// face-up current plane (CR 901.9), or -- Defined$ Remembered (The Fertile
// Lands of Saulvinia) -- each remembered plane of that controller's own
// planar deck (gorge effChaosEnsues' chaosDestinations arm; the dig's
// RememberFound$ put the revealed plane there, bottom seat, face up, and
// gorge planar.go's checkChaosEnsuesTriggers gates the marker on the plane's
// membership in ev.Player's own deck).
func init() {
	apiOp("ChaosEnsues", (*lowerCtx).apiChaosEnsues)
	apiOp("Planeswalk", (*lowerCtx).apiPlaneswalk)
	apiOp("RollPlanarDice", (*lowerCtx).apiRollPlanarDice)
}

// apiPlaneswalk lowers DB$ Planeswalk's plain rotation form: the resolving
// controller planeswalks to the next plane of their planar deck (CR 901.8;
// gorge effPlaneswalk's PlanarWalk emission, dirac zone_planechase.go's
// ExtPlaneswalk). Optional$ True is a "you may" election this build does not
// lower and Defined$ Remembered (Norn's Seedcore's destination walk) names
// destinations the kernel's rotation does not take: both refuse by name, as
// does a ValidTgts$ (gorge reads none -- the verb's subject is its
// controller).
func (lc *lowerCtx) apiPlaneswalk(_ *cards.SA, ps *params) error {
	if _, ok := ps.m["ValidTgts"]; ok {
		return refuse("Planeswalk ValidTgts$")
	}
	if v, ok := ps.m["Optional"]; ok && strings.TrimSpace(v) != "False" {
		return refuse("Planeswalk Optional$ %q", v)
	}
	if v, ok := ps.m["Defined"]; ok && strings.TrimSpace(v) == "Remembered" {
		return refuse("Planeswalk Defined$ Remembered")
	}
	ps.consume("Optional", "Defined")
	who, err := lc.defined("You")
	if err != nil {
		return err
	}
	if left := ps.unread(); len(left) > 0 {
		return refuse("Planeswalk unread %v", left)
	}
	lc.emit(lInstr{Op: "Planeswalk", Obj: who})
	return nil
}

func (lc *lowerCtx) apiChaosEnsues(_ *cards.SA, ps *params) error {
	switch strings.TrimSpace(ps.str("Defined")) {
	case "You", "":
		who, err := lc.objOrTarget(ps, "You", zNone)
		if err != nil {
			return err
		}
		lc.emit(lInstr{Op: "ChaosEnsues", Obj: who})
	case "Remembered":
		lc.emit(lInstr{Op: "ChaosEnsues", Obj: lRef{Kind: rRemembered}})
	default:
		return refuse("ChaosEnsues Defined$ %q", ps.str("Defined"))
	}
	return nil
}

// apiRollPlanarDice lowers DB$ RollPlanarDice (CR 901.3): the resolving
// player rolls Amount$ planar dice (default 1, gorge effRollPlanarDice's
// Num). The count rides B because the instruction's A names the ExtOp
// (dirac zone_planechase.go rollPlanarDiceExt); each die is a d6 whose 5/6
// faces planeswalk/ensue chaos. The roll's own replacements (Ichor Elixir)
// are matched by the kernel at the roll, so no body is lowered here.
func (lc *lowerCtx) apiRollPlanarDice(_ *cards.SA, ps *params) error {
	n, err := lc.amount(ps, "Amount", 1)
	if err != nil {
		return err
	}
	who, err := lc.defined("You")
	if err != nil {
		return err
	}
	lc.emit(lInstr{Op: "RollPlanarDice", Obj: who, B: n, Note: "roll Amount planar dice (CR 901.3; dirac zone_planechase.go)"})
	return nil
}
