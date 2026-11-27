package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The face-down family pool: the generic face-down return (Ashcloud Phoenix),
// the in-place turn-down and face-down cost static (Obscuring
// Aether), the graveyard TurnFaceUp trigger (Deathmist Raptor), the token
// entry counters (Printlifter Ooze) and the hidden-library face-down exile
// (Necropotence), with a Mox/Priest float for the {3} a face-down cast prices
// from the floating pool.
// demoFacedownDeck builds a 40-card deck with four copies of each named
// card, the Moxen and Priests that float the {3} a face-down cast prices from
// the pool, a vanilla creature, and basics.
func demoFacedownDeck(cards ...string) []cardset.Entry {
	d := []cardset.Entry{
		{Name: "Mox Jet", Count: 2}, {Name: "Mox Emerald", Count: 2}, {Name: "Mox Ruby", Count: 2},
		{Name: "Priest of Gix", Count: 2}, {Name: "Priest of Urabrask", Count: 2},
		{Name: "Grizzly Bears", Count: 4},
	}
	for _, c := range cards {
		d = append(d, cardset.Entry{Name: c, Count: 4})
	}
	used := 0
	for _, e := range d {
		used += e.Count
	}
	lands := 40 - used
	d = append(d,
		cardset.Entry{Name: "Forest", Count: lands / 3},
		cardset.Entry{Name: "Swamp", Count: lands / 3},
		cardset.Entry{Name: "Mountain", Count: lands - 2*(lands/3)},
	)
	return d
}

// demoFacedownCases are the five lowered cards whose lockstep passes.
var demoFacedownCases = []struct {
	name string
	deck []cardset.Entry
}{
	{"ashcloud", demoFacedownDeck("Ashcloud Phoenix")},
	{"necropotence", demoFacedownDeck("Necropotence")},
	{"printlifter", demoFacedownDeck("Printlifter Ooze")},
	{"obscuring", demoFacedownDeck("Obscuring Aether", "Ashcloud Phoenix")},
	{"deathmist", demoFacedownDeck("Deathmist Raptor")},
}

func fdIsMox(n string) bool {
	switch n {
	case "Mox Jet", "Mox Emerald", "Mox Ruby", "Mox Sapphire", "Mox Pearl":
		return true
	}
	return false
}

func fdIsFloater(n string) bool {
	switch n {
	case "Priest of Gix", "Priest of Urabrask", "Coal Stoker":
		return true
	}
	return false
}

// demoFacedownPolicy steers a game into the family's paths: lands, the free
// Moxen, the Priests that stack the pool, then a face-down cast, a
// turn-face-up and an activation (Necropotence's pay-life or Obscuring
// Aether's turn-down), else the seed's RandomPolicy.
func demoFacedownPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	acts := map[string]int{}
	return PolicyFunc(func(c *PickCtx) Canon {
		var (
			land, mox, priest, face, special, activate                         Canon
			actName                                                            string
			haveLand, haveMox, havePriest, haveFace, haveSpecial, haveActivate bool
		)
		for _, a := range c.D.Options {
			n := optionFaceName(c, a)
			switch {
			case a.Kind() == ActPlayLand:
				land, haveLand = a, true
			case a.Kind() == ActCast && fdIsMox(n):
				mox, haveMox = a, true
			case a.Kind() == ActCast && fdIsFloater(n):
				priest, havePriest = a, true
			case a.Kind() == ActCastFace:
				face, haveFace = a, true
			case a.Kind() == ActSpecial:
				special, haveSpecial = a, true
			case a.Kind() == ActActivate && (n == "Necropotence" || n == "Obscuring Aether") && acts[n] < 3:
				activate, actName, haveActivate = a, n, true
			}
		}
		switch {
		case haveLand:
			return land
		case haveMox:
			return mox
		case havePriest:
			return priest
		case haveActivate:
			acts[actName]++
			return activate
		case haveFace:
			return face
		case haveSpecial:
			return special
		}
		return rnd.Pick(c)
	})
}

// demoFacedownScan counts the family's new shapes in the gorge log: the
// generic face-down entry and hidden exile, the turn-up, the in-place
// turn-down (Obscuring Aether) and the entry counters a minted token gets
// (Printlifter Ooze).
func demoFacedownScan(gs *GorgeSide, stat map[string]int) {
	if gs == nil {
		return
	}
	for _, ev := range gs.E.L.Events {
		switch ev.Kind {
		case events.TurnFaceUp:
			stat["turnUp"]++
		case events.TurnFaceDown:
			stat["turnDown"]++
		case events.CounterChange:
			// A token's WithCountersType$/WithCountersAmount$ entry counters
			// (Printlifter Ooze) are the entry-counter notice on the mint.
			if ev.Counter == "P1P1" && ev.Amount > 0 && ev.Text == events.EntryCounterNotice {
				stat["tokenCounter"]++
			}
		case events.MoveZone:
			if ev.To == state.ZBattlefield && strings.HasPrefix(ev.Counter, "entered_face_down") {
				stat["faceDownEntry"]++
			}
			if ev.To == state.ZExile && strings.HasPrefix(ev.Counter, "exiled_with_face_down") {
				stat["faceDownExile"]++
			}
		}
	}
}

// demoFacedownRun plays seeds games over the fixture pool and returns the
// divergences and the gorge-counted shapes.
func demoFacedownRun(t *testing.T, deck []cardset.Entry, seeds uint64, report bool) (div int, stat map[string]int) {
	t.Helper()
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("facedownfamily")
	if err != nil {
		t.Fatal(err)
	}
	var cs [2][]*cards.Card
	for s := range cs {
		for _, e := range deck {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				cs[s] = append(cs[s], c)
			}
		}
		if len(cs[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(cs[s]))
		}
	}
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	stat = map[string]int{}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"facedownfamily-a", "facedownfamily-b"}, cs, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		var gs *GorgeSide
		inner := demoFacedownPolicy(seed)
		pol := PolicyFunc(func(c *PickCtx) Canon {
			if c.G != nil {
				gs = c.G
			}
			return inner.Pick(c)
		})
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		CloseSide(k)
		demoFacedownScan(gs, stat)
		if res.Div != nil {
			div++
			lo := res.Div.At - 12
			if lo < 0 {
				lo = 0
			}
			if report {
				t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:res.Div.At]))
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s (implicates %v)\nref %v\nker %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Div.Detail, res.Div.Implicates(m), res.Div.RefOptions, res.Div.KerOptions)
			}
		}
	}
	return div, stat
}

func TestDemoFacedownLockstep(t *testing.T) {
	t.Parallel()
	stat := map[string]int{}
	for _, tc := range demoFacedownCases {
		div, got := demoFacedownRun(t, tc.deck, 64, true)
		if div != 0 {
			t.Errorf("%s: %d games diverged", tc.name, div)
		}
		for k, v := range got {
			stat[k] += v
		}
	}
	t.Logf("family shapes: %v", stat)
	for _, want := range []string{"faceDownEntry", "turnUp", "faceDownExile", "turnDown", "tokenCounter"} {
		if stat[want] == 0 {
			t.Errorf("%s never fired; the pool did not reach the shape", want)
		}
	}
}

func TestFourSeatDemoFacedownLockstep(t *testing.T) {
	t.Parallel()
	var names []string
	for _, e := range demoFacedownDeck("Ashcloud Phoenix", "Printlifter Ooze", "Obscuring Aether") {
		for i := 0; i < e.Count; i++ {
			names = append(names, e.Name)
		}
	}
	decks := [4][]string{names, names, names, names}
	if div := pod4FixtureLockstep(t, "facedownfamily", decks, 64, true, nil); div != 0 {
		t.Errorf("%d four-seat games diverged", div)
	}
}

// TestDemoFacedownMutation: with the face-down entry read disabled some game
// diverges (the new kop.RetFaceDown arm is what the pool exercises).
func TestDemoFacedownMutation(t *testing.T) {
	old := dirac.DebugIgnoreFaceDownEntry
	dirac.DebugIgnoreFaceDownEntry = true
	defer func() { dirac.DebugIgnoreFaceDownEntry = old }()
	if div, _ := demoFacedownRun(t, demoFacedownDeck("Ashcloud Phoenix"), 24, false); div == 0 {
		t.Fatal("no game diverged with the face-down entry read disabled")
	}
}
