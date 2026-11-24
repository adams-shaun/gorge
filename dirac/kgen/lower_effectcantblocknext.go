package kgen

// cantBlockGrant emits the grant of a targeted Effect's CantBlockBy static
// over obj once its shape is read. A ValidBlocker$ form is the ExHaste record
// (until end of turn). Duration$ UntilYourNextTurn (Sly Instigator) is the
// ovNT override record's unblockable keyword: gorge keeps the restriction
// through the opponent's next combat (CR 611.2b), which the end-of-turn
// keyword grant would not. Anything else is the end-of-turn keyword grant.
func (lc *lowerCtx) cantBlockGrant(ps *params, obj lRef, exHaste bool, exC int, exF *lFilter) error {
	next := ps.is("Duration", "UntilYourNextTurn")
	switch {
	case exHaste && next:
		return refuse("Effect CantBlockBy ValidBlocker with Duration UntilYourNextTurn")
	case exHaste:
		lc.emit(lInstr{Op: "ExHaste", Obj: obj, C: exC, F: exF, Note: "until end of turn: blockable only by creatures with haste, or not by creatures whose power compares (C)"})
	case next:
		lc.emitOverride(obj, &lOverride{AddKw: []string{"Unblockable"}}, "UntilYourNextTurn")
	default:
		lc.emit(lInstr{Op: "GrantKeyword", Obj: obj, CS: "Unblockable"})
	}
	return nil
}
