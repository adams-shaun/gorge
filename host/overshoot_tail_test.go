package host

// fb-20260915T094418Z: a parked mid-burst decision made every view request
// at head return HTTP 500. The root cause was the host's burst-boundary
// model: a burst does not always end at its last DecisionAsk — Engine.
// Submit's post-ask continuation (the SBA pass and the step handler the
// ask interrupted) can still emit events after the ask that parks the
// decision — and boundsOf/viewAt treated the last ask as the end of the
// log. These tests pin the fixed model end to end on a deterministic
// commander fixture that reaches the reported shape: a burst whose SBA
// pass asks the CR 903.9 commander_zone replacement decision and then
// keeps sweeping lethal-damage creatures past it.
//
// The fixture is measured, not constructed: decks foundations-wretched-
// ranks vs foundations-reign-of-dragons at seed 1020 (match k=1, the slot
// takeMatchSlot reserves), burst 538 — the resolving spell deals 5 damage
// to four creatures in one SBA pass, sweeps three of them ("lethal
// damage"), asks seat 1's commander_zone, and one more "lethal damage"
// sweep lands AFTER the ask. The park lands on that ask; replaying the
// recorded intents reproduces the tail byte for byte. The shape is
// re-derived from the live match below, so a corpus or engine move fails
// here loudly instead of testing nothing.

import (
	"context"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/botpolicy"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/protocol"
	"github.com/adams-shaun/gorge/replay"
	"github.com/adams-shaun/gorge/seat"
	"github.com/adams-shaun/gorge/state"
	"github.com/adams-shaun/gorge/view"
)

const (
	overshootDeckA   = "foundations-wretched-ranks"
	overshootDeckB   = "foundations-reign-of-dragons"
	overshootSeed    = 1020
	overshootIntents = 538 // intents recorded when parked on the overshoot burst's pending ask
)

// gateSeat is a bot behind a test gate: every decision is signalled to the
// test, which releases it one at a time, so the match can be held at a
// known point. Same shape as cmd/repro's fixture generator uses. bot must
// be exactly the one defaultSeats would have built for the slot (seed ^
// slot+1), so the gated match replays the pure-bot game's prefix.
type gateSeat struct {
	bot     seat.Seat
	reached chan struct{}
	release chan struct{}
}

func (g *gateSeat) Decide(ctx context.Context, v view.View, d decision.Decision) (decision.Intent, error) {
	select {
	case g.reached <- struct{}{}:
	case <-ctx.Done():
		return decision.Intent{}, ctx.Err()
	}
	select {
	case <-g.release:
	case <-ctx.Done():
		return decision.Intent{}, ctx.Err()
	}
	return g.bot.Decide(ctx, v, d)
}

// parkedOvershootMatch builds a two-seat commander table with both seats
// gated, drives it to the measured overshoot burst, and holds it there:
// parked on the burst's pending commander_zone ask, with the burst's
// post-ask tail already on the log. A non-empty dir runs the match with
// persistence on (the archived-load tests); empty is memory mode.
func parkedOvershootMatch(t *testing.T, dir string) (*Registry, *table, *match, [2]*gateSeat) {
	t.Helper()
	takeMatchSlot(t)
	gates := [2]*gateSeat{
		{reached: make(chan struct{}), release: make(chan struct{})},
		{reached: make(chan struct{}), release: make(chan struct{})},
	}
	o := Options{
		LoadDeck: commanderDeckLoader(t),
		Seats: func(names []string, seed uint64) []seat.Seat {
			// defaultSeats' exact bots: seed ^ slot+1. Written from the
			// match goroutine before the first park signals; the test never
			// reads bot, so there is no cross-goroutine read to race.
			gates[0].bot = seat.NewBot(seed ^ 1)
			gates[1].bot = seat.NewBot(seed ^ 2)
			return []seat.Seat{gates[0], gates[1]}
		},
		Sleep: func(time.Duration, <-chan struct{}) {},
	}
	if dir != "" {
		o.Dir = dir
	}
	r, err := New(o)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close() })
	if err := r.AddTable(TableConfig{ID: "t1", Name: "Table t1", Seats: 2,
		Decks: []string{overshootDeckA, overshootDeckB}, Seed: overshootSeed,
		Pace: 0, Spectator: view.Omniscient, Format: FormatCommander}); err != nil {
		t.Fatal(err)
	}
	if err := r.Start("t1"); err != nil {
		t.Fatal(err)
	}
	r.mu.RLock()
	tb := r.tables["t1"]
	r.mu.RUnlock()
	// Drive one gated decision at a time until intents == overshootIntents,
	// then hold: the pending park at that count is the overshoot burst's
	// own ask. The signal received when the count already names it is that
	// park (the submit that produced the count runs before its park), so
	// holding there parks the match exactly on the reported decision.
	for {
		select {
		case <-gates[0].reached:
		case <-gates[1].reached:
		case <-time.After(30 * time.Second):
			t.Fatal("the gated match never reached the overshoot burst")
		}
		tb.mu.RLock()
		m := tb.cur
		tb.mu.RUnlock()
		m.mu.RLock()
		n := m.intents
		m.mu.RUnlock()
		if n >= overshootIntents {
			return r, tb, m, gates
		}
		select {
		case gates[0].release <- struct{}{}:
		case gates[1].release <- struct{}{}:
		case <-time.After(30 * time.Second):
			t.Fatal("the gated match never accepted a release")
		}
	}
}

// assertOvershootShape re-derives the measured shape from the parked match,
// so a corpus or engine move that shifts it fails here with a name instead
// of silently exercising something else. Returns the parked ask's seq and
// the burst's recorded end (one past the log's last event).
func assertOvershootShape(t *testing.T, m *match) (askSeq uint64, burstEnd uint64) {
	t.Helper()
	if got := m.intents; got != overshootIntents {
		t.Fatalf("fixture parked at %d intents, want %d — the measured shape moved", got, overshootIntents)
	}
	if len(m.bounds) != overshootIntents+1 {
		t.Fatalf("fixture has %d burst boundaries, want %d", len(m.bounds), overshootIntents+1)
	}
	lo, hi := m.bounds[overshootIntents-1], m.bounds[overshootIntents]
	damages, lastAsk := 0, uint64(0)
	for s := lo; s < hi; s++ {
		switch m.e.L.Events[s].Kind {
		case events.Damage:
			damages++
		case events.DecisionAsk:
			lastAsk = s
		}
	}
	if lastAsk == 0 || hi <= lastAsk+1 {
		t.Fatalf("burst %d carries no overshoot tail (last ask %d, burst end %d) — the measured shape moved",
			overshootIntents, lastAsk, hi)
	}
	ask := m.e.L.Events[lastAsk]
	if ask.Text != "commander_zone" || ask.Player != 1 {
		t.Fatalf("overshoot ask at %d is %q for seat %d, want commander_zone for seat 1 — the measured shape moved",
			lastAsk, ask.Text, ask.Player)
	}
	lethal := 0
	for s := lastAsk + 1; s < hi; s++ {
		if ev := m.e.L.Events[s]; ev.Kind == events.MoveZone && ev.Text == "lethal damage" {
			lethal++
		}
	}
	if lethal == 0 {
		t.Fatal("no lethal-damage sweep landed after the ask — the measured shape moved")
	}
	if damages < 2 {
		t.Fatalf("burst carries %d damage events, want the shared-lethal pass's several", damages)
	}
	d := m.e.Pending()
	if d == nil || d.Kind != decision.KCommanderZone || d.Player != 1 {
		t.Fatalf("parked pending is %+v, want seat 1's commander_zone — the measured shape moved", d)
	}
	return lastAsk, hi
}

// TestParkedOvershootViewAtHeadServesTheDecision is the reported defect's
// regression: at head — and at every seq of the parked burst's tail, which
// the old model had no boundary for — the seat view must serve, and at head
// it must carry the pending commander_zone decision with its two options,
// because the decision reaches the client only through the seat-scoped view.
func TestParkedOvershootViewAtHeadServesTheDecision(t *testing.T) {
	t.Parallel()
	r, _, m, _ := parkedOvershootMatch(t, "")
	askSeq, burstEnd := assertOvershootShape(t, m)
	head := head(m)
	if head != burstEnd-1 {
		t.Fatalf("head %d, want the burst's recorded end %d", head, burstEnd-1)
	}

	// The seat's own view at head: the fetch that used to 500.
	v, err := r.ViewAtSeat("t1", m.k, head, 1)
	if err != nil {
		t.Fatalf("ViewAtSeat(head) failed: %v", err)
	}
	if v.Decision == nil || v.Decision.Kind != decision.KCommanderZone {
		t.Fatalf("head view carries %+v, want the commander_zone decision", v.Decision)
	}
	if len(v.Decision.Options) != 2 {
		t.Fatalf("commander_zone offers %d options, want 2", len(v.Decision.Options))
	}
	if v.Decision.Options[0].Kind != "command_zone" || v.Decision.Options[1].Kind != "leave" {
		t.Fatalf("commander_zone options are %q then %q, want command_zone then leave",
			v.Decision.Options[0].Kind, v.Decision.Options[1].Kind)
	}
	// The other seat's view at head serves too — it just carries no decision.
	other, err := r.ViewAtSeat("t1", m.k, head, 0)
	if err != nil {
		t.Fatalf("ViewAtSeat(head) for seat 0 failed: %v", err)
	}
	if other.Decision != nil {
		t.Fatalf("seat 0's view carries seat 1's decision")
	}

	// The ask's own seq and every seq of the tail: served. The decision
	// attaches only at head — a historical view never misrepresents what
	// was pending; the tail seqs' projection is the burst's end state, the
	// tolerance viewAt documents for the shape.
	for seq := askSeq; seq < burstEnd; seq++ {
		got, err := r.ViewAtSeat("t1", m.k, seq, 1)
		if err != nil {
			t.Fatalf("ViewAtSeat(%d) failed: %v", seq, err)
		}
		if seq != head && got.Decision != nil {
			t.Fatalf("ViewAtSeat(%d) carries a decision; only head does", seq)
		}
	}

	// The spectator board at head serves as well.
	if _, err := r.ViewAt("t1", m.k, head); err != nil {
		t.Fatalf("ViewAt(head) failed: %v", err)
	}

	// Seqs below the burst behave exactly as before: the previous burst's
	// last seq and its interior are served by the ordinary boundary path.
	prev := m.bounds[overshootIntents-1] - 1
	for _, seq := range []uint64{prev, prev - 1} {
		if _, err := r.ViewAtSeat("t1", m.k, seq, 1); err != nil {
			t.Fatalf("ViewAtSeat(%d) below the burst failed: %v", seq, err)
		}
	}

	// The two read paths (snapshot replay and from-genesis replay) must
	// agree at the tail seqs — the genesis path carries ReplayTo's own
	// byte-compare, the snapshot path viewAt's tail verification.
	for _, seq := range []uint64{askSeq, head} {
		fast, err := viewAt(m.cfg, m.e.L, m.snaps, seq, view.NoSeat, view.Omniscient, nil)
		if err != nil {
			t.Fatalf("viewAt(%d) snapshot path: %v", seq, err)
		}
		slow, err := viewAt(m.cfg, m.e.L, nil, seq, view.NoSeat, view.Omniscient, nil)
		if err != nil {
			t.Fatalf("viewAt(%d) genesis path: %v", seq, err)
		}
		if a, b := viewJSON(t, fast), viewJSON(t, slow); a != b {
			t.Fatalf("seq %d: snapshot path differs from the genesis replay at the overshoot tail", seq)
		}
	}
}

// TestParkedOvershootSnapshotForFeedbackIsComplete pins the feedback half:
// the capture must carry the seat view (the old capture lost it —
// "view unavailable: seq beyond head"), a log whose tail past the last ask
// is present, and a log that replays byte-exactly to its own recorded head.
func TestParkedOvershootSnapshotForFeedbackIsComplete(t *testing.T) {
	t.Parallel()
	r, _, m, _ := parkedOvershootMatch(t, "")
	askSeq, burstEnd := assertOvershootShape(t, m)
	seat1 := state.PlayerID(1)
	snap, err := r.SnapshotForFeedback("t1", &seat1)
	if err != nil {
		t.Fatalf("SnapshotForFeedback: %v", err)
	}
	if snap.ViewErr != "" {
		t.Fatalf("snapshot lost the seat view: %s", snap.ViewErr)
	}
	if snap.View == nil {
		t.Fatal("snapshot carries no view")
	}
	if snap.View.Decision == nil || snap.View.Decision.Kind != decision.KCommanderZone || len(snap.View.Decision.Options) != 2 {
		t.Fatalf("snapshot view carries %+v, want the two-option commander_zone decision", snap.View.Decision)
	}
	if got := len(snap.Log.Events); got != int(burstEnd) {
		t.Fatalf("captured log holds %d events, want the full %d — the tail must not be trimmed", got, burstEnd)
	}
	if snap.Log.Events[askSeq].Text != "commander_zone" {
		t.Fatalf("captured ask at %d is %q", askSeq, snap.Log.Events[askSeq].Text)
	}
	// The head is the chain over the captured events, and the log replays
	// byte-exactly to it — the contract cmd/repro verifies a capture with.
	m.mu.RLock()
	cfg := m.cfg
	m.mu.RUnlock()
	if want := snap.Log.HeadAt(len(snap.Log.Events)); snap.Log.Head != want {
		t.Fatalf("captured head %q, chain over its own events %q", snap.Log.Head, want)
	}
	e, err := replay.Replay(&snap.Log.Log, cfg)
	if err != nil {
		t.Fatalf("captured log does not replay: %v", err)
	}
	if len(e.L.Events) != len(snap.Log.Events) {
		t.Fatalf("replay produced %d events, the capture has %d", len(e.L.Events), len(snap.Log.Events))
	}
	if got := e.L.Head(); got != snap.Log.Head {
		t.Fatalf("replayed head %s, captured %s", got, snap.Log.Head)
	}
}

// TestUndoRewindsAcrossAnOvershootBurst pins the undo/rewind regression the
// brief calls out: rewindToLastIntent's rewind point is m.bounds — recorded
// by afterSubmit as the TRUE burst ends, overshoot tails included — so undo
// must keep working on a match whose history carries overshoot bursts, and
// resubmitting the undone intent must reproduce the overshoot burst byte
// for byte. The call is the internal one (not Registry.Undo) because the
// fixture's seats are gates, not HumanSeats; the rewind point arithmetic
// and the replay are the same code either way. The still-parked gate on the
// abandoned commander_zone ask is left blocked; the cleanup's Close cancels
// its ctx, exactly as the cmd/repro fixture generator tears its own park
// down.
func TestUndoRewindsAcrossAnOvershootBurst(t *testing.T) {
	t.Parallel()
	r, tb, m, _ := parkedOvershootMatch(t, "")
	assertOvershootShape(t, m)
	// The overshoot burst is intent overshootIntents-1's (537); the requester
	// is its own player.
	before := m.intents
	tailStart := m.bounds[before-1]
	saved := append([]events.Event(nil), m.e.L.Events[tailStart:]...)
	in := m.e.L.Intents[before-1]
	brd := botpolicy.NewBoard(2)
	var data *parkedData
	err := m.locked(func() error {
		var err error
		data, err = r.rewindToLastIntent(tb, m, m.slots, &brd, in.Player)
		return err
	})
	if err != nil {
		t.Fatalf("rewind across the overshoot burst: %v", err)
	}
	if m.intents != before-1 || len(m.bounds) != before {
		t.Fatalf("after rewind: %d intents, %d bounds; want %d and %d", m.intents, len(m.bounds), before-1, before)
	}
	if got := len(m.e.L.Events); got != int(tailStart) {
		t.Fatalf("rewound log holds %d events, want the undo boundary %d", got, tailStart)
	}
	if data == nil || data.dc.Player != in.Player {
		t.Fatalf("rewound pending is %+v, want seat %d's decision", data, in.Player)
	}
	// Resubmit the undone intent: the overshoot burst replays byte for byte.
	err = m.locked(func() error { return m.e.Submit(in) })
	if err != nil {
		t.Fatalf("resubmitting the undone intent: %v", err)
	}
	replayed := m.e.L.Events[tailStart:]
	if len(replayed) != len(saved) {
		t.Fatalf("resubmitted burst produced %d events, the original had %d", len(replayed), len(saved))
	}
	for i := range saved {
		if string(saved[i].Append(nil)) != string(replayed[i].Append(nil)) {
			t.Fatalf("resubmitted burst event %d differs from the original", tailStart+uint64(i))
		}
	}
	// The seat view at the rewound head serves, decision attached at head.
	if _, err := r.ViewAtSeat("t1", m.k, head(m), in.Player); err != nil {
		t.Fatalf("ViewAtSeat(head) after rewind: %v", err)
	}
}

// TestArchivedParkedTailMatchLoadsAfterRestart pins defect 3: a match
// parked (here: ended by Close) on an overshoot burst persists the full
// tail, readLog's reconcileLog trims it to the last ask at load, and the
// replay of the recorded intents runs past the trimmed end — which used to
// make matchForLog reject the whole match ("does not replay") and leave
// every view of the archived match dead. The tolerance (the sidecar's
// chain-verified head) must load it, and the views must serve the tail.
func TestArchivedParkedTailMatchLoadsAfterRestart(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r, _, m, _ := parkedOvershootMatch(t, dir)
	askSeq, burstEnd := assertOvershootShape(t, m)
	k := m.k
	r.Close() // the restart's kill: the live match gets its terminal transition

	r2, err := New(Options{
		LoadDeck: commanderDeckLoader(t),
		Sleep:    func(time.Duration, <-chan struct{}) {},
		Dir:      dir,
	})
	if err != nil {
		t.Fatalf("restarting on %s: %v", dir, err)
	}
	t.Cleanup(func() { r2.Close() })
	// The persisted stream is the full burst (tail included); the load
	// path trims and replays past the trim — the tolerated shape.
	sc, ok := r2.tables["t1"].archivedMatch(k)
	if !ok {
		t.Fatal("the restarted registry does not know the match")
	}
	if got := sc.Events; got != int(burstEnd) {
		t.Fatalf("sidecar records %d events, want the persisted %d", got, burstEnd)
	}
	if sc.Head == "" {
		t.Fatal("sidecar carries no head to verify the tail with")
	}
	head := uint64(sc.Events - 1)
	// The archived match's board serves — at head and inside the tail.
	for _, seq := range []uint64{askSeq, head} {
		if _, err := r2.ViewAt("t1", k, seq); err != nil {
			t.Fatalf("archived ViewAt(%d): %v", seq, err)
		}
		if _, err := r2.ViewAtSeat("t1", k, seq, 1); err != nil {
			t.Fatalf("archived ViewAtSeat(%d): %v", seq, err)
		}
	}
	// And the events endpoint serves the tail: the post-ask sweep is there.
	bodies, err := r2.Events("t1", k, askSeq)
	if err != nil {
		t.Fatalf("archived Events: %v", err)
	}
	found := false
	for _, b := range bodies {
		if b.Event.Kind == "move_zone" && b.Event.Text == "lethal damage" {
			found = true
		}
	}
	if !found {
		t.Fatal("the archived match's events lost the post-ask lethal-damage sweep")
	}
}

// TestCrashedMatchFeedbackCaptureStillTrims pins the other half of the
// feedback change: the reconcile is skipped only for a match whose log is
// provably complete under the read lock (every non-crashed state). A
// CRASHED match's log can carry an orphan tail and poison intent (D15),
// and its capture must still come back trimmed to the replayable prefix —
// the same reconcileLog the file-path tests pin. The state is forced on
// the parked fixture because no seat can make the ENGINE panic mid-Submit
// from a test; the conditional reads m.state and nothing else.
func TestCrashedMatchFeedbackCaptureStillTrims(t *testing.T) {
	t.Parallel()
	r, _, m, _ := parkedOvershootMatch(t, "")
	_, burstEnd := assertOvershootShape(t, m)
	seat1 := state.PlayerID(1)
	m.mu.Lock()
	m.state = protocol.MatchCrashed
	m.mu.Unlock()
	snap, err := r.SnapshotForFeedback("t1", &seat1)
	if err != nil {
		t.Fatalf("SnapshotForFeedback: %v", err)
	}
	if got := len(snap.Log.Events); got >= int(burstEnd) {
		t.Fatalf("crashed capture holds %d events, want the trimmed prefix (below the burst end %d)", got, burstEnd)
	}
	last := snap.Log.Events[len(snap.Log.Events)-1]
	if last.Kind != events.DecisionAsk {
		t.Fatalf("crashed capture ends with %s, want the DecisionAsk boundary", last.Kind)
	}
}
