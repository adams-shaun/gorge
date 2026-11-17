package cardcov

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
)

// Game is one finished game's coverage: the cards cast or land-played, the
// cards with any ability pushed, and per card the keys Used and Offered.
type Game struct {
	Cast, Ability map[string]bool
	Used, Offered map[string]map[string]bool
}

// Played walks the finished game for which deck cards were cast/land-played,
// which had an ability put on the stack, and which of their own abilities
// were used (Used) and offered (Offered).
func Played(e *rules.Engine, decks [][]*cards.Card, p *Probe) Game {
	g := Game{Cast: map[string]bool{}, Ability: map[string]bool{}}
	name := func(id state.ObjID) string { return realCardName(e, id) }
	evs := e.L.Events
	for i, ev := range evs {
		switch ev.Kind {
		case events.PutOnStack:
			if ev.To == state.ZStack {
				if n := name(ev.Obj); n != "" {
					g.Cast[n] = true
				}
			}
		case events.LandPlayed:
			if n := name(playedLand(evs, i)); n != "" {
				g.Cast[n] = true
			}
		case events.AbilityPush, events.TriggerPush:
			if n := name(ev.Obj); n != "" {
				g.Ability[n] = true
			}
		}
	}
	g.Used = Used(e, decks, p)
	g.Offered = Offered(e, decks, p)
	return g
}

// playedLand is the object a LandPlayed event (at index i) played. The event
// carries only the player (engine/rules/cast.go's settleLandPlay: it closes the
// land-play accounting AFTER the land's entry boundary), so the land is the
// latest MoveZone before it in the same action -- the scan stops at the
// Priority event the play_land answer emits before moving the card. The land's
// own move is normally hand->battlefield; a replacement that fully replaces
// the entry (CR 305.1: the land was still played) moves it elsewhere, and
// that move is still the played card's. Returns 0 when none is found.
func playedLand(evs []events.Event, i int) state.ObjID {
	for j := i - 1; j >= 0; j-- {
		switch evs[j].Kind {
		case events.MoveZone:
			return evs[j].Obj
		case events.Priority:
			return 0
		}
	}
	return 0
}
