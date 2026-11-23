package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// raniInsubDecks is the raniinsub fixture's two decks: seat 0 carries The
// Rani and Insubordination (with the mana for both, Rani 1UR, Insubordination
// BB), seat 1 its own The Rani copies (the shared bearer of the ticket's pool
// game: an Aura on the opponent's The Rani, the attacking Rani's Mark of the
// Rani token attaching over it) and Grizzly Bears as fallback targets. Each
// deck is 40.
func raniInsubDecks() [2][]cardset.Entry {
	return [2][]cardset.Entry{
		{
			{Name: "The Rani", Count: 4}, {Name: "Insubordination", Count: 4},
			{Name: "Grizzly Bears", Count: 4}, {Name: "Mountain", Count: 7},
			{Name: "Island", Count: 7}, {Name: "Swamp", Count: 7}, {Name: "Forest", Count: 7},
		},
		{
			{Name: "The Rani", Count: 2}, {Name: "Grizzly Bears", Count: 14},
			{Name: "Mountain", Count: 8}, {Name: "Island", Count: 8}, {Name: "Swamp", Count: 8},
		},
	}
}

// raniInsubPod is the fixture set and its two-seat table.
type raniInsubPod struct {
	reg   *cards.Registry
	set   cardset.Set
	tbl   *kop.Table
	decks [2][]cardset.Entry
}

// raniInsubSteering steers the shared-bearer situation: seat 0 casts
// Insubordination onto a seat-1 permanent and then The Rani, attacks with The
// Rani, and points every Rani trigger's "another target creature" ask at the
// creature Insubordination is attached to, so the Mark of the Rani token
// attaches over an existing Aura. Seat 1 never attacks or blocks, so the
// shared bearer survives. Everything else falls back to castFirstPolicy.
func raniInsubSteering(seed uint64) Policy {
	base := castFirstPolicy(seed)
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G == nil || c.G.E == nil {
			return base.Pick(c)
		}
		g := c.G.E.G
		cast := func(name string) (Canon, bool) {
			for _, o := range c.D.Options {
				if o.Kind() != ActCast && o.Kind() != ActCastFace {
					continue
				}
				if opt, ok := c.G.GorgeOption(o); ok {
					if ob := g.Obj(opt.Obj); ob != nil && ob.Face() != nil && ob.Face().Name == name {
						return o, true
					}
				}
			}
			return 0, false
		}
		insubHost := func() state.ObjID {
			for i := range g.Objs {
				ob := &g.Objs[i]
				if ob.Face() == nil || ob.Face().Name != "Insubordination" || ob.Zone != state.ZBattlefield {
					continue
				}
				return ob.AttachedTo
			}
			return 0
		}
		oppTarget := func(needOpp bool) (Canon, bool) {
			// the ticket's shared bearer is the opponent's The Rani; a
			// seat-1 bear is the fallback
			for _, name := range []string{"The Rani", "Grizzly Bears"} {
				for _, o := range c.D.Options {
					if o.Kind() != ActTarget {
						continue
					}
					opt, ok := c.G.GorgeOption(o)
					if !ok {
						continue
					}
					ob := g.Obj(opt.Obj)
					if ob == nil || ob.Face() == nil || ob.Face().Name != name {
						continue
					}
					if !needOpp || ob.Controller != 0 {
						return o, true
					}
				}
			}
			return 0, false
		}
		pass := func() Canon {
			// a multi-pick combat decision's no-op is ActDone (an empty
			// commit), priority's is ActPass
			for _, k := range []CanonKind{ActPass, ActDone} {
				for _, o := range c.D.Options {
					if o.Kind() == k {
						return o
					}
				}
			}
			return c.D.Options[0]
		}
		switch c.D.Kind {
		case DPriority:
			// Insubordination goes down first, onto a seat-1 permanent (the
			// shared bearer), so The Rani's enter and attack triggers ask
			// with the Aura already attached; then The Rani
			if _, ok := oppTarget(true); ok {
				if o, ok := cast("Insubordination"); ok {
					return o
				}
			}
			if o, ok := cast("The Rani"); ok {
				return o
			}
		case DTarget:
			d := c.G.Pending()
			if d == nil || d.Source == 0 {
				return base.Pick(c)
			}
			src := g.Obj(d.Source)
			if src == nil {
				return base.Pick(c)
			}
			// The Rani trigger's ask is posed by the trigger instance, its
			// SA's own TgtPrompt; Insubordination's is the spell on the stack
			switch {
			case src.Face() != nil && src.Face().Name == "Insubordination":
				if o, ok := oppTarget(true); ok {
					return o
				}
			case strings.HasPrefix(d.Prompt, "Choose a target for The Rani"):
				if host := insubHost(); host != 0 {
					for _, o := range c.D.Options {
						if o.Kind() != ActTarget {
							continue
						}
						if opt, ok := c.G.GorgeOption(o); ok && opt.Obj == host {
							return o
						}
					}
				}
				if o, ok := oppTarget(false); ok {
					return o
				}
			}
		case DAttackers:
			if c.D.Player != 0 {
				return pass()
			}
			for _, o := range c.D.Options {
				if o.Kind() != ActAttack {
					continue
				}
				if opt, ok := c.G.GorgeOption(o); ok {
					if ob := g.Obj(opt.Obj); ob != nil && ob.Face() != nil && ob.Face().Name == "The Rani" {
						return o
					}
				}
			}
			return pass()
		case DBlockers:
			if c.D.Player == 1 {
				// chump-block with a bear (the shared bearer, seat 1's The
				// Rani, stays home): bears die, the game runs long, and the
				// bearer's auras sit through many decisions
				for _, o := range c.D.Options {
					if o.Kind() != ActBlock {
						continue
					}
					if opt, ok := c.G.GorgeOption(o); ok {
						if ob := g.Obj(opt.Obj); ob != nil && ob.Face() != nil && ob.Face().Name == "Grizzly Bears" {
							return o
						}
					}
				}
				return pass()
			}
		}
		return base.Pick(c)
	})
}

// raniInsubCounts is what a finished gorge log shows the steering reached.
type raniInsubCounts struct {
	sharedAttaches int // Mark of the Rani tokens attached over Insubordination
}

func countRaniInsubEvents(g *GorgeSide) (c raniInsubCounts) {
	hosts := make(map[state.ObjID]bool)
	for _, e := range g.E.L.Events {
		if e.Kind != events.Attach || len(e.IDs) == 0 {
			continue
		}
		if ob := g.E.G.Obj(e.Obj); ob != nil && ob.Face() != nil {
			if ob.Face().Name == "Insubordination" {
				hosts[e.IDs[0]] = true
			} else if ob.Face().Name == "Mark of the Rani" && hosts[e.IDs[0]] {
				c.sharedAttaches++
			}
		}
	}
	return c
}

func (p raniInsubPod) play(t *testing.T, seed uint64) (Result, raniInsubCounts) {
	t.Helper()
	r := &Repro{Format: reproFormat, Decks: [2]string{"x0", "x1"},
		Cards: [2][]string{expandCombatantsGoadDeck(p.decks[0]), expandCombatantsGoadDeck(p.decks[1])},
		Seed:  seed, PassElision: true, Surface: "full"}
	m, err := r.Matchup(p.reg)
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.MaxDecisions, rc.MaxTurns = 8000, 40
	var c raniInsubCounts
	rc.Finish = func(g *GorgeSide) { c = countRaniInsubEvents(g) }
	k, err := SetSide(p.set, p.tbl, rc.Config)(m, seed)
	if err != nil {
		t.Fatalf("seed %d: %v", seed, err)
	}
	defer CloseSide(k)
	lead, err := NewGorgeSide(m, seed, rc.Config, nil)
	if err != nil {
		t.Fatal(err)
	}
	res := LockstepLed(m, seed, lead, k, raniInsubSteering(seed), rc)
	return res, c
}

func raniInsubFail(t *testing.T, seed uint64, res Result) {
	t.Helper()
	lo := max(res.Div.At-12, 0)
	t.Fatalf("seed %d: divergence at %d: %s %s %v stall=%q over=%v\nref %q\nker %q\n%s last %s\nlast actions %v",
		seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Stall, res.Over,
		res.Div.RefOptions, res.Div.KerOptions, res.Div.Detail, res.Div.LastSourced, actionStrings(res.Actions[lo:res.Div.At]))
}

// TestRaniInsubAttachLockstep is the two-seat lockstep of the Mark of the
// Rani token attaching over an existing Aura, the ticket's shared-bearer
// shape (Insubordination on the opponent's The Rani, the attacking Rani's
// token targeting that same creature): no divergence at any seed, and the
// steering reached the shared bearer (a Mark attached to the creature
// Insubordination is attached to) in at least one game.
func TestRaniInsubAttachLockstep(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	set, err := cardset.RawByName("raniinsub")
	if err != nil {
		t.Fatal(err)
	}
	decks := raniInsubDecks()
	lists := make([][]cardset.Entry, 2)
	for i, d := range decks {
		n := 0
		for _, e := range d {
			n += e.Count
		}
		if n != 40 {
			t.Fatalf("deck %d: %d cards", i, n)
		}
		lists[i] = d
	}
	tbl, _, err := set.MatchupN(lists)
	if err != nil {
		t.Fatal(err)
	}
	p := raniInsubPod{reg: reg, set: set, tbl: tbl, decks: decks}
	var shared int
	for seed := uint64(1); seed <= 40; seed++ {
		res, c := p.play(t, seed)
		if res.Div != nil {
			raniInsubFail(t, seed, res)
		}
		if res.StallUnmodelled {
			t.Fatalf("seed %d: unmodelled stall %s", seed, res.Stall)
		}
		shared += c.sharedAttaches
		t.Logf("seed %d: %d decisions over %v %s; %d shared attach(es)", seed, res.Decisions, res.Over, res.Stall, c.sharedAttaches)
	}
	if shared == 0 {
		t.Fatal("no Mark of the Rani token attached over Insubordination: the steering missed the situation")
	}
}
