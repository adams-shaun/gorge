package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// TestCreatePodTextDeckShape pins the handler's half of a text decklist: a
// seat names exactly one of deck and text, text is capped at 64 KiB, and a
// text deck reaches the builder with Text set and Deck empty.
func TestCreatePodTextDeckShape(t *testing.T) {
	var got CreateGameOptions
	srv := podServer(t, &got)

	text := "Commander\n1 Ghoulcaller Gisa\n\n1 Sol Ring\n35 Swamp\n"
	body, err := json.Marshal(CreateGameRequest{Format: "commander", Seats: 4, Humans: 1,
		Decks: []SeatDeck{{Seat: 0, Text: text}}})
	if err != nil {
		t.Fatal(err)
	}
	if code, e, raw := postGames(t, srv.URL, string(body)); code != http.StatusOK {
		t.Fatalf("text deck: %d %+v %s", code, e, raw)
	}
	if len(got.Decks) != 1 || got.Decks[0].Seat != 0 || got.Decks[0].Text != text || len(got.Decks[0].Deck) != 0 {
		t.Fatalf("decks handed to CreateGame = %+v, want seat 0 with the text and no deck", got.Decks)
	}

	over, err := json.Marshal(CreateGameRequest{Format: "commander", Seats: 4,
		Decks: []SeatDeck{{Seat: 2, Text: strings.Repeat("x", maxDeckTextBytes+1)}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(over) >= maxCreateGameBody {
		t.Fatalf("the over-size request is %d bytes, past the %d body cap: it would fail for the wrong reason", len(over), maxCreateGameBody)
	}
	for _, tc := range []struct{ name, body, want string }{
		{"deck and text", `{"format":"commander","seats":4,"decks":[{"seat":1,"deck":{"name":"x"},"text":"1 Sol Ring"}]}`, "decks: seat 1 needs exactly one of deck and text"},
		{"neither", `{"format":"commander","seats":4,"decks":[{"seat":1}]}`, "decks: seat 1 needs exactly one of deck and text"},
		{"null deck no text", `{"format":"commander","seats":4,"decks":[{"seat":1,"deck":null}]}`, "decks: seat 1 needs exactly one of deck and text"},
		{"text over 64 KiB", string(over), "decks: seat 2 text is over 65536 bytes"},
	} {
		code, e, raw := postGames(t, srv.URL, tc.body)
		if code != http.StatusBadRequest || e.Code != "bad_request" || e.Message != tc.want {
			t.Errorf("%s: %d %q %q (%s), want 400 bad_request %q", tc.name, code, e.Code, e.Message, raw, tc.want)
		}
	}
}
