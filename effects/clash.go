package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func init() { Register("Clash", effClash) }

// effClash implements DB$/SP$/AB$ Clash (CR 701.31, Forge's ClashEffect; 29
// corpus SA lines across 29 files). The resolving ability's controller clashes
// with an opponent:
//
//  1. Each clashing player reveals the top card of their library (a public
//     ids-Note each, the same non-Secret reveal encoding the Dig window and
//     effReveal emit).
//  2. Each revealed card is put on the TOP or the BOTTOM of its owner's
//     library. CR 701.31 leaves that placement to the card's owner; this
//     build uses the documented deterministic stand-in BOTTOM for every
//     player (see the note below), emitted as one Secret events.LibraryOrder
//     reordering the whole library -- the same single-event record rules'
//     handleArrange and Dig's moveRestToBottom use.
//  3. The player whose card had the strictly higher mana value wins; equal
//     mana values (including two empty libraries, both read as -1) leave NO
//     winner (CR 701.31, Forge's ClashEffect).
//  4. Forge's WinSubAbility$ runs when the resolving controller won,
//     OtherwiseSubAbility$ otherwise. Both names are resolved through the
//     source's SVar table and run as ordinary sub-abilities (the FlipCoin
//     WinSubAbility$/LoseSubAbility$ mechanism); the base SA's own
//     SubAbility$ chain is the enclosing Resolve loop's ordinary walk.
//  5. One events.Clash marker per clashing player records that player's
//     Won$ orientation (Amount 1 = won, 0 = lost or tied), which is what
//     trig:Clashed's ValidPlayer$/Won$ gate reads. Forge fires the Clashed
//     trigger once per player for exactly this reason.
//  6. RememberClasher$ True remembers the opponent(s) as player targets, so a
//     chained Defined$ Remembered names them (Captivating Glance's
//     OppControl).
//
// The clash opponent is, in order: the players a Defined$ names
// (Marvo's `Defined$ TriggeredDefendingPlayer`), else the player targets the
// ability chose (Pulling Teeth's `ValidTgts$ Player`), else the resolving
// controller's first living opponent -- Forge's own "choose an opponent"
// fallback, made deterministic rather than random. The resolving controller
// is always the first clashing player.
//
// Placement stand-in (bottom): CR 701.31's "top or bottom" is a per-player
// choice this build does not pose. It is a documented deterministic stand-in,
// not a silent omission: the reveal, the mana-value comparison and the
// win/lose record -- everything the mechanic and its trigger turn on -- are
// exact, and the card always ends in the library where CR 701.31 puts it.
func effClash(h Host, c *Ctx, sa *cards.SA) {
	players := clashParticipants(h, c, sa)
	if len(players) == 0 {
		return
	}

	// Reveal phase: each clashing player's top card, in participant order.
	// revealed[i] is 0 when that player's library is empty (their mana value
	// reads -1 and they cannot beat any real card). The Notes are public and
	// carry the ids, matching the Dig/Explore reveal encoding.
	revealed := make([]state.ObjID, len(players))
	cmc := make([]int, len(players))
	for i, p := range players {
		cmc[i] = -1
		lib := zoneOf(h.Game(), state.ZLibrary, p)
		if len(lib) == 0 {
			continue
		}
		top := lib[0]
		revealed[i] = top
		cmc[i] = manaValueOf(h.Game(), top)
		h.Emit(events.Event{Kind: events.Note, Player: p, IDs: []state.ObjID{top}})
	}

	// The winner is the single participant with the strictly highest mana
	// value; a tie of any width (all -1, or two equal cards) leaves no winner.
	winnerIdx := -1
	for i := range cmc {
		best := true
		for j := range cmc {
			if j != i && cmc[j] >= cmc[i] {
				best = false
				break
			}
		}
		if best && cmc[i] >= 0 {
			winnerIdx = i
			break
		}
	}
	c.ClashWinner = players[0]
	c.ClashWon = winnerIdx == 0
	if winnerIdx >= 0 {
		c.ClashWinner = players[winnerIdx]
	}

	// Placement phase: bottom, in participant order. A card revealed from the
	// top of a library ends on the bottom; an empty library moves nothing.
	for i, p := range players {
		if revealed[i] != 0 {
			clashMoveToBottom(h, p, revealed[i])
		}
	}

	// The win/lose markers FIRST, so a WinSubAbility$/OtherwiseSubAbility$
	// that suspends on a mid-resolution ask cannot suppress them (the queued
	// triggers resolve after this ability either way).
	for i, p := range players {
		won := winnerIdx >= 0 && i == winnerIdx
		var amount int32
		if won {
			amount = 1
		}
		h.Emit(events.Event{Kind: events.Clash, Obj: c.Source, Player: p, Amount: amount})
	}

	// RememberClasher$ True: the opposing clashing players (every participant
	// but the resolving controller) join Remembered as player targets.
	if strings.EqualFold(strings.TrimSpace(sa.Params["RememberClasher"]), "True") {
		for _, p := range players {
			if p == c.Controller {
				continue
			}
			c.Remembered = append(c.Remembered, state.Target{Player: p, IsPlayer: true})
		}
	}

	// Forge's outcome branch, resolved by name like FlipCoin's.
	name := strings.TrimSpace(sa.Params["OtherwiseSubAbility"])
	if c.ClashWon {
		name = strings.TrimSpace(sa.Params["WinSubAbility"])
	}
	if name != "" && c.SVars != nil {
		Resolve(h, c, cards.ResolveSVar(c.SVars, name))
	}
}

// clashParticipants is the deterministic clashing-player list: the resolving
// controller first, then (in order, deduped, living only) the Defined$-named
// players, the ability's chosen player targets, or the controller's first
// living opponent. The controller is always included, matching Forge's
// ClashEffect -- the controller is one of the two clashing players and is the
// one the WinSubAbility$ branch is keyed on.
func clashParticipants(h Host, c *Ctx, sa *cards.SA) []state.PlayerID {
	g := h.Game()
	out := []state.PlayerID{c.Controller}
	seen := map[state.PlayerID]bool{c.Controller: true}
	add := func(p state.PlayerID) {
		if int(p) >= len(g.Players) || seen[p] || g.Players[p].Lost {
			return
		}
		seen[p] = true
		out = append(out, p)
	}

	if strings.TrimSpace(sa.Params["Defined"]) != "" {
		for _, p := range definedPlayers(h, c, sa) {
			add(p)
		}
		if len(out) > 1 {
			return out
		}
		// A present-but-unresolvable Defined$ (an attack trigger whose
		// TriggeredDefendingPlayer role the engine could not capture) must
		// still clash with an opponent: fall through to Forge's own "choose
		// an opponent" fallback rather than clashing with nobody.
	}
	if _, targeted := sa.Params["ValidTgts"]; targeted {
		for _, t := range Defined(h, c, sa) {
			if t.IsPlayer {
				add(t.Player)
			}
		}
		if len(out) > 1 {
			return out
		}
	}
	for _, p := range g.AliveFrom(0) {
		if p != c.Controller {
			add(p)
			break
		}
	}
	return out
}

// clashMoveToBottom puts id on the BOTTOM of player p's library as one Secret
// events.LibraryOrder carrying the whole reordered library -- the same
// single-event record Dig's moveRestToBottom emits, so a replay re-derives
// the same order. The emit is skipped when the card already sits on the
// bottom (a one-card library), which leaves the log byte-identical to the
// no-op it is.
func clashMoveToBottom(h Host, p state.PlayerID, id state.ObjID) {
	g := h.Game()
	lib := zoneOf(g, state.ZLibrary, p)
	idx := -1
	for i, card := range lib {
		if card == id {
			idx = i
			break
		}
	}
	if idx < 0 || idx == len(lib)-1 {
		return
	}
	newLib := make([]state.ObjID, 0, len(lib))
	newLib = append(newLib, lib[:idx]...)
	newLib = append(newLib, lib[idx+1:]...)
	newLib = append(newLib, id)
	h.Emit(events.Event{Kind: events.LibraryOrder, Player: p, IDs: newLib, Secret: true})
}
