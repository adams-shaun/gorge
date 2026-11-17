package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// facedownSweepDeck is a synthetic pool's face-down pair plus lands: Priest of
// Urabrask's enter trigger adds {R}{R}{R}, which pays the {3} face-down cast,
// and Warbreak Trumpeter's {R} turn-up. The deck has no Mox, so the mana can
// only come from the Priest.
var facedownSweepDeck = []cardset.Entry{
	{Name: "Priest of Urabrask", Count: 8},
	{Name: "Warbreak Trumpeter", Count: 8},
	{Name: "Mountain", Count: 12},
	{Name: "Swamp", Count: 12},
}

// TestFaceDownSweepPolicy: FaceDownPolicy steers the fixture deck through the
// face-down cast and the turn-face-up special action that the sweep's own base
// policy (AffordableTargetPolicy over RandomPolicy) never reaches. The
// preconditions assert the deck really holds the floater and the turn-up
// carrier the policy is looking for.
func TestFaceDownSweepPolicy(t *testing.T) {
	t.Parallel()
	reg, set, decks, tbl := facedownSweepEnv(t)
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

	down, up := sweepActions(t, reg, set, decks, tbl, func(seed uint64) Policy {
		return AffordableTargetPolicy(RandomPolicy(seed))
	})
	t.Logf("the sweep's base policy: face-down casts %d, turn-ups %d over 20 seeds", down, up)
	// The base policy is a control, not a zero: on a deck this dense in
	// floaters it floats by luck too. The overlay's value is that it steers:
	// strictly more of both actions than the base it wraps.
	downO, upO := sweepActions(t, reg, set, decks, tbl, func(seed uint64) Policy {
		return FaceDownPolicy(AffordableTargetPolicy(RandomPolicy(seed)))
	})
	t.Logf("face-down casts %d, turn-ups %d over 20 seeds", downO, upO)
	if downO <= down {
		t.Errorf("no steering: the overlay took %d face-down casts, the base %d", downO, down)
	}
	if upO <= up {
		t.Errorf("no steering: the overlay took %d turn-ups, the base %d", upO, up)
	}
}

// sweepActions plays 20 seeds under pol and counts the face-down casts and
// turn-face-up actions the games took.
func sweepActions(t *testing.T, reg *cards.Registry, set cardset.Set, decks [2][]*cards.Card, tbl *kop.Table, pol func(uint64) Policy) (down, up int) {
	t.Helper()
	for seed := uint64(1); seed <= 20; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"sweep-a", "sweep-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, pol(seed), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
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
	}
	return down, up
}

// faceDownSweepFacts counts a pool's face-down sweep halves as the eligibility
// predicate reads them, and the same mana sources under the narrower
// enters-trigger-only walk the selection used before it widened to every
// trigger mode (the shape a widened witness proves necessary).
func faceDownSweepFacts(reg *cards.Registry, set cardset.Set) (carriers, anyMana, etbMana int) {
	supported := set.Supported()
	for i := 0; i < set.NumCards(); i++ {
		if !supported[i] {
			continue
		}
		if set.Def(i).MorphFamily != 0 {
			carriers++
		}
		c, ok := reg.Lookup(set.Names()[i])
		if !ok {
			continue
		}
		for _, f := range c.Faces {
			if !isTriggerManaFace(f) {
				continue
			}
			anyMana++
			for j := range f.Triggers {
				if f.Triggers[j].Mode == "ChangesZone" && etbReachesMana(f.Triggers[j].Effect, 0) {
					etbMana++
					break
				}
			}
			break
		}
	}
	return carriers, anyMana, etbMana
}

// etbReachesMana is the pre-widening walk: the SubAbility$ chain only, a
// ChangesZone (enters-the-battlefield) trigger, no Charm bodies.
func etbReachesMana(sa *cards.SA, depth int) bool {
	if sa == nil || depth > maxManaWalkDepth {
		return false
	}
	if sa.API == "Mana" {
		return true
	}
	return etbReachesMana(sa.Sub, depth)
}

// TestFaceDownSweepEligibility: the overlay is selected exactly where a
// floater can put mana in the pool, and its floater rule is the wide one the
// runtime policy reads. The witnesses are searched over the synthetic pools,
// never named: a pool's card list grows whenever kgen lowers another card, so
// a named witness pins would redden an unrelated landing. The wide witness is
// a pool that holds a carrier beside mana whose trigger is not an
// enters-the-battlefield one (the shape the narrow walk missed); the empty
// witness is a pool with carriers but no trigger mana at all.
func TestFaceDownSweepEligibility(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	var wide, empty, etbOnly string
	for _, name := range cardsettest.SynthNames(t) {
		set := cardsettest.Synth(t, name)
		carriers, anyMana, etbMana := faceDownSweepFacts(reg, set)
		eligible := FaceDownSweepEligible(reg, set)
		switch {
		case wide == "" && eligible && etbMana == 0:
			wide = name
			t.Logf("widened witness %s: %d carriers, %d trigger mana sources, %d of them enters triggers", name, carriers, anyMana, etbMana)
		case empty == "" && !eligible && carriers > 0 && anyMana == 0:
			empty = name
			t.Logf("empty witness %s: %d carriers, no trigger mana source", name, carriers)
		case etbOnly == "" && eligible && etbMana > 0:
			etbOnly = name
			t.Logf("enters-trigger witness %s: %d carriers, %d trigger mana sources", name, carriers, anyMana)
		}
		if wide != "" && empty != "" && etbOnly != "" {
			break
		}
	}
	if wide == "" {
		t.Fatal("no synthetic pool holds a carrier beside non-enters trigger mana: the widened floater walk selects nothing the narrow one did not")
	}
	if empty == "" {
		t.Fatal("no synthetic pool holds a carrier without trigger mana: the negative witness is missing")
	}
	if etbOnly == "" {
		t.Fatal("no synthetic pool holds a carrier beside an enters-trigger mana source: the original witness is missing")
	}
}

// facedownSweepEnv builds the tail-63 table and two copies of the sweep deck.
func facedownSweepEnv(t *testing.T) (*cards.Registry, cardset.Set, [2][]*cards.Card, *kop.Table) {
	t.Helper()
	reg := synthReg(t)
	set := cardsettest.Synth(t, "tail-63")
	var decks [2][]*cards.Card
	for s := range decks {
		for _, e := range facedownSweepDeck {
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
	tbl, _, _, err := set.Matchup(facedownSweepDeck, facedownSweepDeck)
	if err != nil {
		t.Fatal(err)
	}
	return reg, set, decks, tbl
}
