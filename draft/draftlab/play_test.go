package draftlab

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/draft/limited/setdata"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// testLabOpts opens the lab over the shared corpus with explicit options.
func testLabOpts(t *testing.T, o Options) *Lab {
	t.Helper()
	l, err := Open(testutil.CorpusDirOrSkip(t), o)
	if err != nil {
		t.Fatalf("draftlab Open: %v", err)
	}
	return l
}

// testPlayer resolves name as a player and fails the test on an error.
func testPlayer(t *testing.T, l *Lab, name string) NamedPlayer {
	t.Helper()
	p, err := l.Player(AgentRef{Name: name})
	if err != nil {
		t.Fatalf("Player(%q): %v", name, err)
	}
	return p
}

// testEntrants builds n sealed-pool entrants (best23 decks, the bot player),
// each with its own pool and a distinct draft id.
func testEntrants(t *testing.T, l *Lab, n int) []Entrant {
	t.Helper()
	builder := testBuilder(t, l, "best23")
	player := testPlayer(t, l, "bot")
	out := make([]Entrant, n)
	for i := range out {
		pool, err := l.SealedPool("FDN", setdata.Arena, uint64(2000+i), 6)
		if err != nil {
			t.Fatalf("SealedPool %d: %v", i, err)
		}
		d, err := l.Build(pool, builder)
		if err != nil {
			t.Fatalf("Build %d: %v", i, err)
		}
		d.DraftID = fmt.Sprintf("draft-%d", i)
		out[i] = Entrant{Deck: d, Player: player}
	}
	return out
}

// TestPlayGameDeterministic: the same entrants and seed give an identical
// result; a different seed gives a different chain head.
func TestPlayGameDeterministic(t *testing.T) {
	l := testLab(t)
	es := testEntrants(t, l, 2)
	r1, err := l.PlayGame(es[0], es[1], 12345)
	if err != nil {
		t.Fatalf("PlayGame: %v", err)
	}
	r2, err := l.PlayGame(es[0], es[1], 12345)
	if err != nil {
		t.Fatalf("PlayGame: %v", err)
	}
	if !reflect.DeepEqual(r1, r2) {
		t.Errorf("same seed produced different results:\n%+v\n%+v", r1, r2)
	}
	if r1.Head == "" {
		t.Fatal("no chain head (the game did not run)")
	}
	r3, err := l.PlayGame(es[0], es[1], 99999)
	if err != nil {
		t.Fatalf("PlayGame: %v", err)
	}
	if r3.Head == r1.Head {
		t.Errorf("different seeds produced the same head %s", r1.Head)
	}
}

// TestPlayGameTelemetry: opening hand and drawn cards are consistent with
// the London mulligan count and the deck; the sideboard plus the main
// deck's non-basic names reconstruct the pool.
func TestPlayGameTelemetry(t *testing.T) {
	l := testLab(t)
	es := testEntrants(t, l, 2)
	r, err := l.PlayGame(es[0], es[1], 777)
	if err != nil {
		t.Fatalf("PlayGame: %v", err)
	}
	if r.Starter != 0 && r.Starter != 1 {
		t.Errorf("Starter = %d, want 0 or 1", r.Starter)
	}
	for p := 0; p < 2; p++ {
		if len(r.Main[p]) < 40 {
			t.Fatalf("player %d Main has %d cards, want >= 40 (precondition)", p, len(r.Main[p]))
		}
		if r.Mulligans[p] > 2 {
			t.Fatalf("player %d took %d mulligans, want <= 2 (precondition)", p, r.Mulligans[p])
		}
		if got, want := len(r.OpeningHand[p]), 7-r.Mulligans[p]; got != want {
			t.Errorf("player %d opening hand has %d cards, want %d (7 - %d mulligans)", p, got, want, r.Mulligans[p])
		}
		if !subMultiset(r.OpeningHand[p], r.Main[p]) {
			t.Errorf("player %d opening hand %v is not a sub-multiset of the deck", p, r.OpeningHand[p])
		}
		if !subMultiset(r.Drawn[p], r.Main[p]) {
			t.Errorf("player %d drawn %v is not a sub-multiset of the deck", p, r.Drawn[p])
		}
	}

	// Sideboard + main's non-basic names == the pool's names.
	for p := 0; p < 2; p++ {
		if es[p].Deck.Build == nil {
			t.Fatalf("player %d deck is not a built deck (precondition)", p)
		}
		got := append(append([]string{}, r.Sideboard[p]...), nonBasic(r.Main[p])...)
		sort.Strings(got)
		want := poolNames(es[p].Deck)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("player %d: sideboard+main non-basics = %v, want the pool %v", p, got, want)
		}
	}
}

// TestPlayGameVerifies: with Options.Verify the engine replay of a completed
// game agrees with its own chain.
func TestPlayGameVerifies(t *testing.T) {
	l := testLabOpts(t, Options{Verify: true})
	es := testEntrants(t, l, 2)
	r, err := l.PlayGame(es[0], es[1], 4242)
	if err != nil {
		t.Fatalf("PlayGame with Verify: %v", err)
	}
	if r.Stall != "" {
		t.Fatalf("game stalled (%s); the verify path was not exercised", r.Stall)
	}
	if r.Head == "" {
		t.Error("no chain head")
	}
}

// TestPlayRefusals: bad entrant counts and Env players are refused.
func TestPlayRefusals(t *testing.T) {
	l := testLab(t)
	es := testEntrants(t, l, 4)
	if _, err := l.Play(es[:3], PlayOptions{}, 1); err == nil {
		t.Error("Play accepted 3 entrants")
	}
	if _, err := l.Play(nil, PlayOptions{}, 1); err == nil {
		t.Error("Play accepted 0 entrants")
	}

	env := Entrant{
		Deck: es[0].Deck,
		Player: NamedPlayer{
			Ref:            AgentRef{Name: "env"},
			ReadsDecklists: true,
			New: func(seed uint64) (seat.Seat, error) {
				return bots.New("bot", bots.Options{Seed: seed, AutoPayMana: true, Deps: bots.Deps{Cards: l.reg}})
			},
		},
	}
	// Without the opt-in: refused outright.
	if _, err := l.PlayGame(env, es[1], 2); !errors.Is(err, ErrDecklistBot) {
		t.Errorf("Env player without opt-in: err = %v, want ErrDecklistBot", err)
	} else if strings.Contains(err.Error(), "E5-c5") {
		t.Errorf("Env player without opt-in: got the E5-c5 error %q, want the opt-in refusal", err)
	}
	// With the opt-in: admitted; the game plays (E5-c5 lifted the refusal).
	l2 := testLabOpts(t, Options{AllowDecklistBots: true})
	if _, err := l2.PlayGame(env, es[1], 2); err != nil {
		t.Errorf("Env player with opt-in: err = %v, want a played game", err)
	}
}

// poolNames is a built deck's pool card names, sorted.
func poolNames(d Deck) []string {
	out := make([]string, 0, len(d.Pool))
	for _, ref := range d.Pool {
		out = append(out, ref.Name)
	}
	sort.Strings(out)
	return out
}

// nonBasic filters a name list to its non-basic cards.
func nonBasic(names []string) []string {
	var out []string
	for _, n := range names {
		if !isBasicName(n) {
			out = append(out, n)
		}
	}
	return out
}

// isBasicName reports whether name is one of the five basic lands.
func isBasicName(name string) bool {
	switch name {
	case "Plains", "Island", "Swamp", "Mountain", "Forest":
		return true
	}
	return false
}

// subMultiset reports whether every element of sub appears at least as many
// times in super.
func subMultiset(sub, super []string) bool {
	count := make(map[string]int, len(super))
	for _, n := range super {
		count[n]++
	}
	for _, n := range sub {
		if count[n] == 0 {
			return false
		}
		count[n]--
	}
	return true
}
