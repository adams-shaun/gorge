package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The defined-referent value heads lockstep:
// Borrowed Knowledge's TargetedPlayer$CardsInHand, Sting's
// ThisTargetedPlayer$Valid, Glamdring's Equipped$CardPower read by a
// ReduceCost static (and its non-positive clamp), Compel Brutality's
// ParentTargeted$CardCounters.LOYALTY and Kefka's Remembered$CardTypes.

var kvrefhDecks = [][2][]cardset.Entry{{
	{
		{Name: "Kefka, Court Mage", Count: 4}, {Name: "Borrowed Knowledge", Count: 4}, {Name: "Compel Brutality", Count: 4},
		{Name: "Ajani, Caller of the Pride", Count: 3}, {Name: "Grizzly Bears", Count: 4}, {Name: "Opt", Count: 3},
		{Name: "Island", Count: 6}, {Name: "Swamp", Count: 4}, {Name: "Mountain", Count: 4}, {Name: "Plains", Count: 2},
		{Name: "Forest", Count: 2},
	},
	{
		{Name: "Glamdring, Foe-hammer", Count: 4}, {Name: "Sting, Bilbo's Sword", Count: 4}, {Name: "Hill Giant", Count: 4},
		{Name: "Savannah Lions", Count: 6}, {Name: "Lightning Bolt", Count: 4}, {Name: "Divination", Count: 4},
		{Name: "Plains", Count: 5}, {Name: "Mountain", Count: 5}, {Name: "Island", Count: 4},
	},
}}

type kvrefhStats struct {
	games, divs, stalls int
	kefka               int // Kefka entries and attacks (each discards, then draws per type)
	glamCasts           int // instant/sorcery casts with a Glamdring attached to the caster's creature
	sting               int // Sting entries
	compel              int // Compel Brutality's planeswalker mode chosen
}

// kvrefhPolicy takes any non-pass priority option (so the equipment is cast,
// equipped and used), attacks whenever possible, picks the planeswalker mode
// when offered, else random; it counts the carriers' gorge-side events.
func kvrefhPolicy(seed uint64, st *kvrefhStats) Policy {
	rnd := RandomPolicy(seed)
	seen := map[state.ObjID]bool{}
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		buf = buf[:0]
		for _, a := range c.D.Options {
			switch c.D.Kind {
			case DPriority:
				if a.Kind() != ActPass {
					buf = append(buf, a)
				}
			case DAttackers:
				if a.Kind() == ActAttack {
					buf = append(buf, a)
				}
			case DModes:
				if a.Kind() == ActMode && a.Mode() == 1 {
					buf = append(buf, a)
				}
			}
		}
		var a Canon
		if len(buf) == 0 {
			a = rnd.Pick(c)
		} else {
			d := *c.D
			d.Options = buf
			a = rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		}
		if c.G == nil {
			return a
		}
		g := c.G.E.G
		for i := range g.Objs {
			o := &g.Objs[i]
			if o.Zone != state.ZBattlefield || o.Face() == nil || seen[o.ID] {
				continue
			}
			switch o.Face().Name {
			case "Kefka, Court Mage":
				st.kefka++
			case "Sting, Bilbo's Sword":
				st.sting++
			default:
				continue
			}
			seen[o.ID] = true
		}
		if o, ok := c.G.GorgeOption(a); ok {
			switch {
			case c.D.Kind == DModes && a.Kind() == ActMode && a.Mode() == 1:
				st.compel++
			case a.Kind() == ActAttack && chName(g, o.Obj) == "Kefka, Court Mage":
				st.kefka++
			case a.Kind() == ActCast:
				if n := chName(g, o.Obj); n == "Divination" || n == "Lightning Bolt" {
					for i := range g.Objs {
						gl := &g.Objs[i]
						if gl.Zone == state.ZBattlefield && gl.Face() != nil && gl.Face().Name == "Glamdring, Foe-hammer" &&
							gl.AttachedTo != 0 && uint8(gl.Controller) == c.D.Player {
							st.glamCasts++
							break
						}
					}
				}
			}
		}
		return a
	})
}

// runVrefh plays the constructed matchup over seeds 1..seeds; mut sets the
// DebugRemTypesCount mutation.
func runVrefh(t *testing.T, seeds int, mut bool) kvrefhStats {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("vrefh")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugRemTypesCount
	dirac.DebugRemTypesCount = mut
	defer func() { dirac.DebugRemTypesCount = old }()
	var st kvrefhStats
	for mi, md := range kvrefhDecks {
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
			m, err := NewMatchup(reg, [2]string{"kvrefh-a", "kvrefh-b"}, decks, 1)
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
			res := LockstepLed(m, seed, g, k, kvrefhPolicy(seed, &st), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
			CloseSide(k)
			st.games++
			if d := res.Div; d != nil {
				st.divs++
				if !mut {
					t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				}
			}
			if strings.HasPrefix(res.Stall, KernelStallPrefix) {
				st.stalls++
				if !mut {
					t.Errorf("matchup %d seed %d: stalled: %s", mi, seed, res.Stall)
				}
			}
		}
	}
	return st
}

func TestVrefhLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	st := runVrefh(t, 256, false)
	t.Logf("%d games, %d divergences, %d stalls; Kefka %d, Glamdring-reduced casts %d, Sting entries %d, Compel mode 2 %d", st.games, st.divs, st.stalls, st.kefka, st.glamCasts, st.sting, st.compel)
	if st.divs != 0 || st.stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", st.divs, st.stalls)
	}
	for _, f := range []struct {
		name string
		n    int
	}{{"Kefka triggers", st.kefka}, {"Glamdring-reduced casts", st.glamCasts}, {"Sting entries", st.sting}, {"Compel mode 2 picks", st.compel}} {
		if f.n < 20 {
			t.Errorf("only %d %s, want >= 20", f.n, f.name)
		}
	}
}

// TestVrefhMutation counts the remembered objects instead of their distinct
// card types: Kefka then draws too many cards, so the games must diverge.
func TestVrefhMutation(t *testing.T) {
	st := runVrefh(t, 64, true)
	if st.divs == 0 {
		t.Fatalf("mutation left Remembered$CardTypes counting objects in %d games with no divergence", st.games)
	}
	t.Logf("mutation diverged in %d of %d games", st.divs, st.games)
}
