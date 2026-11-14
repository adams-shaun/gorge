package kgen

// libThirdOK reports whether a ChangeZone to the third place from the top
// of a library (LibraryPosition$ 2, kop.LibThird) is read exactly for obj
// leaving origin: the source leaving the battlefield (Braided Quipu), or
// the card a source's own dies / leaves-the-battlefield trigger names
// (Defined$ TriggeredNewCardLKICopy with no Origin$: God-Eternal Oketra's
// "when this dies or is put into exile from the battlefield, you may put it
// into its owner's library third from the top"). The trigger object is the
// card itself, now in the graveyard or exile; gorge moves it wherever it
// is and the kernel's toLibraryThird does the same.
func (lc *lowerCtx) libThirdOK(obj lRef, origin int) bool {
	switch {
	case obj.Kind == rSelf && origin == zBattlefield:
		return true
	case obj.Kind == rTriggeredNew && origin == zNone:
		return lc.selfDiesTrigger() || lc.selfLeavesTrigger()
	}
	return false
}

// selfLeavesTrigger is selfDiesTrigger for a Leaves line (a battlefield
// departure to a set of zones): the trigger is on the source's own card.
func (lc *lowerCtx) selfLeavesTrigger() bool {
	tr := lc.ab.Trigger
	return tr != nil && tr.Event == "Leaves" && tr.Origin == zBattlefield && tr.F != nil && tr.F.Players == 0 && len(tr.F.Clauses) == 1 && tr.F.Clauses[0].Flags&fSelf != 0
}
