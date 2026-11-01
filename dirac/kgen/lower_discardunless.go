package kgen

import (
	"strings"
)

// discardUnless reads Discard's UnlessType$ (Thirst for Discovery's
// "discard two cards unless you discard a basic land card"; gorge effDiscard
// reads it in the TgtChoose arm only): the op becomes OpDiscardUnless with
// the unless spec as its filter. Only the controller's own unfiltered,
// non-optional, unremembered TgtChoose discard lowers.
func (lc *lowerCtx) discardUnless(ps *params, in *lInstr, who lRef) error {
	ut, ok := ps.get("UnlessType")
	if !ok {
		return nil
	}
	if who.Kind != rYou || in.CS != "DiscarderChooses" || in.F != nil || in.C != 0 || in.Note != "" || ps.str("Mode") != "TgtChoose" {
		return refuse("Discard UnlessType$ shape")
	}
	f, err := lc.filter(strings.TrimSpace(ut))
	if err != nil {
		return err
	}
	in.Op, in.F, in.CS = "DiscardUnless", f, ""
	return nil
}
