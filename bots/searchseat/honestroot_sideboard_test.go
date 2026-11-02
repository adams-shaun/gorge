package searchseat

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/bots/searchprobe"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Karn, the Great Creator fetches an artifact from outside the game. The fetched
// card joins its owner's pool, whether or not the main deck holds that name, so a
// search seat keeps its honest root for the rest of the game.
func TestHonestRootAfterAWishFromTheSideboard(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	names := []string{"a", "b"}
	lookup := func(name string) *cards.Card {
		c, ok := reg.Lookup(name)
		if !ok {
			t.Fatalf("%q not in the corpus", name)
		}
		return c
	}
	var main []*cards.Card
	for range 20 {
		main = append(main, lookup("Forest"), lookup("Forest"), lookup("Karn, the Great Creator"))
	}
	side := []*cards.Card{lookup("Mycosynth Lattice"), lookup("Walking Ballista")}
	main[0] = lookup("Walking Ballista")
	cfg := rules.Config{Names: names, Tokens: reg.Tokens, Decks: [][]*cards.Card{main, main}, Sideboards: [][]*cards.Card{side, side}}
	setup := searchprobe.PublicGame{Names: names, Decks: cfg.Decks, Tokens: reg.Tokens}
	sideboard := func(e *rules.Engine, p state.PlayerID) int { return len(e.G.Zone(state.ZSideboard, p)) }
	full := len(cfg.Sideboards[0])
	wished := 0
	for seed := uint64(1); seed <= 12 && wished < 40; seed++ {
		cfg.Seed = seed
		e := rules.New(cfg)
		e.Advance()
		rngs := searchprobe.BotRandoms(seed, 2)
		board := botpolicy.NewBoard(2)
		feeds := [2]*Feed{NewFeed(0), NewFeed(1)}
		for step := 0; !e.G.Over && step < 2000; step++ {
			d := e.Pending()
			if d == nil {
				break
			}
			for i, f := range feeds {
				if _, ok := f.Observe(e); !ok {
					t.Fatalf("seed %d: seat %d's feed stopped: %s", seed, i, f.StopReason())
				}
			}
			if sideboard(e, d.Player) < full && !rules.TapePosed(e) {
				wished++
				if _, why := HonestRoot(setup, feeds[d.Player], e, [2]uint64{seed, uint64(step)}); why != "" {
					t.Fatalf("seed %d step %d turn %d: %s", seed, step, e.G.Turn, why)
				}
			}
			in := botpolicy.Decide(botpolicy.BoardFromGameInto(e.G, e, d.Player, &board), d, rngs[d.Player])
			for _, o := range d.Options {
				if strings.Contains(o.Label, "from outside the game") {
					in = decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{o.Index}}
				}
			}
			if err := feeds[d.Player].RecordAnswer(d, in); err != nil {
				t.Fatal(err)
			}
			if err := e.Submit(in); err != nil {
				t.Fatal(err)
			}
		}
	}
	if wished == 0 {
		t.Fatal("fixture: no game fetched a card from a sideboard")
	}
	t.Logf("%d decisions after a wish", wished)
}
