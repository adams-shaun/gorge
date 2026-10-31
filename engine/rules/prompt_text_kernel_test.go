package rules

// Decision text carrying no engine syntax over whole games, driven through the
// resolution kernel (the only ask path).

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// promptTextChunks is how many independent tests TestDecisionTextHasNoEngineSyntax
// (it plays the repo-deck sample to completion and checks every decision posed
// along the way: its Prompt and every option Label must be free of raw Forge
// cost/filter syntax) is sharded into. Run whole it was 19.4 s wall / 34 CPU-s
// alone at 4 vCPU and 43.7 s under gate contention (2026-10-12). Chunk i plays
// games[i*n/K : (i+1)*n/K] of promptTextGames, so the chunks partition the
// sample exactly -- a deck added to the repo grows a chunk and is never dropped
// (TestDecisionTextHasNoEngineSyntaxChunksPartitionTheGames). One game (the
// 4-seat legacy tron/ur-delver/uw-control/uw-tempo pod, ~13 s alone) is the
// floor of any chunking: a game cannot be split.
const promptTextChunks = 4

// promptTextChunk returns chunk i's games.
func promptTextChunk(games []promptTextGame, i int) []promptTextGame {
	n := len(games)
	return games[i*n/promptTextChunks : (i+1)*n/promptTextChunks]
}

func TestDecisionTextHasNoEngineSyntaxChunk00(t *testing.T) { runDecisionTextChunk(t, 0) }
func TestDecisionTextHasNoEngineSyntaxChunk01(t *testing.T) { runDecisionTextChunk(t, 1) }
func TestDecisionTextHasNoEngineSyntaxChunk02(t *testing.T) { runDecisionTextChunk(t, 2) }
func TestDecisionTextHasNoEngineSyntaxChunk03(t *testing.T) { runDecisionTextChunk(t, 3) }

// TestDecisionTextHasNoEngineSyntaxChunksPartitionTheGames holds the sharding
// to full coverage: one TestDecisionTextHasNoEngineSyntaxChunkN per chunk, and
// the chunks, concatenated, are exactly promptTextGames' games in order.
func TestDecisionTextHasNoEngineSyntaxChunksPartitionTheGames(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("long")
	}
	games := promptTextGames(t, testutil.CorpusRegistry(t))
	var joined []string
	for i := 0; i < promptTextChunks; i++ {
		c := promptTextChunk(games, i)
		if len(c) == 0 {
			t.Errorf("chunk %d of %d games is empty", i, len(games))
		}
		for _, g := range c {
			joined = append(joined, g.label)
		}
	}
	if len(joined) != len(games) {
		t.Fatalf("chunks cover %d of %d games", len(joined), len(games))
	}
	for i, g := range games {
		if joined[i] != g.label {
			t.Fatalf("chunk concatenation diverges at %d: %q vs %q", i, joined[i], g.label)
		}
	}
	if tests := len([]func(*testing.T){TestDecisionTextHasNoEngineSyntaxChunk00, TestDecisionTextHasNoEngineSyntaxChunk01,
		TestDecisionTextHasNoEngineSyntaxChunk02, TestDecisionTextHasNoEngineSyntaxChunk03}); tests != promptTextChunks {
		t.Fatalf("%d chunk tests for %d chunks", tests, promptTextChunks)
	}
}

// runDecisionTextChunk is one shard of TestDecisionTextHasNoEngineSyntax: it
// plays chunk i's games and checks every decision they pose. Repeated
// offending text is reported once per chunk.
func runDecisionTextChunk(t *testing.T, chunk int) {
	t.Parallel()
	if testing.Short() {
		t.Skip("long")
	}
	reg := testutil.CorpusRegistry(t)
	type hit struct{ where, pattern string }
	var mu sync.Mutex
	violations := map[string]hit{} // offending text -> first place seen
	decisions, labels := 0, 0
	kinds := map[decision.Kind]int{}
	// Each game is independent, so the games run as parallel subtests: this
	// test is the rules package's longest single body and the module gate's
	// critical-path tail, and its one goroutine used a single CPU while the
	// gate cgroup's other three sat idle. Per-decision failures stay
	// attributed to their own game; the unique-text report waits for all of
	// them.
	for _, g := range promptTextChunk(promptTextGames(t, reg), chunk) {
		t.Run(g.label, func(t *testing.T) {
			t.Parallel()
			e := New(g.cfg)
			// where is built only for a violation: formatting it for every
			// decision and option label was most of this checker's own cost.
			check := func(d *decision.Decision, suffix, optKind, text string) {
				p := engineSyntaxIn(text)
				if p == "" {
					return
				}
				where := fmt.Sprintf("%s: turn %d, %s decision", g.label, e.G.Turn, d.Kind) + suffix + optKind
				mu.Lock()
				// Keep the lexicographically first place for a repeated text so
				// the report is deterministic however the games interleave.
				if prev, seen := violations[text]; !seen || where < prev.where {
					violations[text] = hit{where, p}
				}
				mu.Unlock()
			}
			b := newTestBot(g.bot)
			e.Advance()
			n := 0
			gameDecisions, gameLabels := 0, 0
			gameKinds := map[decision.Kind]int{}
			for !e.G.Over && e.Pending() != nil && n < 60000 {
				d := e.Pending()
				gameDecisions++
				gameKinds[d.Kind]++
				check(d, " prompt", "", d.Prompt)
				for _, o := range d.Options {
					gameLabels++
					check(d, " option ", o.Kind, o.Label)
				}
				if err := e.Submit(b.answer(e, d)); err != nil {
					t.Fatalf("%s, intent %d: %v", g.label, n, err)
				}
				n++
			}
			mu.Lock()
			decisions, labels = decisions+gameDecisions, labels+gameLabels
			for k, c := range gameKinds {
				kinds[k] += c
			}
			mu.Unlock()
		})
	}
	t.Cleanup(func() {
		var ks []string
		for k, c := range kinds {
			ks = append(ks, fmt.Sprintf("%s=%d", k, c))
		}
		sort.Strings(ks)
		t.Logf("chunk %d: checked %d decisions (%s) and %d option labels", chunk, decisions, strings.Join(ks, " "), labels)
		var texts []string
		for s := range violations {
			texts = append(texts, s)
		}
		sort.Strings(texts)
		for _, s := range texts {
			t.Errorf("%s: %q contains engine syntax (%s)", violations[s].where, s, violations[s].pattern)
		}
	})
}
