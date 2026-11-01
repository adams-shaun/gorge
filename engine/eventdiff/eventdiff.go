// Package eventdiff compares two event streams and names the first event where
// they part ways. It depends only on engine/events, so engine/replay and any
// engine that cannot import engine/rules report divergences with one type.
package eventdiff

import (
	"fmt"

	"github.com/adams-shaun/gorge/engine/events"
)

// Divergence is the first event where a rebuilt stream differs from the
// recorded one.
type Divergence struct {
	// Seq is the index of the first event that differs. A stored event's Seq
	// equals its slice index, so this also indexes both event slices.
	Seq uint64
	// Want and Got are the recorded and rebuilt events at Seq. Want is
	// meaningless when Missing is true and Got when Short is true; a zero
	// events.Event reads as a real GameStart, so test those fields instead.
	Want, Got events.Event
	// Missing means the rebuilt stream has an event at Seq and the recording
	// has ended.
	Missing bool
	// Short means every rebuilt event matched and the recording continues
	// past the rebuilt stream's end.
	Short bool
}

// Error names the first differing event.
func (d *Divergence) Error() string {
	if d.Missing {
		return fmt.Sprintf("replay diverged at event %d: recorded log ends at event %d; replay produced %s",
			d.Seq, d.Seq, d.Got.Kind)
	}
	if d.Short {
		return fmt.Sprintf("replay diverged at event %d: replay ended there; the recorded log continues with %s",
			d.Seq, d.Want.Kind)
	}
	return fmt.Sprintf("replay diverged at event %d: recorded %s, replayed %s",
		d.Seq, d.Want.Kind, d.Got.Kind)
}

// First compares two complete streams, want the reference and got the one
// under test, byte for byte through Event.Append, the encoding the log's hash
// chain folds. It returns nil when they are identical. A got that runs past
// want's end is Missing; a got that ends early with every event matching is
// Short.
func First(want, got []events.Event) *Divergence {
	n := min(len(want), len(got))
	for i := 0; i < n; i++ {
		if !Same(want[i], got[i]) {
			return &Divergence{Seq: uint64(i), Want: want[i], Got: got[i]}
		}
	}
	switch {
	case len(got) > n:
		return &Divergence{Seq: uint64(n), Missing: true, Got: got[n]}
	case len(want) > n:
		return &Divergence{Seq: uint64(n), Short: true, Want: want[n]}
	}
	return nil
}

// Same reports whether two events encode to the same bytes.
func Same(want, got events.Event) bool {
	return string(want.Append(nil)) == string(got.Append(nil))
}
