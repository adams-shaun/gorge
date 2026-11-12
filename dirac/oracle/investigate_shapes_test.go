package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// No Witnesses (dirac's withMostTypeCreature lowering, kop.PlayerPredMostCreatures):
// "Each player who controls the most creatures investigates. Then destroy all
// creatures." The fixture pool is dirac/cardset/pools/fixtures/investshapes:
// the sorcery beside cheap vanilla creatures and burn, so the two steered
// matchups reach the two predicates often -- a strict creature-count lead
// (one player investigates) and a tie (both do, controller first, gorge
// AliveFrom's order). Officious Interrogation's counted Investigate stays
// refused at its RaiseCost (dirac/kgen/investshapes_test.go), so this pool
// carries it nowhere.

var invShapesDecks = []fixtureDecks{
	// M1: seat 0 leads in creatures; seat 1 burn keeps the gap (lead mode).
	{{
		{Name: "No Witnesses", Count: 6}, {Name: "Savannah Lions", Count: 12}, {Name: "Raging Goblin", Count: 10},
		{Name: "Plains", Count: 6}, {Name: "Mountain", Count: 6},
	}, {
		{Name: "Savannah Lions", Count: 6}, {Name: "Raging Goblin", Count: 6}, {Name: "Shock", Count: 6},
		{Name: "Plains", Count: 11}, {Name: "Mountain", Count: 11},
	}},
	// M2: mirrored boards (tie mode).
	{{
		{Name: "No Witnesses", Count: 6}, {Name: "Savannah Lions", Count: 11}, {Name: "Raging Goblin", Count: 9},
		{Name: "Plains", Count: 7}, {Name: "Mountain", Count: 7},
	}, {
		{Name: "Savannah Lions", Count: 11}, {Name: "Raging Goblin", Count: 9},
		{Name: "Plains", Count: 10}, {Name: "Mountain", Count: 10},
	}},
}

// invShapesTally counts the steered No Witnesses casts by the creature-count
// shape the caster's battlefield held (both counts read through the gorge
// side's projected characteristics).
type invShapesTally struct{ lead, tie int }

// invShapesPolicy steers the game: at priority with an empty stack it deploys
// a creature, and once the mode's shape holds (mode 0: the caster strictly
// leads with creatures out; mode 1: the counts tie with creatures out) it
// casts No Witnesses. Every other decision goes to the seed's random policy.
type invShapesPolicy struct {
	mode  int // 0 lead, 1 tie
	rnd   Policy
	tally *invShapesTally
}

func (p *invShapesPolicy) Pick(c *PickCtx) Canon {
	if c.G != nil && c.D.Kind == DPriority && len(c.G.E.G.Stack) == 0 {
		if a, ok := p.steer(c); ok {
			return a
		}
	}
	return p.rnd.Pick(c)
}

func (p *invShapesPolicy) steer(c *PickCtx) (Canon, bool) {
	g := c.G
	n := [2]int{invCreatures(g, 0), invCreatures(g, 1)}
	shape := p.mode == 0 && n[c.D.Player] > n[1-c.D.Player] && n[0]+n[1] > 0 ||
		p.mode == 1 && n[0] == n[1] && n[0] > 0
	if shape {
		for _, a := range c.D.Options {
			if a.Kind() == ActCast && refCardName(g, a.Src()) == "No Witnesses" {
				if p.mode == 0 {
					p.tally.lead++
				} else {
					p.tally.tie++
				}
				return a, true
			}
		}
	}
	for _, a := range c.D.Options {
		if a.Kind() != ActCast {
			continue
		}
		o, ok := g.GorgeOption(a)
		if !ok || o.Obj == 0 {
			continue
		}
		if invTypeNamed(g.E.Chars(o.Obj).Types, "Creature") {
			return a, true
		}
	}
	return 0, false
}

// invCreatures is p's battlefield creature count (the gorge side's projected
// types, the count gorge playerHasMost TypeCreature reads).
func invCreatures(g *GorgeSide, p uint8) int {
	n := 0
	for _, id := range g.E.G.Zone(state.ZBattlefield, state.PlayerID(p)) {
		if invTypeNamed(g.E.Chars(id).Types, "Creature") {
			n++
		}
	}
	return n
}

func invTypeNamed(types []string, want string) bool {
	for _, w := range types {
		if w == want {
			return true
		}
	}
	return false
}

// invShapesTables builds each matchup's card list and its lowered table.
func invShapesTables(t *testing.T, reg interface {
	Lookup(name string) (*cards.Card, bool)
}) ([2]*kop.Table, [2][2][]*cards.Card) {
	t.Helper()
	set, err := cardset.RawByName("investshapes")
	if err != nil {
		t.Fatal(err)
	}
	var cs [2][2][]*cards.Card
	var tbls [2]*kop.Table
	for mi, md := range invShapesDecks {
		for s, d := range md {
			for _, e := range d {
				c, ok := reg.Lookup(e.Name)
				if !ok {
					t.Fatalf("%s not in the corpus", e.Name)
				}
				for i := 0; i < e.Count; i++ {
					cs[mi][s] = append(cs[mi][s], c)
				}
			}
			if len(cs[mi][s]) != 40 {
				t.Fatalf("matchup %d deck %d: %d cards", mi, s, len(cs[mi][s]))
			}
		}
		tbls[mi], _, _, err = set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
	}
	return tbls, cs
}

func invShapesRun(t *testing.T, tbl *kop.Table, cs [2][]*cards.Card, mode int, tally *invShapesTally, seeds uint64, report bool) (divs int) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("investshapes")
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"investshapes-a", "investshapes-b"}, cs, 1)
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
		pol := &invShapesPolicy{mode: mode, rnd: RandomPolicy(seed), tally: tally}
		res := LockstepLed(m, seed, g, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 30})
		CloseSide(k)
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			if report {
				t.Errorf("seed %d: kernel capacity stall %q", seed, res.Stall)
			}
		}
		if d := res.Div; d != nil {
			divs++
			msg := strings.TrimSpace(d.What + " " + d.Field + " " + d.Detail)
			if report {
				t.Errorf("seed %d: diverged at decision %d: %s %v\nref %v\nker %v", seed, d.At, msg, d.Diff, d.RefOptions, d.KerOptions)
			}
			if !report || divs >= 3 {
				break
			}
		}
	}
	return divs
}

// TestInvestigateShapesLockstep plays the two steered matchups in lockstep:
// every divergence fails, and the two shapes must each have fired enough
// times for the run to be evidence of the withMostTypeCreature lowering
// (one leader's mint; both players' mints on a tie, controller first).
func TestInvestigateShapesLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	tbls, cs := invShapesTables(t, reg)
	tally := &invShapesTally{}
	if divs := invShapesRun(t, tbls[0], cs[0], 0, tally, 100, true); divs > 0 {
		return
	}
	if divs := invShapesRun(t, tbls[1], cs[1], 1, tally, 100, true); divs > 0 {
		return
	}
	t.Logf("lead casts %d, tie casts %d", tally.lead, tally.tie)
	if tally.lead < 25 {
		t.Errorf("only %d lead-shape No Witnesses casts", tally.lead)
	}
	if tally.tie < 25 {
		t.Errorf("only %d tie-shape No Witnesses casts", tally.tie)
	}
}

// TestInvestigateShapesMutation rewrites the lowered predicate from
// withMostTypeCreature to controls-an-artifact-or-enchantment
// (kop.PlayerPredArtOrEnch): no card in the pool is either, so nobody would
// investigate, and the lockstep must diverge (gorge still mints for the
// leader, or for both on a tie).
func TestInvestigateShapesMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	tbls, cs := invShapesTables(t, reg)
	mutateCard(t, tbls[0], "No Witnesses", func(a *kop.Ability, in *kop.Instr) {
		if in.B.N == kop.PlayerPredMostCreatures {
			in.B = kop.Const(kop.PlayerPredArtOrEnch)
		}
	})
	if divs := invShapesRun(t, tbls[0], cs[0], 0, &invShapesTally{}, 24, false); divs == 0 {
		t.Fatalf("the mutated predicate did not diverge in the lead matchup")
	}
	t.Logf("mutation diverged")
}
