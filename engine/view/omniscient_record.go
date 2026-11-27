package view

import (
	"errors"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// RecordCaller names who asks for an OmniscientRecord. The zero value is a
// playing seat, which is never entitled to one (CR 400.2 keeps library order
// hidden from every player), so a caller that forgets to say who it is gets
// the refusal, not the data.
type RecordCaller uint8

const (
	// RecordForSeat is a playing seat. ProduceOmniscientRecord refuses it.
	RecordForSeat RecordCaller = iota
	// RecordForTraining is a training-label producer (a search teacher writing
	// a training record).
	RecordForTraining
	// RecordForDiagnostics is a measurement run (a diagnostic feature set).
	RecordForDiagnostics
)

// ErrRecordForSeat is the refusal ProduceOmniscientRecord returns to a
// playing seat (or to any caller that is not a named non-seat purpose).
var ErrRecordForSeat = errors.New("view: an omniscient record is for training and diagnostics, never for a playing seat")

// OmniscientRecord is the full-information export a training label or a
// diagnostic reads and a playing seat never does: the Omniscient projection
// (every hand, face-down faces, the decision whoever it was asked of) plus
// every seat's library in its true order. It is the only place library order
// leaves the engine; no View carries it.
type OmniscientRecord struct {
	// View is ProjectFor(g, ch, viewer, Omniscient, d).
	View View
	// Libraries is indexed by seat; Libraries[s][0] is the top of seat s's
	// library (state.ZLibrary's own order).
	Libraries [][]CardView
}

// ProduceOmniscientRecord builds the record for viewer. It is gated on the
// caller: RecordForSeat (the zero value) and any unknown caller get
// ErrRecordForSeat and an empty record. The projections a seat receives
// (Seat, Public, Omniscient) are unchanged by it; this only reads g.
func ProduceOmniscientRecord(g *state.Game, ch Chars, viewer state.PlayerID, caller RecordCaller, d *decision.Decision) (OmniscientRecord, error) {
	if caller != RecordForTraining && caller != RecordForDiagnostics {
		return OmniscientRecord{}, ErrRecordForSeat
	}
	rec := OmniscientRecord{View: ProjectFor(g, ch, viewer, Omniscient, d)}
	if g == nil {
		return rec, nil
	}
	if sc, ok := ch.(interface {
		BeginDerivedReads()
		EndDerivedReads()
	}); ok {
		sc.BeginDerivedReads()
		defer sc.EndDerivedReads()
	}
	sp := scratchPool.Get().(*projScratch)
	defer scratchPool.Put(sp)
	pr := newProjector(g, ch, viewer, sp)
	rec.Libraries = make([][]CardView, len(g.Players))
	for i := range g.Players {
		p := g.Players[i].ID
		rec.Libraries[i] = pr.cardViews(nil, g.Zone(state.ZLibrary, p), false, p, true, nil)
	}
	return rec, nil
}
