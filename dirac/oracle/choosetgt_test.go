package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
)

// The targeted-chooser ChooseCard lockstep: End of the
// Hunt is a mandatory ChooseCard with ValidTgts$ Opponent. With no Defined$,
// gorge's DefinedRef (engine/effects/context.go:80,130-139) falls back to the
// ability's chosen targets, so the TARGETED opponent answers the pick, not the
// caster (choose_control.go:399-430,584). The pool matches the chooser's
// perspective and TargetedPlayerCtrl names the target either way; an empty
// pool asks nothing. The creatures give both seats candidates at every mana
// value 1..5, with four Jagwasp Swarms and four Sengir Vampires each so 2+
// tie at the greatest mana value often, and Ajani is a kernel-accepted
// planeswalker in the same pool.
var kctDecks = [2][]cardset.Entry{
	{
		// the carrier seat: the densest End of the Hunt package, so the
		// 25-turn random games pose the targeted pick often
		{Name: "End of the Hunt", Count: 6},
		{Name: "Diregraf Ghoul", Count: 4},    // mana value 1
		{Name: "Walking Corpse", Count: 4},    // mana value 2
		{Name: "Vampire Nighthawk", Count: 4}, // mana value 3, flying/lifelink beatdown
		{Name: "Jagwasp Swarm", Count: 4},     // mana value 4, flying
		{Name: "Sengir Vampire", Count: 4},    // mana value 5, flying
		{Name: "Swamp", Count: 14},
	},
	{
		{Name: "End of the Hunt", Count: 2},
		{Name: "Diregraf Ghoul", Count: 4},
		{Name: "Walking Corpse", Count: 4},
		{Name: "Vampire Nighthawk", Count: 4},
		{Name: "Jagwasp Swarm", Count: 4},
		{Name: "Sengir Vampire", Count: 4},
		{Name: "Ajani, Caller of the Pride", Count: 4}, // planeswalker at mana value 3
		{Name: "Swamp", Count: 10},
		{Name: "Plains", Count: 4},
	},
}

// kctTally counts, from gorge's pending choice decisions and its Choose
// events, End of the Hunt resolutions: the exiling picks (answered by the
// targeted opponent, never the caster), the asks with 2+ candidates and the
// empty-pool resolutions (a Choose event with no picked ids, nothing asked).
type kctTally struct {
	seed      uint64
	evAt      int
	seq       uint64
	asked     bool
	askPlayer int
	askOpts   int
	asks      int
	exiles    int
	nonCaster int
	multi     int
	empty     int
	casts     int
	badSeed   uint64
}

func (p *kctTally) reset(seed uint64) {
	// the counters are cumulative across the run; only the per-game cursor
	// and the pending-ask pairing start over
	p.seed = seed
	p.evAt, p.seq, p.asked, p.askPlayer, p.askOpts, p.badSeed = 0, 0, false, 0, 0, 0
}

func (p *kctTally) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < p.evAt {
		p.evAt = 0
		p.asked = false
	}
	for j := p.evAt; j < len(evs); j++ {
		e := &evs[j]
		if e.Kind == events.PutOnStack && chName(g, e.Obj) == "End of the Hunt" {
			p.casts++
			continue
		}
		if e.Kind != events.Choose || e.Counter != "chosen" || chName(g, e.Obj) != "End of the Hunt" {
			continue
		}
		if len(e.IDs) == 0 {
			p.empty++
			continue
		}
		p.exiles++
		caster := -1
		if o := g.Obj(e.Obj); o != nil {
			caster = int(o.Controller)
		}
		if p.asked && p.askPlayer != caster {
			p.nonCaster++
		}
	}
	p.evAt = len(evs)
	p.asked = false
	if d := ref.Pending(); d != nil && d.Kind == decision.KChoose && d.ResumeKind == "choice" &&
		d.ResumeSA != nil && d.ResumeSA.API == "ChooseCard" && chName(g, d.Source) == "End of the Hunt" {
		if uint64(d.Seq) != p.seq {
			p.seq = uint64(d.Seq)
			p.asked = true
			p.askPlayer = int(d.Player)
			p.askOpts = len(d.Options)
			p.asks++
			caster := -1
			if o := g.Obj(d.Source); o != nil {
				caster = int(o.Controller)
			}
			if p.askPlayer == caster && p.badSeed == 0 {
				// gorge would contradict Forge (ChooseCardEffect's
				// getDefinedPlayersOrTargeted): report, do not work around.
				p.badSeed = p.seed
			}
			if p.askOpts >= 2 {
				p.multi++
			}
		}
	}
	return nil
}

func kctSetup(t *testing.T) (*cards.Registry, cardset.Set) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("choosetgt")
	if err != nil {
		t.Fatal(err)
	}
	return reg, set
}

// kctPolicy is the seeded random policy with the pass option dropped while
// the seat can act, End of the Hunt preferred when castable, and combat
// forced through: like the other constructed lockstep carriers, the
// attackers all declare and the blockers stay home so the 25-turn games
// decide. Uniform random over every option leaves most games unfinished and
// seldom reaches a two-mana sorcery; the remaining choices are still picked
// uniformly from the seed's own PCG stream.
func kctPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	pick := func(c *PickCtx, opts []Canon) Canon {
		d := *c.D
		d.Options = opts
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	}
	return PolicyFunc(func(c *PickCtx) Canon {
		switch c.D.Kind {
		case DAttackers:
			att := make([]Canon, 0, len(c.D.Options))
			for _, a := range c.D.Options {
				if a.Kind() == ActAttack {
					att = append(att, a)
				}
			}
			if len(att) > 0 {
				return pick(c, att)
			}
		case DBlockers:
			for _, a := range c.D.Options {
				if a.Kind() == ActDone {
					return a
				}
			}
		case DPriority:
			// the carrier first, so the targeted pick under test is reached
			if c.G != nil {
				for _, a := range c.D.Options {
					if a.Kind() != ActCast {
						continue
					}
					if o, ok := c.G.GorgeOption(a); ok && chName(c.G.E.G, o.Obj) == "End of the Hunt" {
						return a
					}
				}
			}
			act := make([]Canon, 0, len(c.D.Options))
			for _, a := range c.D.Options {
				if a.Kind() != ActPass {
					act = append(act, a)
				}
			}
			if len(act) > 0 {
				return pick(c, act)
			}
		}
		return rnd.Pick(c)
	})
}

// kctMatchup builds the two 40-card decks and their kop table.
func kctMatchup(t *testing.T, reg *cards.Registry, set cardset.Set) ([2][]*cards.Card, *kop.Table) {
	t.Helper()
	var decks [2][]*cards.Card
	for s, d := range kctDecks {
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
	tbl, _, _, err := set.Matchup(kctDecks[0], kctDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	return decks, tbl
}

func TestChoosetgtLockstep(t *testing.T) {
	reg, set := kctSetup(t)
	decks, tbl := kctMatchup(t, reg, set)
	tally := &kctTally{}
	games, divs, stalls := 0, 0, 0
	for seed := uint64(1); seed <= 256; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kct-a", "kct-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		tally.reset(seed)
		res := Lockstep(m, seed, k, kctPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 25, Check: tally.check})
		CloseSide(k)
		games++
		if tally.badSeed != 0 {
			t.Fatalf("seed %d: gorge asked the CASTING seat for End of the Hunt, not the targeted opponent (gorge finding, do not work around)", tally.badSeed)
		}
		if res.Stall != "" {
			stalls++
			t.Errorf("seed %d: stalled: %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			divs++
			lo := max(d.At-12, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	t.Logf("%d games, %d divergences, %d stalls; End of the Hunt casts %d, asks %d, exiling resolutions %d (%d answered by the non-casting seat), 2+ candidates %d, empty pools %d",
		games, divs, stalls, tally.casts, tally.asks, tally.exiles, tally.nonCaster, tally.multi, tally.empty)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	if tally.exiles < 10 {
		t.Errorf("%d End of the Hunt resolutions exiling a card, want at least 10", tally.exiles)
	}
	if tally.nonCaster != tally.exiles {
		t.Errorf("%d of %d exiling resolutions answered by the non-casting seat, want all", tally.nonCaster, tally.exiles)
	}
	if tally.multi < 3 {
		t.Errorf("%d resolutions with 2+ candidates, want at least 3", tally.multi)
	}
	if tally.empty < 1 {
		t.Errorf("%d resolutions with an empty pool, want at least 1", tally.empty)
	}
}

// TestChoosetgtMutation hands the pick to 1-p (the caster) instead of the
// targeted opponent: the kernel's candidate pool is then the caster's board,
// so the lockstep must diverge.
func TestChoosetgtMutation(t *testing.T) {
	reg, set := kctSetup(t)
	decks, tbl := kctMatchup(t, reg, set)
	old := dirac.DebugChooseCardAmount2
	dirac.DebugChooseCardAmount2 = true
	defer func() { dirac.DebugChooseCardAmount2 = old }()
	for seed := uint64(1); seed <= 128; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kct-a", "kct-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, kctPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 25})
		CloseSide(k)
		if res.Div != nil {
			t.Logf("seed %d diverged: %s", seed, res.Div)
			return
		}
	}
	t.Fatal("DebugChooseCardAmount2 did not diverge in 128 seeds")
}
