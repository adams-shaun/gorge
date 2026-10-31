package eventhost

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	_ "github.com/adams-shaun/gorge/bots/all"
	"github.com/adams-shaun/gorge/draft/event"
	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/protocol"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// gamePair is one Host and its Registry, wired together through a Relay so
// event game tables' match results reach the Host. dir "" keeps the event
// store in memory; a dir makes it a disk store, so a fresh pair on the same
// dir resumes the events.
type gamePair struct {
	h     *Host
	reg   *host.Registry
	store *Store
}

// startGamePair builds a Host + Registry pair whose game tables load real
// decks from the event store: the store's deck loader, the real corpus
// tokens and name universe, unbounded bot budgets, the default per-turn
// stall guard, and a fresh relay chain. mut tweaks both option sets before
// anything is built.
func startGamePair(t *testing.T, dir string, mut func(*Options, *host.Options)) *gamePair {
	t.Helper()
	corpus := testutil.CorpusRegistry(t)
	store, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	relay := &Relay{}
	seriesHook := NewLazySeries()
	ho := host.Options{
		LoadDeck:              store.DeckLoader(corpus, nil),
		Tokens:                corpus.Tokens,
		NameUniverse:          corpus.Universe(),
		BotUnboundedDecisions: true,
		MaxDecisionsPerTurn:   host.DefaultMaxDecisionsPerTurn,
		Dir:                   dir,
		OnMatchEnd:            relay.OnMatchEnd,
		// A Series table asks the lazy hook; the Host's Series is
		// installed below, once the Host exists. New's resume advance can
		// start series tables before that, and a call landing in the
		// window parks on the lazy hook instead of reading no plan — a
		// "no plan" answer would end the table and wedge the event.
		Series: seriesHook.Series,
	}
	eo := Options{Store: store, Cards: corpus, Seed: func() uint64 { return 11 }}
	if mut != nil {
		mut(&eo, &ho)
	}
	reg, err := host.New(ho)
	if err != nil {
		t.Fatal(err)
	}
	eo.Registry = reg
	h, err := New(eo)
	if err != nil {
		reg.Close()
		t.Fatal(err)
	}
	relay.Set(h.OnMatchEnd)
	seriesHook.Set(h.Series)
	p := &gamePair{h: h, reg: reg, store: store}
	t.Cleanup(func() { h.Close(); reg.Close() })
	return p
}

// botEventReq is a bot-only four-seat request: FDN and the given game
// policies. The seed comes from the pair's fixed Options.Seed.
func botEventReq(policies ...string) protocol.CreateEventRequest {
	req := protocol.CreateEventRequest{Set: "FDN"}
	for _, p := range policies {
		req.Seats = append(req.Seats, protocol.CreateEventSeat{Kind: "bot", Policy: p})
	}
	return req
}

// pollEvery is the sleep between polls. It is a poll interval, not a
// threshold: every wait here has a 50s ceiling, so load only slows a poll
// and never turns PASS into FAIL.
const pollEvery = time.Second / 50

// waitPhase polls until the event reaches the phase.
func waitPhase(t *testing.T, h *Host, id, phase string, d time.Duration) {
	t.Helper()
	deadline := time.Now().Add(d)
	for {
		st, err := h.EventState(id, "")
		if err != nil {
			t.Fatalf("EventState while waiting for %s: %v", phase, err)
		}
		if st.Phase == phase {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("phase %s not reached in %s (still %s)", phase, d, st.Phase)
		}
		time.Sleep(pollEvery)
	}
}

func TestRelayChains(t *testing.T) {
	var next, target []string
	relay := &Relay{Next: func(host.TableID, int, protocol.MatchInfo) error {
		next = append(next, "next")
		return nil
	}}
	if err := relay.OnMatchEnd("t", 1, protocol.MatchInfo{}); err != nil {
		t.Fatal(err)
	}
	if len(next) != 1 || len(target) != 0 {
		t.Fatalf("before Set: next=%v target=%v", next, target)
	}
	relay.Set(func(host.TableID, int, protocol.MatchInfo) error {
		target = append(target, "target")
		return nil
	})
	if err := relay.OnMatchEnd("t", 1, protocol.MatchInfo{}); err != nil {
		t.Fatal(err)
	}
	if len(next) != 2 || len(target) != 1 {
		t.Fatalf("after Set: next=%v target=%v", next, target)
	}
	relay.Next = func(host.TableID, int, protocol.MatchInfo) error { return errors.New("next failed") }
	if err := relay.OnMatchEnd("t", 1, protocol.MatchInfo{}); err == nil || err.Error() != "next failed" {
		t.Fatalf("chained error = %v", err)
	}

	// Series before SetSeries answers no; after it, the target receives
	// the call with its context, table and match number.
	if g, ok, err := relay.Series(context.Background(), "t", 1); ok || err != nil || g.Chooser != 0 {
		t.Fatalf("Series before SetSeries = %+v ok=%v err=%v", g, ok, err)
	}
	got := ""
	relay.SetSeries(func(_ context.Context, t host.TableID, k int) (host.SeriesGame, bool, error) {
		got = fmt.Sprintf("%s/%d", t, k)
		return host.SeriesGame{Chooser: 2}, true, nil
	})
	if g, ok, err := relay.Series(context.Background(), "tt", 2); err != nil || !ok ||
		g.Chooser != 2 || got != "tt/2" {
		t.Fatalf("Series after SetSeries = %+v ok=%v err=%v got=%q", g, ok, err, got)
	}
}

// playBotEvent drives a bot-only event to COMPLETE and returns the run.
func playBotEvent(t *testing.T, p *gamePair) *run {
	t.Helper()
	resp := createOK(t, p.h, botEventReq("bot", "lethal-pressure", "forge-heuristic", "bot"))
	waitPhase(t, p.h, resp.Event, "COMPLETE", 50*time.Second)
	return runOf(t, p.h, resp.Event)
}

func TestBotOnlyEventCompletes(t *testing.T) {
	p := startGamePair(t, "", nil)
	r := playBotEvent(t, p)

	r.mu.Lock()
	log, standings := r.ev.Log(), r.ev.Standings()
	pub, full := r.ev.LogFor(-1), r.ev.Log()
	r.mu.Unlock()

	starts, ends := map[[2]int]event.Entry{}, map[[2]int]event.Entry{}
	seriesEnded := 0
	for _, en := range log {
		switch en.Kind {
		case event.KindSeriesEnded:
			seriesEnded++
		case event.KindGameEnded:
			ends[[2]int{en.A, en.B}] = en
		case event.KindGameStarted:
			starts[[2]int{en.A, en.B}] = en
		}
	}
	if seriesEnded != 6 {
		t.Fatalf("SeriesEnded count %d, want 6", seriesEnded)
	}
	if len(standings) != 4 {
		t.Fatalf("standings rows %d, want 4", len(standings))
	}
	for pair, en := range ends {
		start, ok := starts[pair]
		if !ok {
			t.Fatalf("series %v has no GameStarted", pair)
		}
		mi, err := p.reg.Matches(host.TableID(start.Table))
		if err != nil || len(mi) == 0 {
			t.Fatalf("Matches(%s): %v", start.Table, err)
		}
		if mi[0].Head != en.Head {
			t.Fatalf("series %v GameEnded head %s != table head %s", pair, en.Head, mi[0].Head)
		}
		// Decks [b, a] + match 1's rotation: table seat 0 plays a's deck,
		// seat 1 b's.
		want0, want1 := DeckID(r.meta.ID, pair[0], r.ev.ReadyRev(pair[0])),
			DeckID(r.meta.ID, pair[1], r.ev.ReadyRev(pair[1]))
		if mi[0].Seats[0].DeckID != want0 || mi[0].Seats[1].DeckID != want1 {
			t.Fatalf("series %v table deck ids %s/%s, want %s/%s", pair,
				mi[0].Seats[0].DeckID, mi[0].Seats[1].DeckID, want0, want1)
		}
	}
	if !reflect.DeepEqual(pub, full) {
		t.Fatal("LogFor(-1) != Log() after COMPLETE")
	}
}

func TestBotOnlyEventIsDeterministic(t *testing.T) {
	mk := func() []byte {
		p := startGamePair(t, "", nil)
		r := playBotEvent(t, p)
		r.mu.Lock()
		defer r.mu.Unlock()
		b, err := event.MarshalLog(r.ev.Log())
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	first, second := mk(), mk()
	if !bytes.Equal(first, second) {
		t.Fatal("two bot events with the same seed diverged")
	}
}

// finishHumanBuild drives a human seat through BUILD: save the
// suggestion-builder's deck, ready it.
func finishHumanBuild(t *testing.T, h *Host, id, tok string) {
	t.Helper()
	pool, err := h.EventPool(id, tok)
	if err != nil {
		t.Fatalf("EventPool: %v", err)
	}
	if _, err := h.SaveDeck(id, tok, pool.Suggestion); err != nil {
		t.Fatalf("SaveDeck: %v", err)
	}
	if _, err := h.Ready(id, tok, protocol.ReadyRequest{Rev: pool.Rev + 1}); err != nil {
		t.Fatalf("Ready: %v", err)
	}
}

func TestHumanGameGetsJoinLink(t *testing.T) {
	var minted []host.TableID
	p := startGamePair(t, "", func(eo *Options, _ *host.Options) {
		eo.MaxHumans = 1
		eo.MintSeat = func(table host.TableID, seat int) (string, error) {
			minted = append(minted, table)
			return fmt.Sprintf("tok-%s-%d", table, seat), nil
		}
	})
	resp := createOK(t, p.h, oneHumanReq())
	id := resp.Event
	tok, _ := humanToken(t, resp)
	joinOK(t, p.h, id, tok, "Ann")
	pickTop(t, p.h, id, tok, 45)
	finishHumanBuild(t, p.h, id, tok)
	waitPhase(t, p.h, id, "ROUND1", 50*time.Second)

	table := gameTableID(id, 1, 0, 3, 1, 1)
	mintedOK := false
	for _, tb := range minted {
		if tb == table {
			mintedOK = true
		}
	}
	if !mintedOK {
		t.Fatalf("MintSeat never called for %s: %v", table, minted)
	}
	wantJoin := fmt.Sprintf("/t/%s?seat=0&token=tok-%s-0&event=%s", table, table, id)
	st := stateOf(t, p.h, id, tok)
	ser := -1
	for i := range st.Series {
		if st.Series[i].Round == 1 && st.Series[i].A == 0 && st.Series[i].B == 3 {
			ser = i
		}
	}
	if ser < 0 {
		t.Fatalf("no (0,3) series in %+v", st.Series)
	}
	got := st.Series[ser]
	if got.Join != wantJoin || got.Table != string(table) {
		t.Fatalf("(0,3) Join/Table = %q/%q, want %q/%q", got.Join, got.Table, wantJoin, table)
	}
	pub := stateOf(t, p.h, id, "")
	for _, s := range pub.Series {
		if s.Join != "" {
			t.Fatalf("public series (%d,%d-%d) carries a Join", s.Round, s.A, s.B)
		}
	}
	// the lobby lists no event table
	rr := runOf(t, p.h, id)
	rr.mu.Lock()
	var gameTables []host.TableID
	for _, en := range rr.ev.Log() {
		if en.Kind == event.KindGameStarted {
			gameTables = append(gameTables, host.TableID(en.Table))
		}
	}
	rr.mu.Unlock()
	for _, table := range gameTables {
		for _, ti := range p.reg.LobbyTables() {
			if host.TableID(ti.ID) == table {
				t.Fatalf("LobbyTables lists event table %s", table)
			}
		}
	}
}

func TestCrashedGamesRetryThenDraw(t *testing.T) {
	p := startGamePair(t, "", func(eo *Options, ho *host.Options) {
		eo.MaxAttempts = 2
		ho.MaxDecisionsPerTurn = 1 // every game stalls in its first turn
	})
	r := playBotEvent(t, p)

	r.mu.Lock()
	log := r.ev.Log()
	r.mu.Unlock()
	if len(log) == 0 || log[len(log)-1].Kind != event.KindEventEnded {
		t.Fatalf("event did not end: last entry %v", log[len(log)-1])
	}
	perSeries := map[[2]int][]event.Entry{}
	firstAbort := ""
	for _, en := range log {
		switch en.Kind {
		case event.KindGameAborted:
			key := [2]int{en.A, en.B}
			perSeries[key] = append(perSeries[key], en)
			if firstAbort == "" {
				firstAbort = en.Reason
			}
		case event.KindGameEnded, event.KindSeriesEnded:
			key := [2]int{en.A, en.B}
			perSeries[key] = append(perSeries[key], en)
		}
	}
	if firstAbort != "crashed" {
		t.Fatalf("first GameAborted reason %q, want \"crashed\"", firstAbort)
	}
	if len(perSeries) != 6 {
		t.Fatalf("%d series logged, want 6", len(perSeries))
	}
	for pair, ens := range perSeries {
		if len(ens) != 4 {
			t.Fatalf("series %v logged %d entries, want 4: %+v", pair, len(ens), ens)
		}
		a1, a2, ge, se := ens[0], ens[1], ens[2], ens[3]
		if a1.Kind != event.KindGameAborted || a1.Attempt != 1 || a1.Reason != "crashed" {
			t.Fatalf("series %v entry 1 = %+v", pair, a1)
		}
		if a2.Kind != event.KindGameAborted || a2.Attempt != 2 || a2.Reason != "crashed" {
			t.Fatalf("series %v entry 2 = %+v", pair, a2)
		}
		if ge.Kind != event.KindGameEnded || ge.Winner != nil {
			t.Fatalf("series %v entry 3 = %+v", pair, ge)
		}
		if se.Kind != event.KindSeriesEnded || se.Score != [2]int{0, 0} {
			t.Fatalf("series %v entry 4 = %+v", pair, se)
		}
	}
}

func TestRestartReplaysInFlightGames(t *testing.T) {
	dir := t.TempDir()
	mut := func(eo *Options, _ *host.Options) {
		eo.MaxHumans = 1
		eo.MintSeat = func(table host.TableID, seat int) (string, error) {
			return fmt.Sprintf("tok-%s-%d", table, seat), nil
		}
	}
	p := startGamePair(t, dir, mut)
	resp := createOK(t, p.h, oneHumanReq())
	id := resp.Event
	tok, _ := humanToken(t, resp)
	joinOK(t, p.h, id, tok, "Ann")
	pickTop(t, p.h, id, tok, 45)
	finishHumanBuild(t, p.h, id, tok)
	waitPhase(t, p.h, id, "ROUND1", 50*time.Second)

	// capture the round-1 attempt-1 facts before the restart
	r := runOf(t, p.h, id)
	r.mu.Lock()
	log1 := r.ev.Log()
	r.mu.Unlock()
	type started struct{ table, seed string }
	start1 := map[[2]int]started{}
	for _, en := range log1 {
		if en.Kind == event.KindGameStarted {
			start1[[2]int{en.A, en.B}] = started{table: en.Table, seed: fmt.Sprint(en.GameSeed)}
		}
	}
	t12 := host.TableID(start1[[2]int{1, 2}].table)
	// wait until the bot series' match finished (its result is buffered but
	// unlogged while the human game is in flight)
	deadline := time.Now().Add(50 * time.Second)
	for {
		mi, err := p.reg.Matches(t12)
		if err != nil {
			t.Fatal(err)
		}
		if len(mi) > 0 && mi[0].State == protocol.MatchFinished {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("(1,2) match never finished: %+v", mi)
		}
		time.Sleep(pollEvery)
	}

	p.h.Close()
	p.reg.Close()

	// reopen: both in-flight attempts are aborted, then replayed on fresh
	// tables with fresh seeds; the buffered (1,2) result is gone
	p2 := startGamePair(t, dir, mut)
	r2 := runOf(t, p2.h, id)
	r2.mu.Lock()
	log2 := r2.ev.Log()
	r2.mu.Unlock()
	start2 := map[[2]int]event.Entry{}
	aborted := map[[2]int][]event.Entry{}
	for _, en := range log2 {
		switch en.Kind {
		case event.KindGameAborted:
			aborted[[2]int{en.A, en.B}] = append(aborted[[2]int{en.A, en.B}], en)
		case event.KindGameStarted:
			start2[[2]int{en.A, en.B}] = en
		}
	}
	for _, pair := range [][2]int{{0, 3}, {1, 2}} {
		ab := aborted[pair]
		if len(ab) != 1 || ab[0].Attempt != 1 || ab[0].Reason != "server restart" {
			t.Fatalf("series %v restart aborts = %+v", pair, ab)
		}
		st2, ok := start2[pair]
		if !ok || st2.Attempt != 2 {
			t.Fatalf("series %v no attempt-2 GameStarted: %+v", pair, st2)
		}
		if st2.Table == start1[pair].table {
			t.Fatalf("series %v attempt 2 reused attempt 1's table %s", pair, st2.Table)
		}
		if fmt.Sprint(st2.GameSeed) == start1[pair].seed {
			t.Fatalf("series %v attempt 2 reused attempt 1's seed", pair)
		}
		found := false
		for _, ti := range p2.reg.Tables() {
			if ti.ID == st2.Table {
				found = true
			}
		}
		if !found {
			t.Fatalf("series %v attempt-2 table %s not registered", pair, st2.Table)
		}
	}
	// the event still waits on its human
	if ph := r2.ev.Phase(); ph != event.RoundPhase(1) {
		t.Fatalf("phase after resume = %s, want ROUND1", ph)
	}
}
