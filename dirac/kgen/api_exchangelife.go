package kgen

import "github.com/adams-shaun/gorge/engine/cards"

func init() { apiOp("ExchangeLife", (*lowerCtx).exchangeLife) }

// exchangeLife lowers api:ExchangeLife's one-target shape (gorge
// engine/effects/life.go effExchangeLife, the len(players) == 1 arm): the
// resolving controller exchanges life totals with the one targeted player
// (Magus of the Mirror, Mirror Universe).
//
// gorge's two-target arm (TargetMin$/TargetMax$ 2: Soul Conduit, Axis of
// Mortality) exchanges the two targets with each other, which the kernel's one
// player slot cannot name; it refuses by name. A Defined$ or default subject
// refuses too: only a target slot of one player lowers. RememberOwnLoss$ and
// RememberDifference$ (Darkforce Inversion, Reverse Polarity's chain) are
// keys this lowering does not read, so the ability refuses on them.
func (lc *lowerCtx) exchangeLife(sa *cards.SA, ps *params) error {
	who, err := lc.objOrTarget(ps, "You", zNone)
	if err != nil {
		return err
	}
	if who.Kind != rTarget {
		return refuse("ExchangeLife of %s: only one targeted player lowers", who)
	}
	if t := lc.ab.Targets[who.N]; t.F == nil || t.F.Players == 0 || len(t.F.Clauses) != 0 || !t.Max.isConst() || t.Max.N != 1 {
		return refuse("ExchangeLife of target slot %d: not one player (gorge's two-target arm exchanges the targets)", who.N)
	}
	lc.emit(lInstr{Op: "ExchangeLife", Obj: who})
	return nil
}
