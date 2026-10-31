package host

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/game/gorgegame"
	"github.com/adams-shaun/gorge/engine/replay"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/protocol"
)

// ReviewMatch is one finished (or cut-off) recorded game a review registry
// serves read-only: the replay input (Config, Log) plus display metadata.
// It carries engine-tier values only, so any loader -- a feedback snapshot
// directory, a match record -- can produce one without the host knowing
// the file format.
//
//   - Config and Log are the replay contract (engine/replay: Config and Log
//     together, never the Log alone). Config's Seed is overwritten by the
//     Log's, exactly as replay does.
//   - Names are the per-seat display names (protocol.SeatInfo.Name); a
//     missing or empty entry falls back to Config.PlayerNames, then to
//     "Player N".
//   - Policies are the per-seat policy names that played, for display only;
//     they never build a seat (a review table has none).
//   - Label is the table's display name; empty uses the table id.
//   - Explain is the record's per-intent explain sidecar
//     (matchrecord.Record.Explain): entry i is a bots/explain.Note's JSON
//     for Log.Intents[i], or JSON null. The host treats each entry as
//     opaque bytes and serves it on the matching decision entry
//     (ReviewDecisions); it is never replay input. A slice whose length is
//     not the intent count is ignored (the loader warns and drops it).
type ReviewMatch struct {
	Config   rules.Config
	Log      *events.Log
	Names    []string
	Policies []string
	Label    string
	Explain  []json.RawMessage
}

// AddReviewTable registers a read-only review table (match reviewer, design
// D9): one recorded match, no slots, no run loop, never started, served to
// spectators with omniscient visibility so the DVR's finished-match route
// (/t/{id}/m/1) fans every hand face up.
//
// It is a separate entry point from AddTable and touches nothing a live
// table uses: the match is rebuilt here, once, at registration. The input is
// gated on replay.Replay exactly as matchForLog gates an archived match -- a
// log that does not reproduce its own event stream and chain head under
// m.Config is refused with the replay error, before anything is registered.
// The load then walks the log once more (replay.Walk) to clone the engine at
// genesis and at every boundary whose burst began a turn, the same
// turn-start snapshots a live match's afterBurst takes, so viewAt serves any
// seq by replaying at most one turn. The same walk builds the decision index
// ReviewDecisions serves: at boundary i the engine's pending decision is
// exactly the one Log.Intents[i] answered (design D8).
//
// Review tables live only in a memory-mode registry (Options.Dir == ""): a
// persisted registry would write them into tables.json and a restart would
// try to play them, so a registry with a persistence directory refuses.
func (r *Registry) AddReviewTable(id string, m ReviewMatch) error {
	tid := TableID(id)
	if id == "" {
		return fmt.Errorf("host: review table has no id")
	}
	if r.opts.Dir != "" {
		return fmt.Errorf("host: review table %s: a review registry keeps no persistence directory", id)
	}
	if m.Log == nil {
		return fmt.Errorf("host: review table %s: nil log", id)
	}
	n := len(m.Config.Decks)
	if n < 1 || n > 8 {
		return fmt.Errorf("host: review table %s: %d seats, want 1..8", id, n)
	}
	r.mu.RLock()
	_, dup := r.tables[tid]
	closed := r.closed
	r.mu.RUnlock()
	if closed {
		return fmt.Errorf("host: registry is closed")
	}
	if dup {
		return fmt.Errorf("host: table %s already exists", id)
	}

	cfg := m.Config
	cfg.Seed = m.Log.Seed // replay's Ruling P5: the log's seed is the match's
	e, err := replay.Replay(m.Log, cfg)
	if err != nil {
		return fmt.Errorf("host: review table %s does not replay: %w", id, err)
	}
	explain := m.Explain
	if len(explain) != len(m.Log.Intents) {
		explain = nil
	}
	snaps, decisions, err := reviewWalk(m.Log, cfg, explain)
	if err != nil {
		return fmt.Errorf("host: review table %s: %w", id, err)
	}

	label := m.Label
	if label == "" {
		label = id
	}
	tc := TableConfig{ID: tid, Name: label, Seats: n, Decks: append([]string(nil), cfg.Names...), Seed: cfg.Seed,
		BotPolicy: joinPolicies(m.Policies), SeatBotPolicies: append([]string(nil), m.Policies...),
		Spectator: view.Omniscient, Mulligans: cfg.Mulligans, StartingLife: cfg.StartingLife}
	if cfg.Format == rules.FormatCommander {
		tc.Format = FormatCommander
	}
	t := newTable(r, tc)
	t.review = true
	t.decisions = decisions
	t.k = 1

	seats := make([]protocol.SeatInfo, n)
	for i := range seats {
		name := fmt.Sprintf("Player %d", i+1)
		if i < len(cfg.PlayerNames) && cfg.PlayerNames[i] != "" {
			name = cfg.PlayerNames[i]
		}
		if i < len(m.Names) && m.Names[i] != "" {
			name = m.Names[i]
		}
		deck := ""
		if i < len(cfg.Names) {
			deck = cfg.Names[i]
		}
		seats[i] = protocol.SeatInfo{Name: name, Deck: deck, Colour: protocol.SeatColours[i%len(protocol.SeatColours)]}
	}
	mt := &match{table: t, k: 1, seed: cfg.Seed, cfg: cfg, seats: seats, decks: tc.Decks,
		bounds: boundsOf(e.L.Events), turnStarts: turnStartsIn(e.L.Events, 0), snaps: snaps,
		intents: len(e.L.Intents), head: e.L.Head()}
	mt.setGame(gorgegame.Game{E: e})
	// A recording that ends before game over (a feedback capture taken
	// mid-game) is served as aborted: nothing will ever extend it.
	mt.state = protocol.MatchAborted
	if e.G.Over {
		mt.state = protocol.MatchFinished
		if e.G.Draw {
			mt.result = "draw"
		} else {
			mt.result = "win"
			w := uint8(e.G.Winner)
			mt.winner = &w
		}
	}
	// The match sits in history, the memory-mode home of a finished match
	// with its engine, so lookup, Matches, ViewAt and Events serve it
	// through their existing paths with no review branch.
	t.history = []*match{mt}

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return fmt.Errorf("host: registry is closed")
	}
	if _, dup := r.tables[tid]; dup {
		return fmt.Errorf("host: table %s already exists", id)
	}
	r.tables[tid] = t
	return nil
}

// reviewWalk walks (l, cfg) once. It clones the engine at genesis (intent
// 0) and at every intent boundary whose preceding burst contained a
// TurnChange: exactly the boundaries a live match snapshots
// (snapshotGenesis and afterBurst), so viewAt finds the same structure it
// finds on a live match. At every boundary before the last it also indexes
// the pending decision (reviewDecisionIndex.at), with explain[i] (nil or
// one entry per intent) riding on entry i.
func reviewWalk(l *events.Log, cfg rules.Config, explain []json.RawMessage) ([]snapshot, []protocol.ReviewDecision, error) {
	var snaps []snapshot
	ix := newReviewDecisionIndex(l, explain)
	prev := 0
	_, err := replay.Walk(l, cfg, len(l.Intents), func(e *rules.Engine, i int) error {
		n := len(e.L.Events)
		if n == 0 {
			return nil
		}
		if i == 0 || len(turnStartsIn(e.L.Events[:n], prev)) > 0 {
			snaps = append(snaps, snapshot{intent: i, seq: uint64(n) - 1, g: gorgegame.Game{E: e.Clone()}})
		}
		prev = n
		if i < len(l.Intents) {
			return ix.at(e, i)
		}
		return nil
	})
	if err != nil {
		return nil, nil, fmt.Errorf("snapshot walk: %w", err)
	}
	return snaps, ix.out, nil
}

// joinPolicies is the table-level policy label: the distinct per-seat
// policies in seat order, comma-joined ("" when none was recorded).
func joinPolicies(ps []string) string {
	var out []string
	for _, p := range ps {
		if p == "" {
			continue
		}
		dup := false
		for _, q := range out {
			if q == p {
				dup = true
				break
			}
		}
		if !dup {
			out = append(out, p)
		}
	}
	return strings.Join(out, ",")
}
