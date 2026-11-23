package oracle

import (
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// The graveyard mana lockstep (CR 605.1a, 605.2a): Jack-o'-Lantern's "{1},
// Exile Jack-o'-Lantern from your graveyard: Add one mana of any color" is a
// payment-window option only while a mana floats, listed after the battlefield
// and the hand sources, and the prefix search funds through it (dirac
// mana_gravemana.go). The Jack deck holds the fixture's self-mill (Stitcher's
// Supplier, Armored Skaab) to stock its graveyard with the card, Heap Gate
// ({1}, {T}: Add one mana of any color), whose live fee gap keeps the relaxed
// planner offering its wrong-colour casts (TestGraveManaGorgeProbe Q2b), and
// Lightning Bolts ({R}) its Islands and Swamps cannot pay: their payment
// windows tap {U}/{B} mana that floats unabsorbed, list Jack-o'-Lantern after
// the battlefield and take it (Q3); the plain deck is the exilegrave pod's.
func graveManaDecks() [2][]cardset.Entry {
	jack := []cardset.Entry{
		{Name: "Jack-o'-Lantern", Count: 8}, {Name: "Stitcher's Supplier", Count: 8}, {Name: "Armored Skaab", Count: 4},
		{Name: "Heap Gate", Count: 4}, {Name: "Mind Rot", Count: 4}, {Name: "Lightning Bolt", Count: 4},
		{Name: "Island", Count: 4}, {Name: "Swamp", Count: 4},
	}
	plain := []cardset.Entry{
		{Name: "Grizzly Bears", Count: 8}, {Name: "Hill Giant", Count: 4}, {Name: "Forest", Count: 14}, {Name: "Mountain", Count: 14},
	}
	return [2][]cardset.Entry{jack, plain}
}

type graveManaPod struct {
	reg   *cards.Registry
	set   cardset.Set
	decks [][]cardset.Entry
}

func newGraveManaPod(t *testing.T) graveManaPod {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	set, err := cardset.RawByName("gravemana")
	if err != nil {
		t.Fatal(err)
	}
	p := graveManaPod{reg: reg, set: set}
	for _, d := range graveManaDecks() {
		n := 0
		for _, e := range d {
			n += e.Count
		}
		if n != 40 {
			t.Fatalf("deck %s: %d cards", d[0].Name, n)
		}
		p.decks = append(p.decks, d)
	}
	return p
}

// graveManaTally counts, in gorge's log, the Jack-o'-Lantern cards exiled from
// a graveyard (the grave activation moves the card to exile; the fixture's
// only other exile from a graveyard is Jack's first ability, which may also
// exile a Jack lying in one, so an exile a ManaAdd follows within a few events
// is counted beside the raw count). scan reads the events a decision added
// (the log is rewound under a lockstep game, so the cursor resets when the log
// shrinks).
type graveManaTally struct {
	exiled, manaAfter int
	lastExile         int
	ref               *GorgeSide
	at                int
}

func (c *graveManaTally) scan(g *GorgeSide) *Divergence {
	evs := g.E.L.Events
	if c.ref != g || len(evs) < c.at {
		c.ref, c.at, c.lastExile = g, 0, -100
	}
	for i := c.at; i < len(evs); i++ {
		ev := &evs[i]
		if ev.Kind == events.MoveZone && ev.From == state.ZGraveyard && ev.To == state.ZExile {
			if o := g.E.G.Obj(ev.Obj); o != nil && o.Face() != nil && o.Face().Name == "Jack-o'-Lantern" {
				c.exiled++
				c.lastExile = i
			}
		}
		if ev.Kind == events.ManaAdd && i-c.lastExile <= 6 {
			c.manaAfter++
		}
	}
	c.at = len(evs)
	return nil
}

// play plays one seed on the full surface, the one that lists a play only a
// scripted prefix pays (the autopay surface drops such plain casts from both
// sides, gorge.go priorityVocabulary).
func (p graveManaPod) play(t *testing.T, seed uint64, maxTurns int32) (Result, *graveManaTally) {
	t.Helper()
	expand := func(d []cardset.Entry) []string {
		var out []string
		for _, e := range d {
			for j := 0; j < e.Count; j++ {
				out = append(out, e.Name)
			}
		}
		return out
	}
	r := &Repro{Format: reproFormat, Decks: [2]string{"x0", "x1"}, Cards: [2][]string{expand(p.decks[0]), expand(p.decks[1])},
		Seed: seed, PassElision: true, Surface: "full"}
	m, err := r.Matchup(p.reg)
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.MaxDecisions, rc.MaxTurns = 8000, maxTurns
	c := &graveManaTally{}
	rc.Check = func(g *GorgeSide, _ Side) *Divergence { return c.scan(g) }
	tbl, _, err := p.set.MatchupN(p.decks)
	if err != nil {
		t.Fatal(err)
	}
	k, err := SetSide(p.set, tbl, rc.Config)(m, seed)
	if err != nil {
		t.Fatalf("seed %d: %v", seed, err)
	}
	defer CloseSide(k)
	lead, err := NewGorgeSide(m, seed, rc.Config, nil)
	if err != nil {
		t.Fatal(err)
	}
	return LockstepLed(m, seed, lead, k, graveManaPolicy(seed), rc), c
}

// graveManaPolicy plays a land, else casts or activates a mana source, and
// takes Jack-o'-Lantern's grave activation whenever it is offered: at a
// payment window gorge lists it as a mana source once unabsorbed mana floats
// (its {1} fee is payable from it, TestGraveManaGorgeProbe Q3), and at raw
// priority only while a mana floats (Q1). The colour ask it answers at random.
func graveManaPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(c *PickCtx) Canon {
		buf := make([]Canon, 0, 8)
		acts := make([]Canon, 0, 8)
		for _, a := range c.D.Options {
			switch k := a.Kind(); k {
			case ActCast, ActCastFace, ActPlayLand:
				buf = append(buf, a)
			case ActActivate:
				acts = append(acts, a)
			}
			if c.G == nil {
				continue
			}
			o, ok := c.G.GorgeOption(a)
			if !ok {
				continue
			}
			ob := c.G.E.G.Obj(o.Obj)
			if ob == nil || ob.Face() == nil || ob.Face().Name != "Jack-o'-Lantern" {
				continue
			}
			if o.Kind == "mana" || ob.Zone == state.ZGraveyard {
				// the window's mana source (unabsorbed mana floats), or the
				// raw-priority activation (the fee is payable from the float)
				return a
			}
		}
		if len(buf) > 0 {
			d := *c.D
			d.Options = buf
			return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		}
		if len(acts) > 0 {
			d := *c.D
			d.Options = acts
			return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		}
		return rnd.Pick(c)
	})
}

// graveManaLockstep plays the pod's seeds; each, when set, requires every seed
// to have exiled a Jack-o'-Lantern from a graveyard (a pinned seed).
func graveManaLockstep(t *testing.T, seeds []uint64, each bool) graveManaTally {
	p := newGraveManaPod(t)
	var sum graveManaTally
	for _, seed := range seeds {
		res, c := p.play(t, seed, 40)
		if res.Div != nil {
			exileGraveFail(t, seed, res)
		}
		if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("seed %d: kernel stopped: %s", seed, res.Stall)
		}
		if each && c.exiled == 0 {
			t.Fatalf("seed %d: grave-exiled Jack-o'-Lantern cards 0: the pinned seed no longer exercises the activation", seed)
		}
		sum.exiled += c.exiled
		sum.manaAfter += c.manaAfter
		t.Logf("seed %d: %d decisions, over %v %s; Jack-o'-Lantern grave exiles %d, a mana add after one %d", seed, res.Decisions, res.Over, res.Stall, c.exiled, c.manaAfter)
	}
	return sum
}

// TestGraveManaTwoSeatLockstep plays the pod's two decks on pinned seeds: no
// divergence, no stall, and a Jack-o'-Lantern paid its grave exile (the
// activation ran in gorge and the kernel's window and funding pass modelled
// it). The pinned seeds are the census's clean, activating ones; the pool's
// other seeds carry a diverging option class beside Heap Gate's fee gap (the
// kernel's mirror offers casts gorge's planner does not), a separate ticket.
func TestGraveManaTwoSeatLockstep(t *testing.T) {
	// Seed 2 pins the fee-gap offer class this file's ticket fixed: its
	// turn-5 priority holds an untapped Heap Gate beside one untapped Swamp
	// and an empty pool, where the bound's {B} funds the filter's {1} fee and
	// both engines offer the casts it pays. Seed 16 pins the fee's spend
	// order: its turn-5 payment holds a {C} and a {U} floating, and gorge's
	// generic spend takes the {C} first (graveManaFee); a W-first pick leaves
	// the pools unequal at decision 42.
	if sum := graveManaLockstep(t, []uint64{1, 2, 7, 8, 13, 16}, true); sum.manaAfter == 0 {
		t.Fatalf("grave-exiled Jack-o'-Lantern cards %d, no mana add after one: the activation was not exercised", sum.exiled)
	}
}

// TestGraveManaFourSeatLockstep asserts the refusal at four seats, no play:
// the grave mana source's window and funding pass play in a two-seat game, so
// the lowered Jack-o'-Lantern is pod-limited (kop.CardTwoSeat, StallTwoSeat
// refuses a table a pod of three or more seats holds it in) and
// `kgen -supported gravemana -four-seat`, whose predicate this assertion is,
// reports the card false.
func TestGraveManaFourSeatLockstep(t *testing.T) {
	// The pool table is built output (make dirac-tables): a clone without it
	// skips, like the corpus lockstep above skips without a corpus
	// (pr-precheck's fresh-clone step).
	if _, err := cardset.TablesDir(); err != nil {
		t.Skipf("no kernel tables: %v", err)
	}
	set, err := cardset.RawByName("gravemana")
	if err != nil {
		t.Fatal(err)
	}
	i := slices.Index(set.Names(), "Jack-o'-Lantern")
	if i < 0 {
		t.Fatal("Jack-o'-Lantern is not in the gravemana pool's table")
	}
	d := set.Def(i)
	if !kop.CardTwoSeat(d) && !kop.CardPodLimited(d) {
		t.Errorf("Jack-o'-Lantern: not two-seat or pod-limited: a four-seat pod would hold it and its window, unverified past two seats, would play")
	}
}
