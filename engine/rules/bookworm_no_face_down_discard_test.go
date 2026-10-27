package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestObliviousBookwormNoFaceDownNoTurnUpDiscardsAfterDraw pins the linked
// discard after the optional draw: with neither a face-down permanent nor a
// turn-up this turn, the "unless" condition is zero, so the discard rider is
// selected and discards the one card the draw remembered. The draw records
// the card on the HOST object so Count$RememberedSize (the discard's
// NumCards$ X) sees it.
func TestObliviousBookwormNoFaceDownNoTurnUpDiscardsAfterDraw(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	const scenario = `{"name":"bookworm-no-face-down-no-turn-up","cr":["608.2c"],
 "why":"the linked discard runs after the optional draw; with neither a face-down entry nor a turn-up this turn the condition is zero, so one card is discarded",
 "setup":{"p0":{"battlefield":["Oblivious Bookworm"],"hand":["Grizzly Bears"],"library":["Island","Mountain","Forest","Plains","Swamp"]},"p1":{"library":["Island","Mountain","Forest","Plains","Swamp"]}},
 "steps":[{"op":"pass_to","step":"end","active":"p0"},{"op":"resolve","answers":[{"kind":"any","pick":["yes"]}]}],
 "expect":[{"hand_size":{"p0":1}},{"graveyard_size":{"p0":1}}]}`

	res, err := RunOracleScenarioJSON(reg, []byte(scenario))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Fails) != 0 {
		t.Fatalf("scenario failed:\n%s", strings.Join(res.Fails, "\n"))
	}

	// Precondition: the setup is the state the card's rule reads -- seat 0
	// controls the Bookworm, its hand holds the one card the discard will
	// take, its library is stocked, and no permanent is face down (so no
	// face-down entry and no turn-up this turn).
	setup := res.Snapshots[0]
	if setup.Checkpoint != "setup" {
		t.Fatalf("precondition: first checkpoint = %q, want setup", setup.Checkpoint)
	}
	if got := len(setup.Players[0].Hand); got != 1 {
		t.Fatalf("precondition: p0 hand size = %d, want 1", got)
	}
	// Setup places the five named cards over the scenario's filler deck, so
	// the stocked library is larger than the named five.
	if got := setup.Players[0].LibraryCount; got < 5 {
		t.Fatalf("precondition: p0 library size = %d, want at least the 5 named cards", got)
	}
	worm := false
	for _, p := range setup.Permanents {
		if p.FaceDown {
			t.Fatalf("precondition: %q starts face down", p.Name)
		}
		if p.Name == "Oblivious Bookworm" && p.Controller == 0 {
			worm = true
		}
	}
	if !worm {
		t.Fatal("precondition: seat 0 does not control Oblivious Bookworm")
	}

	// Precondition: the optional draw asked and was answered yes, so the
	// linked discard ran (the handler under test executed).
	drew := false
	for _, d := range res.Decisions {
		if d.Resume == "draw_optional" && len(d.Picks) == 1 && d.Picks[0] == "Yes — draw" {
			drew = true
		}
	}
	if !drew {
		t.Fatal("precondition: the optional draw was not answered yes")
	}

	// The two sizes differ from the pre-fix failure (hand 2 / graveyard 0):
	// the draw took one and the discard took the remembered card.
	last := res.Snapshots[len(res.Snapshots)-1]
	if got := len(last.Players[0].Hand); got != 1 {
		t.Fatalf("p0 hand size %d, want 1", got)
	}
	if got := len(last.Players[0].Graveyard); got != 1 {
		t.Fatalf("p0 graveyard size %d, want 1", got)
	}
}

// TestObliviousBookwormEmptyHandDiscardsTheDrawnCard pins the scenario shape
// the compliance harness generates: no hand or library in the setup (the
// filler deck supplies the top card), so the drawn card is the only card in
// hand and the discard poses no choice, and the draw is answered with a
// "choose" pick. The drawn card is discarded: hand ends empty.
func TestObliviousBookwormEmptyHandDiscardsTheDrawnCard(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	const scenario = `{"name":"gen1-trigger#0.0","cr":["603.2"],"why":"generated level-B scenario","setup":{"p0":{"battlefield":["Oblivious Bookworm"]},"p1":{"battlefield":["Grizzly Bears"]}},"steps":[{"op":"pass_to","seat":0,"step":"end"},{"op":"resolve","seat":0,"answers":[{"kind":"choose","pick":["Yes — draw"]}]}]}`

	res, err := RunOracleScenarioJSON(reg, []byte(scenario))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Fails) != 0 {
		t.Fatalf("scenario failed:\n%s", strings.Join(res.Fails, "\n"))
	}

	// Precondition: empty hand and graveyard, seat 0 controls the Bookworm,
	// and nothing is face down.
	if len(res.Snapshots) == 0 {
		t.Fatal("precondition: no snapshots")
	}
	setup := res.Snapshots[0]
	if setup.Checkpoint != "setup" {
		t.Fatalf("precondition: first checkpoint = %q, want setup", setup.Checkpoint)
	}
	if got := len(setup.Players[0].Hand); got != 0 {
		t.Fatalf("precondition: p0 hand size = %d, want 0", got)
	}
	if got := len(setup.Players[0].Graveyard); got != 0 {
		t.Fatalf("precondition: p0 graveyard size = %d, want 0", got)
	}
	worm := false
	for _, p := range setup.Permanents {
		if p.FaceDown {
			t.Fatalf("precondition: %q starts face down", p.Name)
		}
		if p.Name == "Oblivious Bookworm" && p.Controller == 0 {
			worm = true
		}
	}
	if !worm {
		t.Fatal("precondition: seat 0 does not control Oblivious Bookworm")
	}

	// Precondition: the optional draw was taken.
	drew := false
	for _, d := range res.Decisions {
		if d.Resume == "draw_optional" && len(d.Picks) == 1 && d.Picks[0] == "Yes — draw" {
			drew = true
		}
	}
	if !drew {
		t.Fatal("precondition: the optional draw was not answered yes")
	}

	// Pre-fix the drawn card stayed in hand (hand 1 / graveyard 0).
	last := res.Snapshots[len(res.Snapshots)-1]
	if got := len(last.Players[0].Hand); got != 0 {
		t.Fatalf("p0 hand %v, want empty", last.Players[0].Hand)
	}
	if got := len(last.Players[0].Graveyard); got != 1 {
		t.Fatalf("p0 graveyard %v, want exactly the drawn card", last.Players[0].Graveyard)
	}
}
