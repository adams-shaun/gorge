package sbv2http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/deck"
	"github.com/adams-shaun/gorge/engine/game"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/protocol"
	"github.com/adams-shaun/gorge/server/spellbench/mapping"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// nativeSeatDeck reaches every ask the v2 codec answers itself: a land that makes
// any colour, a triggered cost and, with both seats blocking, an attacker with
// two blockers.
const nativeSeatDeck = `{"name":"wire","format":"custom","cards":[{"name":"City of Brass","count":24},{"name":"Order of the Golden Cricket","count":36}]}`

// nativeSeatTable starts a table with the given native human seats, the -sbv2
// tap factory armed when hub is non-nil.
func nativeSeatTable(t *testing.T, reg *cards.Registry, hub *Hub, thinkTimeout time.Duration, seed uint64, humans ...int) *host.Registry {
	t.Helper()
	o := host.Options{
		LoadDeck: func(name string) (host.Deck, error) {
			f, err := deck.Parse([]byte(nativeSeatDeck))
			if err != nil {
				return host.Deck{}, err
			}
			cs, err := f.Resolve(reg)
			if err != nil {
				return host.Deck{}, err
			}
			return host.Deck{Name: name, Cards: cs}, nil
		},
		Tokens:       reg.Tokens,
		NameUniverse: reg.Universe(),
		Sleep:        func(time.Duration, <-chan struct{}) {},
		ThinkTimeout: thinkTimeout,
	}
	if hub != nil {
		o.TapHuman = hub.Tap
	}
	r, err := host.New(o)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close() })
	cfg := host.TableConfig{ID: "t", Name: "t", Seats: 2, Decks: []string{"wire", "wire"}, Seed: seed,
		Spectator: view.Omniscient, AutoMana: true, BotAutoPayMana: true, Humans: humans}
	if err := r.AddTable(cfg); err != nil {
		t.Fatal(err)
	}
	if err := r.Start("t"); err != nil {
		t.Fatal(err)
	}
	return r
}

// nativeScript answers both seats the way a native client would, counting the
// routes it was asked. Each seat taps a land for mana once and casts what it can;
// seat 0 attacks with everything and seat 1 blocks the first attacker with every
// creature.
type nativeScript struct {
	asked   map[string]int
	tapped  [2]bool
	colours int // how many colours the last colour ask offered
}

func (s *nativeScript) answer(d *decision.Decision) decision.Intent {
	route := mapping.Route(d)
	s.asked[route]++
	if route == "choose/mana" {
		s.colours = len(d.Options)
	}
	in := decision.Intent{Seq: d.Seq, Player: d.Player}
	switch d.Kind {
	case decision.KPriority:
		for _, o := range d.Options {
			if o.Kind == "play_land" {
				in.Choices = []int{o.Index}
				return in
			}
		}
		if !s.tapped[d.Player] {
			for _, o := range d.Options {
				if o.Kind == "activate" {
					s.tapped[d.Player] = true
					in.Choices = []int{o.Index}
					return in
				}
			}
		}
		if len(d.PaymentActions) > 0 && len(d.PaymentActions[0].Plans) > 0 {
			a := d.PaymentActions[0]
			in.Payment = &decision.PaymentSelection{ActionID: a.ID, Plan: a.Plans[0]}
			return in
		}
		for _, o := range d.Options {
			if o.Kind == "pass" {
				in.Choices = []int{o.Index}
				return in
			}
		}
	case decision.KAttackers:
		in.Choices = []int{}
		for _, o := range d.Options {
			if d.Player == 0 {
				in.Choices = append(in.Choices, o.Index)
			}
		}
		return in
	case decision.KBlockers:
		in.Choices = []int{}
		for _, o := range d.Options {
			if o.Attacker == d.Options[0].Attacker {
				in.Choices = append(in.Choices, o.Index)
			}
		}
		return in
	}
	for i := 0; i < d.Min && i < len(d.Options); i++ {
		in.Choices = append(in.Choices, d.Options[i].Index)
	}
	return in
}

// playNativeScript answers every native decision until the match ends or done
// says to stop, and returns the match's head when it ended.
func playNativeScript(t *testing.T, r *host.Registry, s *nativeScript, done func() bool) string {
	t.Helper()
	answered := [2]uint64{}
	deadline := time.Now().Add(45 * time.Second)
	for {
		if fin, head := matchDone(t, r, "t"); fin {
			return head
		}
		if done != nil && done() {
			return ""
		}
		if time.Now().After(deadline) {
			t.Fatalf("the game did not finish; asked %v", s.asked)
		}
		progressed := false
		for seat := range answered {
			d, err := r.Pending("t", 1, state.PlayerID(seat))
			if err != nil || d.Seq == answered[seat] {
				continue
			}
			answered[seat] = d.Seq
			if err := r.SubmitIntent("t", 1, state.PlayerID(seat), s.answer(d)); err != nil {
				t.Fatalf("seat %d seq %d (%s): %v", seat, d.Seq, mapping.Route(d), err)
			}
			progressed = true
		}
		if !progressed {
			time.Sleep(100 * time.Microsecond)
		}
	}
}

// TestNativeSeatIsAskedWhatV2Answers: with the -sbv2 tap armed and no v2 client
// attached, a native seat is asked a land's colour, an echo cost and the
// division of an attacker's damage. The v2 codec answers all three itself, so a
// tap that drove an unattached seat would swallow them.
func TestNativeSeatIsAskedWhatV2Answers(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	r := nativeSeatTable(t, reg, NewHub(), 0, 11, 0, 1)
	s := &nativeScript{asked: map[string]int{}}
	want := []string{"choose/mana", "choose/trigger_cost", "choose/division"}
	playNativeScript(t, r, s, func() bool {
		for _, route := range want {
			if s.asked[route] == 0 {
				return false
			}
		}
		return true
	})
	for _, route := range want {
		if s.asked[route] == 0 {
			t.Errorf("a native seat was never asked %s; asked %v", route, s.asked)
		}
	}
	if s.colours != 5 {
		t.Errorf("City of Brass offered %d colours", s.colours)
	}
}

// TestNativeSeatPlaysTheSameGameWithTheTapArmed: the same answers on the same
// table reach the same head whether or not the server offers v2.
func TestNativeSeatPlaysTheSameGameWithTheTapArmed(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	heads := [2]string{}
	for i, hub := range []*Hub{nil, NewHub()} {
		r := nativeSeatTable(t, reg, hub, 0, 11, 0, 1)
		heads[i] = playNativeScript(t, r, &nativeScript{asked: map[string]int{}}, nil)
		r.Close()
		if hub != nil {
			if !hub.seat(seatKey{table: "t", match: 1, seat: 0}).lostCodec(t) {
				t.Fatal("the hub still holds the codec of a finished match")
			}
		}
	}
	if heads[0] == "" || heads[0] != heads[1] {
		t.Fatalf("heads differ: flag off %q, flag on %q", heads[0], heads[1])
	}
}

// lostCodec waits for the hub to let go of a finished match's codec.
func (s *seatState) lostCodec(t *testing.T) bool {
	t.Helper()
	for deadline := time.Now().Add(20 * time.Second); time.Now().Before(deadline); time.Sleep(time.Millisecond) {
		s.mu.Lock()
		gone := s.codec == nil
		s.mu.Unlock()
		if gone {
			return true
		}
	}
	return false
}

// TestThinkTimeoutAnswersAPassiveAndAnAttachedSeat: nobody answers, so the think
// timeout does. A passive seat's caretaker plays the game a flag-off server
// plays; an attached seat's answers everything the codec doesn't answer itself.
func TestThinkTimeoutAnswersAPassiveAndAnAttachedSeat(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	const think = time.Millisecond
	finish := func(r *host.Registry) string {
		deadline := time.Now().Add(30 * time.Second)
		for time.Now().Before(deadline) {
			ms, err := r.Matches("t")
			if err == nil && len(ms) == 1 && ms[0].State != protocol.MatchLive {
				if ms[0].State != protocol.MatchFinished {
					t.Fatalf("match %+v", ms[0])
				}
				return ms[0].Head
			}
			time.Sleep(time.Millisecond)
		}
		t.Fatal("no think timeout finished the game")
		return ""
	}
	off := finish(nativeSeatTable(t, reg, nil, think, 11, 0))
	passive := finish(nativeSeatTable(t, reg, NewHub(), think, 11, 0))
	if off != passive {
		t.Fatalf("heads differ: flag off %s, passive seat %s", off, passive)
	}
	hub := NewHub()
	hub.seat(seatKey{table: "t", match: 1, seat: 0}).attach(false)
	finish(nativeSeatTable(t, reg, hub, think, 11, 0))
}

// readerlessGame is an engine's game that a v2 seat cannot read.
type readerlessGame struct{ game.Game }

func (readerlessGame) Caps() game.Caps { return game.Caps{Engine: "other"} }

// A table on an engine with no v2 reader plays natively under -sbv2, and its
// routes say v2 doesn't cover it.
func TestEngineWithNoV2ReaderStaysNative(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	hub := NewHub()
	r := nativeSeatTable(t, reg, hub, 0, 11, 0)
	srv := httptest.NewServer(http.StripPrefix("/api/sbv2/v1/tables", NewHandler(r, hub, Options{Seat: func(*http.Request) (SeatClaim, bool) {
		return SeatClaim{Table: "t", Seat: 0}, true
	}})))
	t.Cleanup(srv.Close)
	const route = "/api/sbv2/v1/tables/t/matches/1/state"
	if status, _ := getSBV2State(t, srv.URL, route, "tok"); status != http.StatusOK {
		t.Fatalf("state on a readable engine: %d", status)
	}
	tap, err := hub.Tap("t", 1, true, &bots.MatchContext{Context: t.Context(), Seats: 2, Format: "constructed", Game: readerlessGame{}})
	if err != nil || tap != nil {
		t.Fatalf("Tap = %v, %v, want no tap and no error", tap, err)
	}
	if status, _ := getSBV2State(t, srv.URL, route, "tok"); status != http.StatusConflict {
		t.Fatalf("state on an engine with no reader: %d", status)
	}
}

// waitV2Request waits for an open request other than the one numbered not.
func waitV2Request(t *testing.T, ss *seatState, not uint64) *request {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if req, _ := ss.snapshot(); req != nil && req.id != not {
			return req
		}
		time.Sleep(100 * time.Microsecond)
	}
	t.Fatal("no v2 request opened")
	return nil
}

// TestV2AttachHoldAndHandBack walks one game through the attach rule: a seat is
// passive until an authenticated v2 read; an open stream keeps it attached
// through a native answer, whose request closes at once; with the stream gone a
// native answer hands it back, and the next read takes it again.
func TestV2AttachHoldAndHandBack(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	hub := NewHub()
	r := nativeSeatTable(t, reg, hub, 0, 11, 0)
	mux := http.NewServeMux()
	mux.Handle("/api/sbv2/v1/tables/", http.StripPrefix("/api/sbv2/v1/tables", NewHandler(r, hub, Options{Seat: func(req *http.Request) (SeatClaim, bool) {
		return SeatClaim{Table: "t", Seat: 0}, strings.TrimPrefix(req.Header.Get("Authorization"), "Bearer ") == "tok"
	}})))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	const route = "/api/sbv2/v1/tables/t/matches/1/"
	ss := hub.seat(seatKey{table: "t", match: 1, seat: 0})

	s := &nativeScript{asked: map[string]int{}, tapped: [2]bool{true, true}}
	var answered uint64
	pending := func() *decision.Decision {
		t.Helper()
		deadline := time.Now().Add(20 * time.Second)
		for time.Now().Before(deadline) {
			if d, err := r.Pending("t", 1, 0); err == nil && d.Seq != answered {
				return d
			}
			time.Sleep(100 * time.Microsecond)
		}
		t.Fatal("seat 0 was not asked anything")
		return nil
	}
	submit := func(in decision.Intent) {
		t.Helper()
		answered = in.Seq
		if err := r.SubmitIntent("t", 1, 0, in); err != nil {
			t.Fatalf("seq %d: %v", in.Seq, err)
		}
	}
	tapOption := func(d *decision.Decision) (int, bool) {
		if d.Kind == decision.KPriority {
			for _, o := range d.Options {
				if o.Kind == "activate" {
					return o.Index, true
				}
			}
		}
		return 0, false
	}
	// nextTap plays natively to a priority decision that offers a mana tap.
	nextTap := func() (*decision.Decision, int) {
		t.Helper()
		for {
			d := pending()
			if i, ok := tapOption(d); ok {
				return d, i
			}
			submit(s.answer(d))
		}
	}

	d, tap := nextTap()
	if status, _ := getSBV2State(t, srv.URL, route+"state", "wrong"); status != http.StatusUnauthorized {
		t.Fatalf("state with a wrong token: %d", status)
	}
	ss.mu.Lock()
	claimed := ss.claimed
	ss.mu.Unlock()
	if claimed {
		t.Fatal("a read with a wrong token claimed the seat")
	}
	if req, _ := ss.snapshot(); req != nil {
		t.Fatalf("a request opened with no authenticated read: %d", req.id)
	}

	// A stream attaches the seat and drives the decision already parked.
	frames, closeStream := dialSBV2Stream(t, srv.URL, route+"stream", "tok")
	go func() {
		for range frames {
		}
	}()
	first := waitV2Request(t, ss, 0)
	if first.ask.Pose == nil || first.ask.Pose.Native.Seq != d.Seq {
		t.Fatalf("first request answers %+v, want the parked seq %d", first.ask.Pose, d.Seq)
	}

	// A native tap under the open stream closes that request and leaves the seat
	// attached, so the codec answers the colour ask.
	submit(decision.Intent{Seq: d.Seq, Player: 0, Choices: []int{tap}})
	second := waitV2Request(t, ss, first.id)
	if rt := mapping.Route(second.ask.Pose.Native); rt == "choose/mana" {
		t.Fatalf("the attached seat posed %s", rt)
	}
	if status, _ := postSBV2Send(t, srv.URL, route+"send", "tok", fmt.Sprint(first.id), 0); status != http.StatusConflict {
		t.Fatalf("send to the request a native answer closed: %d", status)
	}

	// With the stream gone the seat stays attached until a native answer. That
	// answer closes the open request, and from then on the seat is asked.
	closeStream()
	deadline := time.Now().Add(20 * time.Second)
	for ss.held() {
		if time.Now().After(deadline) {
			t.Fatal("the closed stream still holds the seat")
		}
		time.Sleep(100 * time.Microsecond)
	}
	if req, _ := ss.snapshot(); req == nil || req.id != second.id {
		t.Fatalf("closing the stream changed the open request: %+v", req)
	}
	submit(s.answer(pending()))
	for {
		if req, _ := ss.snapshot(); req == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the native answer did not close the request")
		}
		time.Sleep(100 * time.Microsecond)
	}
	d, tap = nextTap()
	submit(decision.Intent{Seq: d.Seq, Player: 0, Choices: []int{tap}})
	colour := pending()
	if rt := mapping.Route(colour); rt != "choose/mana" {
		t.Fatalf("after handing back, a tap asked %s", rt)
	}
	if req, _ := ss.snapshot(); req != nil {
		t.Fatalf("a request opened on a handed-back seat: %d", req.id)
	}

	// A read takes the seat again in the middle of the native tap: the codec
	// answers the colour ask, as it does for any v2 seat, and poses what follows.
	if status, _ := getSBV2State(t, srv.URL, route+"state", "tok"); status != http.StatusOK {
		t.Fatalf("state: %d", status)
	}
	third := waitV2Request(t, ss, second.id)
	if third.ask.Pose == nil || third.ask.Pose.Native.Seq <= colour.Seq {
		t.Fatalf("after re-attaching, request %+v, colour ask seq %d", third.ask.Pose, colour.Seq)
	}
	if status, body := postSBV2Send(t, srv.URL, route+"send", "tok", fmt.Sprint(third.id), 0); status != http.StatusNoContent {
		t.Fatalf("send after re-attaching: %d %s", status, body)
	}
}

// TestV2LeavesAsksToAClientThatTakesThem: a stream with asks=native is told
// about every ask the codec otherwise answers itself, as a native_only request,
// and answers it on the native routes while the seat stays attached.
func TestV2LeavesAsksToAClientThatTakesThem(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	hub := NewHub()
	r := nativeSeatTable(t, reg, hub, 0, 11, 0, 1)
	mux := http.NewServeMux()
	mux.Handle("/api/sbv2/v1/tables/", http.StripPrefix("/api/sbv2/v1/tables", NewHandler(r, hub, Options{Seat: func(req *http.Request) (SeatClaim, bool) {
		tok := strings.TrimPrefix(req.Header.Get("Authorization"), "Bearer ")
		return SeatClaim{Table: "t", Seat: state.PlayerID(strings.TrimPrefix(tok, "tok")[0] - '0')}, strings.HasPrefix(tok, "tok")
	}})))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	const route = "/api/sbv2/v1/tables/t/matches/1/"

	own := map[string]bool{"choose/mana": true, "choose/mana_window": true, "choose/trigger_cost": true, "choose/division": true, "choose/pay_pip": true}
	s := &nativeScript{asked: map[string]int{}}
	var seats [2]*seatState
	for i := range seats {
		seats[i] = hub.seat(seatKey{table: "t", match: 1, seat: state.PlayerID(i)})
		frames, closeStream := dialSBV2Stream(t, srv.URL, route+"stream?asks=native", fmt.Sprintf("tok%d", i))
		t.Cleanup(closeStream)
		go func() {
			for range frames {
			}
		}()
	}
	var last [2]uint64
	deadline := time.Now().Add(45 * time.Second)
	for s.asked["choose/mana"] == 0 || s.asked["choose/trigger_cost"] == 0 || s.asked["choose/division"] == 0 {
		if time.Now().After(deadline) {
			t.Fatalf("not every ask reached the client; asked %v", s.asked)
		}
		if fin, _ := matchDone(t, r, "t"); fin {
			t.Fatalf("the game ended first; asked %v", s.asked)
		}
		progressed := false
		for i, ss := range seats {
			req, _ := ss.snapshot()
			if req == nil || req.id == last[i] {
				continue
			}
			last[i] = req.id
			progressed = true
			if req.nativeOnly {
				d, err := r.Pending("t", 1, state.PlayerID(i))
				if err != nil {
					t.Fatalf("native_only with nothing pending: %v", err)
				}
				if rt := mapping.Route(d); !own[rt] {
					t.Fatalf("native_only for %s", rt)
				}
				if err := r.SubmitIntent("t", 1, state.PlayerID(i), s.answer(d)); err != nil {
					t.Fatalf("native answer to %s: %v", mapping.Route(d), err)
				}
				if !ss.held() {
					t.Fatal("the seat lost its stream")
				}
				continue
			}
			if err := r.SubmitIntent("t", 1, state.PlayerID(i), s.answer(req.ask.Pose.Native)); err != nil {
				t.Fatalf("seat %d: %v", i, err)
			}
		}
		if !progressed {
			time.Sleep(100 * time.Microsecond)
		}
	}
}

// TestSwitchingWiresPlaysOneGame moves a seat between the wires every few
// decisions, leaving a v2 walk in the middle whenever it can, and replays what
// both wires answered on a plain human seat. Attaching must work at whatever
// decision the seat is parked on, nothing of a half-walked pose may reach the
// log, and the head must be the one those answers give.
func TestSwitchingWiresPlaysOneGame(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	for _, seed := range []uint64{3000, 3001, 3002} {
		t.Run(fmt.Sprint(seed), func(t *testing.T) {
			hub := NewHub()
			rec := &recorder{}
			r, base := startExtensionServer(t, reg, hub, rec, "flip", seed)
			ss := hub.seat(seatKey{table: "flip", match: 1, seat: 0})
			const route = "/api/sbv2/v1/tables/flip/matches/1/"
			bot := referenceBot(t, seed)
			p := &picker{bot: bot}
			native, v2, midWalk, attachedOffPriority := 0, 0, 0, 0
			overV2 := false
			count := 0
			seen := map[string]bool{}
			deadline := time.Now().Add(40 * time.Second)
			for {
				if done, _ := matchDone(t, r, "flip"); done {
					break
				}
				if time.Now().After(deadline) {
					t.Fatalf("the game did not finish: %d native, %d v2 answers", native, v2)
				}
				if count >= 5 {
					count, overV2 = 0, !overV2
				}
				if !overV2 {
					d, err := r.Pending("flip", 1, 0)
					if err != nil {
						time.Sleep(100 * time.Microsecond)
						continue
					}
					seq, err := sbv2HeadSeq(r, "flip", 1)
					if err != nil {
						t.Fatal(err)
					}
					v, err := r.ViewAtSeat("flip", 1, seq, 0)
					if err != nil {
						continue // the log moved under the read
					}
					v.Decision = d
					in, err := bot.Decide(context.Background(), v, *d)
					if err != nil {
						t.Fatal(err)
					}
					if err := r.SubmitIntent("flip", 1, 0, in); err != nil {
						continue // a codec still attached answered first
					}
					rec.add(in)
					native++
					count++
					continue
				}
				status, body := getSBV2State(t, base, route+"state", "tok")
				if status != http.StatusOK {
					t.Fatalf("state: %d %s", status, body)
				}
				var doc stateBody
				if err := json.Unmarshal(body, &doc); err != nil {
					t.Fatal(err)
				}
				if doc.Request == nil || seen[doc.Request.RequestID] {
					time.Sleep(100 * time.Microsecond)
					continue
				}
				req, _ := ss.snapshot()
				if req == nil || fmt.Sprint(req.id) != doc.Request.RequestID {
					continue
				}
				seen[doc.Request.RequestID] = true
				if req.ask.Pose == nil {
					t.Fatalf("native-only request: %+v", doc.Request.Decision.Extensions)
				}
				if count == 0 && req.ask.Pose.Native.Kind != decision.KPriority {
					attachedOffPriority++
				}
				cand, miss := p.pick(req.ask)
				if miss != "" {
					cand = 0
				}
				if status, body := postSBV2Send(t, base, route+"send", "tok", doc.Request.RequestID, cand); status != http.StatusNoContent {
					t.Fatalf("send: %d %s", status, body)
				}
				v2++
				count++
				if req.ask.Pose.SubstepIndex+1 < req.ask.Pose.SubstepCount {
					midWalk++
					count, overV2 = 0, false
				}
			}
			head, skipped := playSBV2NativeReference(t, reg, seed, rec)
			if skipped != 0 {
				t.Fatalf("the replay left %d decisions unanswered", skipped)
			}
			if _, got := matchDone(t, r, "flip"); got != head {
				t.Fatalf("heads differ: switching seat %s, plain seat %s", got, head)
			}
			if native == 0 || v2 == 0 {
				t.Fatalf("did not use both wires: %d native, %d v2", native, v2)
			}
			t.Logf("%d native, %d v2 answers; %d walks left in the middle; %d attaches away from priority", native, v2, midWalk, attachedOffPriority)
		})
	}
}
