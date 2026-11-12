package oracle

import (
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// DB$ AddPhase and the FirstAttack$ / FirstCombat$ gates' lockstep
// Aurelia the Warleader, Fear of Missing Out, Genji Glove and
// Raph & Leo grant an extra combat from a first-attack trigger; Obeka grants
// one upkeep per damage dealt; Y'shtola Rhul an extra end step gated on the
// finished end steps. Compared with gorge through the extra phases' triggers,
// priority rounds and resumes. Seats are swapped: each deck plays both seats.

var kaddphaseDecks = [2][]cardset.Entry{
	{ // A: R/W, the four extra-combat carriers
		{Name: "Aurelia, the Warleader", Count: 4}, {Name: "Raph & Leo, Sibling Rivals", Count: 4}, {Name: "Genji Glove", Count: 3},
		{Name: "Fear of Missing Out", Count: 4}, {Name: "Isamaru, Hound of Konda", Count: 3}, {Name: "Savannah Lions", Count: 3},
		{Name: "Lightning Bolt", Count: 2},
		{Name: "Mountain", Count: 8}, {Name: "Plains", Count: 9},
	},
	{ // B: U/B/R, Obeka, Y'shtola and the upkeep and end-step triggers
		{Name: "Obeka, Splitter of Seconds", Count: 4}, {Name: "Y'shtola Rhul", Count: 4}, {Name: "Hill Giant", Count: 5},
		{Name: "Phyrexian Arena", Count: 2}, {Name: "Scrawling Crawler", Count: 2}, {Name: "Perforating Artist", Count: 2},
		{Name: "Shock", Count: 2}, {Name: "Grizzly Bears", Count: 2},
		{Name: "Island", Count: 6}, {Name: "Swamp", Count: 5}, {Name: "Mountain", Count: 6},
	},
}

// kaddphaseStats is what the policy counts across the games, on gorge's state.
type kaddphaseStats struct {
	extraCombats int // combat phases begun beyond the first of a turn
	obekaUps     int // turns where two or more upkeep grants were queued at once
	extraEnds    int // end steps begun beyond the first of a turn
	gated        int // carrier attackers (a first-attack trigger's) declared in a second combat or later
}

type kaddphaseProbe struct {
	st       *kaddphaseStats
	base     Policy
	r        *rand.Rand
	turn     int32
	combats  int32
	endSteps int32
	obeka    bool
}

func (p *kaddphaseProbe) observe(gs *GorgeSide) {
	g := gs.E.G
	if int32(g.Turn) != p.turn {
		p.turn, p.combats, p.endSteps, p.obeka = int32(g.Turn), 0, 0, false
	}
	if g.CombatsThisTurn > p.combats {
		if p.combats >= 1 {
			p.st.extraCombats += int(g.CombatsThisTurn - p.combats)
		}
		p.combats = g.CombatsThisTurn
	}
	if g.EndStepsThisTurn > p.endSteps {
		if p.endSteps >= 1 {
			p.st.extraEnds += int(g.EndStepsThisTurn - p.endSteps)
		}
		p.endSteps = g.EndStepsThisTurn
	}
	if !p.obeka {
		n := 0
		for _, ep := range g.ExtraPhases {
			if ep.Entry == state.StepUpkeep {
				n++
			}
		}
		if n >= 2 {
			p.obeka = true
			p.st.obekaUps++
		}
	}
}

// carrierAttack reports whether the attack a declares an attacker with a
// first-attack trigger: Aurelia, Fear of Missing Out, Raph & Leo, or a
// creature equipped with Genji Glove.
func (p *kaddphaseProbe) carrierAttack(gs *GorgeSide, a Canon) bool {
	switch refCardName(gs, a.Src()) {
	case "Aurelia, the Warleader", "Fear of Missing Out", "Raph & Leo, Sibling Rivals":
		return true
	}
	g := gs.E.G
	id := gs.id.obj[a.Src()]
	for _, bf := range g.Zone(state.ZBattlefield, g.Active) {
		if o := g.Obj(bf); o != nil && firstFaceName(g, bf) == "Genji Glove" && o.AttachedTo == id {
			return true
		}
	}
	return false
}

func (p *kaddphaseProbe) pick(c *PickCtx) Canon {
	gs := c.G
	var pool, attacks []Canon
	for _, a := range c.D.Options {
		switch a.Kind() {
		case ActActivate, ActSpecial:
			pool = append(pool, a)
		case ActAttack:
			attacks = append(attacks, a)
		}
	}
	a := p.base.Pick(c)
	// prefer attacking, then an ability (equip), then the random pick
	if len(attacks) > 0 && p.r.IntN(8) != 0 {
		a = attacks[p.r.IntN(len(attacks))]
	} else if len(pool) > 0 && p.r.IntN(4) != 0 {
		a = pool[p.r.IntN(len(pool))]
	}
	if gs == nil {
		return a
	}
	p.observe(gs)
	if a.Kind() == ActAttack && gs.E.G.CombatsThisTurn >= 2 && p.carrierAttack(gs, a) {
		p.st.gated++
	}
	return a
}

// runAddphase plays the constructed matchups (both seat orders) and returns
// the games, the divergences, the stalls and the carrier counts.
func runAddphase(t *testing.T, seeds int, report bool) (games, divs, stalls int, st kaddphaseStats) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("addphase")
	if err != nil {
		t.Fatal(err)
	}
	for order := 0; order < 2; order++ {
		md := [2][]cardset.Entry{kaddphaseDecks[order], kaddphaseDecks[1-order]}
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
				t.Fatalf("order %d deck %d: %d cards", order, s, len(decks[s]))
			}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= uint64(seeds); seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"kaddphase-a", "kaddphase-b"}, decks, 1)
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
			probe := &kaddphaseProbe{st: &st, base: RandomPolicy(seed), r: rand.New(rand.NewPCG(seed, 0x61646470))}
			res := LockstepLed(m, seed, g, k, PolicyFunc(probe.pick), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
			CloseSide(k)
			games++
			if len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
				stalls++
				if report {
					t.Errorf("order %d seed %d: kernel stall %q", order, seed, res.Stall)
				}
			}
			if d := res.Div; d != nil {
				divs++
				if report {
					lo := max(d.At-12, 0)
					t.Logf("order %d seed %d last actions %v", order, seed, actionStrings(res.Actions[lo:d.At]))
					t.Errorf("order %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", order, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				}
			}
		}
	}
	return games, divs, stalls, st
}

func TestAddphaseLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, stalls, st := runAddphase(t, 128, true)
	t.Logf("%d games, %d divergences, %d stalls, %+v", games, divs, stalls, st)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	for _, c := range []struct {
		n, min int
		what   string
	}{
		{st.extraCombats, 30, "extra combat phases"}, {st.obekaUps, 10, "turns Obeka queued two or more upkeeps"},
		{st.extraEnds, 5, "extra end steps"}, {st.gated, 10, "first-attack carriers declared in a second combat"},
	} {
		if c.n < c.min {
			t.Errorf("%s: %d, want at least %d", c.what, c.n, c.min)
		}
	}
}

// TestAddphaseMutation: resuming a completed extra phase at its own
// successor must make the lockstep diverge.
func TestAddphaseMutation(t *testing.T) {
	dirac.DebugAddPhaseResumeWrong = true
	defer func() { dirac.DebugAddPhaseResumeWrong = false }()
	games, divs, _, _ := runAddphase(t, 32, false)
	if divs == 0 {
		t.Fatalf("DebugAddPhaseResumeWrong: no divergence in %d games", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}
