package eventhost

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/adams-shaun/gorge/bots/all"
	"github.com/adams-shaun/gorge/draft/event"
	"github.com/adams-shaun/gorge/draft/limited/build"
	"github.com/adams-shaun/gorge/engine/deck"
	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/protocol"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func TestBotOnlyEventReachesRound1(t *testing.T) {
	dir := t.TempDir()
	store, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	h, corpus := newHost(t, func(o *Options) { o.Store = store })
	req := protocol.CreateEventRequest{Set: "FDN"}
	for _, p := range []string{"bot", "lethal-pressure", "forge-heuristic", "bot"} {
		req.Seats = append(req.Seats, protocol.CreateEventSeat{Kind: "bot", Policy: p})
	}
	resp := createOK(t, h, req)
	if st := stateOf(t, h, resp.Event, ""); st.Phase != "ROUND1" {
		t.Fatalf("phase after an all-bot create: %s, want ROUND1", st.Phase)
	}
	for s := 0; s < draftSeats; s++ {
		r := runOf(t, h, resp.Event)
		if got := r.ev.SavedRev(s); got != 1 {
			t.Fatalf("seat %d SavedRev = %d, want 1", s, got)
		}
		if got := r.ev.ReadyRev(s); got != 1 {
			t.Fatalf("seat %d ReadyRev = %d, want 1", s, got)
		}
		path := filepath.Join(dir, resp.Event, "decks", fmt.Sprintf("%d-1.json", s))
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("seat %d deck file %s: %v", s, path, err)
		}
		d, err := store.DeckLoader(corpus, nil)(DeckID(resp.Event, s, 1))
		if err != nil {
			t.Fatalf("DeckLoader seat %d: %v", s, err)
		}
		if len(d.Cards) < 40 {
			t.Fatalf("seat %d loaded %d cards, want at least 40", s, len(d.Cards))
		}
	}
}

func TestHumanBuildFlow(t *testing.T) {
	h, _ := newHost(t, nil)
	resp := createOK(t, h, oneHumanReq())
	id, tok := resp.Event, func() string { tk, _ := humanToken(t, resp); return tk }()
	if _, err := h.EventPool(id, tok); err == nil {
		t.Fatal("EventPool before the draft succeeded")
	} else {
		wantErr(t, err, codeConflict, 409)
	}
	joinOK(t, h, id, tok, "Ann")
	pickTop(t, h, id, tok, 64)
	if st := stateOf(t, h, id, tok); st.Phase != "BUILD" {
		t.Fatalf("phase after the draft: %s, want BUILD", st.Phase)
	}

	pool, err := h.EventPool(id, tok)
	if err != nil {
		t.Fatalf("EventPool: %v", err)
	}
	if len(pool.Pool) == 0 {
		t.Fatal("EventPool pool is empty: precondition missing")
	}
	if pool.Deck != nil || pool.Rev != 0 || pool.Ready {
		t.Fatalf("EventPool before a save: deck %+v rev %d ready %v", pool.Deck, pool.Rev, pool.Ready)
	}
	if len(pool.Suggestion.Main) == 0 {
		t.Fatal("EventPool suggestion is empty: the builder did not run")
	}
	check, err := h.CheckDeck(id, tok, pool.Suggestion)
	if err != nil {
		t.Fatalf("CheckDeck(suggestion): %v", err)
	}
	if !check.Valid || check.Total != (build.Deck{Main: pool.Suggestion.Main, Basics: pool.Suggestion.Basics}).Total() {
		t.Fatalf("suggestion check: %+v", check)
	}

	short := protocol.DeckRequest{Main: pool.Suggestion.Main[:10]}
	if len(short.Main) != 10 {
		t.Fatal("pool has fewer than 10 cards: precondition missing")
	}
	shortCheck, err := h.CheckDeck(id, tok, short)
	if err != nil {
		t.Fatalf("CheckDeck(short): %v", err)
	}
	if shortCheck.Valid {
		t.Fatalf("10-card main checked valid: %+v", shortCheck)
	}
	lands := 0
	for _, n := range shortCheck.AutoLands {
		lands += n
	}
	if lands != 30 {
		t.Fatalf("10-card main auto-lands sum %d, want 30", lands)
	}

	logged := func() int {
		t.Helper()
		l, err := h.EventLog(id, tok)
		if err != nil {
			t.Fatalf("EventLog: %v", err)
		}
		return len(l.Entries)
	}
	before := logged()
	if _, err := h.SaveDeck(id, tok, short); err == nil {
		t.Fatal("SaveDeck(short) succeeded")
	} else {
		wantErr(t, err, codeInvalidDeck, 422)
	}
	if after := logged(); after != before {
		t.Fatalf("a refused save logged %d new entries", after-before)
	}

	saved, err := h.SaveDeck(id, tok, pool.Suggestion)
	if err != nil {
		t.Fatalf("SaveDeck(suggestion): %v", err)
	}
	if saved.Rev != 1 || !saved.Valid {
		t.Fatalf("SaveDeck reply: %+v", saved)
	}
	pool2, err := h.EventPool(id, tok)
	if err != nil {
		t.Fatalf("EventPool after save: %v", err)
	}
	if pool2.Rev != 1 || pool2.Deck == nil || len(pool2.Deck.Main) != len(pool.Suggestion.Main) {
		t.Fatalf("EventPool after save: rev %d deck %+v", pool2.Rev, pool2.Deck)
	}

	if _, err := h.Ready(id, tok, protocol.ReadyRequest{Rev: 2}); err == nil {
		t.Fatal("Ready(rev 2) succeeded")
	} else {
		wantErr(t, err, codeConflict, 409)
	}
	st, err := h.Ready(id, tok, protocol.ReadyRequest{Rev: 1})
	if err != nil {
		t.Fatalf("Ready(rev 1): %v", err)
	}
	if st.Phase != "ROUND1" {
		t.Fatalf("phase after the last ready: %s, want ROUND1", st.Phase)
	}
	if _, err := h.SaveDeck(id, tok, pool.Suggestion); err == nil {
		t.Fatal("SaveDeck after the rounds opened succeeded")
	} else {
		wantErr(t, err, codeConflict, 409)
	}
}

func TestDeckLoaderChains(t *testing.T) {
	corpus := testutil.CorpusRegistry(t)
	store, err := OpenStore("")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.writeDeck("e1", 0, 1, deck.File{
		Name:  "loader test",
		Cards: []deck.Entry{{Name: "Plains", Count: 40}},
	}); err != nil {
		t.Fatal(err)
	}

	loader := store.DeckLoader(corpus, nil)
	if _, err := loader("Constructed/foo"); !errors.Is(err, host.ErrNotFound) {
		t.Fatalf("nil next, unknown id: %v", err)
	}
	for _, bad := range []string{"ev/../x", "ev/e1/7/1", "ev/zz/0/1", "ev/e1/0/0"} {
		if _, err := loader(bad); err == nil {
			t.Fatalf("bad event id %q accepted", bad)
		}
	}
	d, err := loader(DeckID("e1", 0, 1))
	if err != nil {
		t.Fatalf("DeckLoader(DeckID): %v", err)
	}
	if len(d.Cards) != 40 {
		t.Fatalf("loaded %d cards, want 40", len(d.Cards))
	}

	calls := 0
	next := func(id string) (host.Deck, error) {
		calls++
		if id == "known" {
			return host.Deck{Name: "next deck"}, nil
		}
		return host.Deck{}, host.ErrNotFound
	}
	loader2 := store.DeckLoader(corpus, next)
	if got, err := loader2("known"); err != nil || got.Name != "next deck" {
		t.Fatalf("next-served id: %v %+v", err, got)
	}
	if calls != 1 {
		t.Fatalf("next called %d times", calls)
	}
	if _, err := loader2("ev/../x"); err == nil {
		t.Fatal("ev/../x accepted")
	}
	if _, err := loader2("ev/e1/7/1"); err == nil {
		t.Fatal("seat 7 accepted")
	}
	if _, err := loader2(DeckID("e1", 1, 9)); err == nil {
		t.Fatal("missing revision accepted")
	}
	if calls != 1 {
		t.Fatalf("next was called for ev/ ids (%d extra calls)", calls-1)
	}
}

func TestEventLogRedactedForSeat(t *testing.T) {
	h, _ := newHost(t, nil)
	resp := createOK(t, h, oneHumanReq())
	id, tok := resp.Event, func() string { tk, _ := humanToken(t, resp); return tk }()
	joinOK(t, h, id, tok, "Ann")
	pickTop(t, h, id, tok, 64)

	parse := func(t *testing.T, l protocol.EventLog) []event.Entry {
		t.Helper()
		var out []event.Entry
		for i, raw := range l.Entries {
			var en event.Entry
			if err := json.Unmarshal(raw, &en); err != nil {
				t.Fatalf("entry %d: %v", i, err)
			}
			out = append(out, en)
		}
		return out
	}
	seatLog, err := h.EventLog(id, tok)
	if err != nil {
		t.Fatalf("EventLog: %v", err)
	}
	entries := parse(t, seatLog)
	ownPick, otherPick, otherPack := 0, 0, 0
	for _, en := range entries {
		switch en.Kind {
		case event.KindPicked:
			if en.Seat == 0 {
				if en.ID == "" {
					t.Fatal("seat 0's own pick id is hidden from itself")
				}
				ownPick++
			} else if en.ID != "" {
				t.Fatalf("log leaked seat %d's pick id %q", en.Seat, en.ID)
			} else {
				otherPick++
			}
		case event.KindPackOpened:
			if en.Seat == 0 {
				if len(en.Cards) == 0 {
					t.Fatal("seat 0's own pack is hidden from itself")
				}
			} else if len(en.Cards) != 0 {
				t.Fatalf("log leaked seat %d's pack (%d cards)", en.Seat, len(en.Cards))
			} else {
				otherPack++
			}
		}
	}
	if ownPick == 0 || otherPick == 0 || otherPack == 0 {
		t.Fatalf("precondition missing: own picks %d, redacted picks %d, redacted packs %d", ownPick, otherPick, otherPack)
	}
	if seatLog.Complete {
		t.Fatal("log says complete before the event is")
	}

	specLog, err := h.EventLog(id, "")
	if err != nil {
		t.Fatalf("spectator EventLog: %v", err)
	}
	sawPack := false
	for _, en := range parse(t, specLog) {
		if en.Kind == event.KindPicked && en.ID != "" {
			t.Fatalf("spectator log leaked seat %d's pick id %q", en.Seat, en.ID)
		}
		if en.Kind == event.KindPackOpened && len(en.Cards) > 0 {
			sawPack = true
		}
	}
	if sawPack {
		t.Fatal("spectator log carries another seat's pack cards")
	}
}

func TestResumeMidBuild(t *testing.T) {
	dir := t.TempDir()
	store, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	reg := newRegistry(t)
	corpus := testutil.CorpusRegistry(t)
	h, err := New(Options{Registry: reg, Store: store, Cards: corpus})
	if err != nil {
		t.Fatal(err)
	}
	resp := createOK(t, h, oneHumanReq())
	tok, _ := humanToken(t, resp)
	joinOK(t, h, resp.Event, tok, "Ann")
	pickTop(t, h, resp.Event, tok, 64)
	pool, err := h.EventPool(resp.Event, tok)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.SaveDeck(resp.Event, tok, pool.Suggestion); err != nil {
		t.Fatalf("SaveDeck: %v", err)
	}
	h.Close()

	store2, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	h2, err := New(Options{Registry: newRegistry(t), Store: store2, Cards: corpus})
	if err != nil {
		t.Fatalf("resume: %v", err)
	}
	t.Cleanup(func() { h2.Close() })
	r := runOf(t, h2, resp.Event)
	if got := r.ev.SavedRev(0); got != 1 {
		t.Fatalf("human SavedRev after resume = %d, want 1", got)
	}
	if got := r.ev.ReadyRev(0); got != 0 {
		t.Fatalf("human ReadyRev after resume = %d, want 0", got)
	}
	if st := stateOf(t, h2, resp.Event, tok); st.Phase != "BUILD" {
		t.Fatalf("phase after resume: %s, want BUILD", st.Phase)
	}
	st, err := h2.Ready(resp.Event, tok, protocol.ReadyRequest{Rev: 1})
	if err != nil {
		t.Fatalf("Ready after resume: %v", err)
	}
	if st.Phase != "ROUND1" {
		t.Fatalf("phase after the resumed ready: %s, want ROUND1", st.Phase)
	}
}

// TestBuildResponsesHideOtherSeatsPools serialises the BUILD-phase seat-0
// bodies and asserts none of seat 1's pool ids appear: a seat response
// never carries another seat's cards before COMPLETE.
func TestBuildResponsesHideOtherSeatsPools(t *testing.T) {
	h, _ := newHost(t, nil)
	resp := createOK(t, h, oneHumanReq())
	tok, _ := humanToken(t, resp)
	joinOK(t, h, resp.Event, tok, "Ann")
	pickTop(t, h, resp.Event, tok, 64)
	r := runOf(t, h, resp.Event)
	r.mu.Lock()
	own := map[string]bool{}
	for _, en := range r.ev.Log() {
		if en.Kind == event.KindPackOpened && en.Seat == 0 {
			for _, c := range en.Cards {
				own[c.ID] = true
			}
		}
	}
	hidden := map[string]bool{}
	for _, ref := range r.ev.Pool(1) {
		if !own[ref.ID] {
			hidden[ref.ID] = true
		}
	}
	r.mu.Unlock()
	if len(hidden) == 0 {
		t.Fatal("no seat-1 pool ids collected: precondition missing")
	}
	pool, err := h.EventPool(resp.Event, tok)
	if err != nil {
		t.Fatalf("EventPool: %v", err)
	}
	if len(pool.Suggestion.Main) == 0 {
		t.Fatal("suggestion is empty: the builder did not run")
	}
	check, err := h.CheckDeck(resp.Event, tok, protocol.DeckRequest{Main: pool.Suggestion.Main})
	if err != nil {
		t.Fatalf("CheckDeck: %v", err)
	}
	log, err := h.EventLog(resp.Event, tok)
	if err != nil {
		t.Fatalf("EventLog: %v", err)
	}
	for _, body := range []struct {
		name string
		b    []byte
	}{
		{"event_pool", mustJSON(t, pool)},
		{"deck_check", mustJSON(t, check)},
		{"event_log", mustJSON(t, log)},
	} {
		for id := range hidden {
			if strings.Contains(string(body.b), `"`+id+`"`) {
				t.Fatalf("%s for seat 0 leaks seat 1's card %q: %s", body.name, id, body.b)
			}
		}
	}
}
