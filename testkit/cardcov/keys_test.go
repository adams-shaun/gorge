package cardcov

import (
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/server/host"
	gbench "github.com/adams-shaun/gorge/testkit/bench"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// play drives one seeded bot-vs-bot game over two decks and returns the
// finished engine with the probe that watched it.
func play(t *testing.T, reg *cards.Registry, decks [][]*cards.Card, seed uint64, maxTurns int) (*rules.Engine, *Probe) {
	t.Helper()
	cfg := rules.Config{Names: []string{"a", "b"}, Decks: decks, Tokens: reg.Tokens, Seed: seed, NameUniverse: reg.Universe()}
	seats := make([]seat.Seat, len(decks))
	for i := range seats {
		s, err := host.NewBotPolicySeatWithAutoPayMana(host.BotPolicy, seed^(0x9e3779b97f4a7c15*uint64(i+1)), false)
		if err != nil {
			t.Fatal(err)
		}
		seats[i] = s
	}
	p := &Probe{}
	_, e, err := gbench.PlayGame(cfg, seats, maxTurns, 20000, gbench.Hooks{
		Guard: func(e *rules.Engine) (string, string) { p.Observe(e); return "", "" },
		Setup: p.Install,
	})
	if err != nil {
		t.Fatalf("game: %v", err)
	}
	return e, p
}

// TestModeKeysCredited plays seeded games over repo decks holding a modal
// spell (a Charm) and expects a played mode to be credited as a "f0/s/m<k>"
// key in Used, one the card's ModeKeys lists and its Keys do not.
func TestModeKeysCredited(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	var credited []string
	for _, name := range []string{"avengers-assemble", "tinker-time", "divine-convocation", "call-for-backup", "foundations-calling-all-angels"} {
		deck := testutil.RepoDeck(t, reg, name)
		modal := map[string]*cards.Card{}
		for _, c := range deck {
			if len(ModeKeys(c)) > 0 {
				modal[cardName(c)] = c
			}
		}
		if len(modal) == 0 {
			t.Fatalf("setup: deck %s holds no modal card", name)
		}
		for seed := uint64(1); seed <= 3; seed++ {
			e, p := play(t, reg, [][]*cards.Card{deck, deck}, seed, 14)
			g := Played(e, [][]*cards.Card{deck, deck}, p)
			for n, keys := range g.Used {
				for k := range keys {
					if !IsModeKey(k) {
						continue
					}
					c := modal[n]
					if c == nil {
						t.Fatalf("%s credited mode key %s but is not a modal card", n, k)
					}
					if !contains(ModeKeys(c), k) || contains(Keys(c), k) {
						t.Errorf("%s: credited %s outside ModeKeys %v (Keys %v)", n, k, ModeKeys(c), Keys(c))
					}
					if strings.HasPrefix(k, "f0/s/m") {
						credited = append(credited, name+"/"+n+"/"+k)
					}
				}
			}
		}
	}
	if len(credited) == 0 {
		t.Fatalf("no f0/s/m<k> key credited across the modal decks")
	}
	sort.Strings(credited)
	t.Logf("credited: %v", credited)
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// TestIsModeKey pins the key split between Keys and ModeKeys.
func TestIsModeKey(t *testing.T) {
	t.Parallel()
	for k, want := range map[string]bool{"f0/s/m0": true, "f1/a2/m10": true, "f0/a0": false, "f0/t3": false, "cast": false, "f0/s/m": false, "f0/s/mx": false} {
		if got := IsModeKey(k); got != want {
			t.Errorf("IsModeKey(%q) = %v, want %v", k, got, want)
		}
	}
}
