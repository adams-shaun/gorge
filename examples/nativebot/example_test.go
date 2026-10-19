package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/protocol"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func TestNativeBotPlaysAGame(t *testing.T) {
	if _, ok := bots.Lookup(Name); !ok {
		t.Fatalf("policy %q is not registered", Name)
	}
	names, decks := testutil.SampleDecks(t, 2)
	load := func(name string) (host.Deck, error) {
		for i, n := range names {
			if n == name {
				return host.Deck{Name: n, Cards: decks[i]}, nil
			}
		}
		return host.Deck{}, fmt.Errorf("unknown sample deck %q", name)
	}
	run := host.RunContractTable(t, host.Options{
		LoadDeck: load,
		Sleep:    func(time.Duration, <-chan struct{}) {},
	}, host.TableConfig{
		ID:              "nativebot",
		Name:            "nativebot",
		Seats:           2,
		Decks:           names,
		Seed:            7,
		BotPolicy:       Name,
		SeatBotPolicies: []string{Name, bots.Default},
		AutoMana:        true,
		BotAutoPayMana:  true,
		Spectator:       view.Omniscient,
	})
	if run.Info.State != protocol.MatchFinished {
		t.Fatalf("match ended in state %q (reason %q), want %q", run.Info.State, run.Reason, protocol.MatchFinished)
	}
	if len(run.Log.Intents) == 0 {
		t.Fatal("match recorded no intents")
	}
}
