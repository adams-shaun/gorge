package oracle

import (
	"encoding/json"
	"fmt"
)

// A repro's actions go to disk as plain numbers, and the corpus under
// testdata/repro (and kfuzz's testdata) holds them as the default size
// class packs a Canon: kind<<24 | source<<16 | target<<8 | mode, one byte per
// field. That is the wire form of every build, so a recording replays under
// dirac_mp as it does by default: repro_wire_default.go reads and writes the
// Canon as it is, repro_wire_mp.go maps the bytes to the four-seat class's
// refs and back. An mp game that names a ref no byte can hold (a seat past
// the second, an instance past 235) is written in the mp packing and says
// so in "packing"; only a dirac_mp build reads that one.

// reproWire is Repro's JSON: the struct itself with its actions as wire words.
type reproWire struct {
	reproPlain
	Actions []uint64 `json:"actions"`
	Packing string   `json:"packing,omitempty"`
}

// reproPlain is Repro without its methods, so encoding it does not recurse
// into MarshalJSON.
type reproPlain Repro

// MarshalJSON writes the actions in the build-independent wire form.
func (r Repro) MarshalJSON() ([]byte, error) {
	words, packing := wireActions(r.Actions)
	return json.Marshal(reproWire{reproPlain: reproPlain(r), Actions: words, Packing: packing})
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
	if format1 {
		p.Format = reproFormat
	}
	*r = p
	return nil
}
