package gorged

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/host/httpapi"
	"github.com/adams-shaun/gorge/server/protocol"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestRestartAfterSubmittedDeckSeatsTheSameDeck: a two-seat vs-bot game whose
// human seat brought a text deck can be restarted with the body the web
// client's startRematch builds from SeatInfo.deck_id (human_deck = the
// import id, bot_deck = the pool id), and the new game seats the same cards
// under its own import id, so it outlives the first table.
func TestRestartAfterSubmittedDeckSeatsTheSameDeck(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	f := testutil.RepoDeckFile(t, "ur-delver")
	text := constructedText(f)
	pool := []string{"p1", "p2"}
	poolCards := testutil.RepoDeck(t, reg, "ur-delver")
	base := func(id string) (host.Deck, error) {
		if slices.Contains(pool, id) {
			return host.Deck{Name: id, Cards: poolCards}, nil
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
		CreateGame: c.createGame(r, gate, []string{"unused"}, pool, view.Public),
		Seat:       gate.resolve,
	}))
	t.Cleanup(srv.Close)

	create := func(body map[string]any) httpapi.CreateGameResponse {
		t.Helper()
		code, raw := podPost(t, srv.URL, podBody(t, body))
		if code != http.StatusOK {
			t.Fatalf("POST /api/games %v: %d %s", body, code, raw)
		}
		var resp httpapi.CreateGameResponse
		if err := json.Unmarshal(raw, &resp); err != nil {
			t.Fatal(err)
		}
		return resp
	}
	seatsOf := func(table string) []protocol.SeatInfo {
		t.Helper()
		deadline := time.Now().Add(testutil.WaitBudget)
		for {
			ms, err := r.Matches(host.TableID(table))
			if err != nil {
				t.Fatal(err)
			}
			if len(ms) == 1 && len(ms[0].Seats) == 2 {
				return ms[0].Seats
			}
			if time.Now().After(deadline) {
				t.Fatalf("match 1 of table %q never seated: %+v", table, ms)
			}
			time.Sleep(time.Millisecond)
		}
	}
	cardNames := func(id string) []string {
		t.Helper()
		d, err := c.imports.loader(base)(id)
		if err != nil {
			t.Fatalf("deck %q: %v", id, err)
		}
		out := make([]string, len(d.Cards))
		for i, cd := range d.Cards {
			out[i] = frontFaceName(cd, "")
		}
		return out
	}

	first := create(map[string]any{
		"format": "constructed",
		"decks":  []map[string]any{{"seat": 0, "text": text}},
	})
	seats := seatsOf(first.Table)
	if len(seats) != 2 {
		t.Fatalf("first game seats %+v, want two", seats)
	}
	humanDeck, botDeck := seats[0].DeckID, seats[1].DeckID
	if !strings.HasPrefix(humanDeck, importDeckID(host.TableID(first.Table), 0)+"-") || !slices.Contains(pool, botDeck) {
		t.Fatalf("first game deck ids %q / %q, want the seat-0 import id and a pool id", humanDeck, botDeck)
	}
	want := cardNames(humanDeck)
	if len(want) < minConstructedDeckCards {
		t.Fatalf("submitted deck seats %d cards", len(want))
	}

	// The body web/src/lib/playvsbot.ts:startRematch posts.
	again := create(map[string]any{
		"format": "constructed", "human_deck": humanDeck, "bot_deck": botDeck,
		"bot_policy": first.BotPolicy, "mulligans": 6,
	})
	if again.Table == first.Table {
		t.Fatalf("rematch reused table %q", first.Table)
	}
	re := seatsOf(again.Table)
	if len(re) != 2 || re[1].DeckID != botDeck {
		t.Fatalf("rematch seats %+v, want bot deck %q at seat 1", re, botDeck)
	}
	if !strings.HasPrefix(re[0].DeckID, importDeckID(host.TableID(again.Table), 0)+"-") {
		t.Fatalf("rematch seat 0 deck id %q, want its own import id", re[0].DeckID)
	}
	if got := cardNames(re[0].DeckID); !slices.Equal(got, want) {
		t.Fatalf("rematch seat 0 seats %d cards %v, want the submitted %d", len(got), got, len(want))
	}

	// A rematch is itself restartable: its own id seats the deck again.
	third := create(map[string]any{
		"format": "constructed", "human_deck": re[0].DeckID, "bot_deck": botDeck,
		"bot_policy": again.BotPolicy, "mulligans": 6,
	})
	re3 := seatsOf(third.Table)
	if re3[0].DeckID == re[0].DeckID || !strings.HasPrefix(re3[0].DeckID, importDeckID(host.TableID(third.Table), 0)+"-") {
		t.Fatalf("second rematch seat 0 deck id %q, want its own import id", re3[0].DeckID)
	}
	if got := cardNames(re3[0].DeckID); !slices.Equal(got, want) {
		t.Fatalf("second rematch seat 0 seats %d cards, want the submitted %d", len(got), len(want))
	}

	// A table with two people keeps its submitted lists to itself: naming
	// one from another request is still an unknown deck.
	shared := create(map[string]any{
		"format": "constructed", "humans": 2,
		"decks": []map[string]any{{"seat": 0, "text": text}},
	})
	sharedID := importDeckID(host.TableID(shared.Table), 0)
	if _, err := c.imports.loader(base)(sharedID); err != nil {
		t.Fatalf("two-human table's deck is not stored: %v", err)
	}
	code, raw := podPost(t, srv.URL, podBody(t, map[string]any{
		"format": "constructed", "human_deck": sharedID, "bot_deck": botDeck,
	}))
	if code != http.StatusBadRequest || !strings.Contains(string(raw), "unknown human deck") {
		t.Fatalf("naming a two-human table's deck: %d %s, want 400 unknown human deck", code, raw)
	}

	// The rematch's copy survives the first table's reaping, which releases
	// the first table's own copy.
	c.imports.dropTable(host.TableID(first.Table))
	if got := cardNames(re[0].DeckID); !slices.Equal(got, want) {
		t.Fatal("rematch deck went with the first table")
	}
	if _, err := c.imports.loader(base)(humanDeck); err == nil {
		t.Fatalf("deck %q is still stored after its table was reaped", humanDeck)
	}
}
