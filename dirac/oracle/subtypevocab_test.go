package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The non-creature-subtype vocabulary lockstep: Sokka's
// "whenever you cast an artifact or Lesson spell" trigger and Intruding
// Soulrager's "Sacrifice a Room" cost are filter words no card in these
// decks has. kgen now reads Room/Lesson from the corpus vocabulary, and the
// kernel's PredSubtype must match nothing exactly as gorge's type test does.
// The second matchup adds one Lesson card and one Room card so the filters
// match, exercising both answers of each word.

var ksubtypevocabDecks = [][2][]cardset.Entry{{
	// A/B: no Lesson and no Room card, and no artifact for Sokka's trigger.
	{
		{Name: "Sokka, Bold Boomeranger", Count: 4}, {Name: "Intruding Soulrager", Count: 4},
		{Name: "Coral Merfolk", Count: 4}, {Name: "Goblin Piker", Count: 4},
		{Name: "Shock", Count: 4}, {Name: "Unsummon", Count: 2},
		{Name: "Island", Count: 9}, {Name: "Mountain", Count: 9},
	},
	{
		{Name: "Sokka, Bold Boomeranger", Count: 4}, {Name: "Intruding Soulrager", Count: 4},
		{Name: "Coral Merfolk", Count: 4}, {Name: "Goblin Piker", Count: 4},
		{Name: "Shock", Count: 4}, {Name: "Unsummon", Count: 2},
		{Name: "Island", Count: 9}, {Name: "Mountain", Count: 9},
	},
}, {
	// A/B: the same carriers with Accumulate Wisdom (a Lesson) and
	// Bottomless Pool (a Room), so each filter has a match.
	{
		{Name: "Sokka, Bold Boomeranger", Count: 4}, {Name: "Intruding Soulrager", Count: 4},
		{Name: "Accumulate Wisdom", Count: 4}, {Name: "Bottomless Pool", Count: 4},
		{Name: "Coral Merfolk", Count: 4}, {Name: "Goblin Piker", Count: 2},
		{Name: "Shock", Count: 2}, {Name: "Island", Count: 8}, {Name: "Mountain", Count: 8},
	},
	{
		{Name: "Sokka, Bold Boomeranger", Count: 4}, {Name: "Intruding Soulrager", Count: 4},
		{Name: "Accumulate Wisdom", Count: 4}, {Name: "Bottomless Pool", Count: 4},
		{Name: "Coral Merfolk", Count: 4}, {Name: "Goblin Piker", Count: 2},
		{Name: "Shock", Count: 2}, {Name: "Island", Count: 8}, {Name: "Mountain", Count: 8},
	},
}}

// ksubtypevocabStats logs how often each carrier's filtered ability was
// reached on the gorge side.
type ksubtypevocabStats struct {
	lessonCasts      int // Accumulate Wisdom (a Lesson) cast, Sokka's trigger condition
	soulragerActs    int // Intruding Soulrager's Sac<1/Room> ability activated
	sokkaLessonTurns int // (game, turn)s with a Sokka out and a Lesson card in that player's graveyard
}

// ksubtypevocabPolicy plays the random policy and
// counts the carrier states the gorge side reaches.
func ksubtypevocabPolicy(seed uint64, st *ksubtypevocabStats) Policy {
	rnd := RandomPolicy(seed)
	lastTurn := int32(-1)
	return PolicyFunc(func(c *PickCtx) Canon {
		a := rnd.Pick(c)
		if c.G == nil {
			return a
		}
		g := c.G.E.G
		if k := a.Kind(); k == ActCast || k == ActActivate {
			if o, ok := c.G.GorgeOption(a); ok {
				if ob := g.Obj(o.Obj); ob != nil && ob.Face() != nil {
					switch {
					case k == ActCast && ob.Face().Name == "Accumulate Wisdom":
						st.lessonCasts++
					case k == ActActivate && ob.Face().Name == "Intruding Soulrager":
						st.soulragerActs++
					}
				}
			}
		}
		if t := int32(g.Turn); t != lastTurn {
			lastTurn = t
			for p := state.PlayerID(0); p < 2; p++ {
				sokka, lesson := false, false
				for _, id := range g.Zone(state.ZBattlefield, p) {
					if ob := g.Obj(id); ob != nil && ob.Face() != nil && ob.Face().Name == "Sokka, Bold Boomeranger" {
						sokka = true
					}
				}
				for _, id := range g.Zone(state.ZGraveyard, p) {
					if ob := g.Obj(id); ob != nil && ob.Face() != nil && ob.Face().Name == "Accumulate Wisdom" {
						lesson = true
					}
				}
				if sokka && lesson {
					st.sokkaLessonTurns++
				}
			}
		}
		return a
	})
}

// runSubtypevocab plays the constructed matchups and returns the games, the
// divergences, the stalls and the carrier counts.
func runSubtypevocab(t *testing.T, seeds int) (games, divs, stalls int, st ksubtypevocabStats) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("subtypevocab")
	if err != nil {
		t.Fatal(err)
	}
	for mi, md := range ksubtypevocabDecks {
		var decks [2][]*cards.Card
		for s, d := range md {
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
				t.Fatalf("matchup %d deck %d: %d cards", mi, s, len(decks[s]))
			}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= uint64(seeds); seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"ksubtypevocab-a", "ksubtypevocab-b"}, decks, 1)
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
			res := LockstepLed(m, seed, g, k, ksubtypevocabPolicy(seed, &st), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 25})
			CloseSide(k)
			games++
			if len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
				stalls++
				t.Errorf("matchup %d seed %d: kernel stall %q", mi, seed, res.Stall)
			}
			if d := res.Div; d != nil {
				divs++
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	return games, divs, stalls, st
}

func TestSubtypevocabLockstep(t *testing.T) {
	if isolate(t) {
		return
	}

	games, divs, stalls, st := runSubtypevocab(t, 128)
	t.Logf("%d games, %d divergences, %d stalls, Lesson casts %d, Intruding Soulrager Room-cost activations %d, Sokka+Lesson turns %d",
		games, divs, stalls, st.lessonCasts, st.soulragerActs, st.sokkaLessonTurns)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	if st.lessonCasts == 0 {
		t.Errorf("no Lesson cast in the matching matchup; Sokka's Lesson filter was never reached")
	}
	if st.soulragerActs == 0 {
		t.Errorf("no Intruding Soulrager Sac<1/Room> activation in the matching matchup")
	}
	if st.sokkaLessonTurns == 0 {
		t.Errorf("no Sokka-with-Lesson turns; the matching matchup never reached the trigger condition")
	}
}
