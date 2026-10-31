package eventhost

import (
	"bytes"
	"encoding/json"
	"errors"

	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	_ "github.com/adams-shaun/gorge/bots/all"
	"github.com/adams-shaun/gorge/draft/limited/setdata"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/protocol"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func newRegistry(t *testing.T) *host.Registry {
	t.Helper()
	reg, err := host.New(host.Options{LoadDeck: func(string) (host.Deck, error) {
		return host.Deck{}, host.ErrNotFound
	}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { reg.Close() })
	return reg
}

// newHost builds one Host on a private corpus with the default in-memory
// store; mut, when non-nil, tweaks the options before New.
func newHost(t *testing.T, mut func(*Options)) (*Host, *cards.Registry) {
	t.Helper()
	reg := newRegistry(t)
	corpus := testutil.CorpusRegistry(t)
	store, err := OpenStore("")
	if err != nil {
		t.Fatal(err)
	}
	opts := Options{Registry: reg, Store: store, Cards: corpus, Seed: func() uint64 { return 7 }}
	if mut != nil {
		mut(&opts)
	}
	h, err := New(opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { h.Close() })
	return h, corpus
}

// newHostR is newHost, also returning the registry the host serves.
func newHostR(t *testing.T, mut func(*Options)) (*Host, *host.Registry) {
	t.Helper()
	reg, err := host.New(host.Options{LoadDeck: func(string) (host.Deck, error) {
		return host.Deck{}, host.ErrNotFound
	}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { reg.Close() })
	corpus := testutil.CorpusRegistry(t)
	store, err := OpenStore("")
	if err != nil {
		t.Fatal(err)
	}
	opts := Options{Registry: reg, Store: store, Cards: corpus, Seed: func() uint64 { return 7 }}
	if mut != nil {
		mut(&opts)
	}
	h, err := New(opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { h.Close() })
	return h, reg
}

// oneHumanReq is the standard request: FDN, one human and three default
// bots. The seed comes from the harness's fixed Options.Seed.
func oneHumanReq() protocol.CreateEventRequest {
	return protocol.CreateEventRequest{
		Set:   "FDN",
		Seats: []protocol.CreateEventSeat{{Kind: "human"}, {Kind: "bot"}, {Kind: "bot"}, {Kind: "bot"}},
	}
}

func createOK(t *testing.T, h *Host, req protocol.CreateEventRequest) protocol.CreateEventResponse {
	t.Helper()
	resp, err := h.CreateEvent(req)
	if err != nil {
		t.Fatalf("CreateEvent: %v", err)
	}
	return resp
}

func humanToken(t *testing.T, resp protocol.CreateEventResponse) (string, string) {
	t.Helper()
	for _, inv := range resp.Invites {
		if strings.Contains(inv.Join, "seat=0") {
			return inv.Token, inv.Join
		}
	}
	t.Fatal("no seat-0 invite")
	return "", ""
}

func joinOK(t *testing.T, h *Host, id, tok, name string) {
	t.Helper()
	if _, err := h.JoinEvent(id, tok, protocol.JoinEventRequest{Name: name}); err != nil {
		t.Fatalf("JoinEvent(%s): %v", name, err)
	}
}

func stateOf(t *testing.T, h *Host, id, tok string) protocol.EventState {
	t.Helper()
	st, err := h.EventState(id, tok)
	if err != nil {
		t.Fatalf("EventState: %v", err)
	}
	return st
}

func packOf(t *testing.T, h *Host, id, tok string) protocol.DraftPack {
	t.Helper()
	p, err := h.DraftPack(id, tok)
	if err != nil {
		t.Fatalf("DraftPack: %v", err)
	}
	return p
}

// pickTop drives the human seat through the draft, always taking the top
// card of the offered pack, until the phase leaves DRAFT or limit picks.
func pickTop(t *testing.T, h *Host, id, tok string, limit int) {
	t.Helper()
	for i := 0; i < limit; i++ {
		p := packOf(t, h, id, tok)
		if p.Waiting {
			if st := stateOf(t, h, id, tok); st.Phase != "BUILD" {
				t.Fatalf("draft stalled: phase %s, picks %d", st.Phase, len(p.Pool))
			}
			return
		}
		if _, err := h.Pick(id, tok, protocol.PickRequest{
			Round: p.Round, PickNo: p.PickNo, ID: p.Pack[0].ID,
		}); err != nil {
			t.Fatalf("pick %d: %v", i+1, err)
		}
	}
}

// runOf returns a run for internal assertions. Runs are never removed
// while the host lives, so find's internal mu lock-and-release suffices.
func runOf(t *testing.T, h *Host, id string) *run {
	t.Helper()
	r, _ := h.find(id)
	if r == nil {
		t.Fatalf("event %s not found", id)
	}
	return r
}

func poolOf(t *testing.T, h *Host, id string, seat int) []string {
	t.Helper()
	r := runOf(t, h, id)
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.ev.Pool(seat)))
	for i, ref := range r.ev.Pool(seat) {
		out[i] = ref.ID
	}
	return out
}

func wantErr(t *testing.T, err error, codeName string, status int) {
	t.Helper()
	if err == nil {
		t.Fatalf("want %s, got success", codeName)
	}
	var e *Error
	if !errors.As(err, &e) {
		t.Fatalf("want *Error, got %T: %v", err, err)
	}
	if e.Code() != codeName || e.Status() != status {
		t.Fatalf("want %s/%d, got %s/%d (%v)", codeName, status, e.Code(), e.Status(), e)
	}
}

func TestCreateEventValidation(t *testing.T) {
	h, _ := newHost(t, nil)
	seatReq := func(n int, kind string) protocol.CreateEventRequest {
		req := oneHumanReq()
		req.Seats = nil
		for i := 0; i < n; i++ {
			req.Seats = append(req.Seats, protocol.CreateEventSeat{Kind: kind})
		}
		return req
	}
	twoHumans := oneHumanReq()
	twoHumans.Seats[1] = protocol.CreateEventSeat{Kind: "human"}

	cases := []struct {
		name string
		req  protocol.CreateEventRequest
		want string
	}{
		{"three seats", seatReq(3, "human"), codeBadRequest},
		{"five seats", seatReq(5, "human"), codeBadRequest},
		{"unknown set", func() protocol.CreateEventRequest { r := oneHumanReq(); r.Set = "XXXX"; return r }(), codeBadRequest},
		{"env policy", func() protocol.CreateEventRequest { r := oneHumanReq(); r.Seats[1].Policy = "sb-tactical"; return r }(), codeBadRequest},
		{"unknown policy", func() protocol.CreateEventRequest { r := oneHumanReq(); r.Seats[1].Policy = "not-a-policy"; return r }(), codeBadRequest},
		{"unknown drafter", func() protocol.CreateEventRequest {
			r := oneHumanReq()
			r.Seats[1].Drafter = "not-a-drafter"
			return r
		}(), codeBadRequest},
	}
	for _, tc := range cases {
		_, err := h.CreateEvent(tc.req)
		wantErr(t, err, tc.want, 400)
	}
	// Two humans are legal at the default cap (4, asserted in
	// sideboard_test.go); a cap of 1 refuses the second one.
	lim, _ := newHost(t, func(o *Options) { o.MaxHumans = 1 })
	_, err := lim.CreateEvent(twoHumans)
	wantErr(t, err, codeBadRequest, 400)
	cap, _ := newHost(t, func(o *Options) { o.MaxLiveEvents = 1 })
	createOK(t, cap, oneHumanReq())
	_, err = cap.CreateEvent(oneHumanReq())
	wantErr(t, err, codeCapacity, 409)
}

func TestHumanDraftWithBots(t *testing.T) {
	h, _ := newHost(t, nil)
	resp := createOK(t, h, oneHumanReq())
	tok, url := humanToken(t, resp)
	if url == "" || !strings.Contains(url, resp.Event) {
		t.Fatalf("invite url %q does not name event %s", url, resp.Event)
	}
	if st := stateOf(t, h, resp.Event, tok); st.Phase != "LOBBY" {
		t.Fatalf("phase after create: %s", st.Phase)
	}
	joinOK(t, h, resp.Event, tok, "Ann")
	if st := stateOf(t, h, resp.Event, tok); st.Phase != "DRAFT" {
		t.Fatalf("phase after join: %s", st.Phase)
	}
	pickTop(t, h, resp.Event, tok, 64)
	r := runOf(t, h, resp.Event)
	r.mu.Lock()
	spec, err := setdata.Load(r.meta.Set)
	r.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	for s := 0; s < draftSeats; s++ {
		pool := poolOf(t, h, resp.Event, s)
		if len(pool) != 3*spec.PackSize {
			t.Fatalf("seat %d pool %d cards, want %d", s, len(pool), 3*spec.PackSize)
		}
	}
	st := stateOf(t, h, resp.Event, tok)
	if st.You == nil || *st.You != 0 {
		t.Fatalf("You = %v", st.You)
	}
	if st.Seats[0].Kind != "human" || st.Seats[0].Name != "Ann" || !st.Seats[0].Joined || st.Seats[0].Picks != 3*spec.PackSize {
		t.Fatalf("seat 0 = %+v", st.Seats[0])
	}
	for _, s := range st.Seats[1:] {
		if s.Kind != "bot" || !s.Joined || s.Name == "" {
			t.Fatalf("bot seat = %+v", s)
		}
	}
}

func TestDraftIsDeterministic(t *testing.T) {
	log := func(t *testing.T, h *Host, id string) []byte {
		t.Helper()
		r := runOf(t, h, id)
		r.mu.Lock()
		defer r.mu.Unlock()
		b, err := json.Marshal(r.ev.Log())
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	var first []byte
	for i := 0; i < 2; i++ {
		h, _ := newHost(t, nil)
		resp := createOK(t, h, oneHumanReq())
		tok, _ := humanToken(t, resp)
		joinOK(t, h, resp.Event, tok, "Ann")
		pickTop(t, h, resp.Event, tok, 64)
		b := log(t, h, resp.Event)
		if i == 0 {
			first = b
			continue
		}
		if !bytes.Equal(first, b) {
			t.Fatal("two identical seeded events diverged")
		}
	}
}

func TestPickErrors(t *testing.T) {
	h, _ := newHost(t, nil)
	resp := createOK(t, h, oneHumanReq())
	id, tok := resp.Event, func() string { t2, _ := humanToken(t, resp); return t2 }()
	_, err := h.Pick(id, tok, protocol.PickRequest{Round: 1, PickNo: 1})
	wantErr(t, err, codeConflict, 409)

	joinOK(t, h, id, tok, "Ann")
	_, err = h.Pick(id, "", protocol.PickRequest{Round: 1, PickNo: 1, ID: "x"})
	wantErr(t, err, codeUnauthorized, 401)
	_, err = h.Pick(id, "deadbeef", protocol.PickRequest{Round: 1, PickNo: 1, ID: "x"})
	wantErr(t, err, codeUnauthorized, 401)

	p := packOf(t, h, id, tok)
	if p.Waiting || len(p.Pack) == 0 {
		t.Fatalf("pack: %+v", p)
	}
	badID := "no-such-card"
	_, err = h.Pick(id, tok, protocol.PickRequest{Round: p.Round, PickNo: p.PickNo + 1, ID: p.Pack[0].ID})
	wantErr(t, err, codeStalePick, 409)
	_, err = h.Pick(id, tok, protocol.PickRequest{Round: p.Round, PickNo: p.PickNo, ID: badID})
	if e := (*Error)(nil); errors.As(err, &e) && e.Code() == codeStalePick {
		t.Fatalf("bad id refused as stale: %v", err)
	}
	wantErr(t, err, codeNotInPack, 400)

	// A second human seat needs a host that allows two: after it spends
	// its three round-1 picks the pod must refuse it.
	h2, _ := newHost(t, func(o *Options) { o.MaxHumans = 2 })
	two := oneHumanReq()
	two.Seats[1] = protocol.CreateEventSeat{Kind: "human"}
	resp2 := createOK(t, h2, two)
	tok0, _ := humanToken(t, resp2)
	tok1 := ""
	for _, inv := range resp2.Invites {
		if strings.Contains(inv.Join, "seat=1") {
			tok1 = inv.Token
		}
	}
	if tok1 == "" {
		t.Fatal("no seat-1 invite")
	}
	joinOK(t, h2, resp2.Event, tok0, "Ann")
	joinOK(t, h2, resp2.Event, tok1, "Bob")
	// Seat 1 (human) takes what it can; with a second human still to
	// pick, its queue empties while the draft is live.
	for i := 0; i < 64; i++ {
		p1 := packOf(t, h2, resp2.Event, tok1)
		if p1.Waiting {
			break
		}
		if _, err := h2.Pick(resp2.Event, tok1, protocol.PickRequest{
			Round: p1.Round, PickNo: p1.PickNo, ID: p1.Pack[0].ID,
		}); err != nil {
			t.Fatalf("seat 1 pick %d: %v", i+1, err)
		}
	}
	if st := stateOf(t, h2, resp2.Event, tok1); st.Phase != "DRAFT" {
		t.Fatalf("seat 1 left the draft early: %s", st.Phase)
	}
	_, err = h2.Pick(resp2.Event, tok1, protocol.PickRequest{})
	wantErr(t, err, codeNotYourTurn, 409)
}

func TestResumeMidDraft(t *testing.T) {
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
	pickTop(t, h, resp.Event, tok, 3)
	packBefore := packOf(t, h, resp.Event, tok)
	pool0 := poolOf(t, h, resp.Event, 0)
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
	st := stateOf(t, h2, resp.Event, tok)
	if st.Phase != "DRAFT" {
		t.Fatalf("phase after resume: %s", st.Phase)
	}
	if poolAfter := poolOf(t, h2, resp.Event, 0); !reflect.DeepEqual(pool0, poolAfter) {
		t.Fatal("pool changed across resume")
	}
	if packAfter := packOf(t, h2, resp.Event, tok); !reflect.DeepEqual(packBefore, packAfter) {
		t.Fatal("pack changed across resume")
	}
	pickTop(t, h2, resp.Event, tok, 64)
	if st := stateOf(t, h2, resp.Event, tok); st.Phase != "BUILD" {
		t.Fatalf("phase after resumed draft: %s", st.Phase)
	}
}

func TestTornLogTailRecovered(t *testing.T) {
	dir := t.TempDir()
	store, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	h, _ := newHost(t, func(o *Options) { o.Store = store })
	resp := createOK(t, h, oneHumanReq())
	tok, _ := humanToken(t, resp)
	joinOK(t, h, resp.Event, tok, "Ann")
	pickTop(t, h, resp.Event, tok, 3)
	good := poolOf(t, h, resp.Event, 0)
	h.Close()

	logPath := filepath.Join(dir, resp.Event, "log.jsonl")
	raw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(logPath, append(raw, []byte(`{"kind":"Pic`)...), 0o600); err != nil {
		t.Fatal(err)
	}
	h2, _ := newHost(t, func(o *Options) { o.Store = mustStore(t, dir) })
	if poolAfter := poolOf(t, h2, resp.Event, 0); !reflect.DeepEqual(good, poolAfter) {
		t.Fatal("pool changed across torn-tail resume")
	}
	if _, err := os.Stat(logPath); err != nil {
		t.Fatal(err)
	}
	fixed, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(fixed, raw) {
		t.Fatal("torn tail not truncated from disk")
	}
}

func mustStore(t *testing.T, dir string) *Store {
	t.Helper()
	s, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestInviteTokenNeverStoredInClear(t *testing.T) {
	dir := t.TempDir()
	store, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	h, _ := newHost(t, func(o *Options) { o.Store = store })
	resp := createOK(t, h, oneHumanReq())
	tok, _ := humanToken(t, resp)
	joinOK(t, h, resp.Event, tok, "Ann")
	h.Close()
	for _, name := range []string{"event.json", "log.jsonl"} {
		b, err := os.ReadFile(filepath.Join(dir, resp.Event, name))
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(b, []byte(tok)) {
			t.Fatalf("%s stores the invite token in clear", name)
		}
	}
}

func TestSubscribeFrames(t *testing.T) {
	h, reg := newHostR(t, nil)
	resp := createOK(t, h, oneHumanReq())
	tok, _ := humanToken(t, resp)
	joinOK(t, h, resp.Event, tok, "Ann")

	s0 := reg.OpenSession()
	if err := h.SubscribeEvent(resp.Event, tok, s0.ID); err != nil {
		t.Fatalf("SubscribeEvent: %v", err)
	}
	f1 := readFrame(t, s0)
	if f1.T != protocol.TEventState {
		t.Fatalf("first frame %s", f1.T)
	}
	var st protocol.EventState
	if err := json.Unmarshal(f1.Body, &st); err != nil {
		t.Fatal(err)
	}
	if st.You == nil || *st.You != 0 {
		t.Fatalf("You = %v", st.You)
	}
	f2 := readFrame(t, s0)
	if f2.T != protocol.TPodProgress {
		t.Fatalf("second frame %s", f2.T)
	}
	var prog protocol.PodProgress
	if err := json.Unmarshal(f2.Body, &prog); err != nil {
		t.Fatal(err)
	}
	if prog.Event != resp.Event || len(prog.Picks) != 4 {
		t.Fatalf("pod_progress = %+v", prog)
	}
	f3 := readFrame(t, s0)
	if f3.T != protocol.TDraftPack {
		t.Fatalf("third frame %s", f3.T)
	}
	var pack protocol.DraftPack
	if err := json.Unmarshal(f3.Body, &pack); err != nil {
		t.Fatal(err)
	}
	if pack.Waiting || pack.PickNo != 1 || len(pack.Pack) == 0 {
		t.Fatalf("pack = %+v", pack)
	}
	p := packOf(t, h, resp.Event, tok)
	if _, err := h.Pick(resp.Event, tok, protocol.PickRequest{
		Round: p.Round, PickNo: p.PickNo, ID: p.Pack[0].ID,
	}); err != nil {
		t.Fatal(err)
	}
	kinds := []protocol.FrameType{readFrame(t, s0).T, readFrame(t, s0).T, readFrame(t, s0).T}
	want := []protocol.FrameType{protocol.TEventState, protocol.TPodProgress, protocol.TDraftPack}
	if !reflect.DeepEqual(kinds, want) {
		t.Fatalf("frames after pick = %v, want %v", kinds, want)
	}
	var pack2 protocol.DraftPack
	var f4 = func() []byte {
		// the pack frame body, already read: re-fetch from the run
		r := runOf(t, h, resp.Event)
		r.mu.Lock()
		defer r.mu.Unlock()
		b, err := json.Marshal(r.packFor(0))
		if err != nil {
			t.Fatal(err)
		}
		return b
	}()
	if err := json.Unmarshal(f4, &pack2); err != nil {
		t.Fatal(err)
	}
	if pack2.PickNo != 2 {
		t.Fatalf("second pack pick %d, want 2", pack2.PickNo)
	}

	// A spectator gets event_state and pod_progress but never a draft_pack.
	sp := reg.OpenSession()
	if err := h.SubscribeEvent(resp.Event, "", sp.ID); err != nil {
		t.Fatalf("spectator subscribe: %v", err)
	}
	f := readFrame(t, sp)
	if f.T != protocol.TEventState {
		t.Fatalf("spectator first frame %s", f.T)
	}
	var sst protocol.EventState
	if err := json.Unmarshal(f.Body, &sst); err != nil {
		t.Fatal(err)
	}
	if sst.You != nil {
		t.Fatal("spectator state carries You")
	}
	if f2 := readFrame(t, sp); f2.T != protocol.TPodProgress {
		t.Fatalf("spectator second frame %s", f2.T)
	}
	p2 := packOf(t, h, resp.Event, tok)
	if _, err := h.Pick(resp.Event, tok, protocol.PickRequest{
		Round: p2.Round, PickNo: p2.PickNo, ID: p2.Pack[0].ID,
	}); err != nil {
		t.Fatal(err)
	}
	k1, k2 := readFrame(t, sp).T, readFrame(t, sp).T
	if k1 != protocol.TEventState || k2 != protocol.TPodProgress {
		t.Fatalf("spectator frames after pick = %s,%s", k1, k2)
	}
}

func readFrame(t *testing.T, s *host.Session) protocol.Frame {
	t.Helper()
	select {
	case f := <-s.Out():
		return f
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for a frame")
		return protocol.Frame{}
	}
}
