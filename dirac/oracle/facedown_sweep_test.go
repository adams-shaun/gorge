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
// face-down cast and the turn-face-up special action that RandomPolicy alone
// never reaches. The preconditions assert the deck really holds the floater
// and the turn-up carrier the policy is looking for.
func TestFaceDownSweepPolicy(t *testing.T) {
	t.Parallel()
	reg, set, decks, tbl := facedownSweepEnv(t)
	if !FaceDownSweepEligible(set) {
		t.Fatal("tail-63 has no Morph carrier beside an ETB mana source")
	}
	priest, ok := reg.Lookup("Priest of Urabrask")
	if !ok || priest.Faces[0] == nil || !isETBManaFace(priest.Faces[0]) {
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

	down, up := sweepActions(t, reg, set, decks, tbl, func(seed uint64) Policy { return noFacePolicy(seed) })
	if down != 0 || up != 0 {
		t.Fatalf("the control policy took a face-down cast (%d) or turn-up (%d)", down, up)
	}
	down, up = sweepActions(t, reg, set, decks, tbl, func(seed uint64) Policy {
		return FaceDownPolicy(noFacePolicy(seed))
	})
	t.Logf("face-down casts %d, turn-ups %d over 20 seeds", down, up)
	if down == 0 {
		t.Error("no face-down cast was taken")
	}
	if up == 0 {
		t.Error("no turn-face-up was taken")
	}
}

// noFacePolicy is RandomPolicy with the face-down cast and the turn-face-up
// removed from every decision: the control the overlay is measured against.
func noFacePolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(c *PickCtx) Canon {
		buf := make([]Canon, 0, len(c.D.Options))
		for _, a := range c.D.Options {
			if a.Kind() == ActCastFace && isFaceDownCast(a) || a.Kind() == ActSpecial && isTurnFaceUp(a) {
				continue
			}
			buf = append(buf, a)
		}
		if len(buf) == 0 {
			return rnd.Pick(c)
		}
		d := *c.D
		d.Options = buf
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
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

// TestFaceDownSweepEligibility: the overlay is selected only where a floater
// can put mana in the pool. tail-63 holds Priest of Urabrask and the
// Morph-family Warbreak Trumpeter; std-mkm holds the named Disguise carriers
// but no ETB mana source, so the sweep cannot reach them there.
func TestFaceDownSweepEligibility(t *testing.T) {
	t.Parallel()
	if set := cardsettest.Synth(t, "tail-63"); !FaceDownSweepEligible(set) {
		t.Error("tail-63 holds Priest of Urabrask and a Morph carrier, but is not eligible")
	}
	if set := cardsettest.Synth(t, "std-mkm"); FaceDownSweepEligible(set) {
		t.Error("std-mkm has no ETB mana source, so it must not select the overlay")
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
