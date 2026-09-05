package view

import (
	"fmt"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// Visibility is how much of the hidden information a projection reveals.
// It is a property of the viewer's relationship to the table, not of the
// game: the same state.Game projects three different Views.
type Visibility uint8

const (
	// Seat is a player's own view: their hand, their mana pool, a decision
	// asked of them; every other seat's hidden zones are counts. This is
	// what Project has always produced.
	Seat Visibility = iota
	// Public is a spectator with no seat: every hidden zone is a count, no
	// decision is attached. Today's spectator redaction under a name.
	Public
	// Omniscient is a spectator who sees every hand and every mana pool —
	// the bot-table default — but never library order: it spoils draws and
	// teaches nothing (spec D12).
	Omniscient
)

var visibilityNames = [...]string{"seat", "public", "omniscient"}

// String is the wire name; an out-of-range value prints "unknown", the same
// total shape as state.Step.String.
func (v Visibility) String() string {
	if int(v) < len(visibilityNames) {
		return visibilityNames[v]
	}
	return "unknown"
}

// ParseVisibility is String's inverse for flags and table configs.
func ParseVisibility(s string) (Visibility, error) {
	for i, n := range visibilityNames {
		if n == s {
			return Visibility(i), nil
		}
	}
	return 0, fmt.Errorf("view: unknown visibility %q (want seat, public or omniscient)", s)
}

// NoSeat is the viewer id of a spectator. state.PlayerID is a uint8 and no
// table has 255 seats, so it can never collide with a real seat; Project's
// own "out-of-range viewer is a spectator" rule does the rest.
const NoSeat state.PlayerID = 255

// ProjectFor is Project with an explicit visibility. Seat is exactly
// Project. Public forces the spectator path regardless of viewer. Omniscient
// projects every seat's hand and pool, but never attaches a decision to any
// viewer -- project is always called with a nil Decision in this branch, so
// even the seat d was asked of sees none: an omniscient view is for
// watching, not acting.
func ProjectFor(g *state.Game, ch Chars, viewer state.PlayerID, vis Visibility, d *decision.Decision) View {
	switch vis {
	case Public:
		v := project(g, ch, NoSeat, nil)
		v.Viewer = viewer
		v.Visibility = vis.String()
		return v
	case Omniscient:
		v := project(g, ch, viewer, nil)
		if g != nil {
			for i := range v.Players {
				p := &g.Players[i]
				v.Players[i].Hand = cardViews(g, ch, g.Zone(state.ZHand, p.ID))
				v.Players[i].Pool = poolView(p.Pool)
			}
		}
		v.Visibility = vis.String()
		return v
	default:
		v := project(g, ch, viewer, d)
		v.Visibility = Seat.String()
		return v
	}
}

// RedactEventsFor is RedactEvents with an explicit visibility. Seat and
// Public are RedactEvents (a Public viewer is NoSeat, so every owner-only
// branch stays closed). Omniscient passes every event through unredacted
// except a Secret event whose payload is or reveals library order —
// Shuffle (genesis order), a Secret Note (a private look at the top of the
// library), or any Secret move landing back IN a library (a Dig/rearrange
// that returns a card to a hidden position reveals where in the order it
// went, per Ruling FL-9) — which keep only their shape. A Secret Draw or
// MoveZone OUT of the library still passes: the card is now in a hand the
// omniscient viewer sees. A non-Secret move into a library (e.g. from a
// public zone) is not this kind of reveal and stays public.
func RedactEventsFor(g *state.Game, evs []events.Event, viewer state.PlayerID, vis Visibility) []events.Event {
	switch vis {
	case Public:
		return RedactEvents(g, evs, NoSeat)
	case Omniscient:
		out := make([]events.Event, 0, len(evs))
		for _, e := range evs {
			e.IDs = append([]state.ObjID(nil), e.IDs...)
			e.Pairs = append([][2]state.ObjID(nil), e.Pairs...)
			if e.Secret && (e.Kind == events.Shuffle || e.Kind == events.Note || e.To == state.ZLibrary) {
				out = append(out, events.Event{
					Seq: e.Seq, Kind: e.Kind, Player: e.Player,
					From: e.From, To: e.To, Step: e.Step, Secret: e.Secret,
				})
				continue
			}
			out = append(out, e)
		}
		return out
	default:
		return RedactEvents(g, evs, viewer)
	}
}

// poolView is the viewer-facing mana pool: only the symbols with mana in
// them, always non-nil so an empty pool marshals "{}" rather than null.
func poolView(m state.Mana) map[string]int32 {
	pool := map[string]int32{}
	for idx, sym := range [...]string{"W", "U", "B", "R", "G", "C"} {
		if n := m[idx]; n > 0 {
			pool[sym] = n
		}
	}
	return pool
}
