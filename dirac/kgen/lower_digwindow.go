package kgen

// digWindowRemember lowers Dig's RememberRevealed$ True over a Reveal$ True
// window: gorge effDig appends the whole window (the top DigNum$ cards, in
// library order) to the resolution's remembered set before the ChangeValid$
// filter and every move (dig.go: the reveal election, then the Remembered
// append). The kernel's public Peek appends the same top cards with no ask
// (OpPeek PeekPublic|PeekRemember, gorge's public PeekAndReveal), so the
// window is remembered by a Peek of the same count just before the Dig. Only
// the window of a constant size over the controller's own library is read
// exactly: a RevealOptional$ window (an election), a NoReveal$ window (never
// revealed, so never remembered), a FromBottom$ window and any value that
// can change between the Peek and the Dig keep their refusals.
func (lc *lowerCtx) digWindowRemember(ps *params, who lRef, n lValue, noReveal bool) (bool, error) {
	v, ok := ps.get("RememberRevealed")
	if !ok {
		return false, nil
	}
	if v != "True" {
		return false, refuse("Dig RememberRevealed %q", v)
	}
	if noReveal || !ps.is("Reveal", "True") {
		return false, refuse("Dig RememberRevealed$ over a window that is not revealed")
	}
	if _, fb := ps.m["FromBottom"]; fb || ps.is("RevealOptional", "True") {
		return false, refuse("Dig RememberRevealed$ beside FromBottom$/RevealOptional$")
	}
	if _, rc := ps.m["RememberChanged"]; rc {
		// gorge's remember semantics for a RememberRevealed$ window beside
		// RememberChanged$ (append after the window, or replace?) is unread;
		// the kernel would append twice. Fail closed.
		return false, refuse("Dig RememberRevealed$ beside RememberChanged$")
	}
	if who.Kind != rYou || !n.isConst() || n.N < 1 {
		return false, refuse("Dig RememberRevealed$ of %v over %v", who, n)
	}
	lc.emit(lInstr{Op: "Peek", Obj: who, A: n, CS: "Public|Remember"})
	return true, nil
}
