package gorged

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/dirac/adapter"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/host/httpapi"
	"github.com/adams-shaun/gorge/server/protocol"
	"github.com/adams-shaun/gorge/testkit/testutil"

	mb "github.com/adams-shaun/gorge/server/protocol/manabrew"
)

const (
	podCommander   = "Pod Test Commander"
	podUnsupported = "Pod Test Unsupported"
)

func podCard(t *testing.T, name, script string) *cards.Card {
	t.Helper()
	c, diags := cards.ParseBytes("pod-create.txt", []byte("Name:"+name+"\n"+script))
	if len(diags) != 0 {
		t.Fatalf("parse %q: %v", name, diags)
	}
	c.Link()
	return c
}

// podFixture is an inline card pool: a legendary commander, 200 colourless
// creatures, and one card whose script names a primitive no engine implements.
func podFixture(t *testing.T) (*cards.Registry, []*cards.Card) {
	t.Helper()
	reg := cards.NewRegistry()
	cmd := podCard(t, podCommander, "Types:Legendary Creature Human\nPT:2/2\n")
	reg.Add(cmd)
	fill := make([]*cards.Card, 200)
	for i := range fill {
		fill[i] = podCard(t, fmt.Sprintf("Pod Test Filler %03d", i), "Types:Creature Human\nPT:1/1\n")
		reg.Add(fill[i])
	}
	reg.Add(podCard(t, podUnsupported, "Types:Sorcery\nA:SP$ PodNoSuchPrimitive | Cost$ 1\n"))
	return reg, append([]*cards.Card{cmd}, fill...)
}

// podDeck is a ManaBrew deck: the commander, then names, one DeckCard each.
func podDeck(name string, names ...string) mb.Deck {
	d := mb.Deck{Name: name, Format: "commander"}
	card := func(n string) mb.DeckCard { return mb.DeckCard{Identity: mb.DeckCardIdentity{ID: "i-" + n, Name: n}} }
	d.Commanders = []mb.DeckCard{card(podCommander)}
	d.Cards = append(d.Cards, card(podCommander))
	for _, n := range names {
		d.Cards = append(d.Cards, card(n))
	}
	return d
}

// podLegal is 99 distinct fillers behind the commander, from offset.
func podLegal(name string, offset int) mb.Deck {
	names := make([]string, 99)
	for i := range names {
		names[i] = fmt.Sprintf("Pod Test Filler %03d", offset+i)
	}
	return podDeck(name, names...)
}

func podBody(t *testing.T, req map[string]any) string {
	t.Helper()
	b, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func podServer(t *testing.T, c config, pool []string) (*httptest.Server, *host.Registry, *seatGate) {
	t.Helper()
	reg, fill := podFixture(t)
	base := func(name string) (host.Deck, error) {
		for _, p := range pool {
			if p == name {
				return host.Deck{Name: name, Cards: fill[:100], Commanders: []int{0}}, nil
			}
		}
		return host.Deck{}, host.ErrNotFound
	}
	c.imports = newDeckImports(reg)
	r, err := host.New(host.Options{LoadDeck: c.imports.loader(base), Sleep: func(time.Duration, <-chan struct{}) {}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close() })
	gate := &seatGate{tokenToClaim: map[string]httpapi.SeatClaim{}, claimTokens: map[httpapi.SeatClaim]string{}}
	srv := httptest.NewServer(httpapi.NewHandler(r, httpapi.Options{
		CreateGame: c.createGame(r, gate, pool, []string{"unused"}, view.Public),
		Seat:       gate.resolve,
	}))
	t.Cleanup(srv.Close)
	return srv, r, gate
}

func podPost(t *testing.T, url, body string) (int, []byte) {
	t.Helper()
	resp, err := http.Post(url+"/api/games", "application/json", bytes.NewReader([]byte(body)))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, buf.Bytes()
}

// TestCreatePodSeatsAFourSeatCommanderTableFromOneRequest is the whole flow:
// one POST yields a 4-seat Commander table, a token per human seat and bots in
// the rest, with the submitted decks (checked by the server) at their seats.
func TestCreatePodSeatsAFourSeatCommanderTableFromOneRequest(t *testing.T) {
	srv, r, gate := podServer(t, config{}, []string{"p1", "p2", "p3", "p4"})
	body := podBody(t, map[string]any{
		"format": "commander", "seats": 4, "humans": 2,
		"decks": []map[string]any{{"seat": 1, "deck": podLegal("Seat one deck", 0)}, {"seat": 3, "deck": podLegal("Seat three deck", 100)}},
	})
	code, raw := podPost(t, srv.URL, body)
	if code != http.StatusOK {
		t.Fatalf("POST /api/games: %d %s", code, raw)
	}
	var resp httpapi.CreateGameResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Seats) != 2 || resp.Seats[0].Seat != 0 || resp.Seats[1].Seat != 1 {
		t.Fatalf("grants = %+v, want one per human seat 0 and 1", resp.Seats)
	}
	if resp.Seats[0].Token == resp.Seats[1].Token || resp.Seat != 0 || resp.Token != resp.Seats[0].Token || resp.Join != resp.Seats[0].Join {
		t.Fatalf("tokens not distinct or seat 0 not mirrored on the response: %+v", resp)
	}
	for _, g := range resp.Seats {
		if got := gate.token(host.TableID(resp.Table), state.PlayerID(g.Seat)); got != g.Token || !strings.Contains(g.Join, fmt.Sprintf("seat=%d&token=%s", g.Seat, g.Token)) {
			t.Errorf("seat %d: gate holds %q, grant %+v", g.Seat, got, g)
		}
	}
	if gate.token(host.TableID(resp.Table), 2) != "" || gate.token(host.TableID(resp.Table), 3) != "" {
		t.Error("a bot seat holds a token")
	}
	// The match is built by the table's goroutine after Start returns. The
	// 10 s deadline is the bound; the 1 ms tick only paces the poll, so no
	// PASS/FAIL depends on a short wall-clock window.
	var ms []protocol.MatchInfo
	for deadline := time.Now().Add(testutil.WaitBudget); len(ms) == 0 && time.Now().Before(deadline); time.Sleep(time.Millisecond) {
		ms, _ = r.Matches(host.TableID(resp.Table))
	}
	if len(ms) != 1 {
		t.Fatalf("matches: %+v", ms)
	}
	seats := ms[0].Seats
	if len(seats) != 4 {
		t.Fatalf("table has %d seats, want 4", len(seats))
	}
	for i, want := range []bool{true, true, false, false} {
		if seats[i].Human != want {
			t.Errorf("seat %d human = %v, want %v", i, seats[i].Human, want)
		}
	}
	if seats[1].DeckID != "import-"+resp.Table+"-s1" || seats[1].Deck != "Seat one deck" || seats[3].DeckID != "import-"+resp.Table+"-s3" {
		t.Errorf("submitted decks not at their seats: %+v", seats)
	}
	for _, i := range []int{0, 2} {
		if strings.HasPrefix(seats[i].DeckID, "import-") || seats[i].DeckID == "" {
			t.Errorf("seat %d got deck %q, want one dealt from the pool", i, seats[i].DeckID)
		}
	}
	if seen := map[string]bool{seats[0].DeckID: true, seats[2].DeckID: true}; len(seen) != 2 {
		t.Errorf("pool decks repeat across seats 0 and 2: %+v", seats)
	}
	var info *protocol.TableInfo
	for _, tb := range r.Tables() {
		if tb.ID == resp.Table {
			info = &tb
		}
	}
	if info == nil || info.Seats != 4 || info.Format != host.FormatCommander.String() {
		t.Fatalf("table info = %+v, want 4 seats commander", info)
	}
}

// TestCreatePodRefusesAnUnsupportedDeckNamingTheCards: the server decides a
// submitted deck is not playable, whatever the client says of it, and leaves
// nothing behind.
func TestCreatePodRefusesAnUnsupportedDeckNamingTheCards(t *testing.T) {
	srv, r, gate := podServer(t, config{}, []string{"p1", "p2", "p3", "p4"})
	bad := podLegal("Unplayable", 0)
	bad.Cards[5].Identity.Name = podUnsupported
	missing := podLegal("Missing", 0)
	missing.Cards[6].Identity.Name = "Pod Test Not A Card"
	for _, tc := range []struct {
		name string
		deck mb.Deck
		want []string
	}{
		{"unsupported", bad, []string{podUnsupported, "PodNoSuchPrimitive", "Unplayable"}},
		{"unresolved", missing, []string{"Pod Test Not A Card"}},
		{"short deck", podDeck("Short", "Pod Test Filler 001"), []string{"100"}},
		{"not commander", func() mb.Deck { d := podLegal("Wrong format", 0); d.Format = "standard"; return d }(), []string{"standard"}},
		{"no commander", func() mb.Deck { d := podLegal("Leaderless", 0); d.Commanders = nil; return d }(), []string{"no commander"}},
	} {
		code, raw := podPost(t, srv.URL, podBody(t, map[string]any{
			"format": "commander", "seats": 4, "decks": []map[string]any{{"seat": 0, "deck": tc.deck}}}))
		if code != http.StatusBadRequest {
			t.Errorf("%s: status %d (%s), want 400", tc.name, code, raw)
			continue
		}
		for _, w := range tc.want {
			if !strings.Contains(string(raw), w) {
				t.Errorf("%s: error %s does not name %q", tc.name, raw, w)
			}
		}
	}
	if n := len(r.Tables()); n != 0 {
		t.Errorf("%d tables left behind by refused requests", n)
	}
	if tok := gate.token("g1", 0); tok != "" {
		t.Error("a refused request minted a token")
	}
	// The deck is accepted once it is playable, in the same server.
	if code, raw := podPost(t, srv.URL, podBody(t, map[string]any{
		"format": "commander", "seats": 4, "decks": []map[string]any{{"seat": 0, "deck": podLegal("Fine", 0)}}})); code != http.StatusOK {
		t.Fatalf("legal deck after refusals: %d %s", code, raw)
	}
}

// TestCreatePodAcceptsManaBrewsOwnDeckShape: the deck a real ManaBrew client
// submits (manabrew-protocol src/deck_dto.rs Deck / DeckCard) carries each
// card's colour as a string ("color":"", "W"), an image-URI set with png, and
// the deck's colour, editor metadata and cover face as a string, an object and
// a number. Typing any of them as gorge's old guesses (a colour list, an
// editor string) refused every real deck with a 400 (F5).
func TestCreatePodAcceptsManaBrewsOwnDeckShape(t *testing.T) {
	srv, _, _ := podServer(t, config{}, []string{"p1", "p2", "p3", "p4"})
	raw, err := json.Marshal(podLegal("ManaBrew shaped", 0))
	if err != nil {
		t.Fatal(err)
	}
	var deck map[string]any
	if err := json.Unmarshal(raw, &deck); err != nil {
		t.Fatal(err)
	}
	deck["color"] = "W"
	deck["editor"] = map[string]any{"layout": map[string]any{"groupBy": "type"}}
	deck["coverCardFace"] = 0
	for _, list := range []string{"cards", "commanders"} {
		for i, c := range deck[list].([]any) {
			card := c.(map[string]any)
			card["color"] = ""
			if i%2 == 1 {
				card["color"] = "W"
			}
			card["colorIdentity"] = []string{}
			card["uris"] = map[string]any{"small": "", "normal": "", "large": "", "png": "", "art_crop": "", "border_crop": ""}
		}
	}
	code, body := podPost(t, srv.URL, podBody(t, map[string]any{
		"format": "commander", "seats": 4, "humans": 4, "decks": []map[string]any{{"seat": 0, "deck": deck}}}))
	if code != http.StatusOK {
		t.Fatalf("a ManaBrew-shaped deck was refused: %d %s", code, body)
	}
}

// TestCreatePodRefusesDiracUntilItSeatsFour: -engine dirac plays two seats, so
// a pod, and a submitted deck it never checked, are refused before any table
// is built.
func TestCreatePodRefusesDiracUntilItSeatsFour(t *testing.T) {
	c := diracConfig()
	srv, r, _ := podServer(t, c, []string{"p1", "p2", "p3", "p4"})
	for _, tc := range []struct{ name, body, want string }{
		{"pod", `{"format":"commander","seats":4}`, "cannot seat a 4-seat pod"},
		{"submitted deck", podBody(t, map[string]any{"format": "commander", "decks": []map[string]any{{"seat": 0, "deck": podLegal("D", 0)}}}), "submitted deck"},
	} {
		code, raw := podPost(t, srv.URL, tc.body)
		if code != http.StatusBadRequest || !strings.Contains(string(raw), tc.want) || !strings.Contains(string(raw), adapter.Name) {
			t.Errorf("%s: %d %s, want 400 naming dirac and %q", tc.name, code, raw, tc.want)
		}
	}
	if len(r.Tables()) != 0 {
		t.Error("a refused dirac request left a table")
	}
}
