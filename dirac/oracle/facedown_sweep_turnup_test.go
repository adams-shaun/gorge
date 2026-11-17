package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// facedownTurnUpDeck is the bare pair a synthetic pool can deal: one Priest of
// Urabrask (a one-shot ETB floater whose {R}{R}{R} is exactly the {3}
// face-down cast) and one Warbreak Trumpeter (a T: Mode$ TurnFaceUp carrier).
// The deck is all lands otherwise, so the only spells the game can take are
// the ones this test steers.
var facedownTurnUpDeck = []cardset.Entry{
	{Name: "Priest of Urabrask", Count: 1},
	{Name: "Warbreak Trumpeter", Count: 1},
	{Name: "Mountain", Count: 19},
	{Name: "Swamp", Count: 19},
}

// TestFaceDownSweepTurnUp: on a synthetic pool's deal of one ETB floater and
// one TurnFaceUp carrier, FaceDownPolicy turns the carrier face up in a clean
// game. The single float is exactly the face-down price, so the turn-up has to
// be a potential play paid by an untapped land (rules' turn_face_up witness),
// not a second float: the overlay's preference alone can only spend the one
// trigger once.
func TestFaceDownSweepTurnUp(t *testing.T) {
	t.Parallel()
	reg, set, decks, tbl := facedownTurnUpEnv(t)
	if !FaceDownSweepEligible(reg, set) {
		t.Fatal("tail-63 has no Morph carrier beside a triggered mana source")
	}
	priest, ok := reg.Lookup("Priest of Urabrask")
	if !ok || priest.Faces[0] == nil || !isTriggerManaFace(priest.Faces[0]) {
		t.Fatal("Priest of Urabrask's enters trigger does not add mana")
	}
	trumpeter, ok := reg.Lookup("Warbreak Trumpeter")
	if !ok || trumpeter.Faces[0] == nil {
		t.Fatal("Warbreak Trumpeter is not in the corpus")
	}
	hasTurnUp := false
	for _, tr := range trumpeter.Faces[0].Triggers {
		if tr.Mode == "TurnFaceUp" {
			hasTurnUp = true
		}
	}
	if !hasTurnUp {
		t.Fatal("Warbreak Trumpeter has no TurnFaceUp trigger")
	}

	// Seed 10 casts the Priest, spends its float on the face-down carrier, and
	// then has only an untapped land left for the {R} turn-up (Warbreak's
	// smallest {X}{X}{R} announcement at X=0).
	down, up := sweepSeedActions(t, reg, set, decks, tbl, 10, Config{GorgeAutopaySurface: true})
	t.Logf("seed 10: face-down casts %d, turn-ups %d", down, up)
	if down == 0 {
		t.Fatal("the overlay never cast the carrier face down: no turn-up could follow")
	}
	if up == 0 {
		t.Fatalf("the overlay turned the carrier face up 0 times: the %d face-down cast(s) never got a land-funded turn-up", down)
	}
}

// sweepSeedActions plays one seed under FaceDownPolicy over the sweep's base
// policy and counts the face-down casts and turn-face-up actions.
func sweepSeedActions(t *testing.T, reg *cards.Registry, set cardset.Set, decks [2][]*cards.Card, tbl *kop.Table, seed uint64, cfg Config) (down, up int) {
	t.Helper()
	m, err := NewMatchup(reg, [2]string{"sweep-a", "sweep-b"}, decks, 1)
	if err != nil {
		t.Fatal(err)
	}
	k, err := SetSide(set, tbl, cfg)(m, seed)
	if err != nil {
		t.Fatal(err)
	}
	res := Lockstep(m, seed, k, FaceDownPolicy(AffordableTargetPolicy(RandomPolicy(seed))), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
	CloseSide(k)
	if d := res.Div; d != nil {
		t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
	}
	for _, a := range res.Actions {
		switch {
		case a.Kind() == ActCastFace && isFaceDownCast(a):
			down++
		case a.Kind() == ActSpecial && isTurnFaceUp(a):
			up++
		}
	}
	return down, up
}

// facedownTurnUpEnv builds the tail-63 table and two copies of the two-copy
// deal, asserting the deal really is the synthetic maximum (synthMaxCopies).
func facedownTurnUpEnv(t *testing.T) (*cards.Registry, cardset.Set, [2][]*cards.Card, *kop.Table) {
	t.Helper()
	for _, e := range facedownTurnUpDeck {
		if e.Name != "Mountain" && e.Name != "Swamp" && e.Count > synthMaxCopies {
			t.Fatalf("%s: %d copies, above the synthetic deal's %d", e.Name, e.Count, synthMaxCopies)
		}
	}
	reg := synthReg(t)
	set := cardsettest.Synth(t, "tail-63")
	var decks [2][]*cards.Card
	for s := range decks {
		for _, e := range facedownTurnUpDeck {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				decks[s] = append(decks[s], c)
			}
		}
		if len(decks[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	tbl, _, _, err := set.Matchup(facedownTurnUpDeck, facedownTurnUpDeck)
	if err != nil {
		t.Fatal(err)
	}
	return reg, set, decks, tbl
}
