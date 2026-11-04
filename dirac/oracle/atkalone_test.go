package oracle

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The can't-attack-alone lockstep: the printed "CARDNAME can't attack
// alone." carriers beside plain creatures and burn that kills their
// partners, so the kernel offers a restricted creature and closes a
// declaration exactly where gorge's AloneRestricted options and its
// whole-answer validation allow (CR 506.5).

var katkaloneDecks = [2][]cardset.Entry{
	// W/G: Pet Sehlat, Trusty Companion, Militia Rallier beside bears
	{
		{Name: "Pet Sehlat", Count: 6}, {Name: "Trusty Companion", Count: 4}, {Name: "Militia Rallier", Count: 3},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Shock", Count: 6},
		{Name: "Plains", Count: 9}, {Name: "Forest", Count: 4}, {Name: "Mountain", Count: 4},
	},
	// R/G: Bonded Construct, Raging Kronch, burn
	{
		{Name: "Bonded Construct", Count: 4}, {Name: "Raging Kronch", Count: 4}, {Name: "Pet Sehlat", Count: 2},
		{Name: "Hill Giant", Count: 4}, {Name: "Grizzly Bears", Count: 2}, {Name: "Shock", Count: 7},
		{Name: "Mountain", Count: 9}, {Name: "Forest", Count: 4}, {Name: "Plains", Count: 4},
	},
}

var katkaloneCarriers = map[string]bool{"Pet Sehlat": true, "Trusty Companion": true, "Militia Rallier": true,
	"Bonded Construct": true, "Raging Kronch": true}

type katkaloneStats struct {
	games, divs, stalls int
	carrierAttacks      int // carrier picks as an attacker
	lonelyWithheld      int // attack decisions where a ready carrier was the only creature able to attack
}

// katkalonePolicy attacks whenever it can, plays lands and casts otherwise,
// else random; it counts carrier attacks and declarations where a ready
// carrier had no partner.
func katkalonePolicy(seed uint64, st *katkaloneStats) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		buf = buf[:0]
		hasAtk, atkKind := false, false
		for _, a := range c.D.Options {
			if a.Kind() == ActAttack {
				hasAtk = true
				buf = append(buf, a)
			}
		}
		if c.G != nil && c.D.Kind == DAttackers {
			atkKind = true
		}
		if atkKind && !hasAtk {
			ready := 0
			lonely := false
			gg := c.G.E.G
			for _, id := range gg.Zone(state.ZBattlefield, state.PlayerID(c.D.Player)) {
				ob := gg.Obj(id)
				if ob == nil || ob.Face() == nil || ob.Tapped || ob.SummonSick || !slices.Contains(ob.Face().Types, "Creature") {
					continue
				}
				ready++
				lonely = lonely || katkaloneCarriers[ob.Face().Name]
			}
			if ready == 1 && lonely {
				st.lonelyWithheld++
			}
		}
		if !hasAtk {
			for _, a := range c.D.Options {
				if k := a.Kind(); k == ActPlayLand || k == ActCast {
					buf = append(buf, a)
				}
			}
		}
		a := Canon(0)
		if len(buf) == 0 {
			a = rnd.Pick(c)
		} else {
			d := *c.D
			d.Options = buf
			a = rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		}
		if a.Kind() == ActAttack && c.G != nil {
			if o, ok := c.G.GorgeOption(a); ok {
				if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil && katkaloneCarriers[ob.Face().Name] {
					st.carrierAttacks++
				}
			}
		}
		return a
	})
}

// runAtkalone plays seeds 1..seeds of the constructed matchup. mutate sets
// DebugAloneAtkOff (the mutation must diverge).
func runAtkalone(t *testing.T, seeds int, mutate bool) *katkaloneStats {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("atkalone")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugAloneAtkOff
	dirac.DebugAloneAtkOff = mutate
	defer func() { dirac.DebugAloneAtkOff = old }()
	st := &katkaloneStats{}
	var decks [2][]*cards.Card
	for s, d := range katkaloneDecks {
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
	tbl, _, _, err := set.Matchup(katkaloneDecks[0], katkaloneDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"katkalone-a", "katkalone-b"}, decks, 1)
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
		res := LockstepLed(m, seed, g, k, katkalonePolicy(seed, st), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
		CloseSide(k)
		st.games++
		if len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
			st.stalls++
			if !mutate {
				t.Errorf("seed %d: kernel stall %q", seed, res.Stall)
			}
		}
		if d := res.Div; d != nil {
			st.divs++
			if !mutate {
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	return st
}

func TestAtkaloneLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	st := runAtkalone(t, 256, false)
	t.Logf("%d games, %d divergences, %d stalls, carrier attacks %d, declarations with a lone ready carrier %d",
		st.games, st.divs, st.stalls, st.carrierAttacks, st.lonelyWithheld)
	if st.divs != 0 || st.stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", st.divs, st.stalls)
	}
	if st.carrierAttacks < 100 {
		t.Errorf("only %d carrier attacks", st.carrierAttacks)
	}
	if st.lonelyWithheld < 10 {
		t.Errorf("only %d declarations with a lone ready carrier", st.lonelyWithheld)
	}
}

// TestAtkaloneMutation lets a restricted creature attack alone: the games
// must diverge.
func TestAtkaloneMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	st := runAtkalone(t, 128, true)
	if st.divs == 0 {
		t.Fatalf("mutation let restricted creatures attack alone in %d games with no divergence", st.games)
	}
	t.Logf("mutation diverged in %d of %d games", st.divs, st.games)
}
