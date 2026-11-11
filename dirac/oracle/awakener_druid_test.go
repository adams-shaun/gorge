package oracle

import (
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Awakener Druid lockstep: its enters trigger's Animate Duration$
// AsLongAsInPlay (CR 611.2b) makes a target Forest a 4/5 green Treefolk
// creature and land for as long as the Druid remains. The kernel's
// WhileHost override carries the constant P/T and ends when the animating
// host leaves, while the animated object's own zone change is a new object
// (CR 400.7). Both engines decide every target, attack and cleanup together.

var kawakenerDecks = [2][]cardset.Entry{
	{
		{Name: "Awakener Druid", Count: 8}, {Name: "Grizzly Bears", Count: 8},
		{Name: "Shock", Count: 4}, {Name: "Forest", Count: 10}, {Name: "Mountain", Count: 10},
	},
	{
		{Name: "Awakener Druid", Count: 8}, {Name: "Hill Giant", Count: 8},
		{Name: "Shock", Count: 4}, {Name: "Forest", Count: 10}, {Name: "Mountain", Count: 10},
	},
}

type awakenerStats struct {
	games, divs, stalls         int
	animated, carried, attacked int
	reverted, wrong             int
}

// awakenerCheck reads gorge's board at every agreed decision. Every animated
// Forest must be exactly the card's grant (4/5, green, Land Creature
// Treefolk, still a land), and a Forest that stays on the battlefield and
// stops being a creature lost the host's animation.
func awakenerCheck(t *testing.T, seed uint64, ref *GorgeSide, first map[state.ObjID]int32, st *awakenerStats) {
	t.Helper()
	for _, pl := range []state.PlayerID{0, 1} {
		for _, id := range ref.E.G.Zone(state.ZBattlefield, pl) {
			o := ref.E.G.Obj(id)
			if o == nil || o.Face() == nil || !o.Face().IsLand() {
				continue
			}
			if !ref.E.IsCreature(id) {
				if _, ok := first[id]; ok {
					st.reverted++
					delete(first, id)
				}
				continue
			}
			d := ref.E.Derived(id)
			st.animated++
			if d.Power != 4 || d.Toughness != 5 || d.Colors != "G" ||
				!slices.Contains(d.Types, "Land") || !slices.Contains(d.Types, "Creature") ||
				!slices.Contains(d.Types, "Treefolk") {
				st.wrong++
				if st.wrong <= 5 {
					t.Errorf("seed %d: animated Forest %d = %d/%d %q %v, want 4/5 green Land Creature Treefolk",
						seed, id, d.Power, d.Toughness, d.Colors, d.Types)
				}
			}
			if f, ok := first[id]; !ok {
				first[id] = ref.E.G.Turn
			} else if ref.E.G.Turn > f {
				st.carried++
			}
			if o.IsAttacking {
				st.attacked++
			}
		}
	}
}

func runAwakener(t *testing.T, seeds int) (st awakenerStats) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("awakenerdruid")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kawakenerDecks {
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
	tbl, _, _, err := set.Matchup(kawakenerDecks[0], kawakenerDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kawakener-a", "kawakener-b"}, decks, 1)
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
		first := map[state.ObjID]int32{}
		res := LockstepLed(m, seed, g, k, castFirstPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20,
			Check: func(ref *GorgeSide, _ Side) *Divergence {
				awakenerCheck(t, seed, ref, first, &st)
				return nil
			}})
		CloseSide(k)
		st.games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			st.stalls++
			t.Errorf("seed %d: stalled: %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			st.divs++
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	return st
}

func TestAwakenerDruidAsLongAsInPlayLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	st := runAwakener(t, 64)
	t.Logf("%+v", st)
	if st.divs != 0 || st.stalls != 0 || st.wrong != 0 {
		t.Fatalf("%d divergences, %d stalls, %d wrong animations", st.divs, st.stalls, st.wrong)
	}
	// The refusal's ability must actually be reached: the Forest is animated,
	// outlives its turn while the Druid stays, attacks, and reverts when the
	// host leaves while the land stays put.
	if st.animated < 20 || st.attacked < 1 || st.carried < 5 || st.reverted < 3 {
		t.Errorf("animated %d, attacked %d, carried %d, reverted %d (want at least 20, 1, 5, 3)",
			st.animated, st.attacked, st.carried, st.reverted)
	}
}
