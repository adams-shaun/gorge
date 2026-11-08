package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Strategic Betrayal lockstep: a mandatory ChooseCard picks a creature
// the targeted opponent controls, then the Origin$ Battlefield,Graveyard
// ChangeZoneAll sweep exiles it and that player's graveyard. The kernel
// encodes the two-zone sweep as kop.CzBattleGrave; gorge walks the origin
// zone-major (battlefield first, one simultaneous departure). Thought Scour
// fills the graveyards; the creatures give both seats a body to choose.

var ksbDeck = []cardset.Entry{
	{Name: "Strategic Betrayal", Count: 4},
	{Name: "Diregraf Ghoul", Count: 4},    // mana value 1
	{Name: "Walking Corpse", Count: 4},    // mana value 2
	{Name: "Vampire Nighthawk", Count: 4}, // mana value 3, flying/lifelink
	{Name: "Sengir Vampire", Count: 2},    // mana value 5, flying
	{Name: "Thought Scour", Count: 4},     // fills a graveyard
	{Name: "Swamp", Count: 14},
	{Name: "Island", Count: 4},
}

// ksbTally counts Strategic Betrayal resolutions in gorge's events: the
// Choose "chosen" answer (the targeted opponent's pick) and, after it, the
// sweep's battlefield-to-exile and graveyard-to-exile MoveZones. A resolution
// counts as exercising both halves when both are present.
type ksbTally struct {
	seed            uint64
	evAt            int
	casts, picks    int
	sweeps          int
	open            bool
	picked          state.ObjID
	battle, graveMv int
}

func (p *ksbTally) reset(seed uint64) {
	p.seed = seed
	p.evAt = 0
	p.finish()
}

// finish closes the resolution opened by a Choose answer and counts it.
func (p *ksbTally) finish() {
	if p.open && p.battle > 0 && p.graveMv > 0 {
		p.sweeps++
	}
	p.open, p.picked, p.battle, p.graveMv = false, 0, 0, 0
}

func (p *ksbTally) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < p.evAt {
		p.evAt = 0
		p.finish()
	}
	for j := p.evAt; j < len(evs); j++ {
		e := &evs[j]
		switch {
		case e.Kind == events.PutOnStack && chName(g, e.Obj) == "Strategic Betrayal":
			p.casts++
		case e.Kind == events.Choose && chName(g, e.Obj) == "Strategic Betrayal":
			p.finish()
			if len(e.IDs) > 0 {
				p.picks++
				p.open, p.picked = true, e.IDs[0]
			}
		case e.Kind == events.MoveZone && e.To == state.ZExile:
			if e.From == state.ZBattlefield && e.Obj == p.picked {
				p.battle++
			}
			if e.From == state.ZGraveyard {
				p.graveMv++
			}
		}
	}
	p.evAt = len(evs)
	return nil
}

// ksbPolicy is the choosetgt policy with Strategic Betrayal preferred: the
// attackers all declare and the blockers stay home so the 25-turn random
// games decide, the carrier is cast first, and Thought Scour is preferred
// when castable so both graveyards fill.
func ksbPolicy(seed uint64) Policy {
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
			// the carrier first; Thought Scour next so the sweep has a graveyard
			for _, want := range []string{"Strategic Betrayal", "Thought Scour"} {
				for _, a := range c.D.Options {
					if a.Kind() != ActCast || c.G == nil {
						continue
					}
					if o, ok := c.G.GorgeOption(a); ok && chName(c.G.E.G, o.Obj) == want {
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

func ksbSetup(t *testing.T) (*cards.Registry, cardset.Set) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("stratbetray")
	if err != nil {
		t.Fatal(err)
	}
	return reg, set
}

func TestStratBetrayLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	reg, set := ksbSetup(t)
	var decks [2][]*cards.Card
	for s := range decks {
		for _, e := range ksbDeck {
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
	tbl, _, _, err := set.Matchup(ksbDeck, ksbDeck)
	if err != nil {
		t.Fatal(err)
	}
	tally := &ksbTally{}
	games, divs, stalls := 0, 0, 0
	for seed := uint64(1); seed <= 128; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"ksb-a", "ksb-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		tally.reset(seed)
		res := Lockstep(m, seed, k, ksbPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40, Check: tally.check})
		CloseSide(k)
		games++
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
	t.Logf("%d games, %d divergences, %d stalls; Strategic Betrayal casts %d, picks %d, sweeps exiling both halves %d",
		games, divs, stalls, tally.casts, tally.picks, tally.sweeps)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	if tally.sweeps < 10 {
		t.Errorf("%d Strategic Betrayal sweeps exiled both a battlefield creature and graveyard cards, want at least 10", tally.sweeps)
	}
}
