package gorged

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/host/httpapi"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestCreatePodFromTextDeck: a plain-text decklist built from a repo
// Commander deck seats a 4-seat commander table, its 100 cards at seat 0, and
// the same list with one name the card pool does not hold is refused naming
// that card.
func TestCreatePodFromTextDeck(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	name := testutil.CommanderAcceptanceDeckNames()[0]
	f := testutil.RepoDeckFile(t, name)
	cmdrs := f.CommanderNames()
	if len(cmdrs) != 1 {
		t.Fatalf("%s names commanders %q, want one", name, cmdrs)
	}
	var lines []string
	total := 0
	for _, e := range f.Cards {
		if e.Name == cmdrs[0] {
			continue
		}
		for range e.Count {
			lines = append(lines, "1 "+e.Name)
		}
		total += e.Count
	}
	if total != 99 {
		t.Fatalf("%s has %d cards beside its commander, want 99", name, total)
	}
	text := "Commander\n1 " + cmdrs[0] + "\n\n" + strings.Join(lines, "\n") + "\n"

	pool := []string{"p1", "p2", "p3"}
	poolCards := testutil.RepoDeck(t, reg, name)
	base := func(id string) (host.Deck, error) {
		for _, p := range pool {
			if p == id {
				return host.Deck{Name: id, Cards: poolCards, Commanders: []int{f.CommanderIndex()}}, nil
			}
		}
		return host.Deck{}, host.ErrNotFound
	}
	c := config{imports: newDeckImports(reg)}
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

	post := func(text string) (int, []byte) {
		return podPost(t, srv.URL, podBody(t, map[string]any{
			"format": "commander", "seats": 4, "humans": 1,
			"decks": []map[string]any{{"seat": 0, "text": text}},
		}))
	}

	code, raw := post(text)
	if code != http.StatusOK {
		t.Fatalf("POST /api/games with a text deck: %d %s", code, raw)
	}
	var resp httpapi.CreateGameResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatal(err)
	}
	var info bool
	for _, tb := range r.Tables() {
		if tb.ID == resp.Table && tb.Seats == 4 && tb.Format == host.FormatCommander.String() {
			info = true
		}
	}
	if !info {
		t.Fatalf("no 4-seat commander table %q among %+v", resp.Table, r.Tables())
	}
	got, err := c.imports.loader(base)(fmt.Sprintf("import-%s-s0", resp.Table))
	if err != nil {
		t.Fatalf("seat 0's submitted deck is not stored: %v", err)
	}
	if len(got.Cards) != 100 || len(got.Commanders) != 1 || got.Cards[got.Commanders[0]].Faces[0].Name != cmdrs[0] {
		t.Fatalf("seat 0 deck has %d cards, commanders %v; want 100 with %s the commander", len(got.Cards), got.Commanders, cmdrs[0])
	}

	bad := strings.Replace(text, lines[0]+"\n", "1 Not A Real Card\n", 1)
	if bad == text {
		t.Fatal("the refused list is the accepted one")
	}
	code, raw = post(bad)
	if code != http.StatusBadRequest || !strings.Contains(string(raw), "Not A Real Card") {
		t.Fatalf("a text deck naming an unknown card: %d %s, want 400 naming it", code, raw)
	}
}
