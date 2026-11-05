package eventhost

import (
	"fmt"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/draft/event"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/protocol"
)

// rebuildPtr is a create body's rebuild field naming a non-default value.
func rebuildPtr(v bool) *bool { return &v }

// driveHumanBo1 answers every decision the human side of the Bo1 series
// (a, b) is asked with the first Min options until the series ends, following
// the live table across crash retries. The human sits at table slot 0 (event
// seat a, match 1's rotation); the bot side answers itself. It fails the test
// on deadline.
func driveHumanBo1(t *testing.T, reg *host.Registry, r *run, a, b int) {
	t.Helper()
	deadline := time.Now().Add(50 * time.Second)
	var last uint64
	for {
		r.mu.Lock()
		v, ok := r.ev.SeriesState(a, b)
		entries := append([]event.Entry(nil), r.ev.Log()...)
		r.mu.Unlock()
		if ok && v.Ended {
			return
		}
		if !ok {
			// The round rolled over and the pairing is no longer current:
			// ended only if its SeriesEnded is on the log.
			for _, en := range entries {
				if en.Kind == event.KindSeriesEnded && en.A == a && en.B == b {
					return
				}
			}
			t.Fatalf("series (%d,%d) missing before its SeriesEnded", a, b)
		}
		if time.Now().After(deadline) {
			t.Fatalf("series (%d,%d) never ended: %+v", a, b, v)
		}
		if v.InFlight == nil {
			time.Sleep(pollEvery)
			continue
		}
		table, k := host.TableID(v.InFlight.Table), v.InFlight.Game
		d, err := reg.Pending(table, k, state.PlayerID(0))
		if err != nil || d.Seq == last {
			time.Sleep(pollEvery)
			continue
		}
		if err := reg.SubmitIntent(table, k, state.PlayerID(0), legalIntent(d)); err != nil {
			// The table may have been replaced by a fresh attempt between
			// the Pending and the submit; the loop re-reads the state.
			last = 0
			time.Sleep(pollEvery)
			continue
		}
		last = d.Seq
	}
}

// rebuildHumanEvent creates a one-human Bo1 prerelease event on a fresh pair,
// joins and builds the human (seat 0), and plays its round-1 series (0,3) to
// its end, leaving the event in ROUND2 with seat 0's series in its rebuild
// window. mut tweaks the option sets before anything is built.
func rebuildHumanEvent(t *testing.T, mut func(*Options, *host.Options)) (p *gamePair, r *run, id, tok string) {
	t.Helper()
	p = startGamePair(t, "", func(eo *Options, ho *host.Options) {
		eo.MintSeat = func(table host.TableID, seat int) (string, error) {
			return fmt.Sprintf("tok-%s-%d", table, seat), nil
		}
		if mut != nil {
			mut(eo, ho)
		}
	})
	req := humanSeatsReq([]int{0}, 1)
	req.Format = formatPrerelease
	resp := createOK(t, p.h, req)
	id = resp.Event
	tok = inviteToken(t, resp, 0)
	joinOK(t, p.h, id, tok, "Ann")
	finishHumanBuild(t, p.h, id, tok)
	waitPhase(t, p.h, id, "ROUND1", 50*time.Second)
	r = runOf(t, p.h, id)
	driveHumanBo1(t, p.reg, r, 0, 3)
	waitPhase(t, p.h, id, "ROUND2", 50*time.Second)
	return p, r, id, tok
}

// waitRebuildBotLocked polls until seat's series is in its rebuild window
// with the other side (a bot, locked by the advance pass) locked, and
// returns the window view.
func waitRebuildBotLocked(t *testing.T, r *run, a, b, human int) event.SeriesView {
	t.Helper()
	bot := a
	if a == human {
		bot = b
	}
	botSide := 0
	if bot == b {
		botSide = 1
	}
	deadline := time.Now().Add(50 * time.Second)
	for {
		r.mu.Lock()
		v, ok := r.ev.SeriesState(a, b)
		r.mu.Unlock()
		if ok && v.Opening && v.Locked[botSide] != 0 {
			return v
		}
		if time.Now().After(deadline) {
			t.Fatalf("series (%d,%d): bot side never locked in the window: %+v", a, b, v)
		}
		time.Sleep(pollEvery)
	}
}

// seatSeries is the current-round pairing that contains seat.
func seatSeries(t *testing.T, r *run, seat int) (int, int) {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	a, b, ok := r.ev.SeriesOf(seat)
	if !ok {
		t.Fatalf("seat %d has no series in round %d", seat, r.ev.Round())
	}
	return a, b
}

// startedGames counts the GameStarted entries the log holds for (round, a, b).
func startedGames(entries []event.Entry, round, a, b int) int {
	n := 0
	for _, en := range entries {
		if en.Kind == event.KindGameStarted && en.Round == round && en.A == a && en.B == b {
			n++
		}
	}
	return n
}

func TestRebuildDefaultsByFormat(t *testing.T) {
	p := startGamePair(t, "", func(eo *Options, _ *host.Options) {
		eo.MaxLiveEvents = 8
	})
	for _, tc := range []struct {
		name    string
		format  string
		rebuild *bool
		want    bool
	}{
		{"prerelease", formatPrerelease, nil, true},
		{"sealed", formatSealed, nil, true},
		{"draft", "", nil, false},
		{"prerelease opt-out", formatPrerelease, rebuildPtr(false), false},
		{"draft opt-in", "", rebuildPtr(true), true},
	} {
		req := botEventReq("bot", "lethal-pressure", "forge-heuristic", "bot")
		req.Format = tc.format
		req.Rebuild = tc.rebuild
		resp := createOK(t, p.h, req)
		if resp.State.Rebuild != tc.want {
			t.Errorf("%s: EventState.rebuild = %v, want %v", tc.name, resp.State.Rebuild, tc.want)
		}
		r := runOf(t, p.h, resp.Event)
		r.mu.Lock()
		metaRebuild := r.meta.Rebuild
		r.mu.Unlock()
		en := createdEntry(t, p.h, resp.Event)
		if metaRebuild != tc.want {
			t.Errorf("%s: meta.rebuild = %v, want %v", tc.name, metaRebuild, tc.want)
		}
		if en.Rebuild != tc.want {
			t.Errorf("%s: EventCreated.rebuild = %v, want %v", tc.name, en.Rebuild, tc.want)
		}
	}
}

// TestRebuildBotsKeepLastDeck drives a bot-only prerelease (Bo3) event to
// COMPLETE: every round-2 and round-3 series logs two Sideboarded Game 1
// entries before its first GameStarted, each locking the revision the seat
// played last, and round 1 logs none.
func TestRebuildBotsKeepLastDeck(t *testing.T) {
	p := startGamePair(t, "", nil)
	req := botEventReq("bot", "lethal-pressure", "forge-heuristic", "bot")
	req.Format = formatPrerelease
	resp := createOK(t, p.h, req)
	waitPhase(t, p.h, resp.Event, "COMPLETE", 50*time.Second)

	r := runOf(t, p.h, resp.Event)
	r.mu.Lock()
	log := append([]event.Entry(nil), r.ev.Log()...)
	lastPlayed := map[int]int{}
	for s := 0; s < 4; s++ {
		lastPlayed[s] = r.ev.LastPlayedRev(s)
	}
	r.mu.Unlock()

	type key struct{ round, a, b int }
	type seen struct {
		boards  int
		starts  int
		revs    map[int]int
		gameOne bool // a GameStarted of this series has been logged
	}
	saw := map[key]*seen{}
	for _, en := range log {
		switch en.Kind {
		case event.KindGameStarted, event.KindSideboarded:
		default:
			continue
		}
		k := key{en.Round, en.A, en.B}
		s := saw[k]
		if s == nil {
			s = &seen{revs: map[int]int{}}
			saw[k] = s
		}
		switch en.Kind {
		case event.KindGameStarted:
			s.starts++
			s.gameOne = true
		case event.KindSideboarded:
			if en.Game == 1 && s.gameOne {
				t.Errorf("series (%d,%d) round %d: Sideboarded for game 1 after its first GameStarted", en.A, en.B, en.Round)
				continue
			}
			if en.Game != 1 {
				continue // the between window's game 2/3 locks are the Bo3 path's own
			}
			if en.Rev != lastPlayed[en.Seat] {
				t.Errorf("series (%d,%d) round %d: seat %d locked rev %d, want its last-played rev %d",
					en.A, en.B, en.Round, en.Seat, en.Rev, lastPlayed[en.Seat])
			}
			s.boards++
			s.revs[en.Seat] = en.Rev
		}
	}
	for k, s := range saw {
		if k.round == 1 {
			if s.boards != 0 {
				t.Errorf("round-1 series (%d,%d) logged %d Sideboarded entries, want 0", k.a, k.b, s.boards)
			}
			continue
		}
		if s.boards != 2 {
			t.Errorf("round-%d series (%d,%d) logged %d Sideboarded game-1 entries, want 2", k.round, k.a, k.b, s.boards)
		}
		if len(s.revs) != 2 {
			t.Errorf("round-%d series (%d,%d) locked by %v seats, want both", k.round, k.a, k.b, s.revs)
		}
	}
}

// TestRebuildHumanWindow is the human half of the window: in round 2 the
// seat's row is between with game 1 and waiting [0] (the bot side has
// locked), no game of the series has started, the pool reports the deck it
// played last, and a deck submission saves a new revision and locks it —
// game 1 then plays it.
func TestRebuildHumanWindow(t *testing.T) {
	p, r, id, tok := rebuildHumanEvent(t, nil)
	a, b := seatSeries(t, r, 0)
	waitRebuildBotLocked(t, r, a, b, 0)

	r.mu.Lock()
	entries := append([]event.Entry(nil), r.ev.Log()...)
	lastRev := r.ev.LastPlayedRev(0)
	r.mu.Unlock()
	row := seriesRow(stateOf(t, p.h, id, tok), a, b)
	if row == nil || row.State != "between" || row.Game != 1 {
		t.Fatalf("round-2 series (%d,%d) row = %+v, want between with game 1", a, b, row)
	}
	if len(row.Waiting) != 1 || row.Waiting[0] != 0 {
		t.Fatalf("round-2 series (%d,%d) waiting = %v, want [0] (the bot side has locked)", a, b, row.Waiting)
	}
	if n := startedGames(entries, 2, a, b); n != 0 {
		t.Fatalf("round-2 series (%d,%d) logged %d GameStarted entries in the window, want 0", a, b, n)
	}

	// The pool reports the deck the seat played last while the window is
	// open: the sideboard panel's starting point.
	pool, err := p.h.EventPool(id, tok)
	if err != nil {
		t.Fatalf("EventPool: %v", err)
	}
	if pool.Playing == nil || pool.PlayingRev != lastRev {
		t.Fatalf("pool playing = %+v rev %d, want the last-played deck rev %d", pool.Playing, pool.PlayingRev, lastRev)
	}

	// The seat submits a new deck: an unused pool card replaces the
	// suggestion's first main card, so the revision is real.
	d := pool.Suggestion
	if len(d.Main) < 1 {
		t.Fatalf("suggestion too small: %+v", d)
	}
	used := map[string]bool{}
	for _, mid := range d.Main {
		used[mid] = true
	}
	swap := ""
	for _, c := range pool.Pool {
		if !used[c.ID] {
			swap = c.ID
			break
		}
	}
	if swap == "" {
		t.Fatal("precondition: the pool has no unused card to swap in")
	}
	d.Main[0] = swap
	if _, err := p.h.Sideboard(id, tok, protocol.SideboardRequest{Game: 1, Deck: &d}); err != nil {
		t.Fatalf("Sideboard(deck): %v", err)
	}
	rev := pool.Rev + 1

	// The game plays the locked decks: the table's deck ids are the
	// locked revisions. Matches names finished matches, so the seat drives
	// its game to its end first.
	att, _ := waitSeriesInFlight(t, r, a, b, 1, 50*time.Second)
	if att.Game != 1 {
		t.Fatalf("game 1 attempt = %+v", att)
	}
	// A Bo1 game's table is the attempt's own; the run's seriesTable row
	// only names Bo3 series tables.
	table := host.TableID(att.Table)
	driveHumanBo1(t, p.reg, r, a, b)
	mi, err := p.reg.Matches(table)
	if err != nil || len(mi) == 0 {
		t.Fatalf("Matches(%s): %v", table, err)
	}
	want0, want1 := DeckID(id, 0, rev), DeckID(id, b, 1)
	if mi[0].Seats[0].DeckID != want0 || mi[0].Seats[1].DeckID != want1 {
		t.Fatalf("table deck ids %s/%s, want %s/%s", mi[0].Seats[0].DeckID, mi[0].Seats[1].DeckID, want0, want1)
	}
	r.mu.Lock()
	entries = append([]event.Entry(nil), r.ev.Log()...)
	saved := r.ev.SavedRev(0)
	r.mu.Unlock()
	var botLocked, humanLocked *event.Entry
	for i := range entries {
		en := entries[i]
		if en.Kind != event.KindSideboarded || en.Round != 2 || en.A != a || en.B != b || en.Game != 1 {
			continue
		}
		if en.Auto {
			t.Errorf("Sideboarded entry for seat %d is Auto: %+v", en.Seat, en)
		}
		if en.Seat == 0 {
			humanLocked = &entries[i]
		} else {
			botLocked = &entries[i]
		}
	}
	if humanLocked == nil || humanLocked.Rev != rev {
		t.Fatalf("seat 0's Sideboarded game-1 entry = %+v, want rev %d", humanLocked, rev)
	}
	if botLocked == nil || botLocked.Rev != 1 {
		t.Fatalf("bot side's Sideboarded game-1 entry = %+v, want the kept rev 1", botLocked)
	}
	if saved != rev {
		t.Errorf("SavedRev(0) = %d, want %d (the sideboard save)", saved, rev)
	}
}

// TestRebuildHumanKeep is the same window with keep: true — the seat locks
// the deck it played last and the game plays it, with no new revision saved.
func TestRebuildHumanKeep(t *testing.T) {
	p, r, id, tok := rebuildHumanEvent(t, nil)
	a, b := seatSeries(t, r, 0)
	waitRebuildBotLocked(t, r, a, b, 0)
	r.mu.Lock()
	lastRev := r.ev.LastPlayedRev(0)
	r.mu.Unlock()

	if _, err := p.h.Sideboard(id, tok, protocol.SideboardRequest{Game: 1, Keep: true}); err != nil {
		t.Fatalf("Sideboard(keep): %v", err)
	}
	att, _ := waitSeriesInFlight(t, r, a, b, 1, 50*time.Second)
	table := host.TableID(att.Table)
	driveHumanBo1(t, p.reg, r, a, b)
	r.mu.Lock()
	saved := r.ev.SavedRev(0)
	r.mu.Unlock()
	mi, err := p.reg.Matches(table)
	if err != nil || len(mi) == 0 {
		t.Fatalf("Matches(%s): %v", table, err)
	}
	want0, want1 := DeckID(id, 0, lastRev), DeckID(id, b, 1)
	if mi[0].Seats[0].DeckID != want0 || mi[0].Seats[1].DeckID != want1 {
		t.Fatalf("table deck ids %s/%s, want the kept decks %s/%s", mi[0].Seats[0].DeckID, mi[0].Seats[1].DeckID, want0, want1)
	}
	if saved != 1 {
		t.Errorf("SavedRev(0) = %d, want 1 (a keep saves no revision)", saved)
	}
}

// TestRebuildIdleAutoKeeps is the idle half: a human idle past IdleTimeout in
// its series' rebuild window is auto-kept — a Sideboarded Game 1 entry with
// Auto and the revision it played last.
func TestRebuildIdleAutoKeeps(t *testing.T) {
	clock := newTestClock()
	p, r, _, _ := rebuildHumanEvent(t, func(eo *Options, _ *host.Options) {
		eo.IdleTimeout = 30 * time.Minute
		eo.Now = clock.Now
	})
	a, b := seatSeries(t, r, 0)
	waitRebuildBotLocked(t, r, a, b, 0)
	r.mu.Lock()
	lastRev := r.ev.LastPlayedRev(0)
	round := r.ev.Round()
	r.mu.Unlock()

	clock.Add(30*time.Minute + time.Minute)
	p.h.checkTimeouts(clock.Now())

	r.mu.Lock()
	entries := append([]event.Entry(nil), r.ev.Log()...)
	r.mu.Unlock()
	var keep *event.Entry
	for i := range entries {
		en := entries[i]
		if en.Kind == event.KindSideboarded && en.Round == round && en.A == a && en.B == b && en.Game == 1 && en.Seat == 0 {
			keep = &entries[i]
		}
	}
	if keep == nil {
		t.Fatalf("no Sideboarded game-1 entry for seat 0's round-%d series (%d,%d)", round, a, b)
	}
	if !keep.Auto {
		t.Errorf("auto-keep Sideboarded entry is not Auto: %+v", keep)
	}
	if keep.Rev != lastRev {
		t.Errorf("auto-keep Sideboarded rev = %d, want the last-played rev %d", keep.Rev, lastRev)
	}
}

// TestRebuildPoolPlayingInWindow checks GET /pool in the open window: playing
// is the LastPlayedRev deck and playing_rev names it.
func TestRebuildPoolPlayingInWindow(t *testing.T) {
	p, r, id, tok := rebuildHumanEvent(t, nil)
	a, b := seatSeries(t, r, 0)
	waitRebuildBotLocked(t, r, a, b, 0)
	r.mu.Lock()
	lastRev := r.ev.LastPlayedRev(0)
	saved, savedOK := r.ev.SavedDeck(0)
	r.mu.Unlock()
	pool, err := p.h.EventPool(id, tok)
	if err != nil {
		t.Fatalf("EventPool: %v", err)
	}
	if pool.PlayingRev != lastRev {
		t.Fatalf("pool playing_rev = %d, want %d", pool.PlayingRev, lastRev)
	}
	if pool.Playing == nil {
		t.Fatal("pool playing = nil, want the last-played deck")
	}
	if savedOK && (len(pool.Playing.Main) != len(saved.Main) || pool.Playing.Main[0] != saved.Main[0]) {
		t.Fatalf("pool playing = %+v, want the saved revision %d's deck", pool.Playing, lastRev)
	}
}

// TestRebuildOffNoWindow is the negative: a draft event (rebuild off) logs no
// Sideboarded game-1 entries in round 2, and its rows never show between with
// game 1.
func TestRebuildOffNoWindow(t *testing.T) {
	p := startGamePair(t, "", nil)
	r := playBotEvent(t, p)
	r.mu.Lock()
	log := append([]event.Entry(nil), r.ev.Log()...)
	st := r.stateFor(-1)
	metaRebuild := r.meta.Rebuild
	r.mu.Unlock()
	if metaRebuild {
		t.Fatal("a draft event's meta has rebuild on")
	}
	if en := createdEntry(t, p.h, r.meta.ID); en.Rebuild {
		t.Fatal("a draft event's EventCreated carries rebuild")
	}
	for _, en := range log {
		if en.Kind == event.KindSideboarded {
			t.Errorf("Sideboarded entry in a rebuild-off event: %+v", en)
		}
	}
	for i := range st.Series {
		if s := st.Series[i]; s.State == "between" && s.Game == 1 {
			t.Errorf("series row (%d,%d) round %d shows between with game 1 in a rebuild-off event",
				s.A, s.B, s.Round)
		}
	}
}
