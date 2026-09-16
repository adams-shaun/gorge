package main

import (
	"fmt"
	"sort"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/internal/testutil"
)

// coveragePlan is a deterministic, static deck-covering plan. It measures
// registered cards.Primitives represented by the selected deck lists, not
// successful resolutions: the latter depend on seed and bot choices and are
// reported separately by -decision-stats. A primitive is counted only when
// effects.Supported says this build implements it, so this is a selection
// metric rather than a claim that every corpus primitive works.
type coveragePlan struct {
	Names     []string
	Covered   int
	Available int
	Baseline  int
}

// coverageDeckPlan greedily chooses repo decks that add the most previously
// unrepresented supported primitive. Ties use the deck name, and the returned
// names are sorted before they enter fullPairs, so the plan and its resulting
// matrix are independent of registry-map order. It stops once no remaining
// deck can add coverage: adding a deck that changes no metric only repeats
// games and is exactly what coverage mode is intended to avoid.
func coverageDeckPlan(reg *cards.Registry, names []string) (coveragePlan, error) {
	pool := append([]string(nil), names...)
	sort.Strings(pool)
	supported := effects.Supported()
	byDeck := make(map[string]map[string]bool, len(pool))
	available := make(map[string]bool)
	for _, name := range pool {
		deck, err := testutil.LoadRepoDeck(reg, name)
		if err != nil {
			return coveragePlan{}, err
		}
		set := make(map[string]bool)
		for _, card := range deck {
			for _, p := range card.Primitives() {
				if supported[p] {
					set[p] = true
					available[p] = true
				}
			}
		}
		byDeck[name] = set
	}

	covered := make(map[string]bool)
	remaining := make(map[string]bool, len(pool))
	for _, name := range pool {
		remaining[name] = true
	}
	var selected []string
	for {
		best, bestGain := "", 0
		// pool is sorted; iterating it rather than remaining makes the equal
		// gain tie deterministic and keeps map order out of the selection.
		for _, name := range pool {
			if !remaining[name] {
				continue
			}
			gain := 0
			for p := range byDeck[name] {
				if !covered[p] {
					gain++
				}
			}
			if gain > bestGain {
				best, bestGain = name, gain
			}
		}
		if bestGain == 0 {
			break
		}
		selected = append(selected, best)
		delete(remaining, best)
		for p := range byDeck[best] {
			covered[p] = true
		}
	}
	if len(selected) < 2 {
		return coveragePlan{}, fmt.Errorf("coverage selection found %d decks with registered primitives; need at least 2", len(selected))
	}
	sort.Strings(selected)
	return coveragePlan{Names: selected, Covered: len(covered), Available: len(available)}, nil
}

// deckPrimitiveCoverage is the same static registered-primitive metric used
// by coverageDeckPlan, for a named set whose deck choice is already known.
// It is kept separate so tests and reports can compare the historical first
// two-deck bench to the coverage plan without duplicating the selection loop.
func deckPrimitiveCoverage(reg *cards.Registry, names []string) (int, error) {
	supported := effects.Supported()
	set := make(map[string]bool)
	for _, name := range names {
		deck, err := testutil.LoadRepoDeck(reg, name)
		if err != nil {
			return 0, err
		}
		for _, card := range deck {
			for _, p := range card.Primitives() {
				if supported[p] {
					set[p] = true
				}
			}
		}
	}
	return len(set), nil
}
