package kgen

import (
	"github.com/adams-shaun/gorge/engine/cards"
)

// The api: effects for zone changes, library order and reveals (lower_api.go).
func init() {
	apiPS("ChangeZone", (*lowerCtx).changeZone)
	apiPS("Assimilate", (*lowerCtx).assimilate) // lower_assimilate.go
	apiOp("ChangeZoneAll", (*lowerCtx).apiChangeZoneAll)
	apiPS("Dig", (*lowerCtx).dig)
	apiPS("DigUntil", (*lowerCtx).digUntil)
	apiPS("Encore", (*lowerCtx).encore) // lower_encore.go
	apiPS("PeekAndReveal", (*lowerCtx).peekAndReveal)
	apiPS("TwoPiles", (*lowerCtx).twoPiles)
	apiPS("Play", (*lowerCtx).play)
	apiPS("Seek", (*lowerCtx).seek)
	apiPS("Reveal", (*lowerCtx).reveal)
	apiPS("RevealHand", (*lowerCtx).revealHand)
	apiPS("Explore", (*lowerCtx).explore)
	apiPS("Exploit", (*lowerCtx).exploit) // lower_exploit.go
	apiPS("Discover", (*lowerCtx).discover)
	apiPS("Hideaway", (*lowerCtx).hideaway) // lower_hideaway.go
	apiOp("RearrangeTopOfLibrary", (*lowerCtx).apiRearrangeTopOfLibrary)
	apiOp("Shuffle", (*lowerCtx).apiShuffle)
}

func (lc *lowerCtx) apiChangeZoneAll(sa *cards.SA, ps *params) error {
	// AbilityCount$ X names the SVar the ChangeType$ X-comparison reads;
	// gorge reads the comparison's own SVar and never this param
	// (Profaner of the Dead), so it is inert here too
	if ps.str("AbilityCount") == "X" {
		ps.consume("AbilityCount")
	}
	return lc.changeZoneAll(ps)
}

func (lc *lowerCtx) apiRearrangeTopOfLibrary(sa *cards.SA, ps *params) error {
	// gorge effRearrangeTopOfLibrary: the controller orders the top
	// NumCards$ of the one acting player's library (MayShuffle$ and
	// RearrangePlayer$ refuse; a constant count up to 8)
	if _, ok := ps.get("RearrangePlayer"); ok {
		return refuse("RearrangeTopOfLibrary RearrangePlayer$")
	}
	in := lInstr{Op: "Rearrange"}
	if ms, ok := ps.get("MayShuffle"); ok {
		if ms != "True" {
			return refuse("RearrangeTopOfLibrary MayShuffle %q", ms)
		}
		in.C = 1 // gorge rearrangeMayShuffleTape: a yes/no shuffle ask
	}
	n, err := lc.amount(ps, "NumCards", 1)
	if err != nil {
		return err
	}
	if !(n.isConst() && n.N >= 1 && n.N <= 8) && !rearrangeWholeLibrary(n) {
		return refuse("RearrangeTopOfLibrary NumCards %v", n)
	}
	who, err := lc.objOrTarget(ps, "You", zNone)
	if err != nil {
		return err
	}
	switch who.Kind {
	case rYou:
	case rTarget:
		if t := lc.ab.Targets[who.N]; t.F == nil || t.F.Players == 0 || len(t.F.Clauses) != 0 || !t.Max.isConst() || t.Max.N > 1 {
			return refuse("RearrangeTopOfLibrary of target slot %d", who.N)
		}
	default:
		return refuse("RearrangeTopOfLibrary of %s", who)
	}
	in.Obj, in.A = who, n
	lc.emit(in)
	return nil
}

func (lc *lowerCtx) apiShuffle(sa *cards.SA, ps *params) error {
	// gorge effShuffle: the controller, or the one Defined$ / target
	// player (several players and object owners refuse)
	if ps.is("Defined", "RememberedOwner") {
		return lc.shuffleRememberedOwners(ps) // lower_shuffleremowners.go
	}
	if ps.is("Defined", "Player.IsRemembered") {
		// each player who searched this way shuffles, in search order (Wave
		// of Vitriol). The kernel holds the seats in a mask
		// (zone_remembersearcher.go); only the remembered-searcher search
		// that wrote it lowers.
		if _, tgt := ps.get("ValidTgts"); tgt {
			return refuse("Shuffle Defined$ Player.IsRemembered with a target")
		}
		if !lc.hasRememberedSearcher() {
			return refuse("Shuffle Defined$ Player.IsRemembered without a RememberSearcher$ search in the ability")
		}
		ps.consume("Defined")
		lc.emit(lInstr{Op: "Shuffle", C: 2})
		return nil
	}
	who, err := lc.objOrTarget(ps, "You", zNone)
	if err != nil {
		return err
	}
	switch who.Kind {
	case rYou, rTriggeredPlayer, rTargetController:
	case rTarget:
		if t := lc.ab.Targets[who.N]; t.F == nil || t.F.Players == 0 || len(t.F.Clauses) != 0 || !t.Max.isConst() || t.Max.N > 1 {
			return refuse("Shuffle of target slot %d", who.N)
		}
	default:
		return refuse("Shuffle of %s", who)
	}
	lc.emit(lInstr{Op: "Shuffle", Obj: who})
	return nil
}
