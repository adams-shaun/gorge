package bench

import (
	"encoding/json"
	"fmt"
	"slices"

	"github.com/adams-shaun/gorge/bots/explain"
	"github.com/adams-shaun/gorge/bots/searchseat"
	"github.com/adams-shaun/gorge/engine/seat"
)

// explainTape is one recorded game's decision overlay (match-reviewer design
// §3): the per-intent bots/explain notes PlayGame writes into the record's
// Explain. It exists only while Hooks.Record is set; with Record nil
// PlayGame never builds one and every explain site is one nil check.
//
// Per decision: begin clears it; the deciding seat's note (an
// explain.Emitter the tape installed itself on) lands in cur; a driver
// route -- a stopped feed, a refused root, the refusal ladder -- overrides
// the note's Source (and Reason, Rung) but keeps the seat's Search and Net;
// commit files the note at the index the accepted intent was logged at.
//
// The sink is per seat and per game -- never the process-wide azmcts or
// searchseat Watch, since botbench plays games in parallel.
type explainTape struct {
	policies []string
	notes    []json.RawMessage
	cur      explain.Note
	has      bool
	// seat is the deciding seat, read when the decision is posed: the
	// engine recycles the pending decision once it is answered.
	seat int
	// frozen stops captures once the engine refused the seat's answer: the
	// note keeps the seat's own reasoning, and a re-ask (AnswerRefused) that
	// consults the seat again cannot overwrite it.
	frozen bool
	src    explain.Source
	reason string
	rung   int
	err    error
}

// newExplainTape builds the tape and installs its sink on every seat that
// explains itself. policies are the record's Producer.Policies (seat order),
// for the notes' Policy field.
func newExplainTape(seats []seat.Seat, policies []string) *explainTape {
	x := &explainTape{policies: policies}
	for _, s := range seats {
		if em, ok := s.(explain.Emitter); ok {
			em.SetExplain(x.capture)
		}
	}
	return x
}

// release uninstalls the tape's sink, so a seat that outlives the game (a
// caller reusing it) never writes into a finished tape.
func (x *explainTape) release(seats []seat.Seat) {
	for _, s := range seats {
		if em, ok := s.(explain.Emitter); ok {
			em.SetExplain(nil)
		}
	}
}

// capture is the seats' sink. The note is deep-copied: its slices may alias
// a seat's or the engine's scratch (an intent's Choices), which the next
// decision reuses before commit marshals it.
func (x *explainTape) capture(n explain.Note) {
	if x.frozen {
		return
	}
	n.Bot = slices.Clone(n.Bot)
	if n.Search != nil {
		sr := *n.Search
		sr.Rows = slices.Clone(sr.Rows)
		for i := range sr.Rows {
			sr.Rows[i].Choices = slices.Clone(sr.Rows[i].Choices)
		}
		n.Search = &sr
	}
	if n.Net != nil {
		nt := *n.Net
		nt.Scores = slices.Clone(nt.Scores)
		n.Net = &nt
	}
	x.cur, x.has = n, true
}

// begin starts a decision of seat seatIdx.
func (x *explainTape) begin(seatIdx int) {
	x.seat = seatIdx
	x.cur, x.has, x.frozen = explain.Note{}, false, false
	x.src, x.reason, x.rung = "", "", 0
}

// route records a driver source for the decision being answered (rung is
// RefusalFallback's, 0 otherwise).
func (x *explainTape) route(src explain.Source, reason string, rung int) {
	x.src, x.reason, x.rung = src, reason, rung
}

// envRoute tags the Env branch's driver route: a stopped feed (fd nil) or a
// refused honest root on a live feed. A built root is no route.
func (x *explainTape) envRoute(fd *searchseat.Feed, reason string) {
	switch {
	case fd == nil:
		x.route(explain.FallbackFeed, "feed stopped", 0)
	case reason != "":
		x.route(explain.FallbackRoot, reason, 0)
	}
}

// searchRoute tags the search branch's plain-bot fallback: a refused root
// is already routed; otherwise the feed stopped (or failed this capture).
func (x *explainTape) searchRoute(fd *searchseat.Feed, captured bool, rootRefused string) {
	if rootRefused != "" {
		return
	}
	reason := "feed stopped"
	if !captured && fd != nil && fd.StopReason() != "" {
		reason += ": " + fd.StopReason()
	}
	x.route(explain.FallbackFeed, reason, 0)
}

// refused marks the seat's answer as refused by the engine: the note is
// frozen as it stands, and the ladder's outcome (route) will override its
// Source.
func (x *explainTape) refused() { x.frozen = true }

// commit files the decision's note at intent index at, the index the
// accepted intent was logged at.
func (x *explainTape) commit(at int) {
	seatIdx := x.seat
	n := x.cur
	if !x.has {
		n = explain.Note{Source: explain.Heuristic}
	}
	n.Seat = seatIdx
	if n.Policy == "" && seatIdx < len(x.policies) {
		n.Policy = x.policies[seatIdx]
	}
	if x.src != "" {
		n.Source, n.Rung = x.src, x.rung
		if x.reason != "" {
			n.Reason = x.reason
		}
	}
	raw, err := json.Marshal(n)
	if err != nil && x.err == nil {
		x.err = err
	}
	for len(x.notes) <= at {
		x.notes = append(x.notes, nil)
	}
	x.notes[at] = raw
}

// final is the record's Explain for a log of intents entries: one note per
// intent, in log order.
func (x *explainTape) final(intents int) ([]json.RawMessage, error) {
	if x == nil {
		return nil, nil
	}
	if x.err != nil {
		return nil, x.err
	}
	if len(x.notes) != intents {
		// An intent the drive loop did not answer (none is expected with
		// Record set) would shift every later note off its intent.
		return nil, fmt.Errorf("explain: %d intents logged, %d answered by the drive loop; the notes cannot be aligned", intents, len(x.notes))
	}
	return x.notes, nil
}
