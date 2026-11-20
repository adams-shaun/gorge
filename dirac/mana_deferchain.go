package dirac

// deferChainLen is the number of pass nodes gorge's visit walks for the
// triggers a prefix queued (t.npend of them): each resolves under one pass,
// and p gets priority again after every resolution that leaves the payer
// alive. A resolution that kills the payer (Manabarbs' damage at the last
// life points) is the line's last node -- gorge's visit returns at the
// resulting priority's c.G.Over -- so the chain stops there instead of
// running through every queued trigger (CR 704.5a).
//
// The count is found by resolving the queue one stack item at a time on a
// fork. A trigger that does not touch the payer keeps the full length.
func (s *State) deferChainLen(k *prep, t *State) int {
	full := int(t.npend)
	u := t.fork()
	defer t.unfork()
	sp0 := u.sp
	u.resolvePrefixStack(k, sp0, false) // places the queued triggers, resolves none
	n := 0
	for !u.over && u.sp > sp0 && n < full {
		u.resolvePrefixStack(k, u.sp-1, true)
		n++
	}
	if n == 0 {
		return full
	}
	return n
}
