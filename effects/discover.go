package effects

import (
	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// Discover (CR 701.57) exiles cards from the top of the controller's library
// until a nonland card with mana value at most Num is found.  The found card
// is offered as a free Play; every other card exiled by the scan is put on the
// bottom first.  The engine's deterministic existing-order convention is used
// for the CR's random bottom order, as it is for Cascade.
func init() {
	Register("Discover", effDiscover)
	Register("DiscoverBottom", effDiscoverBottom)
}

func discoverPlaySA() *cards.SA {
	play := &cards.SA{Kind: "DB", API: "Play", Params: map[string]string{
		"Defined": "Remembered", "WithoutManaCost": "True", "Optional": "True",
		"TriggerDescription": "Discover",
	}}
	play.Sub = &cards.SA{Kind: "DB", API: "DiscoverBottom", Params: map[string]string{}}
	return play
}

func effDiscover(h Host, c *Ctx, sa *cards.SA) {
	value := Num(h, c, sa, "Num", 0)
	if value < 0 {
		value = 0
	}
	g := h.Game()
	p := c.Controller
	if int(p) >= len(g.Players) || g.Players[p].Lost {
		return
	}
	lib := append([]state.ObjID(nil), g.Zone(state.ZLibrary, p)...)
	var exiled []state.ObjID
	var found state.ObjID
	for _, id := range lib {
		o := g.Obj(id)
		if o == nil || o.Face() == nil {
			continue
		}
		h.Emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZLibrary,
			To: state.ZExile, Text: "discovered"})
		exiled = append(exiled, id)
		if !o.Face().IsLand() && int(o.Face().Cmc()) <= int(value) {
			found = id
			break
		}
	}
	if len(exiled) > 0 {
		h.Emit(events.Event{Kind: events.Note, Player: p, IDs: exiled,
			Text: "discovers, exiling cards from the top of the library"})
	}
	// The found card is left in exile for the Play election.  All preceding
	// cards are already known not to be the discover card and can be bottomed
	// before the election without changing anything observable.
	rest := exiled
	if found != 0 {
		rest = exiled[:len(exiled)-1]
	}
	for _, id := range rest {
		h.Emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZExile,
			To: state.ZLibrary, Text: "undiscovered card put on the bottom of the library"})
	}
	if found == 0 {
		return
	}
	c.Remembered = []state.Target{{Obj: found}}
	play := discoverPlaySA()
	effPlay(h, c, play)
	if !h.Suspended() {
		// R-9's no-host fallback declines the optional Play.
		effDiscoverBottom(h, c, play.Sub)
	}
}

// effDiscoverBottom is the chained tail of the optional cast.  On a decline
// the found card remains in exile and is bottomed; after a free cast it has
// left exile and is therefore skipped.
func effDiscoverBottom(h Host, c *Ctx, sa *cards.SA) {
	g := h.Game()
	remembered := c.Remembered
	c.Remembered = nil
	for _, t := range remembered {
		if t.IsPlayer || t.Obj == 0 {
			continue
		}
		o := g.Obj(t.Obj)
		if o == nil || o.Zone != state.ZExile {
			continue
		}
		h.Emit(events.Event{Kind: events.MoveZone, Obj: t.Obj, From: state.ZExile,
			To: state.ZLibrary, Text: "the undiscovered card is put on the bottom of the library"})
	}
}
