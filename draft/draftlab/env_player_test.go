package draftlab

import (
	"errors"
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/draft/limited/setdata"
)

// envDeck builds one sealed-pool FDN deck (best23) with the given pool seed.
func envDeck(t *testing.T, l *Lab, seed uint64) Deck {
	t.Helper()
	builder := testBuilder(t, l, "best23")
	pool, err := l.SealedPool("FDN", setdata.Arena, seed, 6)
	if err != nil {
		t.Fatalf("SealedPool: %v", err)
	}
	d, err := l.Build(pool, builder)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return d
}

// TestEnvPlayerNeedsOptIn: an Env policy (sb-tactical, bots.Info.Env) is
// refused by Player with ErrDecklistBot unless Options.AllowDecklistBots is
// set (the bot-only opt-in of the operator ruling of 2026-10-13).
func TestEnvPlayerNeedsOptIn(t *testing.T) {
	l := testLab(t)
	if _, err := l.Player(AgentRef{Name: "sb-tactical"}); !errors.Is(err, ErrDecklistBot) {
		t.Fatalf("Player(sb-tactical) without opt-in: err = %v, want ErrDecklistBot", err)
	}
}

// TestEnvPlayerPlaysWithOptIn: with Options.AllowDecklistBots an Env player
// (sb-tactical) is admitted, resolves as ReadsDecklists (the flag E5-c3's
// NewManifest marks the result with), and its game runs through the bench Env
// driver to completion and deterministically. E5-c3's Manifest type is not on
// this base, so the manifest's allow_decklist_bots assertion is exercised by
// that ticket; this test pins the precondition it reads.
func TestEnvPlayerPlaysWithOptIn(t *testing.T) {
	l := testLabOpts(t, Options{AllowDecklistBots: true})
	p, err := l.Player(AgentRef{Name: "sb-tactical"})
	if err != nil {
		t.Fatalf("Player(sb-tactical) with opt-in: %v", err)
	}
	if !p.ReadsDecklists {
		t.Fatal("sb-tactical player does not declare ReadsDecklists; a manifest cannot mark the game")
	}
	bot := testPlayer(t, l, "bot")

	a := Entrant{Deck: envDeck(t, l, 3000), Player: p}
	b := Entrant{Deck: envDeck(t, l, 3001), Player: bot}

	r1, err := l.PlayGame(a, b, 424242)
	if err != nil {
		t.Fatalf("Env game with opt-in: %v", err)
	}
	if r1.Stall != "" {
		t.Fatalf("Env game stalled (%s); the Env driver did not finish it", r1.Stall)
	}
	if r1.Head == "" {
		t.Fatal("Env game produced no chain head")
	}
	r2, err := l.PlayGame(a, b, 424242)
	if err != nil {
		t.Fatalf("Env game rerun: %v", err)
	}
	if !reflect.DeepEqual(r1, r2) {
		t.Errorf("Env game is not deterministic:\n%+v\n%+v", r1, r2)
	}
}
