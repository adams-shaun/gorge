package main

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/protocol"
)

func TestServesTablesOverHTTP(t *testing.T) {
	testutil.CorpusRegistry(t) // Skips when .cards/ is absent
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	cfg := config{cards: "../../.cards", decks: "../../internal/testutil/decks", tables: 1, seats: 2, pace: 0,
		cooldown: 0, dir: t.TempDir(), spectator: "omniscient", seed: 1, perpetual: false}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- serve(ctx, cfg, ln) }()
	url := "http://" + ln.Addr().String()
	var tables []protocol.TableInfo
	deadline := time.Now().Add(10 * time.Second)
	for {
		resp, err := http.Get(url + "/api/tables")
		if err == nil {
			_ = json.NewDecoder(resp.Body).Decode(&tables)
			resp.Body.Close()
			if len(tables) == 1 {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("no tables served: %v %+v", err, tables)
		}
		time.Sleep(50 * time.Millisecond)
	}
	if tables[0].ID != "t1" || tables[0].Seats != 2 || tables[0].Spectator != "omniscient" {
		t.Fatalf("%+v", tables[0])
	}
	resp, err := http.Get(url + "/")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 && resp.StatusCode != 503 {
		t.Fatalf("/: %d", resp.StatusCode)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("serve: %v", err)
	}
}

// TestHostThreadsCorpusTokens is the FL-40 required proof that gorged
// populates host.Options.Tokens from the opened corpus. A hosted table that
// does not get the token corpus mints no tokens and every token card is
// silently dead, so opening the corpus but dropping its Tokens onto the
// floor is exactly the failure this test is meant to catch.
func TestHostThreadsCorpusTokens(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	cfg := config{cards: "../../.cards", decks: "../../internal/testutil/decks", dir: t.TempDir()}
	opts := cfg.hostOptions(reg, deckLoader(reg, cfg.decks))
	if len(opts.Tokens) == 0 {
		t.Fatalf("host.Options.Tokens empty: token corpus not threaded from %s", cfg.cards)
	}
	if reg.Tokens == nil {
		t.Fatalf("corpus returned a nil Tokens map")
	}
}

func TestDeckDirectoryListingIsSortedAndComplete(t *testing.T) {
	names, err := deckFiles("../../internal/testutil/decks")
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 12 || names[0] != "death-n-taxes" || names[len(names)-1] != "uw-tempo" {
		t.Fatalf("%v", names)
	}
}
