package kgen

import "strings"

// A graveyard pick's DefinedPlayer$ (ChangeZone Hidden$ True, Origin$
// Graveyard, to the hand or the battlefield). gorge effHiddenPick is game-wide
// only when no fetch player is named (gameWide); with DefinedPlayer$ the pool
// is that player's own graveyard and the player is the chooser
// (hiddenPickChooser). Two spellings lower:
//
//   - You: the controller's own graveyard, the kernel's plain walk
//     (Genju of the Fields' "return CARDNAME from your graveyard", the pick
//     filtered to the source by Card.StrictlySelf).
//   - Player: each player in turn, from the controller, picks from their own
//     graveyard (Exhume). The kernel's RepeatEach runs the one-owner
//     ChooseMove with the repeated player (RefEach) as the picker, the shape
//     eachSearchLoop gives a library search.
type dpScope int

const (
	dpNone   dpScope = iota // no DefinedPlayer$: the game-wide pool
	dpYou                   // DefinedPlayer$ You
	dpPlayer                // DefinedPlayer$ Player
)

func graveDefinedPlayer(ps *params, lc *lowerCtx, dest int, flags []string, counterKind string) (dpScope, error) {
	dp, ok := ps.get("DefinedPlayer")
	if !ok {
		return dpNone, nil
	}
	switch strings.TrimSpace(dp) {
	case "You":
		ps.consume("DefinedPlayer")
		return dpYou, nil
	case "Player":
		if _, ok := ps.m["Chooser"]; ok {
			return dpNone, refuse("graveyard pick DefinedPlayer$ Player with Chooser$")
		}
		if _, ok := ps.m["ValidTgts"]; ok {
			return dpNone, refuse("graveyard pick DefinedPlayer$ Player with ValidTgts$")
		}
		if dest != zBattlefield || counterKind != "" {
			return dpNone, refuse("graveyard pick DefinedPlayer$ Player to %s", zoneLabel[dest])
		}
		for _, fl := range flags {
			// the entry under the picker's control and the other riders are
			// not walked per player
			if fl != "Mandatory" {
				return dpNone, refuse("graveyard pick DefinedPlayer$ Player with %s", fl)
			}
		}
		if _, ok := ps.m["Optional"]; ok {
			return dpNone, refuse("graveyard pick DefinedPlayer$ Player with Optional$")
		}
		if lc.repeatPl > 0 || lc.repeatCard > 0 {
			// the kernel's each-player loop variable is single
			return dpNone, refuse("graveyard pick DefinedPlayer$ Player inside a RepeatEach body")
		}
		ps.consume("DefinedPlayer")
		return dpPlayer, nil
	}
	return dpNone, nil // every other spelling keeps the lowering it had (a target player, an opponent's own zone)
}

// emitGravePick emits the ChooseMove, inside a one-instruction each-player
// loop when the fetch players are every player.
func (lc *lowerCtx) emitGravePick(sc dpScope, in lInstr) error {
	if sc != dpPlayer {
		lc.emit(in)
		return nil
	}
	lc.emit(lInstr{Op: "RepeatEach", Obj: lRef{Kind: rEachPlayer}, C: 1, Skip: 1})
	in.Obj = lRef{Kind: rEach}
	lc.emit(in)
	return nil
}
