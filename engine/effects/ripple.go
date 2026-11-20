package effects

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Ripple (CR 702.60). The keyword is expanded into a real triggered ability
// (engine/cards/kw_ripple.go: printed K:Ripple lines) and synthesized for
// layer-6 grants (rules' checkGrantedRippleTriggers, the Thrumming Stone
// shape); both push a respondable stack ability whose body is the DB$ Ripple
// primitive registered here, and the ability's RESOLUTION runs the
// reveal + may-cast + bottom sequence:
//
//   - "you may reveal the top N cards of your library (all of them if
//     fewer)" — the reveal-may is the trigger's own OptionalDecider$ You
//     (CR 603.5): a declined may resolves nothing and the library is
//     untouched. An accepted one reveals min(N, library) cards from the
//     top of the caster's library in zone order (index 0 = top) with ONE
//     public Note (the ids payload the cascade exile Note and effDig's
//     reveal Note share), and the cards stay in the library while they are
//     revealed.
//   - "you may cast any of those cards with the same name as this spell
//     without paying their mana costs" — a real optional election through
//     the effPlay shape (DB$ Play | Defined$ Remembered | WithoutManaCost$
//     True | Optional$ True | Amount$ All, population = the same-name
//     revealed cards on the resolution's Remembered), whose answer
//     rules' playAnswerSettle turns into zero-mana-cost casts begun from
//     the library (the Sunbird's Invocation route; CR 608.2g casts during
//     resolution, as Cascade). The name comparison is the cards' FACE name
//     against the ripple spell's face name (CR 201.2), read at resolution.
//   - "put all revealed cards not cast this way on the bottom of your
//     library in any order" — the chained RippleBottom tail runs after the
//     election (after every parked free cast has begun, the playAnswerSettle
//     / startQueuedPlay resume Cascade's residue shares): the revealed cards
//     still in the library are offered to the existing CR 401.4 arrangement
//     ask (libraryPlacementOrder) and bottomed in the answer's order.
//
// A card the free cast took left the library for the stack, so it is neither
// offered for casting again nor bottomed; a cast the player declined left
// its card in the library and it bottoms with the rest.
//
// The count N is the Num$ param through the ordinary Num grammar; an
// unresolvable or non-positive value is a no-op (the documented Num
// convention), never a guess.
func init() {
	Register("Ripple", effRipple)
	// RippleBottom is the ripple instruction's tail (bottom the revealed
	// cards that were not cast). It is never a corpus API: effRipple builds
	// it so a resolution that suspends at a free cast resumes into it.
	Register("RippleBottom", effRippleBottom)
	// kw:Ripple (the expansion marker the coverage walk and the
	// granted-keyword census read) is registered rules-side, with the other
	// trigger-expanded keywords: engine/rules/trigger_match.go's init.
}

// ripplePlaySA builds the free-cast election's Play SA with the RippleBottom
// tail chained as its Sub. Hand-built (the ripple trigger's body is
// synthesized from the keyword, so no face SVar table carries it) and
// freshly built per call: SA values are immutable shared card data
// elsewhere, but this one is per-resolution scratch, built here and
// referenced only by the resolution it is built for (the same shape
// cascadePlaySA keeps). revealed rides the tail's Params: the tail needs the
// FULL revealed set (cast cards included, so it can skip them), which the
// election's Remembered population alone does not carry.
func ripplePlaySA(revealed []state.ObjID) *cards.SA {
	ids := make([]string, 0, len(revealed))
	for _, id := range revealed {
		ids = append(ids, strconv.FormatUint(uint64(id), 10))
	}
	play := &cards.SA{Kind: "DB", API: "Play", Params: map[string]string{
		"Defined":            "Remembered",
		"WithoutManaCost":    "True",
		"Optional":           "True",
		"Amount":             "All",
		"TriggerDescription": "Ripple",
	}}
	play.Sub = &cards.SA{Kind: "DB", API: "RippleBottom",
		Params: map[string]string{"Revealed": strings.Join(ids, ",")}}
	return play
}

// effRipple runs one ripple trigger's reveal + may-cast sequence. Source is
// the ripple spell (the trigger's Source -- the stack object the cast
// pushed), Controller the caster; the library revealed from is the
// controller's own.
func effRipple(h Host, c *Ctx, sa *cards.SA) {
	g := h.Game()
	src := g.Obj(c.Source)
	if src == nil || src.Face() == nil {
		return
	}
	p := c.Controller
	if int(p) >= len(g.Players) || g.Players[p].Lost {
		return
	}
	n, _ := NumResolved(h, c, sa, cards.PKNum.String(), 0)
	if n <= 0 {
		return
	}
	// Snapshot the library before anything moves: the free casts below move
	// cards out of the zone slice under us.
	lib := append([]state.ObjID(nil), g.Zone(state.ZLibrary, p)...)
	count := int(n)
	if count > len(lib) {
		count = len(lib) // "(all of them if fewer)"
	}
	revealed := lib[:count]
	if len(revealed) == 0 {
		return
	}
	// The reveal is public: one Note naming every revealed id.
	h.Emit(events.Event{Kind: events.Note, Player: p, IDs: append([]state.ObjID(nil), revealed...),
		Text: "ripple reveals the top of the library"})
	// CR 201.2: the name comparison is face-name to face-name.
	name := src.Face().Name
	var same []state.ObjID
	for _, id := range revealed {
		if o := g.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name {
			same = append(same, id)
		}
	}
	playSA := ripplePlaySA(revealed)
	if len(same) > 0 {
		c.Remembered = make([]state.Target, 0, len(same))
		for _, id := range same {
			c.Remembered = append(c.Remembered, state.Target{Obj: id})
		}
		effPlay(h, c, playSA)
	}
	if !h.Suspended() {
		effRippleBottom(h, c, playSA.Sub)
	}
}

// effRippleBottom is the ripple instruction's tail, run either inline (the
// election answered in place) or chained after a Play that suspended at a
// free cast's own ask. It bottoms every revealed card still in the library
// -- the cards the casts did not take -- in an order the controller chooses
// (CR 702.60a, 401.4): the existing arrangement ask offers the revealed
// cards the library still holds, and the answer's order is the bottom
// order. A cast card is on the stack and skipped; a card a response moved is
// skipped by the same zone recheck.
func effRippleBottom(h Host, c *Ctx, sa *cards.SA) {
	var revealed []state.ObjID
	for _, s := range strings.Split(sa.ParamStr(cards.PKRevealed), ",") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		v, err := strconv.ParseUint(s, 10, 32)
		if err != nil {
			return // an unreadable scratch param bottoms nothing
		}
		revealed = append(revealed, state.ObjID(v))
	}
	if len(revealed) == 0 {
		return
	}
	g := h.Game()
	p := c.Controller
	if int(p) >= len(g.Players) || g.Players[p].Lost {
		return
	}
	var inLib []state.ObjID
	for _, id := range revealed {
		if o := g.Obj(id); o != nil && o.Zone == state.ZLibrary {
			inLib = append(inLib, id)
		}
	}
	if len(inLib) == 0 {
		return
	}
	order := libraryPlacementOrder(h, c, sa, p, inLib, true)
	libraryOrderPlacement(h, p, order, true)
}
