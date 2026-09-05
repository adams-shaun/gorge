package protocol

import (
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// Event is events.Event for the wire: the same fields, with Kind, zones and
// step as their names. Zone and step names are set only for the kinds that
// carry them, so a Tap never reads as "from library". events.Event's own
// JSON is untouched — it is what the .events file holds.
type Event struct {
	Seq     uint64      `json:"seq"`
	Kind    string      `json:"kind"`
	Player  uint8       `json:"player"`
	Obj     uint32      `json:"obj,omitempty"`
	From    string      `json:"from,omitempty"`
	To      string      `json:"to,omitempty"`
	Amount  int32       `json:"amount,omitempty"`
	Step    string      `json:"step,omitempty"`
	Counter string      `json:"counter,omitempty"`
	Text    string      `json:"text,omitempty"`
	IDs     []uint32    `json:"ids,omitempty"`
	Pairs   [][2]uint32 `json:"pairs,omitempty"`
	Secret  bool        `json:"secret,omitempty"`
}

// EventFrom converts one (already redacted) engine event.
func EventFrom(e events.Event) Event {
	w := Event{
		Seq: e.Seq, Kind: e.Kind.String(), Player: uint8(e.Player), Obj: uint32(e.Obj),
		Amount: e.Amount, Counter: e.Counter, Text: e.Text, Secret: e.Secret,
	}
	switch e.Kind {
	case events.MoveZone, events.Draw, events.PutOnStack:
		w.From, w.To = zoneName(e.From), zoneName(e.To)
	case events.StepChange:
		w.Step = e.Step.String()
	}
	if len(e.IDs) > 0 {
		w.IDs = make([]uint32, len(e.IDs))
		for i, id := range e.IDs {
			w.IDs[i] = uint32(id)
		}
	}
	if len(e.Pairs) > 0 {
		w.Pairs = make([][2]uint32, len(e.Pairs))
		for i, p := range e.Pairs {
			w.Pairs[i] = [2]uint32{uint32(p[0]), uint32(p[1])}
		}
	}
	return w
}

func zoneName(z state.Zone) string {
	if !z.Valid() {
		return "unknown"
	}
	return z.String()
}
