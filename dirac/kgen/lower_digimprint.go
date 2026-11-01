package kgen

// lower_digimprint.go: DigUntil's ImprintFound$ + RememberRevealed$ (Jodah, the
// Unifier): "exile cards from the top of your library until you
// exile a legendary nonland card with lesser mana value. You may cast that
// card without paying its mana cost. Put the rest on the bottom of your
// library in a random order." gorge effDigUntil imprints the found card and
// appends every revealed card (the found one included) to the remembered
// set; the chain then plays Defined$ Imprinted and moves Card.IsRemembered
// from exile to the library bottom with RandomOrder$ (zone_changeall.go: the
// exile scan order Fisher-Yates'd through the engine rng, then placed).
//
// The kernel's remembered set holds only the found card (RememberFound),
// which is what the Play reads, and the DigUntil marks where its cards start
// in the exile (DugMark): the bottom move (OpDiscover C=DiscoverDugBottom)
// takes every card from there still in exile and not queued to be cast, the
// same set as gorge's remembered cards left in exile. Only the exact chain
// is read: one found card, both piles to exile, then that Play, that move
// and a Cleanup.

func (lc *lowerCtx) digUntilImprint(ps *params, in lInstr, found, rev int) error {
	if !ps.is("ImprintFound", "True") || !ps.is("RememberRevealed", "True") {
		return refuse("DigUntil ImprintFound$/RememberRevealed$ apart")
	}
	if _, ok := ps.get("RememberFound"); ok {
		return refuse("DigUntil RememberFound$ beside ImprintFound$")
	}
	if in.A.Kind != vConst || in.A.N != 1 || found != zExile || rev != zExile || in.CS != "revealed=Exile" {
		return refuse("DigUntil ImprintFound$ shape")
	}
	if in.Obj.Kind != rYou {
		return refuse("DigUntil ImprintFound$ of another player's library")
	}
	n := lc.curNode.Sub
	if n == nil || n.API != "Play" || n.Params["Defined"] != "Imprinted" {
		return refuse("DigUntil ImprintFound$ without a Play of the imprinted card")
	}
	m := n.Sub
	if m == nil || m.API != "ChangeZoneAll" || m.Params["ChangeType"] != "Card.IsRemembered" || m.Params["Origin"] != "Exile" ||
		m.Params["Destination"] != "Library" || m.Params["RandomOrder"] != "True" || m.Params["LibraryPosition"] != "-1" {
		return refuse("DigUntil ImprintFound$ without the random-order bottom of the revealed cards")
	}
	if c := m.Sub; c == nil || c.API != "Cleanup" || c.Sub != nil {
		return refuse("DigUntil ImprintFound$ chain tail")
	}
	in.CS += "|RememberFound|DugMark"
	lc.digImprinted = true
	lc.emit(in)
	return nil
}

func (lc *lowerCtx) digImprintBottom(ps *params) error {
	for _, k := range []string{"ChangeType", "Origin", "Destination", "RandomOrder", "LibraryPosition"} {
		ps.consume(k)
	}
	lc.emit(lInstr{Op: "DigBottom"})
	return nil
}
