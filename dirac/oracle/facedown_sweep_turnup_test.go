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

// facedownTurnUpSeeds is how many seeds the witness search plays.
const facedownTurnUpSeeds = 64

// TestFaceDownSweepTurnUp: on a synthetic pool's deal of one ETB floater and
// one TurnFaceUp carrier, FaceDownPolicy turns the carrier face up in a clean
// game. The single float is exactly the face-down price, so the turn-up has to
// be a potential play paid by an untapped land (rules' turn_face_up witness),
// not a second float: the overlay's preference alone can only spend the one
// trigger once.
//
// The test names neither a pool nor a seed: a pool's membership is a hash cut
// that any lowered card re-deals, and a seed's game moves with every policy or
// engine pick. It takes the first synthetic pool that can deal both cards and
// passes when some seed in 1..facedownTurnUpSeeds casts the carrier face down
// and then turns a carrier up.
func TestFaceDownSweepTurnUp(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
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

	set, tbl, name := facedownTurnUpPool(t)
	decks := facedownTurnUpDecks(t, reg)
	if !FaceDownSweepEligible(reg, set) {
		t.Fatalf("%s deals both cards but has no Morph carrier beside a triggered mana source", name)
	}
	totalDown, totalUp := 0, 0
	for seed := uint64(1); seed <= facedownTurnUpSeeds; seed++ {
		down, up := sweepSeedActions(t, reg, set, decks, tbl, seed, Config{GorgeAutopaySurface: true})
		totalDown += down
		totalUp += up
		if down > 0 && up > 0 {
			t.Logf("%s seed %d: face-down casts %d, turn-ups %d", name, seed, down, up)
			return
		}
	}
	if totalDown == 0 {
		t.Fatalf("%s: no seed in 1..%d cast the carrier face down, so no turn-up could follow", name, facedownTurnUpSeeds)
	}
	t.Fatalf("%s: %d face-down casts over %d seeds and no turn-up: the carrier never got a land-funded turn-up", name, totalDown, facedownTurnUpSeeds)
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

// facedownTurnUpPool is the first synthetic pool whose set deals both cards of
// facedownTurnUpDeck, with its matchup table and name.
func facedownTurnUpPool(t *testing.T) (cardset.Set, *kop.Table, string) {
	t.Helper()
	for _, e := range facedownTurnUpDeck {
		if e.Name != "Mountain" && e.Name != "Swamp" && e.Count > synthMaxCopies {
			t.Fatalf("%s: %d copies, above the synthetic deal's %d", e.Name, e.Count, synthMaxCopies)
		}
	}
	for _, name := range cardsettest.SynthNames(t) {
		set := cardsettest.Synth(t, name)
		if tbl, _, _, err := set.Matchup(facedownTurnUpDeck, facedownTurnUpDeck); err == nil {
			return set, tbl, name
		}
	}
	t.Fatal("no synthetic pool deals Priest of Urabrask beside Warbreak Trumpeter")
	return nil, nil, ""
}

// facedownTurnUpDecks is two copies of facedownTurnUpDeck from the corpus.
func facedownTurnUpDecks(t *testing.T, reg *cards.Registry) (decks [2][]*cards.Card) {
	t.Helper()
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
	return decks
}
