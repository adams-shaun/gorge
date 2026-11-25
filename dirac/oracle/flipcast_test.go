package oracle

import (
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/dirac/canon"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The back-face cast lockstep (dirac/face_flipcast.go): More Than Meets the
// Eye's converted cast from the hand (CR 702.162a; Goldbug, Starscream,
// Ultra Magnus, whose backs carry Living metal, CR 702.161a) and disturb's
// transformed cast from the graveyard (CR 702.146a; Beloved Beggar, Kindly
// Ancestor, whose backs exile themselves instead of reaching a graveyard).
// The policy takes every back-face cast it is offered and otherwise plays,
// attacks and casts, so carriers die into the graveyard and a Cancel can
// send a back-face spell off the stack.

var flipCastDecks = [2][]cardset.Entry{
	{
		{Name: "Goldbug, Humanity's Ally", Count: 4}, {Name: "Starscream, Power Hungry", Count: 4},
		{Name: "Beloved Beggar", Count: 3}, {Name: "Kindly Ancestor", Count: 3}, {Name: "Cancel", Count: 3},
		{Name: "Wind Drake", Count: 2}, {Name: "Savannah Lions", Count: 2},
		{Name: "Plains", Count: 7}, {Name: "Island", Count: 6}, {Name: "Swamp", Count: 6},
	},
	{
		{Name: "Ultra Magnus, Tactician", Count: 4}, {Name: "Kindly Ancestor", Count: 3}, {Name: "Beloved Beggar", Count: 2},
		{Name: "Shock", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 3}, {Name: "Savannah Lions", Count: 2},
		{Name: "Plains", Count: 6}, {Name: "Mountain", Count: 6}, {Name: "Forest", Count: 6},
	},
}

type flipCastStats struct {
	games, divs, stalls int
	converted, disturb  int
}

// flipCastPolicy takes an offered back-face cast (seven times in eight),
// attacks with everything it may, and otherwise plays a land or casts.
func flipCastPolicy(seed uint64, st *flipCastStats) Policy {
	rnd := RandomPolicy(seed)
	r := rand.New(rand.NewPCG(seed, 0xf11c))
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.D.Kind == DPriority {
			for _, o := range c.D.Options {
				if o.Kind() != ActCastFace || r.IntN(8) == 0 {
					continue
				}
				switch o.Mode() {
				case canon.FaceCastConverted:
					st.converted++
					return o
				case canon.FaceCastDisturb:
					st.disturb++
					return o
				}
			}
		}
		if c.D.Kind == DAttackers {
			for _, o := range c.D.Options {
				if o.Kind() == ActAttack {
					return o
				}
			}
		}
		for _, o := range c.D.Options {
			if o.Kind() == ActPlayLand || o.Kind() == ActCast && r.IntN(2) == 0 {
				return o
			}
		}
		return rnd.Pick(c)
	})
}

// runFlipCast plays seeds of the constructed matchup, both autopay surfaces.
func runFlipCast(t *testing.T, seeds int) *flipCastStats {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("flipcast")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range flipCastDecks {
		for _, e := range d {
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
	tbl, _, _, err := set.Matchup(flipCastDecks[0], flipCastDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	st := &flipCastStats{}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"flipcast-a", "flipcast-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		g, err := NewGorgeSide(m, seed, cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		res := LockstepLed(m, seed, g, k, flipCastPolicy(seed, st), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
		CloseSide(k)
		st.games++
		if len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
			st.stalls++
			t.Errorf("seed %d: kernel stall %q", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			st.divs++
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	return st
}

// TestFlipCastLockstep plays the converted and disturb casts through both
// engines: 200 games, both autopay surfaces, no divergence, no stall, and
// both casts taken.
func TestFlipCastLockstep(t *testing.T) {
	st := runFlipCast(t, 200)
	t.Logf("%d games, %d divergences, %d stalls, converted casts %d, disturb casts %d",
		st.games, st.divs, st.stalls, st.converted, st.disturb)
	if st.converted < 50 {
		t.Errorf("only %d converted casts", st.converted)
	}
	if st.disturb < 20 {
		t.Errorf("only %d disturb casts", st.disturb)
	}
}
