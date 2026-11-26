package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestStartBackFaceWalkerLoyaltyMatchesOracleSetup pins the one convention the
// two setup paths must share for a back-face planeswalker whose entering face
// prints Forge's "Loyalty: 0" carry-over marker: the placement grants the front
// face's printed starting loyalty (XMage's addCard), then the start's counters
// are added on top. The live scripted start and the oracle runner must agree.
func TestStartBackFaceWalkerLoyaltyMatchesOracleSetup(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	arlinn, ok := reg.Lookup("Arlinn Kord")
	if !ok {
		t.Fatal("Arlinn Kord is not in the corpus")
	}
	if len(arlinn.Faces) < 2 || arlinn.Faces[0].Loyalty != "3" || arlinn.Faces[1].Loyalty != "0" {
		t.Fatalf("precondition: Arlinn Kord is not the 3/0 back-face walker: %+v", arlinn.Faces)
	}

	// The live start: the deck's second card is the corpus Arlinn, seated on
	// its back face with one setup LOYALTY counter.
	decks := [][]*cards.Card{startDeck(t, "A", nil), startDeck(t, "B", nil)}
	decks[0][1] = arlinn
	start := &StartSpec{Seats: []StartSeat{{
		Battlefield: []int{1},
		BackFace:    []int{1},
		Counters:    []StartCounter{{Card: 1, Kind: "LOYALTY", N: 1}},
	}}}
	e := New(Config{Seed: 5, Names: []string{"a", "b"}, Decks: decks, Start: start})

	var startLoyalty int32
	found := 0
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		o := e.G.Obj(id)
		if o != nil && o.Card == arlinn {
			found++
			startLoyalty = o.Counter("LOYALTY")
		}
	}
	if found != 1 {
		t.Fatalf("precondition: the live start placed Arlinn Kord %d times on the battlefield, want 1", found)
	}

	// The oracle runner: the same position through a fixture scenario.
	res, _ := runFixtureScenario(t, `{"name":"arlinn-back-loyalty","setup":{"p0":{"battlefield":["Arlinn Kord"],"back_face":["Arlinn Kord"],"counters":{"Arlinn Kord":{"LOYALTY":1}}},"p1":{"battlefield":["Grizzly Bears"]}},"steps":[]}`)
	snap := res.Snapshots[0]
	var oracleLoyalty int32
	found = 0
	for _, p := range snap.Permanents {
		if p.Name == "Arlinn, Embraced by the Moon" {
			found++
			oracleLoyalty = p.Counters["LOYALTY"]
		}
	}
	if found != 1 {
		t.Fatalf("precondition: the oracle setup has %d back-face Arlinn permanents, want 1: %+v", found, snap.Permanents)
	}

	if startLoyalty != oracleLoyalty {
		t.Fatalf("back-face Arlinn loyalty: live start %d, oracle runner %d", startLoyalty, oracleLoyalty)
	}
	if startLoyalty != 4 {
		t.Fatalf("back-face Arlinn loyalty = %d, want 4 (front printed 3 + setup counter 1)", startLoyalty)
	}
}
