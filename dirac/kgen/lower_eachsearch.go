package kgen

import "strings"

// searchOwner lowers a library search's DefinedPlayer$: whose library is
// searched. Gorge reads it for this ChangeZone shape (params.DefinedPlayerRef);
// absent or blank is the controller's own library.
//
// DefinedPlayer$ Player is "each player searches their library" (Field of
// Ruin, Veteran Explorer). gorge effSearchLibrary walks definedSpecPlayer
// (AliveFrom the controller) and gives each player their own confirm, pick
// and shuffle (searchChooser -> the library's owner, CR 701.23a). The
// kernel's OpRepeatEachPlayer with C&1 walks the same aliveFrom(ctrl) order,
// and startSearch resolves RefEach to the loop's player, so the search runs
// as a one-instruction RepeatEach body over rEach (eachSearchLoop).
//
// arm reports a WithCountersType$ arm: its arm and clear would sit outside
// the one-instruction loop, so that shape refuses.
func (lc *lowerCtx) searchOwner(ps *params, arm bool) (lRef, error) {
	dp, ok := ps.get("DefinedPlayer")
	if !ok || strings.TrimSpace(dp) == "" {
		return lRef{Kind: rYou}, nil
	}
	v := strings.TrimSpace(dp)
	if v != "Player" {
		return lc.definedPlayer(v) // lower_dpsearch.go
	}
	if ch, ok := ps.m["Chooser"]; ok {
		// Jace, Architect of Thought's -8: one player searches every
		// library and imprints across them, a different shape from each
		// player searching their own.
		return lRef{}, refuse("ChangeZone DefinedPlayer$ Player with Chooser$ %s: one player searching every library and imprinting across them is not lowered", strings.TrimSpace(ch))
	}
	if arm {
		return lRef{}, refuse("ChangeZone DefinedPlayer$ Player with WithCountersType$: the counter arm would sit outside the each-player loop")
	}
	if lc.repeatPl > 0 || lc.repeatCard > 0 {
		// the kernel's each-player loop variable is single: a search over
		// every player inside another repeat would clobber it
		return lRef{}, refuse("ChangeZone DefinedPlayer$ Player inside a RepeatEach body")
	}
	return lRef{Kind: rEach}, nil
}

// eachSearchLoop opens the one-instruction each-player loop around the Search
// emitted next when searchOwner returned the loop variable.
func (lc *lowerCtx) eachSearchLoop(obj lRef) {
	if obj.Kind == rEach {
		lc.emit(lInstr{Op: "RepeatEach", Obj: lRef{Kind: rEachPlayer}, C: 1, Skip: 1})
	}
}
