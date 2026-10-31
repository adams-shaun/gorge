package draftlab

import (
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/draft/limited/setdata"
)

// gauntletFixture builds the gauntlet's entrants: a sealed-built hero and
// two catalog opponents.
func gauntletFixture(t *testing.T, l *Lab) (hero Entrant, opponents []Entrant) {
	t.Helper()
	builder := testBuilder(t, l, "best23")
	player := testPlayer(t, l, "bot")
	pool, err := l.SealedPool("FDN", setdata.Arena, 3000, 6)
	if err != nil {
		t.Fatalf("SealedPool: %v", err)
	}
	d, err := l.Build(pool, builder)
	if err != nil {
		t.Fatalf("Build hero: %v", err)
	}
	hero = Entrant{Deck: d, Player: player}
	catalog, err := l.CatalogDecks("fdn-limited")
	if err != nil {
		t.Fatalf("CatalogDecks: %v", err)
	}
	for i := 0; i < 2; i++ {
		opponents = append(opponents, Entrant{Deck: catalog[i], Player: player})
	}
	return hero, opponents
}

// TestGauntletTalliesAndSeats: the gauntlet's tallies sum, the hero's deck
// sits in the alternating player seat every game expects, and the worker
// count never changes the result.
func TestGauntletTalliesAndSeats(t *testing.T) {
	seed := uint64(77)
	var want GauntletResult
	for _, workers := range []int{1, 3} {
		l := testLabOpts(t, Options{Workers: workers})
		hero, opponents := gauntletFixture(t, l)
		res, err := l.Gauntlet(hero, opponents, 2, seed)
		if err != nil {
			t.Fatalf("Gauntlet: %v", err)
		}
		if workers == 1 {
			want = res
		} else if !reflect.DeepEqual(res, want) {
			t.Fatalf("workers 3 result differs from workers 1")
		}
		if len(res.Games) != 4 {
			t.Fatalf("%d games, want 4", len(res.Games))
		}
		if len(res.PerOpponent) != 2 {
			t.Fatalf("%d per-opponent tallies, want 2", len(res.PerOpponent))
		}
		for i, g := range res.Games {
			j, n := i/2, i%2
			if g.A != 0 || g.B != j+1 {
				t.Errorf("game %d has A=%d B=%d, want 0 and %d", i, g.A, g.B, j+1)
			}
			if got := g.Decks[n]; got != hero.Deck.Name {
				t.Errorf("game %d: player %d deck is %q, want the hero's %q", i, n, got, hero.Deck.Name)
			}
		}
		var wins, losses, draws, games int
		for _, tal := range res.PerOpponent {
			wins, losses, draws, games = wins+tal.Wins, losses+tal.Losses, draws+tal.Draws, games+tal.Games
		}
		p := res.Pooled
		if wins != p.Wins || losses != p.Losses || draws != p.Draws || games != p.Games {
			t.Errorf("per-opponent tallies %d-%d-%d over %d do not sum to pooled %d-%d-%d over %d",
				wins, losses, draws, games, p.Wins, p.Losses, p.Draws, p.Games)
		}
		if p.WinRate != float64(p.Wins)/float64(p.Games) {
			t.Errorf("pooled win rate %v is not wins/games %v", p.WinRate, float64(p.Wins)/float64(p.Games))
		}
		if p.Lo95 < 0 || p.Hi95 > 1 || p.Lo95 > p.WinRate || p.WinRate > p.Hi95 {
			t.Errorf("pooled ci95 [%v, %v] does not bracket win rate %v in [0, 1]", p.Lo95, p.Hi95, p.WinRate)
		}
		if res.Manifest.Kind != "gauntlet" || res.Manifest.Games != 4 {
			t.Errorf("manifest kind %q games %d, want gauntlet/4", res.Manifest.Kind, res.Manifest.Games)
		}
		if want := hero.Deck.Name + " / " + hero.Player.Ref.String(); res.Hero != want {
			t.Errorf("hero label %q, want %q", res.Hero, want)
		}
	}
}
