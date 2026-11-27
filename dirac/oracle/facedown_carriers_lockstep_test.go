package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// The three face-down carriers this repro is about: Ainok Survivalist
// (megamorph), Alley Assailant (disguise) and Acid-Spewer Dragon (megamorph),
// each with a Mode$ TurnFaceUp trigger. They stand in for the 83 lowered
// face-down cards that never reach clean evidence.
var facedownCarrierNames = []string{
	"Ainok Survivalist", "Alley Assailant", "Acid-Spewer Dragon",
}

// A face-down cast or turn-face-up is priced from the floating pool alone
// (face_facedown.go: gorge's planner does not price the mode), and a normal
// cast's offer is priced from untapped sources, so the pool only ever holds
// what one ETB floater's trigger added — the next cast's payment spends it.
// RandomPolicy alone never floats mana at all, so facedownCarrierPolicy
// steers into the face-down path while the seed's RandomPolicy plays
// everything else. The Moxen are the free mana the payment planner taps for
// the floaters; the floaters' triggers ({B}{B}{B}, {R}{R}{R}) are the pool.
// The carriers' turn-face-up costs ({1}{G}, {4}{B}{B}, {5}{B}{B}) are above
// what a single trigger floats, so the harness exercises the face-down entry,
// its projection and the face-down cast pricing, not the turn-up: the
// turn-face-up special action stays covered by TestFacedownLockstep.
var facedownCarrierDeck = []cardset.Entry{
	{Name: "Mox Jet", Count: 4},
	{Name: "Mox Emerald", Count: 2},
	{Name: "Mox Sapphire", Count: 2},
	{Name: "Mox Ruby", Count: 4},
	{Name: "Priest of Gix", Count: 6},
	{Name: "Priest of Urabrask", Count: 4},
	{Name: "Coal Stoker", Count: 6},
	{Name: "Ainok Survivalist", Count: 2},
	{Name: "Alley Assailant", Count: 3},
	{Name: "Acid-Spewer Dragon", Count: 3},
	{Name: "Swamp", Count: 4},
}

// isFloaterName: the ETB mana floaters whose triggers stack the pool in one
// step — the two Priests ({B}{B}{B}, {R}{R}{R}) and Coal Stoker ({R}{R}{R},
// cast from hand).
func isFloaterName(n string) bool {
	switch n {
	case "Priest of Gix", "Priest of Urabrask", "Coal Stoker":
		return true
	}
	return false
}

// facedownCarrierPolicy: the setup's Moxen, then every Priest (their floats
// stack in one step), then a turn-face-up of a carrier not yet turned up, then
// a face-down cast of one not yet cast face down, then whichever of the two is
// still offered; everything else goes to the seed's RandomPolicy.
func facedownCarrierPolicy(seed uint64, down, up map[string]int, setup *[2]int) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(c *PickCtx) Canon {
		var (
			mox, priest, land, downA, upA         Canon
			downName, upName                      string
			downCount, upCount                    int
			haveMox, havePriest, haveDown, haveUp bool
			haveLand                              bool
		)
		for _, a := range c.D.Options {
			n := optionFaceName(c, a)
			switch {
			case a.Kind() == ActPlayLand:
				land, haveLand = a, true
			case a.Kind() == ActCast && isMoxName(n):
				mox, haveMox = a, true
			case a.Kind() == ActCast && isFloaterName(n):
				priest, havePriest = a, true
			case a.Kind() == ActCastFace:
				if !haveDown || down[n] < downCount {
					downA, downName, downCount, haveDown = a, n, down[n], true
				}
			case a.Kind() == ActSpecial && a.Mode()&0xf == kfacedownTurnUp:
				if !haveUp || up[n] < upCount {
					upA, upName, upCount, haveUp = a, n, up[n], true
				}
			}
		}
		switch {
		case haveLand:
			return land
		case havePriest:
			return priest
		case haveMox && setup[0] < 12:
			setup[0]++
			return mox
		case haveUp && up[upName] == 0:
			up[upName]++
			return upA
		case haveDown && down[downName] == 0:
			down[downName]++
			return downA
		case haveUp:
			up[upName]++
			return upA
		case haveDown:
			down[downName]++
			return downA
		}
		var buf []Canon
		for _, a := range c.D.Options {
			switch a.Kind() {
			case ActCast, ActCastFace, ActSpecial, ActActivate:
				buf = append(buf, a)
			}
		}
		if len(buf) == 0 {
			return rnd.Pick(c)
		}
		d := *c.D
		d.Options = buf
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

// TestFaceDownCarriersLockstep: the three named face-down carriers play
// lockstep in gorge and the kernel when the games actually cast them face
// down and turn them up. This is the repro for the 83 lowered face-down cards
// that never reach clean evidence: a pass with the counters met means the
// kernel path is clean and the cause is pool membership (the ticket's Change
// (b)); a divergence shows the first class on land.
func TestFaceDownCarriersLockstep(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	var decks [2][]*cards.Card
	for s := range decks {
		for _, e := range facedownCarrierDeck {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				decks[s] = append(decks[s], c)
			}
		}
	}
	var deckNames [2][]string
	for s := range deckNames {
		for _, e := range facedownCarrierDeck {
			for i := 0; i < e.Count; i++ {
				deckNames[s] = append(deckNames[s], e.Name)
			}
		}
	}
	set := printedDelveSet(t, deckNames)
	tbl, _, _, err := set.Matchup(facedownCarrierDeck, facedownCarrierDeck)
	if err != nil {
		t.Fatal(err)
	}
	down := map[string]int{}
	up := map[string]int{}
	turnedUp := map[string]int{}
	for seed := uint64(1); seed <= 50; seed++ {
		var setup [2]int
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"facedown-a", "facedown-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		seen := 0
		check := func(ref *GorgeSide, _ Side) *Divergence {
			evs := ref.E.L.Events
			for ; seen < len(evs); seen++ {
				ev := evs[seen]
				if ev.Kind != events.TurnFaceUp {
					continue
				}
				o := ref.E.G.Obj(ev.Obj)
				if o != nil && o.Face() != nil {
					turnedUp[o.Face().Name]++
				}
			}
			return nil
		}
		res := Lockstep(m, seed, k, facedownCarrierPolicy(seed, down, up, &setup), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40, Check: check})
		CloseSide(k)
		if d := res.Div; d != nil {
			lo := d.At - 12
			if lo < 0 {
				lo = 0
			}
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	t.Logf("down=%v up=%v turnedUp=%v", down, up, turnedUp)
	for _, n := range facedownCarrierNames {
		if down[n] == 0 {
			t.Errorf("%s was never cast face down (down=%v)", n, down)
		}
	}
}

// TestFaceDownCarriersMutation: with the kernel's face-down offers switched
// off (dirac.DebugIgnoreFaceDown) the same games diverge from gorge: the
// lockstep sees the face-down path this repro exercises (modelled on
// TestFacedownMutation).
func TestFaceDownCarriersMutation(t *testing.T) {
	reg := synthReg(t)
	var decks [2][]*cards.Card
	for s := range decks {
		for _, e := range facedownCarrierDeck {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				decks[s] = append(decks[s], c)
			}
		}
	}
	var deckNames [2][]string
	for s := range deckNames {
		for _, e := range facedownCarrierDeck {
			for i := 0; i < e.Count; i++ {
				deckNames[s] = append(deckNames[s], e.Name)
			}
		}
	}
	set := printedDelveSet(t, deckNames)
	tbl, _, _, err := set.Matchup(facedownCarrierDeck, facedownCarrierDeck)
	if err != nil {
		t.Fatal(err)
	}
	dirac.DebugIgnoreFaceDown = true
	defer func() { dirac.DebugIgnoreFaceDown = false }()
	for seed := uint64(1); seed <= 8; seed++ {
		var setup [2]int
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"facedown-m", "facedown-n"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, facedownCarrierPolicy(seed, map[string]int{}, map[string]int{}, &setup), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		CloseSide(k)
		if res.Div != nil {
			t.Logf("seed %d: mutation diverges at decision %d (%s %s)", seed, res.Div.At, res.Div.What, res.Div.Field)
			return
		}
	}
	t.Fatal("ignoring face-down casts/turn-ups diverged in no game: the lockstep does not see the face-down path")
}
