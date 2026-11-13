package eventhost

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/draft/event"
	"github.com/adams-shaun/gorge/draft/limited/build"
	"github.com/adams-shaun/gorge/server/protocol"
)

// formatReq is oneHumanReq naming a format.
func formatReq(format string) protocol.CreateEventRequest {
	req := oneHumanReq()
	req.Format = format
	return req
}

// packsPerSeat counts the PackOpened entries of each seat in the event's log.
func packsPerSeat(t *testing.T, h *Host, id string) [4]int {
	t.Helper()
	r := runOf(t, h, id)
	r.mu.Lock()
	defer r.mu.Unlock()
	var n [4]int
	for _, en := range r.ev.Log() {
		if en.Kind == event.KindPackOpened {
			n[en.Seat]++
		}
	}
	return n
}

// createdEntry is the event's EventCreated log entry.
func createdEntry(t *testing.T, h *Host, id string) event.Entry {
	t.Helper()
	r := runOf(t, h, id)
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, en := range r.ev.Log() {
		if en.Kind == event.KindEventCreated {
			return en
		}
	}
	t.Fatal("no EventCreated in the log")
	return event.Entry{}
}

func TestCreatePrereleaseEvent(t *testing.T) {
	h, _ := newHost(t, nil)
	resp := createOK(t, h, formatReq("prerelease"))
	tok, _ := humanToken(t, resp)
	if st := stateOf(t, h, resp.Event, tok); st.Phase != "LOBBY" {
		t.Fatalf("phase before join: %s", st.Phase)
	}
	joinOK(t, h, resp.Event, tok, "Ann")
	st := stateOf(t, h, resp.Event, tok)
	if st.Format != "prerelease" || st.BestOf != 3 || st.Phase != "BUILD" {
		t.Fatalf("state: format %q best_of %d phase %s", st.Format, st.BestOf, st.Phase)
	}
	en := createdEntry(t, h, resp.Event)
	if en.Mode != event.ModeSealed || en.Promos != 1 {
		t.Fatalf("EventCreated mode %q promos %d", en.Mode, en.Promos)
	}
	for s, n := range packsPerSeat(t, h, resp.Event) {
		if n != 7 {
			t.Fatalf("seat %d: %d PackOpened, want 7", s, n)
		}
	}
	pool, err := h.EventPool(resp.Event, tok)
	if err != nil {
		t.Fatal(err)
	}
	if len(pool.Pool) < 7 {
		t.Fatalf("pool has %d cards", len(pool.Pool))
	}
	promos := 0
	for _, c := range pool.Pool {
		if c.Promo {
			promos++
			if c.Rarity != "R" && c.Rarity != "M" {
				t.Fatalf("promo %s has rarity %s", c.Name, c.Rarity)
			}
		}
	}
	if promos != 1 {
		t.Fatalf("%d promo cards in the pool, want 1", promos)
	}
}

func TestCreatePrereleaseBestOfOneHonoured(t *testing.T) {
	h, _ := newHost(t, nil)
	req := formatReq("prerelease")
	req.BestOf = 1
	resp := createOK(t, h, req)
	tok, _ := humanToken(t, resp)
	if got := runOf(t, h, resp.Event).meta.BestOf; got != 1 {
		t.Fatalf("stored best_of %d, want 1", got)
	}
	if st := stateOf(t, h, resp.Event, tok); st.BestOf != 1 {
		t.Fatalf("reported best_of %d, want 1", st.BestOf)
	}
}

func TestCreateSealedEvent(t *testing.T) {
	h, _ := newHost(t, nil)
	req := formatReq("sealed")
	req.PickTimer = "standard"
	req.Seats[1] = protocol.CreateEventSeat{Kind: "human"}
	resp := createOK(t, h, req)
	if got := runOf(t, h, resp.Event).meta.PickTimer; got != "off" {
		t.Fatalf("stored pick timer %q, want off", got)
	}
	var toks [2]string
	for _, inv := range resp.Invites {
		toks[inv.Seat] = inv.Token
	}
	joinOK(t, h, resp.Event, toks[0], "Ann")
	joinOK(t, h, resp.Event, toks[1], "Bo")
	st := stateOf(t, h, resp.Event, toks[0])
	if st.Format != "sealed" || st.BestOf != 1 || st.Phase != "BUILD" || st.PickTimer != "off" {
		t.Fatalf("state: format %q best_of %d phase %s timer %q", st.Format, st.BestOf, st.Phase, st.PickTimer)
	}
	for s, n := range packsPerSeat(t, h, resp.Event) {
		if n != 6 {
			t.Fatalf("seat %d: %d PackOpened, want 6", s, n)
		}
	}
	pool, err := h.EventPool(resp.Event, toks[0])
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range pool.Pool {
		if c.Promo {
			t.Fatalf("sealed pool carries promo %s", c.Name)
		}
	}
	if en := createdEntry(t, h, resp.Event); en.Mode != event.ModeSealed || en.Promos != 0 {
		t.Fatalf("EventCreated mode %q promos %d", en.Mode, en.Promos)
	}
}

func TestCreateEventRefusesUnknownFormat(t *testing.T) {
	h, _ := newHost(t, nil)
	_, err := h.CreateEvent(formatReq("cube"))
	wantErr(t, err, "bad_request", 400)
	if want := `eventhost: format must be draft, sealed, prerelease or commander, got "cube"`; !strings.Contains(err.Error(), want) {
		t.Fatalf("error %q lacks %q", err, want)
	}
}

func TestSealedEventRefusesPick(t *testing.T) {
	h, _ := newHost(t, nil)
	resp := createOK(t, h, formatReq("sealed"))
	tok, _ := humanToken(t, resp)
	joinOK(t, h, resp.Event, tok, "Ann")
	_, err := h.Pick(resp.Event, tok, protocol.PickRequest{Round: 1, PickNo: 1, ID: "x"})
	wantErr(t, err, "conflict", 409)
}

func TestDraftFormatUnchanged(t *testing.T) {
	h, _ := newHost(t, nil)
	resp := createOK(t, h, formatReq("draft"))
	tok, _ := humanToken(t, resp)
	joinOK(t, h, resp.Event, tok, "Ann")
	if got := runOf(t, h, resp.Event).meta.Format; got != "" {
		t.Fatalf("stored format %q, want empty", got)
	}
	en := createdEntry(t, h, resp.Event)
	if en.Mode != "" || en.Promos != 0 {
		t.Fatalf("EventCreated mode %q promos %d", en.Mode, en.Promos)
	}
	st := stateOf(t, h, resp.Event, tok)
	if st.Phase != "DRAFT" {
		t.Fatalf("phase %s", st.Phase)
	}
	b, err := json.Marshal(st)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), `"format"`) {
		t.Fatalf("draft state carries format: %s", b)
	}
	for s, n := range packsPerSeat(t, h, resp.Event) {
		if n != 3 {
			t.Fatalf("seat %d: %d PackOpened, want 3", s, n)
		}
	}
}

func TestPrereleaseEventResumes(t *testing.T) {
	dir := t.TempDir()
	h, corpus := newHost(t, func(o *Options) { o.Store = mustStore(t, dir) })
	resp := createOK(t, h, formatReq("prerelease"))
	tok, _ := humanToken(t, resp)
	joinOK(t, h, resp.Event, tok, "Ann")
	before, err := h.EventPool(resp.Event, tok)
	if err != nil {
		t.Fatal(err)
	}
	h.Close()

	h2, err := New(Options{Registry: newRegistry(t), Store: mustStore(t, dir), Cards: corpus})
	if err != nil {
		t.Fatalf("resume: %v", err)
	}
	t.Cleanup(func() { h2.Close() })
	st := stateOf(t, h2, resp.Event, tok)
	if st.Phase != "BUILD" || st.Format != "prerelease" || st.BestOf != 3 {
		t.Fatalf("resumed state: phase %s format %q best_of %d", st.Phase, st.Format, st.BestOf)
	}
	after, err := h2.EventPool(resp.Event, tok)
	if err != nil {
		t.Fatal(err)
	}
	if len(before.Pool) == 0 || !reflect.DeepEqual(before.Pool, after.Pool) {
		t.Fatal("pool (IDs and promo flag) changed across resume")
	}
}

func TestPrereleaseBotsBuildLegalDecks(t *testing.T) {
	h, _ := newHost(t, nil)
	resp := createOK(t, h, formatReq("prerelease"))
	tok, _ := humanToken(t, resp)
	joinOK(t, h, resp.Event, tok, "Ann")
	r := runOf(t, h, resp.Event)
	r.mu.Lock()
	defer r.mu.Unlock()
	for s := 1; s < 4; s++ {
		pool := r.ev.Pool(s)
		if len(pool) < 7 {
			t.Fatalf("seat %d pool has %d cards", s, len(pool))
		}
		d, ok := r.ev.SavedDeck(s)
		if !ok || r.ev.ReadyRev(s) == 0 {
			t.Fatalf("seat %d: deck saved %v, ready rev %d", s, ok, r.ev.ReadyRev(s))
		}
		if err := build.Validate(pool, d); err != nil {
			t.Fatalf("seat %d deck: %v", s, err)
		}
	}
}
