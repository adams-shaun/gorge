package host

import (
	"fmt"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/protocol"
	"github.com/adams-shaun/gorge/state"
	"github.com/adams-shaun/gorge/view"
)

// This file is the server-side half of the feedback capture (task
// fbrepro1): when a player files a bug report naming a table, the report
// must carry enough to reproduce the match they were in — because the
// event log a deploy wipes is the one thing a later retelling loses. The
// shapes here are what Registry.SnapshotForFeedback hands the caller
// (cmd/gorged's feedback store), which marshals them beside the report.

// FeedbackMatch is match.json's content: the sidecar of the table's current
// match — the same fields host/persist.go persists, in the same shape —
// plus DeckCards, the deck CONTENTS (card name per card, in deck order,
// one list per seat). The sidecar's Decks names deck FILES, which a deploy
// can change under an old snapshot; DeckCards is what a replay is actually
// built from, so the snapshot never depends on the deck files still
// existing. The fields mirror sidecar's on purpose: a test pins that the
// two marshal the same keys, so the snapshot cannot drift from the
// persisted shape without failing.
type FeedbackMatch struct {
	Table        string              `json:"table"`
	Match        int                 `json:"match"`
	Seed         uint64              `json:"seed"`
	Seats        []protocol.SeatInfo `json:"seats"`
	Names        []string            `json:"names"`
	PlayerNames  []string            `json:"player_names,omitempty"`
	Decks        []string            `json:"decks"`
	DeckCards    [][]string          `json:"deck_cards"`
	Spectator    string              `json:"spectator"`
	State        string              `json:"state"`
	Result       string              `json:"result,omitempty"`
	Winner       *uint8              `json:"winner"`
	Head         string              `json:"head,omitempty"`
	Events       int                 `json:"events"`
	Turns        int32               `json:"turns"`
	Reason       string              `json:"reason,omitempty"`
	Mulligans    int                 `json:"mulligans,omitempty"`
	Format       Format              `json:"format,omitempty"`
	StartingLife int32               `json:"starting_life,omitempty"`
	Commanders   [][]int             `json:"commanders,omitempty"`
}

// FeedbackLog is log.json's content: the match's events.Log as of the
// snapshot — Seed, Events and Intents, the exact replay input — plus where
// that prefix stood: Head (the chain hash at that prefix, what a replay
// must reproduce), IntentCount, and the game's Turn, Step and
// Priority/Active players read off the same locked instant the log was
// copied. The Log is embedded, so its own keys (seed, events, intents)
// marshal at the top level and a caller unmarshals straight back into an
// events.Log.
type FeedbackLog struct {
	events.Log
	Head        string         `json:"head"`
	IntentCount int            `json:"intent_count"`
	Turn        int32          `json:"turn"`
	Step        string         `json:"step"`
	Priority    state.PlayerID `json:"priority"`
	Active      state.PlayerID `json:"active"`
}

// FeedbackSnapshot is everything a feedback report captures about the
// table's match: the replayable configuration (Match), the event log as of
// the submit (Log), and — when the report named a seat — that seat's own
// redacted view of the board (View). View is nil when the report named no
// seat, or when the projection failed (ViewErr says why): a failed view
// never discards the match and log, which are the part a reproduction
// needs, and it can never carry another seat's hidden zones — View is
// projected with seat (viewer-scoped) visibility, the same redaction
// ViewAtSeat serves the player's own client.
type FeedbackSnapshot struct {
	Match   FeedbackMatch
	Log     FeedbackLog
	View    *view.View
	ViewErr string
}

// SnapshotForFeedback captures the state of table id's CURRENT match — the
// live one, or, when the table is between matches, the one it just
// finished, so a report filed moments after game over still snapshots — as
// of one consistent instant. The match's read lock is held exactly long
// enough to copy what the snapshot needs (the log via Log.Clone, the
// sidecar, the deck contents, the game facts, the pending decision), and
// the seat's view is projected OUTSIDE the lock from those copies, on the
// replayed engine, exactly as ViewAtSeat does — a snapshot therefore never
// blocks a concurrent Submit for longer than the copies take, and never
// drives the live engine.
//
// seat is the reporting seat's own seat, or nil when the report named
// none. A named seat out of range is an error, not a nil view: a feedback
// caller should learn it asked for a seat the table does not have.
//
// The returned snapshot is a stable copy: later bursts append to the live
// match but never to it, so marshalling it needs no lock.
func (r *Registry) SnapshotForFeedback(id TableID, seat *state.PlayerID) (FeedbackSnapshot, error) {
	r.mu.RLock()
	t, ok := r.tables[id]
	r.mu.RUnlock()
	if !ok {
		return FeedbackSnapshot{}, ErrNotFound
	}
	t.mu.RLock()
	m := t.cur
	if m == nil && len(t.history) > 0 {
		m = t.history[len(t.history)-1]
	}
	t.mu.RUnlock()
	if m == nil {
		return FeedbackSnapshot{}, fmt.Errorf("host: table %s has no match", id)
	}

	m.mu.RLock()
	l := m.e.L.Clone()
	// Reconcile the copy to the consistent prefix a replay can rebuild
	// (persist.go's reconcileLog): a no-op for a healthy match — every burst
	// is complete under the lock — but it also trims a crashed match's
	// orphan tail and poison intent (D15), so a report filed at a crashed
	// table still snapshots a log that replays.
	reconcileLog(l)
	sc := m.sidecar()
	deckCards := make([][]string, len(m.cfg.Decks))
	for i, d := range m.cfg.Decks {
		names := make([]string, len(d))
		for j, c := range d {
			names[j] = c.Faces[0].Name
		}
		deckCards[i] = names
	}
	// Game facts as of the same instant the log was copied: the log's tail
	// burst is complete and the game state is the state those events left,
	// so Turn/Step/Priority/Active describe exactly the prefix recorded
	// below.
	turn, step, prio, active := m.e.G.Turn, m.e.G.Step.String(), m.e.G.Priority, m.e.G.Active
	if seat != nil && int(*seat) >= len(sc.Seats) {
		n := len(sc.Seats)
		m.mu.RUnlock()
		return FeedbackSnapshot{}, fmt.Errorf("host: table %s: seat %d out of range (%d seats)", id, *seat, n)
	}
	// The pending decision, copied the way ViewAtSeat copies it: the view
	// carries the decision only when the projection lands exactly at the
	// live head and the engine is parked on one — a historical prefix never
	// misrepresents what was pending.
	var d *decision.Decision
	if p := m.e.Pending(); p != nil {
		cp := *p
		cp.Options = append([]decision.Option(nil), p.Options...)
		d = &cp
	}
	snaps := append([]snapshot(nil), m.snaps...)
	cfg := m.cfg
	headSeq := head(m)
	m.mu.RUnlock()

	snap := FeedbackSnapshot{
		Match: feedbackMatch(sc, deckCards),
		Log: FeedbackLog{
			Log:         *l,
			Head:        l.Head(),
			IntentCount: len(l.Intents),
			Turn:        turn,
			Step:        step,
			Priority:    prio,
			Active:      active,
		},
	}
	if seat != nil {
		v, err := viewAt(cfg, l, snaps, headSeq, *seat, view.Seat, d)
		if err != nil {
			snap.ViewErr = err.Error()
		} else {
			snap.View = &v
		}
	}
	return snap, nil
}

// feedbackMatch copies the sidecar's fields into the exported FeedbackMatch
// and attaches the deck contents.
func feedbackMatch(sc sidecar, deckCards [][]string) FeedbackMatch {
	return FeedbackMatch{
		Table: sc.Table, Match: sc.Match, Seed: sc.Seed, Seats: sc.Seats, Names: sc.Names,
		PlayerNames: sc.PlayerNames, Decks: sc.Decks, DeckCards: deckCards, Spectator: sc.Spectator,
		State: sc.State, Result: sc.Result, Winner: sc.Winner, Head: sc.Head, Events: sc.Events,
		Turns: sc.Turns, Reason: sc.Reason, Mulligans: sc.Mulligans, Format: sc.Format,
		StartingLife: sc.StartingLife, Commanders: sc.Commanders,
	}
}
