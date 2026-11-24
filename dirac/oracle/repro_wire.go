package oracle

import (
	"encoding/json"
	"fmt"

	"github.com/adams-shaun/gorge/engine/rules"
)

// A repro's actions go to disk as plain numbers, and the corpus under
// testdata/repro (and kfuzz's testdata) holds them as the two-seat build
// packed a Canon: kind<<24 | source<<16 | target<<8 | mode, one byte per
// field. That stays the wire form, so every recording replays as it was
// made: repro_wire_default.go maps the bytes to the kernel's nine-bit refs
// and back. A game that names a ref no byte can hold (a seat past the
// second, an instance past 235) is written in the "mp" packing, the Canon
// as it is, and says so in "packing".

// reproWire is Repro's JSON: the struct itself with its actions as wire words.
type reproWire struct {
	reproPlain
	Actions []uint64   `json:"actions"`
	Packing string     `json:"packing,omitempty"`
	Start   *startWire `json:"start,omitempty"`
}

// reproPlain is Repro without its methods, so encoding it does not recurse
// into MarshalJSON.
type reproPlain Repro

// MarshalJSON writes the actions in the build-independent wire form.
//
// A repro with a scripted start is koracle-repro-3, whatever Format says, and
// a repro without one is never written as koracle-repro-3 (see reproFormat3).
func (r Repro) MarshalJSON() ([]byte, error) {
	words, packing := wireActions(r.Actions)
	p := reproPlain(r)
	if r.Start != nil {
		p.Format = reproFormat3
	} else if p.Format == reproFormat3 {
		p.Format = reproFormat
	}
	return json.Marshal(reproWire{reproPlain: p, Actions: words, Packing: packing, Start: startToWire(r.Start)})
}

// UnmarshalJSON decodes a repro and maps a format-1 one's actions to the
// current Refs (format1Ref), so every reader of a recorded repro (ReadRepro,
// the pinned-game files) sees the same bytes. The file keeps its format-1
// bytes; the decoded repro says koracle-repro-2.
func (r *Repro) UnmarshalJSON(b []byte) error {
	var w reproWire
	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}
	format1 := w.Format == reproFormat1
	acts, err := actionsFromWire(w.Actions, w.Packing, format1)
	if err != nil {
		return fmt.Errorf("repro actions: %w", err)
	}
	p := Repro(w.reproPlain)
	p.Actions = acts
	p.Start = startFromWire(w.Start)
	if format1 {
		p.Format = reproFormat
	}
	*r = p
	return nil
}

// startWire is a scripted start's JSON: rules.StartSpec with its own lowercase
// keys, so the recording does not depend on the Go field names. Every slice is
// in the spec's order (the start's determinism lives in it), and a slice with
// nothing in it is left out.
type startWire struct {
	Seats []startSeatWire `json:"seats"`
}

type startSeatWire struct {
	Hand        []int              `json:"hand,omitempty"`
	Battlefield []int              `json:"battlefield,omitempty"`
	Graveyard   []int              `json:"graveyard,omitempty"`
	Exile       []int              `json:"exile,omitempty"`
	Library     []int              `json:"library,omitempty"`
	Tapped      []int              `json:"tapped,omitempty"`
	BackFace    []int              `json:"back_face,omitempty"`
	Counters    []startCounterWire `json:"counters,omitempty"`
	Attach      []startAttachWire  `json:"attach,omitempty"`
	Life        *int32             `json:"life,omitempty"`
}

type startCounterWire struct {
	Card int    `json:"card"`
	Kind string `json:"kind"`
	N    int32  `json:"n"`
}

type startAttachWire struct {
	Card   int `json:"card"`
	ToSeat int `json:"to_seat"`
	To     int `json:"to"`
}

// startToWire is sp as it is written; nil is nil.
func startToWire(sp *rules.StartSpec) *startWire {
	if sp == nil {
		return nil
	}
	w := &startWire{Seats: make([]startSeatWire, len(sp.Seats))}
	for i, sd := range sp.Seats {
		o := startSeatWire{Hand: sd.Hand, Battlefield: sd.Battlefield, Graveyard: sd.Graveyard,
			Exile: sd.Exile, Library: sd.Library, Tapped: sd.Tapped, BackFace: sd.BackFace, Life: sd.Life}
		for _, c := range sd.Counters {
			o.Counters = append(o.Counters, startCounterWire{Card: c.Card, Kind: c.Kind, N: c.N})
		}
		for _, a := range sd.Attach {
			o.Attach = append(o.Attach, startAttachWire{Card: a.Card, ToSeat: a.ToSeat, To: a.To})
		}
		w.Seats[i] = o
	}
	return w
}

// startFromWire is startToWire's inverse; nil is nil.
func startFromWire(w *startWire) *rules.StartSpec {
	if w == nil {
		return nil
	}
	sp := &rules.StartSpec{Seats: make([]rules.StartSeat, len(w.Seats))}
	for i, o := range w.Seats {
		sd := rules.StartSeat{Hand: o.Hand, Battlefield: o.Battlefield, Graveyard: o.Graveyard,
			Exile: o.Exile, Library: o.Library, Tapped: o.Tapped, BackFace: o.BackFace, Life: o.Life}
		for _, c := range o.Counters {
			sd.Counters = append(sd.Counters, rules.StartCounter{Card: c.Card, Kind: c.Kind, N: c.N})
		}
		for _, a := range o.Attach {
			sd.Attach = append(sd.Attach, rules.StartAttach{Card: a.Card, ToSeat: a.ToSeat, To: a.To})
		}
		sp.Seats[i] = sd
	}
	return sp
}

// gameWire is KernelGame's JSON: the struct itself with its start in the
// wire form. A game with no start encodes as it always did.
type gameWire struct {
	gamePlain
	Start *startWire `json:"start,omitempty"`
}

// gamePlain is KernelGame without its methods, so encoding it does not
// recurse into MarshalJSON.
type gamePlain KernelGame

// MarshalJSON writes the game, its start (when it has one) as koracle-game-3.
func (g KernelGame) MarshalJSON() ([]byte, error) {
	p := gamePlain(g)
	if g.Start != nil {
		p.Format = gameFormat3
	} else if p.Format == gameFormat3 {
		p.Format = gameFormat
	}
	return json.Marshal(gameWire{gamePlain: p, Start: startToWire(g.Start)})
}

// UnmarshalJSON decodes a game and its start.
func (g *KernelGame) UnmarshalJSON(b []byte) error {
	var w gameWire
	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}
	p := KernelGame(w.gamePlain)
	p.Start = startFromWire(w.Start)
	*g = p
	return nil
}
