package httpapi

// The seat endpoints. The four merged Registry methods
// (ViewAtSeat, EventsSeat, Pending, SubmitIntent) are plumbed onto the REST
// surface behind an optional ?seat= parameter, gated by a separate claim
// resolver (Options.Seat) that never touches Authorize.
//
// The tests pin the redaction property, not the status codes: seat A cannot
// read seat B's hand or seat B's pending decision through any of the four
// endpoints. The fixture is deliberately a HUMAN-seated table whose seats 0
// and 1 park on the first decision asked of either, played head to that
// point — the openers are still in hand, so a "card that entered another
// seat's hidden zone never surfaces" is observable in the event stream too,
// and the table's spectator visibility is omniscient on purpose: the seat
// path must redact even where the spectator god view would not.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/protocol"
)

// claimResolver resolves a request to the seat its X-Seat-Session header
// names — the local-gorged convention ("the claim comes from the existing
// session: a session claims a seat and holds it"), with the map standing in
// for the session store gorged owns. ok is false for an unknown
// session: the request presents no claim.
func claimResolver(by map[string]state.PlayerID) func(*http.Request) (SeatClaim, bool) {
	return func(r *http.Request) (SeatClaim, bool) {
		s, ok := by[r.Header.Get("X-Seat-Session")]
		if !ok {
			return SeatClaim{}, false
		}
		return SeatClaim{Table: "t1", Seat: s}, true
	}
}

// parkedSeatServer starts a 4-seat table with seats 0 and 1 human (via
// TableConfig.Humans). It parks on the first decision asked of either human
// — ThinkTimeout 0 means a human never answers on its own — and stays
// parked; bot slots 2 and 3 answer their own decisions, so the parked point
// is the first human decision, where both human openers are still in hand.
// claims maps X-Seat-Session to the seat it holds on t1 and is installed as
// Options.Seat; nil claims means no resolver at all, the spectator-only
// default. The registry is closed on cleanup; with the table's context
// cancelled the parked seat falls to its deterministic caretaker and the
// loop's stop check aborts the match, so cleanup never plays the game out.
func parkedSeatServer(t *testing.T, claims map[string]state.PlayerID) (*httptest.Server, *host.Registry) {
	t.Helper()
	r, err := host.New(host.Options{LoadDeck: loader(t), Sleep: func(time.Duration, <-chan struct{}) {}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close() })
	cfg := host.TableConfig{ID: "t1", Name: "Table 1", Seats: 4, Decks: []string{"a", "b", "c", "d"},
		Seed: 5, Spectator: view.Omniscient, Humans: []int{0, 1}}
	if err := r.AddTable(cfg); err != nil {
		t.Fatal(err)
	}
	if err := r.Start("t1"); err != nil {
		t.Fatal(err)
	}
	o := Options{}
	if claims != nil {
		o.Seat = claimResolver(claims)
	}
	srv := httptest.NewServer(NewHandler(r, o))
	t.Cleanup(srv.Close)
	return srv, r
}

// tableClaimServer has two tables but only starts t1, which is enough to
// prove the authorisation boundary: every t2 request must be rejected before
// a missing match or pending decision can reveal whether t2 has one.
func tableClaimServer(t *testing.T) (*httptest.Server, *host.Registry) {
	t.Helper()
	r, err := host.New(host.Options{LoadDeck: loader(t), Sleep: func(time.Duration, <-chan struct{}) {}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close() })
	for _, id := range []host.TableID{"t1", "t2"} {
		cfg := host.TableConfig{ID: id, Name: string(id), Seats: 4, Decks: []string{"a", "b", "c", "d"},
			Seed: 5, Spectator: view.Omniscient, Humans: []int{0}}
		if err := r.AddTable(cfg); err != nil {
			t.Fatal(err)
		}
	}
	if err := r.Start("t1"); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(NewHandler(r, Options{Seat: func(r *http.Request) (SeatClaim, bool) {
		switch r.Header.Get("X-Seat-Session") {
		case "t1-seat0":
			return SeatClaim{Table: "t1", Seat: 0}, true
		case "legacy-seat0":
			return SeatClaim{Seat: 0}, true // the pre-deploy token shape
		default:
			return SeatClaim{}, false
		}
	}}))
	t.Cleanup(srv.Close)
	return srv, r
}

// parkedSeat waits until exactly one human seat has a decision pending and
// returns it and the other human seat. The probe is the registry's own
// Pending, not the HTTP layer, so a test can name the parked seat before it
// drives it over HTTP. Which of the two is asked first is the engine's
// business; the test must not care.
func parkedSeat(t *testing.T, r *host.Registry) (parked, other state.PlayerID) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for {
		if time.Now().After(deadline) {
			t.Fatal("no human seat parked on a decision within deadline")
		}
		if _, err := r.Pending("t1", 1, 0); err == nil {
			return 0, 1
		}
		if _, err := r.Pending("t1", 1, 1); err == nil {
			return 1, 0
		}
		time.Sleep(time.Millisecond)
	}
}

// pastStartingChoice parks a human seat on a decision AFTER the CR 103.1
// starting-player choice. While that choice is pending no seat may see any
// hand card (CR 103.5), so a test about hand redaction between seats
// has to look at the next decision (the London mulligan), where each seat
// sees its own opening hand again. The choice is answered over HTTP by the
// seat that owns it, exactly as a client would.
func pastStartingChoice(t *testing.T, srv *httptest.Server, r *host.Registry) (parked, other state.PlayerID) {
	t.Helper()
	for i := 0; i < 8; i++ {
		parked, other = parkedSeat(t, r)
		d, err := r.Pending("t1", 1, parked)
		if err != nil {
			t.Fatalf("pending for parked seat %d: %v", parked, err)
		}
		if d.Kind != decision.KStartingPlayer {
			return parked, other
		}
		sess := "s" + strconv.Itoa(int(parked))
		if code, e, _ := seatReq(t, http.MethodPost, srv.URL+"/api/tables/t1/matches/1/intent", sess, answerFor(d)); code >= 300 {
			t.Fatalf("answering the starting-player choice: %d %+v", code, e)
		}
		// Wait for the head to move past the answered choice before re-parking.
		deadline := time.Now().Add(30 * time.Second)
		for {
			nd, err := r.Pending("t1", 1, parked)
			if err != nil || nd.Seq != d.Seq {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("the starting-player answer was never consumed")
			}
			time.Sleep(time.Millisecond)
		}
	}
	t.Fatal("still parked on a starting-player choice after 8 answers")
	return 0, 0
}

// seatReq runs an HTTP request carrying the X-Seat-Session claim header
// ("" = no claim presented, the spectator path) and returns the status, the
// decoded error body (zero when the status is < 400) and the raw body, so a
// caller can decode into its own type or byte-compare directly.
func seatReq(t *testing.T, method, url, session string, body any) (int, protocol.ErrorBody, []byte) {
	t.Helper()
	var rd io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		rd = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, url, rd)
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if session != "" {
		req.Header.Set("X-Seat-Session", session)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode >= 400 {
		var e protocol.ErrorBody
		_ = json.Unmarshal(raw, &e)
		return resp.StatusCode, e, raw
	}
	return resp.StatusCode, protocol.ErrorBody{}, raw
}

// answerFor builds a valid intent for d: the first Min options, legal for
// any Decision.Kind. The HTTP client's whole rules knowledge is "echo the
// pending decision"; this is that echo.
func answerFor(d *decision.Decision) decision.Intent {
	n := d.Min
	if n > len(d.Options) {
		n = len(d.Options)
	}
	ch := make([]int, 0, n)
	for i := 0; i < n; i++ {
		ch = append(ch, d.Options[i].Index)
	}
	return decision.Intent{Seq: d.Seq, Player: d.Player, Choices: ch}
}

// liveHead is the parked match's chain head, the seq ViewAtSeat attaches
// the pending decision at.
func liveHead(t *testing.T, r *host.Registry) uint64 {
	t.Helper()
	ms, err := r.Matches("t1")
	if err != nil || len(ms) != 1 {
		t.Fatalf("live matches: %v %+v", err, ms)
	}
	if ms[0].Events < 1 {
		t.Fatalf("live match has no events: %+v", ms[0])
	}
	return uint64(ms[0].Events - 1)
}

var seatClaims = map[string]state.PlayerID{"s0": 0, "s1": 1}

// TestTableBoundSeatClaimRejectsEveryOtherTable is the table-driven audit of
// the entire current seat-authorised REST surface. Each request carries the
// exact same table-A token yet is rejected before table B's match state is
// consulted; the companion table-A request reaches its normal handler. Add a
// new seat-authorised route here with its registration so this boundary stays
// reviewed as one set rather than as a collection of ad hoc checks.
func TestTableBoundSeatClaimRejectsEveryOtherTable(t *testing.T) {
	srv, r := tableClaimServer(t)
	parked, _ := parkedSeat(t, r)
	pending := fmt.Sprintf("/api/tables/t1/matches/1/pending?seat=%d", parked)
	code, _, raw := seatReq(t, http.MethodGet, srv.URL+pending, "t1-seat0", nil)
	if code != http.StatusOK {
		t.Fatalf("table-A pending: %d", code)
	}
	if code, e, _ := seatReq(t, http.MethodGet, srv.URL+pending, "legacy-seat0", nil); code != http.StatusForbidden || e.Code != "forbidden" {
		t.Fatalf("legacy unbound claim on table A: %d %+v", code, e)
	}
	var d decision.Decision
	if err := json.Unmarshal(raw, &d); err != nil {
		t.Fatal(err)
	}

	routes := []struct {
		name, method, path string
		body               any
	}{
		{"view", http.MethodGet, "/api/tables/%s/matches/1/view?seat=0", nil},
		{"events", http.MethodGet, "/api/tables/%s/matches/1/events?seat=0", nil},
		{"pending", http.MethodGet, "/api/tables/%s/matches/1/pending?seat=0", nil},
		{"intent", http.MethodPost, "/api/tables/%s/matches/1/intent", answerFor(&d)},
		{"undo", http.MethodPost, "/api/tables/%s/matches/1/undo", nil},
		{"advice", http.MethodGet, "/api/tables/%s/matches/1/advice?seq=1", nil},
	}
	for _, route := range routes {
		t.Run(route.name, func(t *testing.T) {
			pathB := fmt.Sprintf(route.path, "t2")
			if code, e, _ := seatReq(t, route.method, srv.URL+pathB, "t1-seat0", route.body); code != http.StatusForbidden || e.Code != "forbidden" {
				t.Fatalf("table-B %s with table-A claim: %d %+v", route.name, code, e)
			}
			pathA := fmt.Sprintf(route.path, "t1")
			if code, e, _ := seatReq(t, route.method, srv.URL+pathA, "t1-seat0", route.body); code == http.StatusForbidden || code == http.StatusUnauthorized {
				t.Fatalf("table-A %s did not reach its handler: %d %+v", route.name, code, e)
			}
		})
	}
}

// TestSeatParamAbsentIsByteIdenticalToToday pins the contract
// "?seat= absent keeps today's spectator behaviour exactly" at the byte
// level: a finished match serves identical view and events bodies with and
// without a Seat resolver configured, because the no-?seat= path never
// consults the claim. (The pre-existing tests, which run unchanged, are the
// other half of the pin.) The same resolver that must not disturb the
// spectator path still serves seat-scoped views for a finished match, and
// seat 1 under seat 0's claim is refused.
func TestSeatParamAbsentIsByteIdenticalToToday(t *testing.T) {
	plain, _ := finishedServer(t, Options{})
	seated, r := finishedServer(t, Options{Seat: claimResolver(seatClaims)})

	for _, p := range []string{
		"/api/tables/t1/matches/1/view",
		"/api/tables/t1/matches/1/view?seq=0",
		"/api/tables/t1/matches/1/events",
		"/api/tables/t1/matches/1/events?since=0",
	} {
		ca, _, ra := seatReq(t, http.MethodGet, plain.URL+p, "", nil)
		cb, _, rb := seatReq(t, http.MethodGet, seated.URL+p, "", nil)
		if ca != http.StatusOK || cb != http.StatusOK {
			t.Fatalf("%s: statuses %d %d", p, ca, cb)
		}
		if !bytes.Equal(ra, rb) {
			t.Fatalf("%s: bodies differ with and without a Seat resolver", p)
		}
	}

	// The seated server still serves a seat-scoped view of the finished
	// match: seat 0's own (empty) hand present, seat 1's hand hidden, no
	// decision (a finished match asks nobody), and seat 1 under seat 0's
	// claim is refused outright.
	head := liveHead(t, r)
	var v view.View
	if code, e, raw := seatReq(t, http.MethodGet,
		fmt.Sprintf("%s/api/tables/t1/matches/1/view?seat=0&seq=%d", seated.URL, head), "s0", nil); code != http.StatusOK {
		t.Fatalf("finished seat view: %d %+v", code, e)
	} else if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	if v.Viewer != 0 || v.Visibility != view.Seat.String() {
		t.Fatalf("finished seat view: viewer %d visibility %q", v.Viewer, v.Visibility)
	}
	if v.Players[0].Hand == nil {
		t.Fatal("seat 0's own hand is hidden from it")
	}
	if v.Players[1].Hand != nil {
		t.Fatal("seat 0 reads seat 1's hand through its seat view")
	}
	if v.Decision != nil {
		t.Fatalf("a finished match asks nobody, but the seat view carries %+v", v.Decision)
	}
	if code, e, _ := seatReq(t, http.MethodGet,
		fmt.Sprintf("%s/api/tables/t1/matches/1/view?seat=1", seated.URL), "s0", nil); code != http.StatusForbidden || e.Code != "forbidden" {
		t.Fatalf("seat 0's claim reading seat 1's view: %d %+v", code, e)
	}
	// Pending against a finished match is a 409 conflict with a reason, not
	// a crash and not a silent empty reply.
	if code, e, _ := seatReq(t, http.MethodGet, seated.URL+"/api/tables/t1/matches/1/pending?seat=0", "s0", nil); code != http.StatusConflict || e.Code != "conflict" {
		t.Fatalf("pending on a finished match: %d %+v", code, e)
	}
}

// TestSeatViewRedactsTheOtherSeatsHand is the fixture's core redaction
// assertion on the view endpoint: while the match is parked on one human
// seat, that seat's view shows its own hand and the pending decision asked
// of it, and hides the other human seat's hand entirely; the other seat's
// view is its own game, not the table's. The spectator view (no ?seat=) on
// the same table is the omniscient god view showing BOTH hands — the seat
// path must not degrade to it. And each claim's request for the other
// seat's ?seat= is refused before any projection runs.
func TestSeatViewRedactsTheOtherSeatsHand(t *testing.T) {
	srv, r := parkedSeatServer(t, seatClaims)
	parked, other := pastStartingChoice(t, srv, r)
	ps, os := "s"+strconv.Itoa(int(parked)), "s"+strconv.Itoa(int(other))
	head := liveHead(t, r)

	// The no-?seat= path keeps the table's omniscient spectator view: both
	// human hands visible. This is the "local god view" the fixture exists
	// to prove the seat path does NOT fall back to.
	var spec view.View
	if code, e, raw := seatReq(t, http.MethodGet, srv.URL+"/api/tables/t1/matches/1/view", "", nil); code != http.StatusOK {
		t.Fatalf("spectator view: %d %+v", code, e)
	} else if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatal(err)
	}
	if spec.Visibility != view.Omniscient.String() {
		t.Fatalf("spectator visibility %q, want %q", spec.Visibility, view.Omniscient.String())
	}
	if spec.Players[parked].Hand == nil || spec.Players[other].Hand == nil {
		t.Fatal("the spectator view should show every hand; the test's god view is missing one")
	}

	// The parked seat's own view: its hand, its decision, and no other
	// seat's hand content — the other human seat contributes only a count.
	var pv view.View
	if code, e, raw := seatReq(t, http.MethodGet,
		fmt.Sprintf("%s/api/tables/t1/matches/1/view?seat=%d", srv.URL, parked), ps, nil); code != http.StatusOK {
		t.Fatalf("parked seat view: %d %+v", code, e)
	} else if err := json.Unmarshal(raw, &pv); err != nil {
		t.Fatal(err)
	}
	if pv.Viewer != parked || pv.Visibility != view.Seat.String() {
		t.Fatalf("parked seat view: viewer %d visibility %q", pv.Viewer, pv.Visibility)
	}
	// The fixture parks PAST the CR 103.1 starting-player ask (see
	// pastStartingChoice), so the CR 103.5 pregame blind has lifted by the
	// time this reads: the parked seat's own hand is visible again in full,
	// count matching. While the choice is pending the blind hides it (the
	// shape engine/view/starting_choice_test.go pins) and the seat's own Secret
	// draws are served shape-only (server/host/starting_choice_events_test.go pins
	// both halves of the window).
	if pv.Players[parked].Hand == nil || len(pv.Players[parked].Hand) != int(pv.Players[parked].HandSize) || pv.Players[parked].HandSize == 0 {
		t.Fatalf("the parked seat's own hand is not present: %+v", pv.Players[parked])
	}
	if pv.Players[other].Hand != nil {
		t.Fatalf("seat %d reads seat %d's hand through its own seat view", parked, other)
	}
	if pv.Players[other].HandSize != pv.Players[parked].HandSize {
		t.Fatalf("the hidden seat's count disagrees with the open one: %d vs %d", pv.Players[other].HandSize, pv.Players[parked].HandSize)
	}
	if pv.Decision == nil || pv.Decision.Player != parked || pv.Decision.Seq != head {
		t.Fatalf("the parked seat's view must carry its own pending decision at the head: %+v (head %d)", pv.Decision, head)
	}

	// The other human seat's view is its own game: its hand visible, the
	// parked seat's hand hidden, and no decision attached (it is not the
	// one being asked — the decision is the parked seat's and must not leak
	// through a view built for someone else).
	var ov view.View
	if code, e, raw := seatReq(t, http.MethodGet,
		fmt.Sprintf("%s/api/tables/t1/matches/1/view?seat=%d", srv.URL, other), os, nil); code != http.StatusOK {
		t.Fatalf("other seat view: %d %+v", code, e)
	} else if err := json.Unmarshal(raw, &ov); err != nil {
		t.Fatal(err)
	}
	// The other seat's view is under the same post-choice terms: its own
	// hand visible (the blind lifted once the choice resolved), the parked
	// seat's hand nil.
	if ov.Viewer != other || ov.Players[other].Hand == nil || ov.Players[parked].Hand != nil {
		t.Fatalf("the other seat's view is not its own game: viewer %d hands %v/%v", ov.Viewer, ov.Players[parked].Hand, ov.Players[other].Hand)
	}
	if ov.Decision != nil {
		t.Fatalf("seat %d sees the decision asked of seat %d: %+v", other, parked, ov.Decision)
	}

	// Crossing claims: neither seat can request the other's seat number and
	// get a view at all.
	for _, seat := range []state.PlayerID{parked, other} {
		claim := ps
		if seat == parked {
			claim = os
		}
		if code, e, _ := seatReq(t, http.MethodGet,
			fmt.Sprintf("%s/api/tables/t1/matches/1/view?seat=%d", srv.URL, seat), claim, nil); code != http.StatusForbidden || e.Code != "forbidden" {
			t.Fatalf("claim for the other seat reading seat %d's view: %d %+v", seat, code, e)
		}
	}
}

// TestSeatEventsRedactEveryOtherSeatsDraws pins the redaction of the event
// stream: seat P's stream reduces every other seat's draw — the two human
// seats and both bots — to its shape ("draws a card", no object, no name),
// and (CR 103.5) while the CR 103.1 starting-player choice is
// pending — the fixture's parked state — P's own Secret opening-hand draws
// are redacted to that same shape, so no opening-hand id or name leaves the
// server during the window. The mirror direction (other's claim, other's
// stream) is asserted too, so the property is per-viewer, not a quirk of
// one claim.
func TestSeatEventsRedactEveryOtherSeatsDraws(t *testing.T) {
	srv, r := parkedSeatServer(t, seatClaims)
	parked, other := parkedSeat(t, r)
	ps, os := "s"+strconv.Itoa(int(parked)), "s"+strconv.Itoa(int(other))

	var spec []protocol.EventBody
	if code, e, raw := seatReq(t, http.MethodGet, srv.URL+"/api/tables/t1/matches/1/events?since=0", "", nil); code != http.StatusOK {
		t.Fatalf("spectator events: %d %+v", code, e)
	} else if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatal(err)
	}
	var pstream []protocol.EventBody
	if code, e, raw := seatReq(t, http.MethodGet,
		fmt.Sprintf("%s/api/tables/t1/matches/1/events?since=0&seat=%d", srv.URL, parked), ps, nil); code != http.StatusOK {
		t.Fatalf("seat events: %d %+v", code, e)
	} else if err := json.Unmarshal(raw, &pstream); err != nil {
		t.Fatal(err)
	}
	var ostream []protocol.EventBody
	if code, e, raw := seatReq(t, http.MethodGet,
		fmt.Sprintf("%s/api/tables/t1/matches/1/events?since=0&seat=%d", srv.URL, other), os, nil); code != http.StatusOK {
		t.Fatalf("other seat events: %d %+v", code, e)
	} else if err := json.Unmarshal(raw, &ostream); err != nil {
		t.Fatal(err)
	}

	// Redaction never drops events: the three streams cover the same chain,
	// in the same order.
	if len(pstream) != len(spec) || len(ostream) != len(spec) {
		t.Fatalf("stream lengths differ: spectator %d, seat %d, other %d", len(spec), len(pstream), len(ostream))
	}
	for i := range spec {
		if pstream[i].Event.Seq != spec[i].Event.Seq || ostream[i].Event.Seq != spec[i].Event.Seq {
			t.Fatalf("stream seq %d/%d diverges from spectator seq %d", pstream[i].Event.Seq, ostream[i].Event.Seq, spec[i].Event.Seq)
		}
	}

	// The viewer's name for the shape-line assertion; every player's name
	// comes from the spectator view (the sample decks name seats a/b/c/d).
	var sv view.View
	if code, e, raw := seatReq(t, http.MethodGet, srv.URL+"/api/tables/t1/matches/1/view", "", nil); code != http.StatusOK {
		t.Fatalf("spectator view for names: %d %+v", code, e)
	} else if err := json.Unmarshal(raw, &sv); err != nil {
		t.Fatal(err)
	}

	assertDraws := func(stream []protocol.EventBody, viewer state.PlayerID) {
		t.Helper()
		for _, b := range stream {
			if b.Event.Kind != "draw" {
				continue
			}
			who := state.PlayerID(b.Event.Player)
			if who == viewer {
				// CR 103.5: the fixture reads these streams while
				// the CR 103.1 starting-player choice is pending, so the
				// seat's OWN Secret draws are redacted to the same shape an
				// opponent gets — no object, a shape transcript line. Once
				// the choice resolves the same request serves the ids again
				// (server/host/starting_choice_events_test.go pins both halves).
				if b.Event.Obj != 0 {
					t.Fatalf("seat %d's own draw at seq %d carries obj %d while the starting-player choice is pending", viewer, b.Event.Seq, b.Event.Obj)
				}
				want := sv.Players[who].Name + " draws a card"
				if b.Line != want {
					t.Fatalf("seat %d's own blind draw at seq %d reads %q, want the shape line %q", viewer, b.Event.Seq, b.Line, want)
				}
				continue
			}
			if b.Event.Obj != 0 {
				t.Fatalf("seat %d sees the card seat %d drew at seq %d (obj %d)", viewer, who, b.Event.Seq, b.Event.Obj)
			}
			want := sv.Players[who].Name + " draws a card"
			if b.Line != want {
				t.Fatalf("seat %d's stream names seat %d's hidden draw at seq %d: %q, want %q", viewer, who, b.Event.Seq, b.Line, want)
			}
		}
	}
	assertDraws(pstream, parked)
	assertDraws(ostream, other)
}

// TestPendingIsScopedToTheClaim pins the redaction of the pending decision
// endpoint: the parked seat reads its own decision; the other human seat,
// through its own claim, gets a 409 conflict with a reason — the decision
// exists and is simply not theirs — and neither claim can request the other
// seat's number at all. Pending without a ?seat= is a client error: there
// is no spectator notion of a pending decision.
func TestPendingIsScopedToTheClaim(t *testing.T) {
	srv, r := parkedSeatServer(t, seatClaims)
	parked, other := parkedSeat(t, r)
	ps, os := "s"+strconv.Itoa(int(parked)), "s"+strconv.Itoa(int(other))

	want, err := r.Pending("t1", 1, parked)
	if err != nil {
		t.Fatalf("registry Pending for the parked seat: %v", err)
	}
	var got decision.Decision
	if code, e, raw := seatReq(t, http.MethodGet,
		fmt.Sprintf("%s/api/tables/t1/matches/1/pending?seat=%d", srv.URL, parked), ps, nil); code != http.StatusOK {
		t.Fatalf("parked seat pending: %d %+v", code, e)
	} else if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.Player != parked || got.Seq != want.Seq {
		t.Fatalf("pending decision is for %d seq %d, want %d seq %d", got.Player, got.Seq, parked, want.Seq)
	}

	// The other seat cannot read the parked decision through its own claim.
	if code, e, _ := seatReq(t, http.MethodGet,
		fmt.Sprintf("%s/api/tables/t1/matches/1/pending?seat=%d", srv.URL, other), os, nil); code != http.StatusConflict || e.Code != "conflict" {
		t.Fatalf("other seat reading the parked decision: %d %+v", code, e)
	}
	// The parked seat's claim cannot request the other seat's number at all.
	if code, e, _ := seatReq(t, http.MethodGet,
		fmt.Sprintf("%s/api/tables/t1/matches/1/pending?seat=%d", srv.URL, other), ps, nil); code != http.StatusForbidden || e.Code != "forbidden" {
		t.Fatalf("claim crossing to the other seat's pending: %d %+v", code, e)
	}
	if code, e, _ := seatReq(t, http.MethodGet,
		fmt.Sprintf("%s/api/tables/t1/matches/1/pending?seat=%d", srv.URL, parked), os, nil); code != http.StatusForbidden || e.Code != "forbidden" {
		t.Fatalf("other claim crossing to the parked seat's pending: %d %+v", code, e)
	}
	if code, e, _ := seatReq(t, http.MethodGet, srv.URL+"/api/tables/t1/matches/1/pending", ps, nil); code != http.StatusBadRequest || e.Code != "bad_request" {
		t.Fatalf("pending without a seat: %d %+v", code, e)
	}
}

// TestIntentSubmitsThroughTheClaimAndRejectsOthers pins SubmitIntent on the
// wire: the claim's seat answers its own parked decision with a 204; a
// wrong-player intent and a stale-seq intent are refused with a 409 whose
// body is the registry's own validation reason — the game is left exactly
// where it was, so the valid answer still lands afterwards (audit item 17:
// the rejected intent has somewhere to go). A request with no claim is
// refused before the body is read. There is no ?seat= on intent: the claim
// alone names the seat, and the intent's own Player field is the fence.
func TestIntentSubmitsThroughTheClaimAndRejectsOthers(t *testing.T) {
	srv, r := parkedSeatServer(t, seatClaims)
	parked, other := parkedSeat(t, r)
	ps := "s" + strconv.Itoa(int(parked))

	d, err := r.Pending("t1", 1, parked)
	if err != nil {
		t.Fatalf("registry Pending for the parked seat: %v", err)
	}
	full := answerFor(d)

	wrongPlayer := full
	wrongPlayer.Player = other
	if code, e, _ := seatReq(t, http.MethodPost, srv.URL+"/api/tables/t1/matches/1/intent", ps, wrongPlayer); code != http.StatusConflict || e.Code != "conflict" {
		t.Fatalf("wrong-player intent: %d %+v", code, e)
	} else if !strings.Contains(e.Message, "player") {
		t.Fatalf("wrong-player rejection does not name the player mismatch: %q", e.Message)
	}

	stale := full
	stale.Seq = full.Seq + 1
	if code, e, _ := seatReq(t, http.MethodPost, srv.URL+"/api/tables/t1/matches/1/intent", ps, stale); code != http.StatusConflict || e.Code != "conflict" {
		t.Fatalf("stale intent: %d %+v", code, e)
	} else if !strings.Contains(e.Message, "seq") {
		t.Fatalf("stale rejection does not name the seq mismatch: %q", e.Message)
	}

	// Neither rejection un-parked the seat: the valid answer still lands.
	if code, _, _ := seatReq(t, http.MethodPost, srv.URL+"/api/tables/t1/matches/1/intent", ps, full); code != http.StatusNoContent {
		t.Fatalf("valid intent: %d", code)
	}

	if code, e, _ := seatReq(t, http.MethodPost, srv.URL+"/api/tables/t1/matches/1/intent", "", full); code != http.StatusUnauthorized || e.Code != "unauthorized" {
		t.Fatalf("intent without a claim: %d %+v", code, e)
	}
}

// TestSeatNilDefaultIsSpectatorOnly pins the spectator-only default of a nil
// Options.Seat: no request may act as a seat, and every endpoint that names
// one is refused outright with 403 — the resolver is the seat trust
// boundary, and today's deployments have none.
func TestSeatNilDefaultIsSpectatorOnly(t *testing.T) {
	srv, _ := parkedSeatServer(t, nil) // no resolver: spectator-only, today's behaviour
	for _, p := range []string{
		"/api/tables/t1/matches/1/view?seat=0",
		"/api/tables/t1/matches/1/events?seat=0",
		"/api/tables/t1/matches/1/pending?seat=0",
	} {
		if code, e, _ := seatReq(t, http.MethodGet, srv.URL+p, "", nil); code != http.StatusForbidden || e.Code != "forbidden" {
			t.Fatalf("%s: %d %+v, want 403 forbidden", p, code, e)
		}
	}
	if code, e, _ := seatReq(t, http.MethodPost, srv.URL+"/api/tables/t1/matches/1/intent", "", decision.Intent{}); code != http.StatusForbidden || e.Code != "forbidden" {
		t.Fatalf("intent on a spectator-only server: %d %+v", code, e)
	}
}

// TestSeatParamAndClaimErrors pins the request-level rejections around the
// claim boundary and the routes themselves: a non-numeric seat is a 400, a
// request that presents no (or an unknown) claim is a 401, and the wrong
// method on the new endpoints answers 405 in JSON like every other route.
func TestSeatParamAndClaimErrors(t *testing.T) {
	srv, r := parkedSeatServer(t, seatClaims)
	parkedSeat(t, r) // let the match park; the assertions below do not need the seat

	if code, e, _ := seatReq(t, http.MethodGet, srv.URL+"/api/tables/t1/matches/1/view?seat=abc", "s0", nil); code != http.StatusBadRequest || e.Code != "bad_request" {
		t.Fatalf("non-numeric seat: %d %+v", code, e)
	}
	if code, e, _ := seatReq(t, http.MethodGet, srv.URL+"/api/tables/t1/matches/1/view?seat=0", "", nil); code != http.StatusUnauthorized || e.Code != "unauthorized" {
		t.Fatalf("view with no claim: %d %+v", code, e)
	}
	if code, e, _ := seatReq(t, http.MethodGet, srv.URL+"/api/tables/t1/matches/1/view?seat=0", "nobody", nil); code != http.StatusUnauthorized || e.Code != "unauthorized" {
		t.Fatalf("view with an unknown claim: %d %+v", code, e)
	}
	if code, e, _ := seatReq(t, http.MethodPost, srv.URL+"/api/tables/t1/matches/1/pending?seat=0", "s0", nil); code != http.StatusMethodNotAllowed || e.Code != "method_not_allowed" {
		t.Fatalf("POST pending: %d %+v", code, e)
	}
	if code, e, _ := seatReq(t, http.MethodGet, srv.URL+"/api/tables/t1/matches/1/intent", "s0", nil); code != http.StatusMethodNotAllowed || e.Code != "method_not_allowed" {
		t.Fatalf("GET intent: %d %+v", code, e)
	}
}
