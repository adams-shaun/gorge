package draftlab

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/engine/deck"
)

// TestCV1PumpProtectionReplay plays the exact game that used to abort the
// engine mid-resolution (ticket CV-1, cardval game 80175, seed
// 16588193633715634951): deck A (BR) controls Akroma's Memorial, so its
// Drakuseth, Maw of Flames derives "Protection from red", and deck A's
// Embercleave-equipped red creature (obj 16) targets that Drakuseth with a
// Pump ability. protectedFrom ranged the target's Derived keyword slice --
// an alias of the shared chars scratch -- while sourceHasQuality inside the
// loop derived the source's colours, rewriting the same buffer mid-range;
// whether the nested build overwrote in place or reallocated depended on the
// buffer's growth history, which differs between an execution and its
// rewind-to-checkpoint re-execution, so the CR 608.2b target recheck
// disagreed with itself and the log's verify window aborted the game with
// "recorded stack_resolve obj=98, re-executed move_zone obj=98 fizzled". The
// fix copies the ranged list. The rules-correct outcome is the fizzle (a
// red source cannot target a Protection-from-red bearer); the fixed engine
// completes the game at winner 0 in 19 turns (head 6bd83d64a5ffbfb9 at the
// fix commit). The decks are the two cardval decks' card-name lists
// (testdata/cv1/cv1_a.json, cv1_b.json); no Forge script text is committed.
func TestCV1PumpProtectionReplay(t *testing.T) {
	l := testLab(t)
	player := testPlayer(t, l, "bot")
	var entrants [2]Entrant
	for i, name := range [2]string{"cv1_a.json", "cv1_b.json"} {
		raw, err := os.ReadFile(filepath.Join("testdata", "cv1", name))
		if err != nil {
			t.Fatalf("read fixture %s: %v", name, err)
		}
		f, err := deck.Parse(raw)
		if err != nil {
			t.Fatalf("deck.Parse %s: %v", name, err)
		}
		d, err := l.ListDeck(f)
		if err != nil {
			t.Fatalf("ListDeck %s: %v", name, err)
		}
		entrants[i] = Entrant{Deck: d, Player: player}
	}
	// The bug aborted the game (an engine panic surfaced as a PlayGame
	// error); the seed is deterministic, so the abort reproduced on every
	// run. The fixed game runs in about a second.
	r, err := l.PlayGame(entrants[0], entrants[1], 16588193633715634951)
	if err != nil {
		t.Fatalf("PlayGame aborted: %v", err)
	}
	if r.Head == "" {
		t.Error("no chain head (the game did not run)")
	}
}
