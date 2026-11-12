package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The lockstep: constructed matchups over the generated graveleaves pool --
// "whenever one or more cards leave your graveyard" carriers beside Raise
// Dead, Regrowth, Skeleton Crew's self-return, an artifact creature and
// vanilla creatures -- so the expected answer at every decision is gorge's.

var graveleavesDecks = [][2][]cardset.Entry{{
	{
		{Name: "Skeleton Crew", Count: 2}, {Name: "Soul Enervation", Count: 2}, {Name: "Undead Hand Ninja", Count: 2},
		{Name: "Rot Farm Mortipede", Count: 2}, {Name: "Thran Vigil", Count: 2}, {Name: "Attuned Hunter", Count: 2},
		{Name: "Insidious Roots", Count: 2}, {Name: "Fang, Fearless l'Cie", Count: 1},
		{Name: "Raise Dead", Count: 3}, {Name: "Regrowth", Count: 2},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Swamp", Count: 8}, {Name: "Forest", Count: 8},
	},
	{
		{Name: "Chalk Outline", Count: 1}, {Name: "Desecrated Tomb", Count: 1}, {Name: "Fuming Effigy", Count: 1},
		{Name: "Garrison Excavator", Count: 1}, {Name: "On Wings of Gold", Count: 1}, {Name: "Spirit Mascot", Count: 1},
		{Name: "Stonebound Mentor", Count: 1}, {Name: "Hardened Academic", Count: 1}, {Name: "Owlin Historian", Count: 1},
		{Name: "Quintorius, Field Historian", Count: 1}, {Name: "Cyan, Vengeful Samurai", Count: 1},
		{Name: "Imotekh the Stormlord", Count: 1}, {Name: "Teval, the Balanced Scale", Count: 1},
		{Name: "Fang, Fearless l'Cie", Count: 1},
		{Name: "Raise Dead", Count: 3}, {Name: "Ornithopter", Count: 3}, {Name: "Grizzly Bears", Count: 1},
		{Name: "Plains", Count: 5}, {Name: "Swamp", Count: 5}, {Name: "Mountain", Count: 3},
		{Name: "Island", Count: 3}, {Name: "Forest", Count: 3},
	},
}}

var graveleavesCarrierNames = []string{
	"Attuned Hunter", "Chalk Outline", "Cyan, Vengeful Samurai", "Desecrated Tomb",
	"Fang, Fearless l'Cie", "Fuming Effigy", "Garrison Excavator", "Hardened Academic",
	"Imotekh the Stormlord", "Insidious Roots", "On Wings of Gold", "Owlin Historian",
	"Quintorius, Field Historian", "Rot Farm Mortipede", "Skeleton Crew", "Soul Enervation",
	"Spirit Mascot", "Stonebound Mentor", "Teval, the Balanced Scale", "Thran Vigil",
	"Undead Hand Ninja",
}

var graveleavesCarrier = func() map[string]bool {
	m := make(map[string]bool, len(graveleavesCarrierNames))
	for _, n := range graveleavesCarrierNames {
		m[n] = true
	}
	return m
}()

// graveleavesTriggerWatch records each carrier whose triggered ability reaches the
// kernel's stack. A per-game boolean is enough: the test asserts every
// carrier triggers in at least one game.
type graveleavesTriggerWatch struct {
	Side
	seen map[string]bool
}

func (w *graveleavesTriggerWatch) Step() CDecision {
	d := w.Side.Step()
	if r, ok := w.Side.(*realSide); ok {
		for i := 0; i < int(r.kp.NStack); i++ {
			e := &r.kp.Stack[i]
			if e.Kind != 2 || int(e.Card) >= len(r.gname) {
				continue
			}
			if n := r.gname[e.Card]; graveleavesCarrier[n] {
				w.seen[n] = true
			}
		}
	}
	return d
}

func (w *graveleavesTriggerWatch) Close() { CloseSide(w.Side) }

func TestLeavesGraveyardTriggerLockstep(t *testing.T) {
	t.Parallel()

	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("graveleaves")
	if err != nil {
		t.Fatal(err)
	}
	games, casts, acts := 0, 0, 0
	triggered := map[string]int{}
	for mi, md := range graveleavesDecks {
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
		for seed := uint64(1); seed <= 32; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"c8b-a", "c8b-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			watch := &graveleavesTriggerWatch{Side: k, seen: map[string]bool{}}
			pol := castActivatePolicy(seed)
			if seed%4 == 0 {
				pol = RandomPolicy(seed)
			}
			res := Lockstep(m, seed, watch, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
			CloseSide(watch)
			for n := range watch.seen {
				triggered[n]++
			}
			games++
			for _, a := range res.Actions {
				switch a.Kind() {
				case ActCast:
					casts++
				case ActActivate:
					acts++
				}
			}
			if d := res.Div; d != nil {
				lo := d.At - 12
				if lo < 0 {
					lo = 0
				}
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	t.Logf("%d games, %d casts, %d activations", games, casts, acts)
	if casts == 0 {
		t.Fatal("no cast was played")
	}
	for _, n := range graveleavesCarrierNames {
		if triggered[n] == 0 {
			t.Errorf("carrier %q never triggered across %d games", n, games)
		}
	}
	t.Logf("carrier trigger games: %v", triggered)
}
