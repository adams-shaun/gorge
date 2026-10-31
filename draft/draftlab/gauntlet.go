package draftlab

import (
	"fmt"
	"math"
	"math/rand/v2"
)

// gauntletSaltBase tags the gauntlet's per-game seed streams (E5-plan §5:
// game (j, n) = rand.NewPCG(seed, 0x1AB6A4<<32|j<<16|n)). It sits next to
// Games' 0x1AB6A3 so the two batch shapes never share a stream.
const gauntletSaltBase = 0x1AB6A4

// Tally is one measured side's win-loss-draw record over its gauntlet
// games: the counts, the win proportion and its Wilson score 95% interval.
// Wins + Losses + Draws = Games.
type Tally struct {
	Wins, Losses, Draws int
	Games               int
	WinRate             float64
	Lo95, Hi95          float64
}

// GauntletResult is one finished gauntlet: the hero and opponent labels,
// a tally per opponent, the pooled tally, every game in opponent order
// then game order, and the run's manifest.
type GauntletResult struct {
	// Hero and Opponents label the entrants "<deck name> / <player ref>".
	Hero      string
	Opponents []string
	// PerOpponent is one tally per opponent, hero's side, in the order the
	// opponents were given.
	PerOpponent []Tally
	// Pooled folds every per-opponent tally together.
	Pooled Tally
	// Games is every game in opponent order then game order. A and B are
	// hero's view: A is 0 (the hero) and B is the opponent's index + 1,
	// whichever player seat each sat in.
	Games []GameResult
	// Manifest is the run's provenance (kind "gauntlet").
	Manifest Manifest
}

// GauntletGameSeed is the engine seed of game g against opponent j in the
// gauntlet seeded seed.
func GauntletGameSeed(seed uint64, j, g int) uint64 {
	return rand.New(rand.NewPCG(seed, gauntletSaltBase<<32|uint64(j)<<16|uint64(g))).Uint64()
}

// GauntletGame plays exactly one gauntlet game -- game g against opp at the
// engine seed gs (GauntletGameSeed) -- as Gauntlet plays it, so a single
// game of a long run can be reproduced on its own.
func (l *Lab) GauntletGame(hero, opp Entrant, g int, gs uint64) (GameResult, error) {
	a, b := hero, opp
	if g%2 == 1 { // the player seat the hero sits in for game g
		a, b = b, a
	}
	return l.playGame(a, b, entrantNames(a, b), gs, false)
}

// Gauntlet plays the hero deck against every opponent games times — game
// n of a pairing alternates which side sits in player 0's seat (one on the
// play's seat, one on the draw's; the engine's CR 103.1 toss still decides
// who is actually on the play, cmd/draftsim's rule) — and tallies the
// results from the hero's side. Up to Options.Workers games run at once
// and the results are returned in index order, so the result never depends
// on the worker count. A stall counts as a drawn game. games must be at
// least 1 and there must be at least one opponent.
func (l *Lab) Gauntlet(hero Entrant, opponents []Entrant, games int, seed uint64) (GauntletResult, error) {
	if games < 1 {
		return GauntletResult{}, fmt.Errorf("draftlab: gauntlet: %d games, want at least 1", games)
	}
	if len(opponents) < 1 {
		return GauntletResult{}, fmt.Errorf("draftlab: gauntlet: %d opponents, want at least 1", len(opponents))
	}
	results, err := l.runGames(len(opponents)*games, func(i int) (GameResult, error) {
		j, g := i/games, i%games
		res, err := l.GauntletGame(hero, opponents[j], g, GauntletGameSeed(seed, j, g))
		if err != nil {
			return GameResult{}, err
		}
		res.A, res.B = 0, j+1
		return res, nil
	})
	if err != nil {
		return GauntletResult{}, fmt.Errorf("draftlab: gauntlet: %w", err)
	}
	per := make([]Tally, len(opponents))
	for i, res := range results {
		t := &per[i/games]
		recordTally(t, res.Winner, (i%games)%2)
	}
	pooled := pooledTallies(per)
	result := GauntletResult{
		Hero:        entrantLabel(hero),
		Opponents:   make([]string, len(opponents)),
		PerOpponent: per,
		Pooled:      pooled,
		Games:       results,
	}
	for j, op := range opponents {
		result.Opponents[j] = entrantLabel(op)
	}
	agents := make([]RoleRef, 0, len(opponents)+1)
	agents = append(agents, RoleRef{Role: RolePlayer, Ref: hero.Player.Ref})
	for _, op := range opponents {
		agents = append(agents, RoleRef{Role: RolePlayer, Ref: op.Player.Ref})
	}
	result.Manifest = l.NewManifest("gauntlet", seed, agents, results)
	return result, nil
}

// recordTally folds one game into a tally: winner is the winning player
// index (-1 for a draw or a watchdog stall, cmd/draftsim's rule) and me is
// the player index the measured side sat in.
func recordTally(t *Tally, winner, me int) {
	t.Games++
	switch {
	case winner < 0:
		t.Draws++
	case winner == me:
		t.Wins++
	default:
		t.Losses++
	}
}

// pooledTallies sums a tally slice into its pooled tally.
func pooledTallies(ts []Tally) Tally {
	var out Tally
	for _, t := range ts {
		out.Wins += t.Wins
		out.Losses += t.Losses
		out.Draws += t.Draws
		out.Games += t.Games
	}
	fillRates(&out)
	return out
}

// fillRates fills a tally's WinRate and Wilson interval from its counts.
func fillRates(t *Tally) {
	t.WinRate, t.Lo95, t.Hi95 = wilsonCI(t.Wins, t.Games)
}

// entrantLabel spells an entrant the way a gauntlet row names it.
func entrantLabel(e Entrant) string {
	return fmt.Sprintf("%s / %s", e.Deck.Name, e.Player.Ref)
}

// entrantNames is PlayGame's engine config naming for two entrants: the
// deck names, "p<index>" for an empty name.
func entrantNames(a, b Entrant) [2]string {
	names := [2]string{a.Deck.Name, b.Deck.Name}
	for i, n := range names {
		if n == "" {
			names[i] = fmt.Sprintf("p%d", i)
		}
	}
	return names
}

// wilsonCI returns the win proportion over n games and its Wilson score
// 95% interval; n == 0 returns zeros. It is cmd/draftsim's gauntlet.go
// wilsonCI, copied verbatim so the lab reports the same interval as the
// reference driver.
func wilsonCI(wins, n int) (p, lo, hi float64) {
	if n == 0 {
		return 0, 0, 0
	}
	p = float64(wins) / float64(n)
	const z = 1.959963984540054 // the two-sided 95% normal quantile
	den := 1 + z*z/float64(n)
	center := (p + z*z/(2*float64(n))) / den
	rad := z / den * math.Sqrt(p*(1-p)/float64(n)+z*z/(4*float64(n)*float64(n)))
	return p, math.Max(0, center-rad), math.Min(1, center+rad)
}
