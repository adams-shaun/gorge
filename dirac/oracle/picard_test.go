package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Captain Jean-Luc Picard's search puts a Spacecraft from the library onto
// the battlefield with seven CHARGE counters (WithCountersType$ CHARGE,
// WithCountersAmount$ 7). Seven Officer tokens from Picard's own cast trigger
// pay the tapXType<7/Officer> cost, and the found Spacecraft must enter with
// the counters gorge places.
var picardDecks = [2][]cardset.Entry{
	{
		{Name: "Captain Jean-Luc Picard", Count: 4},
		{Name: "Sledge-Class Seedship", Count: 4},
		{Name: "Grizzly Bears", Count: 4},
		{Name: "Savannah Lions", Count: 4},
		{Name: "Elite Vanguard", Count: 4},
		{Name: "Soul Warden", Count: 4},
		{Name: "Ajani's Pridemate", Count: 4},
		{Name: "Ornithopter", Count: 4},
		{Name: "Memnite", Count: 4},
		{Name: "Plains", Count: 12},
		{Name: "Forest", Count: 8},
	},
	{
		{Name: "Plains", Count: 20},
		{Name: "Forest", Count: 20},
	},
}

// picardPolicy plays Picard first (so the creatures cast after it make the
// seven Officers), activates whenever it can, and keeps the Sledge-Class
// Seedship in the library for the search when another spell is available.
func picardPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	name := func(c *PickCtx, a Canon) string {
		if c.G == nil {
			return ""
		}
		if o, ok := c.G.GorgeOption(a); ok {
			return chName(c.G.E.G, o.Obj)
		}
		return ""
	}
	return PolicyFunc(func(c *PickCtx) Canon {
		buf = buf[:0]
		for _, a := range c.D.Options {
			if a.Kind() == ActActivate && name(c, a) == "Captain Jean-Luc Picard" {
				buf = append(buf, a)
			}
		}
		if len(buf) == 0 {
			for _, a := range c.D.Options {
				if (a.Kind() == ActCast || a.Kind() == ActCastFace) && name(c, a) == "Captain Jean-Luc Picard" {
					buf = append(buf, a)
				}
			}
		}
		if len(buf) == 0 {
			for _, a := range c.D.Options {
				switch a.Kind() {
				case ActCast, ActCastFace, ActPlayLand:
					if name(c, a) != "Sledge-Class Seedship" {
						buf = append(buf, a)
					}
				}
			}
		}
		if len(buf) == 0 {
			for _, a := range c.D.Options {
				switch a.Kind() {
				case ActCast, ActCastFace, ActPlayLand:
					buf = append(buf, a)
				}
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

// kpicardRun plays seeds 1..n and returns the diverging games, the games the
// kernel stalled in, gorge's Spacecraft library->battlefield entries and how
// many of those carried entry counters (the WithCountersType$ rider).
func kpicardRun(t *testing.T, n uint64, report bool) (div, stalls, searched, armed int) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("picard")
	if err != nil {
		t.Fatal(err)
	}
	var gd [2][]*cards.Card
	for s, d := range picardDecks {
		for _, e := range d {
			card, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				gd[s] = append(gd[s], card)
			}
		}
	}
	tbl, _, _, err := set.Matchup(picardDecks[0], picardDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= n; seed++ {
		m, err := NewMatchup(reg, [2]string{"pi-a", "pi-b"}, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, Config{})(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, picardPolicy(seed), RunConfig{
			Config: Config{}, MaxDecisions: 20000, MaxTurns: 20,
			Finish: func(g *GorgeSide) {
				for i := range g.E.L.Events {
					ev := &g.E.L.Events[i]
					if ev.Kind != events.MoveZone || ev.From != state.ZLibrary || ev.To != state.ZBattlefield {
						continue
					}
					o := g.E.G.Obj(ev.Obj)
					if o == nil || o.Face() == nil || !picardSpacecraft(o.Face()) {
						continue
					}
					searched++
					if len(ev.Pairs) > 0 {
						armed++
					}
				}
			},
		})
		CloseSide(k)
		if res.Stall != "" {
			stalls++
		}
		if d := res.Div; d != nil {
			div++
			if !report {
				return div, stalls, searched, armed
			}
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	return div, stalls, searched, armed
}

func picardSpacecraft(f *cards.Face) bool {
	for _, typ := range f.Types {
		if typ == "Spacecraft" {
			return true
		}
	}
	return false
}

// TestPicardLockstep: 600 games, 0 divergences, with Picard's search finding
// a Spacecraft that enters with the seven counters.
func TestPicardLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	// not parallel: TestPicardMutation flips dirac.DebugSearchNoCounters
	div, stalls, searched, armed := kpicardRun(t, 600, true)
	t.Logf("600 games: %d divergences, %d stalls, %d Spacecraft searched in, %d with entry counters", div, stalls, searched, armed)
	if armed < 20 {
		t.Errorf("%d searched-in Spacecraft carried entry counters; want at least 20", armed)
	}
}

// TestPicardMutation: with the search's entry counters dropped some seed
// diverges.
func TestPicardMutation(t *testing.T) {
	dirac.DebugSearchNoCounters = true
	defer func() { dirac.DebugSearchNoCounters = false }()
	if div, _, _, _ := kpicardRun(t, 600, false); div == 0 {
		t.Fatal("no seed diverged with the search's entry counters dropped")
	}
}
