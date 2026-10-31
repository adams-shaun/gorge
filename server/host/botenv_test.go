package host

import (
	"context"
	"reflect"
	"testing"
	"weak"

	"github.com/adams-shaun/gorge/bots/heuristic"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/bots/searchprobe"
	"github.com/adams-shaun/gorge/bots/searchseat"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/replay"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
)

// spyEnvSeat is a non-human seat that satisfies bots.EnvSeat and records
// exactly what the host handed each DecideEnv call — the evidence BP-08's
// tests assert on. It plays the wrapped bot from env.Board, the same fallback
// every real adapter plays when the root is refused, so a match it seats runs
// identically to one driven through the plain Board path (that is what makes
// TestHostFeedEqualsRebuildFeed's live-vs-rebuilt comparison still hold: the
// Env changes nothing the engine can see).
// Embedding *heuristic.Bot promotes Decide and DecideBoard, so the seat plays
// deterministically.
type spyEnvSeat struct {
	*heuristic.Bot
	decided int
	envs    []envRecord
	// retainRoots keeps each record's root engine strongly reachable
	// (envRecord.engine), for the one test that compares two roots'
	// hidden worlds after the fact. Every other spy records only the
	// roots' identities (envRecord.engineID/gID): a whole-match spy that
	// retained every root held hundreds of full engines live until the
	// test ended -- 2.7 GB peak RSS for a single test, the bulk of the
	// host package's 7 GB peak.
	retainRoots bool
}

func (s *spyEnvSeat) WantsEnv(d *decision.Decision) bool { return true }

func (s *spyEnvSeat) DecideEnv(ctx context.Context, env bots.Env, d decision.Decision) (decision.Intent, error) {
	s.decided++
	rec := envRecord{
		seq: d.Seq, rootRef: env.RootRefused, feed: env.Search.Feed,
		viewRound: env.View.Round,
	}
	if e := env.Search.Engine; e != nil {
		rec.engineID, rec.gID = weak.Make(e), weak.Make(e.G)
		if s.retainRoots {
			rec.engine, rec.g = e, e.G
		}
		rec.actorHand = zoneNames(e.G.Zone(state.ZHand, 0), e)
		rec.oppHand = zoneNames(e.G.Zone(state.ZHand, 1), e)
		rec.actorLib = zoneNames(e.G.Zone(state.ZLibrary, 0), e)
		rec.oppLib = zoneNames(e.G.Zone(state.ZLibrary, 1), e)
	}
	in, err := s.Bot.DecideBoard(ctx, env.Board, d)
	rec.in = in
	s.envs = append(s.envs, rec)
	return in, err
}

// recorded returns the env record of the decision with the given seq, or nil.
func (s *spyEnvSeat) recorded(seq uint64) *envRecord {
	for i := range s.envs {
		if s.envs[i].seq == seq {
			return &s.envs[i]
		}
	}
	return nil
}

// matchFor returns a finished match's engine and feeds after r.Wait. It is the
// in-package hook the feed tests use to inspect match-owned state that no
// Registry method exposes (the feeds are intentionally unexported and never
// reach the wire, §5.2).
func matchFor(t *testing.T, r *Registry, id TableID) (*match, *rules.Engine) {
	t.Helper()
	r.mu.RLock()
	tbl, ok := r.tables[id]
	r.mu.RUnlock()
	if !ok {
		t.Fatalf("table %s not found", id)
	}
	tbl.mu.RLock()
	defer tbl.mu.RUnlock()
	if len(tbl.history) == 0 {
		t.Fatalf("table %s kept no in-memory match history", id)
	}
	m := tbl.history[len(tbl.history)-1]
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m, m.e
}

// TestHostFeedEqualsRebuildFeed is BP-07's central assertion: a live match's
// host-owned feed, after every intent, equals searchseat.RebuildFeed over the
// same log and actor. That equality is what makes the feed derived state the
// host can rebuild after an undo (BP-09) and never persist (§5.2).
//
// The table seats a spy EnvSeat at slot 0 (so the host creates a feed) and a
// plain bot at slot 1, and it must reach a real game — asserted by the
// precondition block — or a comparison of two empty feeds would pass
// vacuously.
func TestHostFeedEqualsRebuildFeed(t *testing.T) {
	t.Parallel()
	opts := testOptions(t)
	const seed = uint64(20260928)
	opts.Seats = func(names []string, gotSeed uint64) []seat.Seat {
		return []seat.Seat{
			&spyEnvSeat{Bot: heuristic.NewBot(gotSeed ^ 1)},
			heuristic.NewBot(gotSeed ^ 2),
		}
	}
	r, err := New(opts)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	cfg := TableConfig{ID: "t1", Name: "envfeeds", Seats: 2, Decks: []string{"a", "b"},
		Seed: seed, Pace: 0, Spectator: view.Public}
	if err := r.AddTable(cfg); err != nil {
		t.Fatal(err)
	}
	if err := r.Start("t1"); err != nil {
		t.Fatal(err)
	}
	r.Wait("t1")

	m, engine := matchFor(t, r, "t1")

	// Precondition: the host built a feed for slot 0 and only slot 0, and the
	// game actually ran. Without this a mis-wired feed and a 0-intent game
	// would both compare equal and pass.
	if m.feeds == nil {
		t.Fatal("no feeds built for a table with an EnvSeat: Create is not wired")
	}
	if len(m.feeds.bySlot) != 2 {
		t.Fatalf("feeds slots = %d, want 2", len(m.feeds.bySlot))
	}
	if m.feeds.bySlot[0] == nil {
		t.Fatal("slot 0 (spy EnvSeat) has no feed")
	}
	if m.feeds.bySlot[1] != nil {
		t.Fatal("slot 1 (plain bot) has a feed; only EnvSeats must get one")
	}
	live := m.feeds.bySlot[0]
	if !live.Live() {
		t.Fatalf("live feed stopped: %s", live.StopReason())
	}
	if n := len(engine.L.Intents); n < 20 {
		t.Fatalf("match logged only %d intents; too short to compare feeds", n)
	}
	if live.Frames() < 20 {
		t.Fatalf("live feed captured only %d frames; Observe is not wired", live.Frames())
	}

	actor := state.PlayerID(0)
	rebuilt, err := searchseat.RebuildFeed(m.rulesConfig(), engine.L, len(engine.L.Intents), actor)
	if err != nil {
		t.Fatalf("RebuildFeed: %v", err)
	}
	if !reflect.DeepEqual(live.History(), rebuilt.History()) {
		lh, rh := live.History(), rebuilt.History()
		t.Fatalf("live feed != RebuildFeed(final): frames %d/%d answers %d/%d actors %d/%d",
			len(lh.Frames), len(rh.Frames), len(lh.Answers), len(rh.Answers), lh.Actor, rh.Actor)
	}
	if !reflect.DeepEqual(live.Collector().Clone(), rebuilt.Collector().Clone()) {
		t.Fatal("live feed collector state != RebuildFeed(final) collector state")
	}

	// The same equality must hold at every prefix, not only at the end: the
	// live feed truncated to boundary n is RebuildFeed(n).
	for _, n := range []int{1, len(engine.L.Intents) / 2, len(engine.L.Intents) - 1} {
		prefix, err := searchseat.RebuildFeed(m.rulesConfig(), engine.L, n, actor)
		if err != nil {
			t.Fatalf("RebuildFeed(%d): %v", n, err)
		}
		truncated := live.History()
		truncated.Frames = truncated.Frames[:n+1]
		truncated.Answers = map[int][]searchprobe.Action{}
		for frame, ans := range live.History().Answers {
			if frame < n {
				truncated.Answers[frame] = ans
			}
		}
		if !reflect.DeepEqual(prefix.History(), truncated) {
			t.Fatalf("prefix %d: RebuildFeed != live feed truncated (%d/%d frames)",
				n, len(prefix.History().Frames), len(truncated.Frames))
		}
	}
}

// TestNoEnvSeatNoFeed is BP-07's fast-path and no-regression gate: a table
// whose seats are all plain bots builds no feed at all (nil matchFeeds, no
// backing slice), and the nil-receiver Observe allocates nothing. Every match
// that existed before this ticket takes exactly this path, so its behaviour is
// unchanged (the Done-means TestHeads run pins the rules half of that).
func TestNoEnvSeatNoFeed(t *testing.T) {
	r, err := New(testOptions(t))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err := r.AddTable(fourSeatTable("t1", false)); err != nil {
		t.Fatal(err)
	}
	if err := r.Start("t1"); err != nil {
		t.Fatal(err)
	}
	r.Wait("t1")

	m, _ := matchFor(t, r, "t1")
	if m.feeds != nil {
		t.Fatalf("a table with no EnvSeat built feeds: %+v", m.feeds)
	}
	// The match genuinely ran, so the nil above is the fast path and not an
	// early bail (an unwired Observe would also leave feeds nil).
	m.mu.RLock()
	intents := m.intents
	m.mu.RUnlock()
	if intents < 20 {
		t.Fatalf("match logged only %d intents; the fast path was not exercised by a real game", intents)
	}

	// Allocation-free: the nil receiver returns before touching the engine, so
	// a nil *rules.Engine is safe to pass and proves nothing is built.
	var nilFeeds *matchFeeds
	if n := testing.AllocsPerRun(1000, func() { nilFeeds.observe(nil) }); n != 0 {
		t.Fatalf("nil matchFeeds.observe allocated %v per call, want 0", n)
	}
}

// TestHostedEnvSeatsNeverSeeTheLiveEngine is BP-08's central claim (spec §5.4),
// asserted through a full live host match: every engine an EnvSeat is handed
// across a real game is the honest root — never the live engine, never a
// pointer that aliases it, never reused across DecideEnv calls — and the
// root's opponent hand differs from the live opponent hand at the same
// boundary at least once, which is what proves a REDEAL happened (a plain
// clone of the live engine would pass every pointer check and still leak).
// The deciding seat's own hand, which the seat is entitled to, must match the
// live hand at every boundary: the redeal keeps what the seat knows. The one
// exception is the boundary where the CR 103.1 winner-chooses choice is still
// pending (E3-fx1): CR 103.5 hides every hand from every player there -- the
// deciding seat's own included -- so the seat's own frame shows no hand and
// its honest root deals its hand from the derivable pool like any other
// hidden card. A root that held the LIVE hand at that boundary would be the
// leak, not the redeal, so the pending boundary asserts the opposite.
func TestHostedEnvSeatsNeverSeeTheLiveEngine(t *testing.T) {
	t.Parallel()
	var spy *spyEnvSeat
	opts := testOptions(t)
	const seed = uint64(20260928)
	opts.Seats = func(names []string, gotSeed uint64) []seat.Seat {
		spy = &spyEnvSeat{Bot: heuristic.NewBot(gotSeed ^ 1)}
		return []seat.Seat{spy, heuristic.NewBot(gotSeed ^ 2)}
	}
	r, err := New(opts)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err := r.AddTable(envTestTable("t1", seed)); err != nil {
		t.Fatal(err)
	}
	if err := r.Start("t1"); err != nil {
		t.Fatal(err)
	}
	r.Wait("t1")

	m, engine := matchFor(t, r, "t1")

	// Precondition: the game ran and the Env dispatch actually engaged — a
	// spy that was never parked on DecideEnv would prove nothing here.
	m.mu.RLock()
	intents := m.intents
	feeds := m.feeds
	m.mu.RUnlock()
	if intents < 20 {
		t.Fatalf("match logged only %d intents; too short to assert on", intents)
	}
	if spy.decided < 20 {
		t.Fatalf("DecideEnv answered only %d of %d intents; the Env dispatch never engaged", spy.decided, intents)
	}
	if len(spy.envs) != spy.decided {
		t.Fatalf("env records %d != DecideEnv calls %d", len(spy.envs), spy.decided)
	}
	if feeds == nil || feeds.bySlot[0] == nil {
		t.Fatal("the EnvSeat has no feed; newMatchFeeds is not wired")
	}

	seen := map[weak.Pointer[rules.Engine]]bool{}
	roots := 0
	for _, rec := range spy.envs {
		if rec.engineID == (weak.Pointer[rules.Engine]{}) {
			t.Fatalf("env decision seq %d built no root (refused %q); the honest path failed on these decks", rec.seq, rec.rootRef)
		}
		if rec.rootRef != "" {
			t.Fatalf("env decision seq %d: root built but RootRefused %q", rec.seq, rec.rootRef)
		}
		if rec.engineID == weak.Make(engine) {
			t.Fatalf("env decision seq %d: the host handed the EnvSeat the LIVE engine", rec.seq)
		}
		if rec.gID == weak.Make(engine.G) {
			t.Fatalf("env decision seq %d: the root's game aliases the live engine's", rec.seq)
		}
		if seen[rec.engineID] {
			t.Fatalf("env decision seq %d: a root was REUSED after an earlier DecideEnv returned", rec.seq)
		}
		seen[rec.engineID] = true
		if rec.feed != feeds.bySlot[0] {
			t.Fatalf("env decision seq %d: Search.Feed is not the actor's own feed", rec.seq)
		}
		if rec.viewRound == 0 {
			t.Fatalf("env decision seq %d: env.View carries no Round", rec.seq)
		}
		roots++
	}
	if roots < 10 {
		t.Fatalf("only %d roots built across %d Env decisions", roots, spy.decided)
	}

	// The live opponent hand at every slot-0 decision boundary, rebuilt by
	// deterministic replay of the finished log, plus which of those
	// boundaries still hold the pending CR 103.1 winner-chooses choice
	// (E3-fx1: CR 103.5 hides the deciding seat's own hand there, so the
	// honest root redeals it -- see the test's doc comment).
	liveOpp, liveActor := map[uint64][]string{}, map[uint64][]string{}
	choicePending := map[uint64]bool{}
	if _, err := replay.Walk(engine.L, m.rulesConfig(), len(engine.L.Intents), func(e *rules.Engine, i int) error {
		if d := e.Pending(); d != nil && d.Player == 0 {
			liveOpp[d.Seq] = zoneNames(e.G.Zone(state.ZHand, 1), e)
			liveActor[d.Seq] = zoneNames(e.G.Zone(state.ZHand, 0), e)
			choicePending[d.Seq] = e.StartingChoicePending()
		}
		return nil
	}); err != nil {
		t.Fatalf("replay walk: %v", err)
	}

	same, diff, blind := 0, 0, 0
	for _, rec := range spy.envs {
		live, ok := liveOpp[rec.seq]
		if !ok {
			t.Fatalf("no replay boundary for env decision seq %d", rec.seq)
		}
		if sortedEqual(rec.oppHand, live) {
			same++
		} else {
			diff++
		}
		if choicePending[rec.seq] {
			// CR 103.5: while the CR 103.1 choice is pending the seat may
			// not see its own hand either, so its root must NOT hold the
			// live hand -- the redeal is the honest world. (A uniform redeal
			// from the derivable pool could theoretically deal the live hand
			// by chance; the probability is ~1/C(60,7), the same order of
			// exposure the opponent-hand redeal check below already carries.)
			if sortedEqual(rec.actorHand, liveActor[rec.seq]) {
				t.Fatalf("env decision seq %d: the root held the deciding seat's own hand while the starting-player choice was pending (%v); the CR 103.5 blind did not reach the root",
					rec.seq, rec.actorHand)
			}
			blind++
			continue
		}
		if !sortedEqual(rec.actorHand, liveActor[rec.seq]) {
			t.Fatalf("env decision seq %d: the root redealt the deciding seat's OWN hand (%v, live %v)",
				rec.seq, rec.actorHand, liveActor[rec.seq])
		}
	}
	if blind == 0 {
		t.Fatalf("no Env decision fell in the pending starting-player window; the blind-boundary branch never ran")
	}
	if diff == 0 {
		t.Fatalf("the root's opponent hand equalled the live opponent hand at all %d Env decisions; no redeal happened", len(spy.envs))
	}
	t.Logf("roots: %d, live-equal opponent hands: %d, redealt-different: %d", roots, same, diff)
}

// TestHostedEnvSeatsIgnoreTheRealHiddenCards is the seat-level leak test
// (spec §5.3): two live matches built from the same table config and seed
// differ ONLY in hidden cards after the swap fixture is applied to the second
// match's live engine (every unpinned opponent hand card exchanged with a
// same-named-different library card, that library reversed — all secret
// events, public state untouched). Each is driven through the REAL host Env
// path — projectNext under the lock, parkSeat outside it — and the decision
// played at the swapped boundary must be answered identically from roots
// whose hidden worlds are name-identical: what an honest root deals is a
// function of the seat's observation and the seed alone, never of the real
// hidden cards. A host that handed a plain clone of the live engine instead
// of the redeal would pass the pointer checks and fail exactly here.
func TestHostedEnvSeatsIgnoreTheRealHiddenCards(t *testing.T) {
	t.Parallel()
	r, err := New(testOptions(t))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	const seed = uint64(20260928)
	if err := r.AddTable(envTestTable("t1", seed)); err != nil {
		t.Fatal(err)
	}
	r.mu.RLock()
	tbl := r.tables["t1"]
	r.mu.RUnlock()

	build := func() (*match, *spyEnvSeat, []seat.Seat) {
		spy := &spyEnvSeat{Bot: heuristic.NewBot(seed ^ 1), retainRoots: true}
		seats := []seat.Seat{spy, heuristic.NewBot(seed ^ 2)}
		m, err := r.newMatch(tbl, 0)
		if err != nil {
			t.Fatal(err)
		}
		m.mu.Lock()
		m.slots = seats
		m.feeds = newMatchFeeds(seats)
		m.mu.Unlock()
		return m, spy, seats
	}
	m1, spy1, seats1 := build()
	m2, spy2, seats2 := build()

	swapped := false
	var targetSeq uint64
	swapFn := func(m *match, d *decision.Decision) {
		if swapped {
			return
		}
		if n := swapOpponentHidden(t, m.e, m.feeds.bySlot[0]); n > 0 {
			swapped = true
			targetSeq = d.Seq
		}
	}

	brd1, brd2 := botpolicy.NewBoard(2), botpolicy.NewBoard(2)
	steps := 0
	for ; steps < 20000; steps++ {
		_, pd1 := driveEnvStep(t, m1, seats1, &brd1, nil)
		if pd1 == nil {
			t.Fatal("match 1 ended before the swapped decision was reached")
		}
		_, pd2 := driveEnvStep(t, m2, seats2, &brd2, swapFn)
		if pd2 == nil {
			t.Fatal("match 2 ended before the swapped decision was reached")
		}
		submitEnvStep(t, r, tbl, m1, pd1)
		submitEnvStep(t, r, tbl, m2, pd2)
		if rec1, rec2 := spy1.recorded(targetSeq), spy2.recorded(targetSeq); swapped && rec1 != nil && rec2 != nil {
			break
		}
	}
	if steps >= 20000 || !swapped {
		t.Fatal("fixture: no opponent hand card could be swapped at any Env decision")
	}

	t.Logf("swap target: seq %d at drive step %d; roots real %v, swapped %v", targetSeq, steps, len(spy1.envs), len(spy2.envs))
	rec1, rec2 := spy1.recorded(targetSeq), spy2.recorded(targetSeq)
	if rec1 == nil || rec2 == nil {
		t.Fatalf("the swapped decision (seq %d) was never parked on DecideEnv (records %d/%d)",
			targetSeq, len(spy1.envs), len(spy2.envs))
	}

	// Fixture precondition: the two live positions genuinely differ in hidden
	// cards — the swap did its work and nothing public moved.
	if hiddenWorldsEqual(m1.e, m2.e) {
		t.Fatal("fixture: the two live positions hold the same hidden cards")
	}
	// The Env carried the honest root, not the live engine, in both matches.
	for i, rec := range []*envRecord{rec1, rec2} {
		if rec.engine == nil {
			t.Fatalf("match %d: root refused at the swapped boundary (%q)", i+1, rec.rootRef)
		}
	}
	if rec1.engine == m1.e || rec2.engine == m2.e {
		t.Fatal("the host handed a live engine to an EnvSeat")
	}
	// Central claim: the two roots are name-identical in every hidden zone,
	// and the decision the seat answers is identical in both worlds.
	if !hiddenWorldsEqual(rec1.engine, rec2.engine) {
		t.Fatalf("two roots for the same seed and feed depend on the real hidden cards:\nreal hand %v swapped %v\nreal lib %v swapped %v",
			rec1.oppHand, rec2.oppHand, rec1.oppLib, rec2.oppLib)
	}
	if !reflect.DeepEqual(rec1.in, rec2.in) {
		t.Fatalf("the two matches answered the swapped decision differently: %+v vs %+v", rec1.in, rec2.in)
	}
	// The deciding seat's own zones are untouched by the fixture and must
	// match in the roots too (the redeal keeps what the seat knows).
	if !sortedEqual(rec1.actorHand, rec2.actorHand) {
		t.Fatal("the two roots dealt the actor's own hand differently")
	}
}

// TestEnvRootRefusalPlaysFallback drives a refusal through the REAL host path
// (spec §5.1): once the actor's feed is stopped — the host-side state a failed
// capture or a failed Record leaves (§5.2) — the seat must still be ASKED,
// with env.Search.Engine nil and env.RootRefused set to HonestRoot's
// "no live observation feed" reason, and must still answer (its DecideBoard
// fallback), so the match never wedges. A healthy root at the first Env
// decision is asserted first, so the nil engine below is the refusal and not
// the norm, and the counter is asserted to have counted exactly the refusal.
func TestEnvRootRefusalPlaysFallback(t *testing.T) {
	t.Parallel()
	r, err := New(testOptions(t))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	const seed = uint64(20260929)
	if err := r.AddTable(envTestTable("t1", seed)); err != nil {
		t.Fatal(err)
	}
	r.mu.RLock()
	tbl := r.tables["t1"]
	r.mu.RUnlock()
	spy := &spyEnvSeat{Bot: heuristic.NewBot(seed ^ 1)}
	seats := []seat.Seat{spy, heuristic.NewBot(seed ^ 2)}
	m, err := r.newMatch(tbl, 0)
	if err != nil {
		t.Fatal(err)
	}
	m.mu.Lock()
	m.slots = seats
	m.feeds = newMatchFeeds(seats)
	m.mu.Unlock()

	healthySeen := false
	var refused *parkedData
	brd := botpolicy.NewBoard(2)
	for steps := 0; steps < 20000 && refused == nil; steps++ {
		stopFeed := healthySeen
		data, pd := driveEnvStep(t, m, seats, &brd, func(m *match, d *decision.Decision) {
			if stopFeed && m.feeds != nil && m.feeds.bySlot[0] != nil {
				// The state §5.2 leaves behind after a failed capture or
				// Record: the feed is dead for the rest of the game.
				m.feeds.stopped[0] = true
			}
		})
		if data == nil {
			t.Fatal("match ended before a refused root was answered")
		}
		if data.env == nil {
			submitEnvStep(t, r, tbl, m, pd) // slot 1's decision; keep driving
			continue
		}
		if !healthySeen {
			if data.env.Search.Engine == nil || data.env.RootRefused != "" {
				t.Fatalf("the FIRST Env decision (seq %d) built no healthy root: %q", data.dc.Seq, data.env.RootRefused)
			}
			healthySeen = true
			submitEnvStep(t, r, tbl, m, pd)
			continue
		}
		refused = data
		// The refusal is HonestRoot's canonical dead-feed reason, routed
		// through the host-side stopped bit (see envData).
		if data.env.RootRefused != "no live observation feed" {
			t.Fatalf("refused root reason %q, want the canonical dead-feed reason", data.env.RootRefused)
		}
		// §5.2: "the seat plays its fallback from then on" — the stopped feed
		// is not handed to the seat at all (a live fd would let an adapter
		// read a History that misses every future recorded answer).
		if data.env.Search.Feed != nil {
			t.Fatal("the refused Env handed the seat a stopped feed")
		}
		t.Logf("refused root at env decision seq %d (%q) after %d steps", data.dc.Seq, data.env.RootRefused, steps)
		// The fallback proof: the parked answer is exactly the wrapped bot's
		// DecideBoard answer from a freshly built copy of the same board.
		var want decision.Intent
		err := m.locked(func() error {
			fresh := botpolicy.NewBoard(2)
			want, _ = spy.Bot.DecideBoard(context.Background(), botpolicy.BoardFromGameInto(m.e.G, m.e, 0, &fresh), data.dc)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(pd.in, want) {
			t.Fatalf("the refused root's seat did not play its DecideBoard fallback: %+v vs %+v", pd.in, want)
		}
		submitEnvStep(t, r, tbl, m, pd)
	}
	if !healthySeen {
		t.Fatal("no healthy root was ever built; the refusal half proves nothing")
	}
	if refused == nil {
		t.Fatal("the stopped feed never produced a refused root")
	}
	if refused.env.Search.Engine != nil {
		t.Fatal("a root was built from a dead feed")
	}
	if refused.env.RootRefused != "no live observation feed" {
		t.Fatalf("a refused root carried %q, not the fail-closed reason", refused.env.RootRefused)
	}
	if refused.env.Search.Feed != nil {
		t.Fatal("the refused Env did carry the stopped feed")
	}
	m.mu.RLock()
	rootRefusals := m.rootRefusals
	m.mu.RUnlock()
	if rootRefusals != 1 {
		t.Fatalf("rootRefusals = %d, want 1", rootRefusals)
	}
	m.mu.RLock()
	sc := m.sidecar()
	m.mu.RUnlock()
	if sc.RootRefusals != 1 {
		t.Fatalf("sidecar RootRefusals = %d, want 1", sc.RootRefusals)
	}
}
