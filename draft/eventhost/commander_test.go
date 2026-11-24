package eventhost

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/draft/event"
	"github.com/adams-shaun/gorge/draft/limited"
	"github.com/adams-shaun/gorge/draft/limited/build"
	"github.com/adams-shaun/gorge/draft/limited/pickbot"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/protocol"
)

// commanderReq is a one-human MBC Commander-draft request: seat 0 human,
// seats 1-3 bots on the server's default policy.
func commanderReq() protocol.CreateEventRequest {
	return protocol.CreateEventRequest{
		Set: "MBC", Format: formatCommander,
		Seats: []protocol.CreateEventSeat{{Kind: "human"}, {Kind: "bot"}, {Kind: "bot"}, {Kind: "bot"}},
	}
}

// TestCommanderCreateRefusals: a Commander draft needs a set the server
// hosts one of and a single free-for-all game per round; a commander set is
// drafted only as a Commander draft; the sets listing marks it.
func TestCommanderCreateRefusals(t *testing.T) {
	h, _ := newHost(t, nil)
	for name, req := range map[string]protocol.CreateEventRequest{
		"limited set": func() protocol.CreateEventRequest { r := commanderReq(); r.Set = "FDN"; return r }(),
		"best of 3":   func() protocol.CreateEventRequest { r := commanderReq(); r.BestOf = 3; return r }(),
		"MBC limited": func() protocol.CreateEventRequest { r := commanderReq(); r.Format = ""; return r }(),
	} {
		if _, err := h.CreateEvent(req); err == nil {
			t.Errorf("%s: CreateEvent succeeded, want a refusal", name)
		}
	}
	found := false
	for _, s := range h.Sets() {
		if s.Commander != (s.Code == "MBC") {
			t.Errorf("set %s: commander %v", s.Code, s.Commander)
		}
		found = found || s.Code == "MBC"
	}
	if !found {
		t.Fatal("the sets listing has no MBC")
	}
}

// TestCommanderEventDraftBuildGame plays a seeded MBC Commander draft with
// one scripted human and three bots end to end: pick-two passes of 20-card
// packs to a 60-card pool, a commander deck built and checked (an
// out-of-identity card and a commander-less deck refused), and the pod's
// four-seat Commander game at 40 life played to its PodGameEnded and the
// event's end.
func TestCommanderEventDraftBuildGame(t *testing.T) {
	p := startGamePair(t, "", func(eo *Options, _ *host.Options) {
		eo.MintSeat = func(table host.TableID, seat int) (string, error) {
			return fmt.Sprintf("tok-%s-%d", table, seat), nil
		}
	})
	resp := createOK(t, p.h, commanderReq())
	id := resp.Event
	tok, _ := humanToken(t, resp)
	joinOK(t, p.h, id, tok, "Ann")

	st := stateOf(t, p.h, id, tok)
	if st.Format != formatCommander || st.PicksPerPass != 2 || st.DeckSize != 60 || len(st.Series) != 0 {
		t.Fatalf("state format %q picks/pass %d deck size %d series %d", st.Format, st.PicksPerPass, st.DeckSize, len(st.Series))
	}
	for s := 1; s < 4; s++ {
		if st.Seats[s].Drafter != commanderDrafter {
			t.Errorf("bot seat %d drafter %q", s, st.Seats[s].Drafter)
		}
	}

	// Draft: every pass offers the same pack twice, pass_pick 1 then 2.
	for i := 0; ; i++ {
		pk := packOf(t, p.h, id, tok)
		if pk.Waiting {
			break
		}
		if i == 0 && len(pk.Pack) != 20 {
			t.Fatalf("first pack has %d cards, want 20", len(pk.Pack))
		}
		if want := i%2 + 1; pk.PicksPerPass != 2 || pk.PassPick != want {
			t.Fatalf("pick %d: picks_per_pass %d pass_pick %d, want 2 and %d", i, pk.PicksPerPass, pk.PassPick, want)
		}
		if i == 0 && pk.Pack[0].Identity == "" && pk.Pack[0].Colors != "" {
			t.Fatalf("a coloured commander-draft card carries no identity: %+v", pk.Pack[0])
		}
		if i < 2 {
			adv, err := p.h.DraftAdvice(id, tok)
			if err != nil || adv.Waiting || len(adv.Picks) != 2-i || adv.Picks[0].Reason == "" || adv.PickNo != pk.PickNo {
				t.Fatalf("pick %d advice %+v err %v", i, adv, err)
			}
		}
		// The scripted human takes a commander while it has none (by the
		// pack's commander flag), else the pack's first card.
		pick := pk.Pack[0].ID
		if !hasCommander(pk.Pool) {
			for _, c := range pk.Pack {
				if c.Commander {
					pick = c.ID
					break
				}
			}
		}
		if _, err := p.h.Pick(id, tok, protocol.PickRequest{Round: pk.Round, PickNo: pk.PickNo, ID: pick}); err != nil {
			t.Fatalf("pick %d: %v", i, err)
		}
		if i > 100 {
			t.Fatal("draft never ended")
		}
	}
	waitPhase(t, p.h, id, "BUILD", 20*time.Second)

	pool, err := p.h.EventPool(id, tok)
	if err != nil {
		t.Fatal(err)
	}
	if len(pool.Pool) != 60 {
		t.Fatalf("pool has %d cards, want 60", len(pool.Pool))
	}
	sug := pool.Suggestion
	if len(sug.Commanders) == 0 {
		t.Fatalf("the suggestion names no commander: %+v", sug)
	}
	chk, err := p.h.CheckDeck(id, tok, sug)
	if err != nil || !chk.Valid || chk.Size != 60 || chk.Total < 60 {
		t.Fatalf("suggestion check %+v err %v", chk, err)
	}
	// A deck with no commander, and one with a card outside the identity,
	// are refused.
	bare := sug
	bare.Commanders = nil
	if chk, _ := p.h.CheckDeck(id, tok, bare); chk.Valid {
		t.Fatal("a commander-less deck checked valid")
	}
	if out := outsideIdentity(pool.Pool, chk.Identity, sug.Main); out != "" {
		bad := sug
		bad.Main = append(append([]string(nil), sug.Main...), out)
		if chk, _ := p.h.CheckDeck(id, tok, bad); chk.Valid || !strings.Contains(chk.Error, "identity") {
			t.Fatalf("an out-of-identity card checked %+v", chk)
		}
		if _, err := p.h.SaveDeck(id, tok, bad); err == nil {
			t.Fatal("an out-of-identity deck saved")
		}
	}
	if _, err := p.h.SaveDeck(id, tok, sug); err != nil {
		t.Fatalf("SaveDeck: %v", err)
	}
	if _, err := p.h.Ready(id, tok, protocol.ReadyRequest{Rev: 1}); err != nil {
		t.Fatalf("Ready: %v", err)
	}
	waitPhase(t, p.h, id, "ROUND1", 20*time.Second)

	// The pod game: one live four-seat row with seat 0's join link.
	st = stateOf(t, p.h, id, tok)
	if len(st.Pod) != 1 || st.Pod[0].State != "live" || st.Pod[0].Join == "" {
		t.Fatalf("pod rows %+v", st.Pod)
	}
	table := host.TableID(st.Pod[0].Table)
	if pub := stateOf(t, p.h, id, ""); pub.Pod[0].Join != "" {
		t.Fatal("the spectator view carries a join link")
	}
	sawLife := false
	// A generous budget: the loop ends with the event, so only a loaded CI
	// runner ever waits this long.
	deadline := time.Now().Add(120 * time.Second)
	var last uint64
	for {
		if stateOf(t, p.h, id, "").Phase == "COMPLETE" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the pod game never ended: %+v", stateOf(t, p.h, id, "").Pod)
		}
		if !sawLife {
			if v, err := p.reg.ViewAt(table, 1, 1); err == nil && len(v.Players) == 4 {
				for _, pl := range v.Players {
					if pl.Life != 40 {
						t.Fatalf("a pod player starts at %d life", pl.Life)
					}
				}
				sawLife = true
			}
		}
		d, err := p.reg.Pending(table, 1, state.PlayerID(0))
		if err != nil || d == nil || d.Seq == last {
			time.Sleep(pollEvery)
			continue
		}
		if err := p.reg.SubmitIntent(table, 1, state.PlayerID(0), legalIntent(d)); err == nil {
			last = d.Seq
		}
	}
	st = stateOf(t, p.h, id, tok)
	if len(st.Pod) != 1 || st.Pod[0].State != "ended" || len(st.Standings) != 4 {
		t.Fatalf("after the game: pod %+v standings %+v", st.Pod, st.Standings)
	}
	if !sawLife {
		t.Log("the table ended before its first view was read")
	}
}

// outsideIdentity returns a pool card not already maindecked whose identity
// is outside ident, or "".
func outsideIdentity(pool []protocol.DraftCard, ident string, main []string) string {
	in := map[string]bool{}
	for _, id := range main {
		in[id] = true
	}
	for _, c := range pool {
		if in[c.ID] {
			continue
		}
		for _, ch := range c.Identity {
			if !strings.ContainsRune(ident, ch) {
				return c.ID
			}
		}
	}
	return ""
}

// hasCommander reports whether a card list holds a commander-eligible card.
func hasCommander(cs []protocol.DraftCard) bool {
	for _, c := range cs {
		if c.Commander {
			return true
		}
	}
	return false
}

// TestCommanderBotPodCompletes: a bot-only Commander draft drafts, builds and
// plays its pod game to COMPLETE with no human anywhere, and records one
// PodGameEnded.
func TestCommanderBotPodCompletes(t *testing.T) {
	p := startGamePair(t, "", nil)
	req := commanderReq()
	req.Seats[0] = protocol.CreateEventSeat{Kind: "bot"}
	resp := createOK(t, p.h, req)
	waitPhase(t, p.h, resp.Event, "COMPLETE", 50*time.Second)
	st := stateOf(t, p.h, resp.Event, "")
	if len(st.Pod) != 1 || st.Pod[0].State != "ended" || st.Pod[0].Table == "" {
		t.Fatalf("pod rows %+v", st.Pod)
	}
	r := runOf(t, p.h, resp.Event)
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, en := range r.ev.Log() {
		if en.Kind == "PodGameEnded" {
			t.Logf("pod game: winner %v head %q attempts %d", en.Winner, en.Head, r.podAttempts[1])
			if en.Head == "crashed" || en.Head == "" {
				t.Fatalf("the pod game did not finish: head %q", en.Head)
			}
		}
	}
}

// TestCommanderDeckSizeFollowsTheEvent: a run's Commander deck size is the
// event's EventCreated DeckSize, build.CommanderDeckSize before the event is
// created or when it names none, and the Commander persona's drafter (Pick
// and the advice endpoint's Rank) and builder score for it.
func TestCommanderDeckSizeFollowsTheEvent(t *testing.T) {
	r := &run{ev: event.New()}
	if got := r.commanderDeckSize(); got != build.CommanderDeckSize {
		t.Fatalf("before EventCreated: deck size %d, want %d", got, build.CommanderDeckSize)
	}
	seats := make([]event.SeatSpec, 4)
	for i := range seats {
		seats[i] = event.SeatSpec{Kind: "bot", Name: fmt.Sprintf("bot%d", i)}
	}
	for _, tc := range []struct{ named, want int }{{0, build.CommanderDeckSize}, {50, 50}} {
		ev := event.New()
		if err := ev.Apply(event.Entry{Kind: event.KindEventCreated, Seed: 9, Set: "TST", Seats: seats,
			Format: event.FormatCommander, Structure: event.StructureNone, PicksPerPass: 2, DeckSize: tc.named}); err != nil {
			t.Fatal(err)
		}
		r := &run{ev: ev}
		if got := r.commanderDeckSize(); got != tc.want {
			t.Errorf("EventCreated deck size %d: run deck size %d, want %d", tc.named, got, tc.want)
		}
		d, b := commanderPersona(nil, r.commanderDeckSize(), nil)
		if got := d.(*pickbot.CommanderDrafter).DeckSize; got != tc.want {
			t.Errorf("EventCreated deck size %d: drafter DeckSize %d, want %d", tc.named, got, tc.want)
		}
		if got := b.(*pickbot.CommanderBuilder).DeckSize; got != tc.want {
			t.Errorf("EventCreated deck size %d: builder DeckSize %d, want %d", tc.named, got, tc.want)
		}
	}
}

// commanderFirstPicks creates a seeded one-human MBC Commander draft on a
// host built with value (nil = Heuristic) and reads back the human's first
// pack, its /advice for that pack, and bot seat 1's first pick (its name).
func commanderFirstPicks(t *testing.T, value func(*cards.Card, limited.CardRef) float64) (protocol.DraftPack, protocol.DraftAdvice, string) {
	t.Helper()
	h, _ := newHost(t, func(o *Options) { o.CommanderValue = value })
	resp := createOK(t, h, commanderReq())
	tok, _ := humanToken(t, resp)
	joinOK(t, h, resp.Event, tok, "Ann")
	pk := packOf(t, h, resp.Event, tok)
	adv, err := h.DraftAdvice(resp.Event, tok)
	if err != nil || len(adv.Picks) == 0 {
		t.Fatalf("advice %+v err %v", adv, err)
	}
	r := runOf(t, h, resp.Event)
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, en := range r.ev.Log() {
		if en.Kind == event.KindPicked && en.Seat == 1 && en.Round == 1 && en.PickNo == 1 {
			for _, c := range r.ev.Pool(1) {
				if c.ID == en.ID {
					return pk, adv, c.Name
				}
			}
		}
	}
	t.Fatal("bot seat 1 made no first pick")
	return pk, adv, ""
}

// nameOfID is the pack card's name for an id.
func nameOfID(pk protocol.DraftPack, id string) string {
	for _, c := range pk.Pack {
		if c.ID == id {
			return c.Name
		}
	}
	return ""
}

// TestCommanderValueReachesBotsAndAdvice: Options.CommanderValue (a ratings
// table loaded as gorged's -commander-ratings does, through
// pickbot.ParseRatingsOverride) replaces Heuristic for every Commander-draft
// seat: a stub table that rates the default top picks lowest and the human's
// other pack commanders highest changes a bot's first pick and the human's
// /advice rank order, and the advice's Value is the table's.
func TestCommanderValueReachesBotsAndAdvice(t *testing.T) {
	pk, adv, botPick := commanderFirstPicks(t, nil)
	top := nameOfID(pk, adv.Picks[0].ID)
	table := map[string]map[string]float64{top: {"blended": 0}, botPick: {"blended": 0}}
	for _, c := range pk.Pack {
		if c.Commander && c.Name != top && c.Name != botPick {
			table[c.Name] = map[string]float64{"blended": 1}
		}
	}
	if len(table) < 3 {
		t.Fatalf("seed 7's first pack holds no second commander: %d table entries", len(table))
	}
	raw, err := json.Marshal(table)
	if err != nil {
		t.Fatal(err)
	}
	ov, err := pickbot.ParseRatingsOverride(raw, "stub.json", "blended", "MBC")
	if err != nil {
		t.Fatal(err)
	}
	value := ov.CommanderValue(pickbot.DefaultTuning)

	pk2, adv2, botPick2 := commanderFirstPicks(t, value)
	if nameOfID(pk2, pk2.Pack[0].ID) != nameOfID(pk, pk.Pack[0].ID) {
		t.Fatal("the two hosts dealt different packs")
	}
	top2 := nameOfID(pk2, adv2.Picks[0].ID)
	t.Logf("advice top %q -> %q; bot seat 1 first pick %q -> %q", top, top2, botPick, botPick2)
	if top2 == top {
		t.Errorf("advice top pick %q unchanged by the value table", top)
	}
	if table[top2]["blended"] != 1 {
		t.Errorf("advice top pick %q is not a card the table rates highest", top2)
	}
	if botPick2 == botPick {
		t.Errorf("bot seat 1's first pick %q unchanged by the value table", botPick)
	}
	want := value(nil, limited.CardRef{Name: top2})
	if got := adv2.Picks[0].Value; got != want {
		t.Errorf("advice value of %q = %v, want the table's %v", top2, got, want)
	}
}
