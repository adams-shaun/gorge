package oracle

import (
	"errors"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// TestPlayInteractionRefusedPairReplays is kfuzz's -play-interaction-refused:
// Black Vise beside Orb of Dreams is refused as chooseplayerorder before play,
// and with the guards skipped the pair builds with the game's compact CardIDs
// and plays to the unmodelled stall the guard predicted instead of being
// skipped.
func TestPlayInteractionRefusedPairReplays(t *testing.T) {
	reg := synthReg(t)
	set := chosenOppPool(t)

	decks := [2][]string{
		deckOf(10, "Black Vise", deckOf(30, "Island")...),
		deckOf(10, "Orb of Dreams", deckOf(30, "Plains")...),
	}
	var entries [2][]cardset.Entry
	for s, d := range decks {
		for _, n := range d {
			if len(entries[s]) > 0 && entries[s][len(entries[s])-1].Name == n {
				entries[s][len(entries[s])-1].Count++
				continue
			}
			entries[s] = append(entries[s], cardset.Entry{Name: n, Count: 1})
		}
	}

	// The refused pair: the precondition the replay depends on.
	if _, _, err := matchupTable(set, entries, false); err == nil {
		t.Fatal("the normal table build did not refuse")
	} else {
		var refused *cardset.InteractionRefusal
		if !errors.As(err, &refused) || refused.Reason != "chooseplayerorder" {
			t.Fatalf("Black Vise beside Orb of Dreams: got %v, want the chooseplayerorder interaction refusal", err)
		}
	}

	// The replay: the flag's helper builds the same pair with the guard
	// skipped and keeps the guard's reason.
	tbl, refused, err := matchupTable(set, entries, true)
	if err != nil {
		t.Fatalf("replayed table: %v, want the refused pair to build", err)
	}
	if refused != "chooseplayerorder" {
		t.Fatalf("replayed table reason %q, want chooseplayerorder", refused)
	}
	if len(tbl.Cards) != 4 {
		t.Fatalf("replayed table holds %d cards, want the pair's 4 (two spells, two lands)", len(tbl.Cards))
	}
	for i := range tbl.Cards {
		if tbl.Cards[i].Name == "" {
			t.Fatalf("replayed table card %d has no name: not the game's compact table", i)
		}
	}

	r := &Repro{Format: reproFormat, Decks: [2]string{"vise", "orb"}, Cards: decks, Seed: 1, PassElision: true, Surface: "full"}
	m, err := r.Matchup(reg)
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.MaxDecisions, rc.MaxTurns = 3000, 30
	k, err := SetSide(set, tbl, rc.Config)(m, 1)
	if err != nil {
		t.Fatal(err)
	}
	res := Lockstep(m, 1, k, RandomPolicy(1), rc)
	CloseSide(k)

	want := KernelStallPrefix + refused
	if res.Div != nil {
		t.Fatalf("replayed pair diverged at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if res.Stall != want || !res.StallUnmodelled {
		t.Fatalf("replayed pair outcome %q (unmodelled %v), want the unmodelled %q", res.Stall, res.StallUnmodelled, want)
	}
}
