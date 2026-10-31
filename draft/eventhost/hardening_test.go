package eventhost

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/draft/event"
	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/protocol"
)

// testClock is an injectable clock for the sweep tests. The sweep goroutine
// reads Now from another goroutine, so access is mutex-guarded.
type testClock struct {
	mu  sync.Mutex
	now time.Time
}

func newTestClock() *testClock {
	return &testClock{now: time.Unix(1_700_000_000, 0).UTC()}
}

func (c *testClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *testClock) Add(d time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(d)
	c.mu.Unlock()
}

// TestLobbyTimeoutCancelsAndFreesSlot covers the lobby timeout: an
// event abandoned in LOBBY (an unjoined human seat past the window) is
// cancelled by the sweep, the cancellation is a replayable log entry, the
// cancelled event's capacity is released and JoinEvent against it is a
// conflict.
func TestLobbyTimeoutCancelsAndFreesSlot(t *testing.T) {
	clock := newTestClock()
	h, _ := newHost(t, func(o *Options) {
		o.MaxLiveEvents = 1
		o.Now = clock.Now
	})
	resp := createOK(t, h, oneHumanReq())
	id := resp.Event

	// Advance past the default 15-minute window and sweep.
	clock.Add(15*time.Minute + time.Minute)
	h.checkTimeouts(clock.Now())

	if st := stateOf(t, h, id, ""); st.Phase != "CANCELLED" {
		t.Fatalf("phase %s, want CANCELLED", st.Phase)
	}
	r := runOf(t, h, id)
	r.mu.Lock()
	last := r.ev.Log()[len(r.ev.Log())-1]
	r.mu.Unlock()
	if last.Kind != event.KindLobbyTimeout {
		t.Fatalf("last entry %s, want lobby_timeout", last.Kind)
	}

	// The cancelled event is dead: joining is a conflict.
	tok, _ := humanToken(t, resp)
	_, err := h.JoinEvent(id, tok, protocol.JoinEventRequest{Name: "Ann"})
	wantErr(t, err, "conflict", 409)

	// Its capacity is released despite MaxLiveEvents=1.
	createOK(t, h, oneHumanReq())
}

// TestLobbyTimeoutDisabled keeps a negative LobbyTimeout from ever
// cancelling.
func TestLobbyTimeoutDisabled(t *testing.T) {
	clock := newTestClock()
	h, _ := newHost(t, func(o *Options) {
		o.LobbyTimeout = -1
		o.Now = clock.Now
	})
	resp := createOK(t, h, oneHumanReq())
	clock.Add(24 * time.Hour)
	h.checkTimeouts(clock.Now())
	if st := stateOf(t, h, resp.Event, ""); st.Phase != "LOBBY" {
		t.Fatalf("phase %s, want LOBBY (timeout disabled)", st.Phase)
	}
}

// TestIdleSeatAutoActsAndEventCompletes covers the idle timeout: after
// one real pick the human seat vanishes, and the sweep alone drives the rest
// of the event — auto-picking the draft, auto-submitting the build and
// conceding every series — to COMPLETE, with the idle actions visible in the
// log.
func TestIdleSeatAutoActsAndEventCompletes(t *testing.T) {
	clock := newTestClock()
	p := startGamePair(t, "", func(eo *Options, _ *host.Options) {
		eo.MaxHumans = 1
		eo.Now = clock.Now
		eo.MintSeat = func(table host.TableID, seat int) (string, error) {
			return fmt.Sprintf("tok-%s-%d", table, seat), nil
		}
	})
	resp := createOK(t, p.h, oneHumanReq())
	id := resp.Event
	tok, _ := humanToken(t, resp)
	joinOK(t, p.h, id, tok, "Ann")
	// One real pick, then the human never acts again.
	pack0 := packOf(t, p.h, id, tok)
	pickTop(t, p.h, id, tok, 1)

	clock.Add(2*time.Hour + time.Minute)
	pumpSweep(t, p.h, id, clock.Now, "COMPLETE", 50*time.Second)

	r := runOf(t, p.h, id)
	r.mu.Lock()
	defer r.mu.Unlock()
	var idle, conceded, autoPicks int
	for _, en := range r.ev.Log() {
		switch {
		case en.Kind == event.KindIdleTimeout && en.Seat == 0:
			idle++
		case en.Kind == event.KindGameEnded && en.Head == "conceded":
			conceded++
		case en.Kind == event.KindPicked && en.Seat == 0 && en.Auto:
			autoPicks++
		}
	}
	if idle == 0 {
		t.Error("no idle_timeout marker for seat 0")
	}
	if conceded != 3 {
		t.Errorf("conceded series %d, want 3 (one per round)", conceded)
	}
	if autoPicks == 0 {
		t.Error("no auto pick recorded for seat 0")
	}
	if pool := len(r.ev.Pool(0)); pool != 3*len(pack0.Pack) {
		t.Errorf("seat 0 drafted %d cards, want %d", pool, 3*len(pack0.Pack))
	}
	if got := len(r.ev.Standings()); got != 4 {
		t.Errorf("standings rows %d, want 4", got)
	}
}

// TestSubscribePublishesPodProgress covers subscribing mid-draft: a
// subscriber attaching during the draft gets the current pod progress
// immediately, before any pack frame, and spectators get it too.
func TestSubscribePublishesPodProgress(t *testing.T) {
	h, reg := newHostR(t, nil)
	resp := createOK(t, h, oneHumanReq())
	id := resp.Event
	tok, _ := humanToken(t, resp)
	joinOK(t, h, id, tok, "Ann")
	pickTop(t, h, id, tok, 2)

	for _, who := range []struct {
		name, token string
	}{
		{"seat", tok}, {"spectator", ""},
	} {
		s := reg.OpenSession()
		if err := h.SubscribeEvent(id, who.token, s.ID); err != nil {
			t.Fatalf("%s: SubscribeEvent: %v", who.name, err)
		}
		f1 := readFrame(t, s)
		if f1.T != protocol.TEventState {
			t.Fatalf("%s: first frame %s", who.name, f1.T)
		}
		f2 := readFrame(t, s)
		if f2.T != protocol.TPodProgress {
			t.Fatalf("%s: second frame %s, want pod_progress", who.name, f2.T)
		}
		var prog protocol.PodProgress
		if err := json.Unmarshal(f2.Body, &prog); err != nil {
			t.Fatalf("%s: pod_progress body: %v", who.name, err)
		}
		if len(prog.Picks) != 4 {
			t.Errorf("%s: %d picks in pod_progress, want 4", who.name, len(prog.Picks))
		}
		if prog.Picks[0] != 2 {
			t.Errorf("%s: seat 0 picks %d, want 2", who.name, prog.Picks[0])
		}
		if prog.Round != 1 {
			t.Errorf("%s: round %d, want 1", who.name, prog.Round)
		}
	}
}

// TestDraftCardCarriesFacts covers the card facts: every card a
// pack offers carries mana_cost/mana_value/colors/type_line, agreeing with
// the corpus's spelling for known cards (cost text, WUBRG-ordered colours,
// the printed face types).
func TestDraftCardCarriesFacts(t *testing.T) {
	h, corpus := newHost(t, nil)
	resp := createOK(t, h, oneHumanReq())
	id := resp.Event
	tok, _ := humanToken(t, resp)
	joinOK(t, h, id, tok, "Ann")
	p := packOf(t, h, id, tok)
	if len(p.Pack) == 0 {
		t.Fatal("empty first pack")
	}
	known := 0
	for _, c := range p.Pack {
		if c.TypeLine == "" && c.ManaValue == 0 && c.Colors == "" && c.ManaCost == "" {
			t.Fatalf("card %s (%s) carries no facts at all", c.ID, c.Name)
		}
		if c.ManaCost == "" && c.ManaValue != 0 {
			t.Errorf("card %s: mana_value %d with no cost", c.ID, c.ManaValue)
		}
		for i := 0; i < len(c.Colors); i++ {
			if i > 0 && !wubrgOrder(c.Colors[i-1], c.Colors[i]) {
				t.Errorf("card %s: colors %q out of WUBRG order", c.ID, c.Colors)
			}
		}
		card, ok := corpus.Lookup(c.Name)
		if !ok {
			continue
		}
		known++
		f := card.Faces[0]
		wantCost := f.ManaCost
		if strings.EqualFold(wantCost, "no cost") {
			wantCost = ""
		}
		if c.ManaCost != wantCost {
			t.Errorf("card %s: mana_cost %q, want %q", c.ID, c.ManaCost, wantCost)
		}
		if c.ManaValue != int(f.ManaValue()) {
			t.Errorf("card %s: mana_value %d, want %d", c.ID, c.ManaValue, f.ManaValue())
		}
		if c.Colors != wubrgOf(card.ColourIdentity()) {
			t.Errorf("card %s: colors %q, want %q", c.ID, c.Colors, wubrgOf(card.ColourIdentity()))
		}
		if c.TypeLine != strings.Join(f.Types, " ") {
			t.Errorf("card %s: type_line %q, want %q", c.ID, c.TypeLine, strings.Join(f.Types, " "))
		}
	}
	if known == 0 {
		t.Fatal("no pack card resolved against the corpus")
	}
}

// wubrgOrder reports whether colours a then b are in WUBRG order.
func wubrgOrder(a, b byte) bool { return strings.IndexByte("WUBRG", a) < strings.IndexByte("WUBRG", b) }

// TestEventSetsAndPersonasContent covers the discovery endpoints at the
// host boundary: every loadable set appears with a pack size and a rating
// source, and the drafter/builder persona lists are non-empty and include
// the rating-colour / best23 defaults.
func TestEventSetsAndPersonasContent(t *testing.T) {
	h, _ := newHost(t, nil)
	sets := h.Sets()
	if len(sets) == 0 {
		t.Fatal("Sets() returned nothing")
	}
	found := false
	for _, s := range sets {
		if s.PackSize <= 0 {
			t.Errorf("set %s: pack size %d", s.Code, s.PackSize)
		}
		if s.Ratings == "" {
			t.Errorf("set %s: empty ratings", s.Code)
		}
		if s.Code == "FDN" {
			found = true
			if s.Ratings != "17lands" {
				t.Errorf("FDN ratings %q, want 17lands", s.Ratings)
			}
		}
	}
	if !found {
		t.Error("FDN missing from Sets()")
	}
	for i := 1; i < len(sets); i++ {
		if sets[i-1].Code >= sets[i].Code {
			t.Fatalf("Sets() not sorted by code: %s before %s", sets[i-1].Code, sets[i].Code)
		}
	}
	p := h.Personas()
	if len(p.Drafters) == 0 || len(p.Builders) == 0 {
		t.Fatalf("Personas: %d drafters, %d builders", len(p.Drafters), len(p.Builders))
	}
	has := func(list []string, name string) bool {
		for _, n := range list {
			if n == name {
				return true
			}
		}
		return false
	}
	if !has(p.Drafters, "rating-colour") || !has(p.Builders, "best23") {
		t.Errorf("personas missing defaults: %v / %v", p.Drafters, p.Builders)
	}
}

// TestStandingsEmptyBeforeFirstResult pins when standings appear: an
// event in ROUND1 with no series result yet reports an empty standings list,
// and the list fills once the round completes.
func TestStandingsEmptyBeforeFirstResult(t *testing.T) {
	p := startGamePair(t, "", nil)
	resp := createOK(t, p.h, botEventReq("lethal-pressure", "lethal-pressure", "lethal-pressure", "lethal-pressure"))
	id := resp.Event
	waitPhase(t, p.h, id, "ROUND1", 50*time.Second)

	st := stateOf(t, p.h, id, "")
	if len(st.Standings) != 0 {
		t.Fatalf("standings mid-round: %+v, want empty", st.Standings)
	}
	if len(st.Series) == 0 {
		t.Fatal("no series rows in ROUND1")
	}

	waitPhase(t, p.h, id, "COMPLETE", 50*time.Second)
	st = stateOf(t, p.h, id, "")
	if len(st.Standings) != 4 {
		t.Fatalf("standings at COMPLETE: %d rows, want 4", len(st.Standings))
	}
}
