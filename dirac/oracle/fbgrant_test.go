package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// kfbGrantDecks: Iroh, Grand Lotus's two graveyard flashback grants, on your
// turn only: plain instants and sorceries (Opt) at their mana cost, Think Twice
// beside its printed flashback, and Lessons (Boomerang
// Basics, How to Start a Riot) at {1}, and Rockalanche beside its printed
// flashback. Burning-Tree Emissary speeds Iroh out.
var kfbGrantDecks = [2][]cardset.Entry{
	{
		{Name: "Iroh, Grand Lotus", Count: 8}, {Name: "Opt", Count: 4}, {Name: "Think Twice", Count: 3},
		{Name: "How to Start a Riot", Count: 3}, {Name: "Rockalanche", Count: 1}, {Name: "Boomerang Basics", Count: 3}, {Name: "Burning-Tree Emissary", Count: 6},
		{Name: "Island", Count: 4}, {Name: "Mountain", Count: 4}, {Name: "Forest", Count: 4},
	},
	{
		{Name: "Iroh, Grand Lotus", Count: 8}, {Name: "Opt", Count: 4}, {Name: "Think Twice", Count: 3},
		{Name: "How to Start a Riot", Count: 3}, {Name: "Rockalanche", Count: 1}, {Name: "Boomerang Basics", Count: 3}, {Name: "Burning-Tree Emissary", Count: 6},
		{Name: "Island", Count: 4}, {Name: "Mountain", Count: 4}, {Name: "Forest", Count: 4},
	},
}

// kfbGrantStats counts what the flashback grant lockstep saw on gorge's side.
type kfbGrantStats struct {
	lessonOffers, plainOffers  int // flashback offers of a Lesson / of a card without printed flashback that is no Lesson
	offTurn                    int // grant-only flashback offers to a player whose turn it is not
	casts                      int // flashback casts the policy took
	exiledLessons, exiledPlain int // Lessons / other instants and sorceries in exile at the game's end
}

// isLesson reports whether f is a Lesson.
func isLesson(f *cards.Face) bool {
	for _, s := range f.Types {
		if s == "Lesson" {
			return true
		}
	}
	return false
}

// kfbGrantPolicy takes a flashback cast whenever one is offered, else any cast,
// else answers at random, and tallies gorge's flashback offers.
func kfbGrantPolicy(seed uint64, st *kfbGrantStats) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G == nil {
			return rnd.Pick(c)
		}
		d := c.G.E.Pending()
		if d == nil || d.Kind != decision.KPriority {
			return rnd.Pick(c)
		}
		var objs []state.ObjID
		for _, o := range d.Options {
			if o.Kind == "cast" && o.Mode == "flashback" {
				objs = append(objs, o.Obj)
			}
		}
		for _, a := range d.PaymentActions {
			if a.Cast.Origin == decision.OriginFlashback {
				objs = append(objs, a.Cast.Object)
			}
		}
		for _, id := range objs {
			f := c.G.E.G.Obj(id).Face()
			if f == nil || f.HasKeyword("Flashback") {
				continue // a printed flashback (Think Twice) needs no grant
			}
			if isLesson(f) {
				st.lessonOffers++
			} else {
				st.plainOffers++
			}
			if c.G.E.G.Active != d.Player {
				st.offTurn++
			}
		}
		var cast Canon
		haveCast := false
		for _, a := range c.D.Options {
			if a.Kind() != ActCast {
				continue
			}
			if a.Mode()&15 == 2 {
				st.casts++
				return a
			}
			if !haveCast {
				cast, haveCast = a, true
			}
		}
		if haveCast && seed%3 != 0 {
			return cast
		}
		return rnd.Pick(c)
	})
}

// TestFbGrantLockstep: Iroh's grants offer a graveyard instant or sorcery's
// flashback on its controller's turn only, a Lesson's at {1} and any other at
// its mana cost (both engines charge the same, decision by decision), and the
// card is exiled after; Think Twice keeps its printed flashback beside the
// grant.
func TestFbGrantLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("fbgrant")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kfbGrantDecks {
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
	tbl, _, _, err := set.Matchup(kfbGrantDecks[0], kfbGrantDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	var st kfbGrantStats
	for seed := uint64(1); seed <= 64; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kfb-a", "kfb-b"}, decks, 1)
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
		res := LockstepLed(m, seed, g, k, kfbGrantPolicy(seed, &st), RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 60})
		CloseSide(k)
		if res.Stall != "" {
			t.Fatalf("seed %d: stall %s after %d decisions", seed, res.Stall, res.Decisions)
		}
		if d := res.Div; d != nil {
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v",
				seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
		for p := state.PlayerID(0); p < 2; p++ {
			for _, id := range g.E.G.Zone(state.ZExile, p) {
				f := g.E.G.Obj(id).Face()
				if f == nil || f.IsCreature() {
					continue
				}
				if isLesson(f) {
					st.exiledLessons++
				} else {
					st.exiledPlain++
				}
			}
		}
	}
	t.Logf("%+v", st)
	if st.offTurn != 0 {
		t.Errorf("%d grant-only flashback offers off the controller's turn, want none", st.offTurn)
	}
	if st.lessonOffers < 5 || st.plainOffers < 5 || st.casts < 10 {
		t.Errorf("thin coverage: %+v", st)
	}
	if st.exiledLessons == 0 || st.exiledPlain == 0 {
		t.Errorf("no flashed-back card exiled: %+v", st)
	}
}
