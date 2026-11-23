package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Thornvault Forager's "{T}, Forage: Add two mana in any combination of colors."
// gorge pays the forage in its mana-activation walk (the forage_exile / forage_food
// ask over pay.ForageOptions, asked only with two or more options) and its planner
// prices neither the forage nor the combination, so the source is a Combo Any census
// gap: a play only its mana pays is found by the prefix search, which answers the
// forage first and the colours after. The kernel models it as a forage variant of the
// Combo Any gap (dirac/mana_forage.go). The decks play the foragemana fixture: the
// Forager deck stocks its graveyard with self-mill creatures (Stitcher's Supplier,
// Armored Skaab), makes Foods (Fierce Witchstalker, Eager Trufflesnout, Curious Pair)
// and casts spells (Hill Giant, Craw Wurm) and an activation (Jayemdae Tome's {4},
// {T}) the extra mana pays, against a plain deck.
var forageManaDecks = [2][]cardset.Entry{
	{
		{Name: "Thornvault Forager", Count: 6}, {Name: "Fierce Witchstalker", Count: 3}, {Name: "Eager Trufflesnout", Count: 2},
		{Name: "Curious Pair", Count: 3}, {Name: "Stitcher's Supplier", Count: 4}, {Name: "Armored Skaab", Count: 2},
		{Name: "Hill Giant", Count: 4}, {Name: "Craw Wurm", Count: 3}, {Name: "Jayemdae Tome", Count: 2},
		{Name: "Forest", Count: 6}, {Name: "Swamp", Count: 2}, {Name: "Mountain", Count: 2}, {Name: "Island", Count: 1},
	},
	{
		{Name: "Grizzly Bears", Count: 10}, {Name: "Hill Giant", Count: 8}, {Name: "Forest", Count: 11}, {Name: "Mountain", Count: 11},
	},
}

// forageFoodDecks is the Food-rich variant: eight Foragers on ten lands, Treats to Share (Curious Pair's
// adventure) and Fierce Witchstalker for the Foods, so a forage often has both arms live (the graveyard
// holds three cards and a Food is out), the one case the forage ask poses two options.
var forageFoodDecks = [2][]cardset.Entry{
	{
		{Name: "Thornvault Forager", Count: 8}, {Name: "Curious Pair", Count: 6}, {Name: "Fierce Witchstalker", Count: 4},
		{Name: "Stitcher's Supplier", Count: 4}, {Name: "Armored Skaab", Count: 2}, {Name: "Hill Giant", Count: 3}, {Name: "Craw Wurm", Count: 3},
		{Name: "Forest", Count: 7}, {Name: "Swamp", Count: 1}, {Name: "Mountain", Count: 1}, {Name: "Island", Count: 1},
	},
	forageManaDecks[1],
}

type foragePod struct {
	reg   *cards.Registry
	set   cardset.Set
	decks [2][]cardset.Entry
}

func newForagePod(t *testing.T, decks [2][]cardset.Entry) foragePod {
	t.Helper()
	set, err := cardset.RawByName("foragemana")
	if err != nil {
		t.Fatal(err)
	}
	for i, d := range decks {
		n := 0
		for _, e := range d {
			n += e.Count
		}
		if n != 40 {
			t.Fatalf("deck %d: %d cards", i, n)
		}
	}
	return foragePod{reg: testutil.CorpusRegistry(t), set: set, decks: decks}
}

// forageTally counts, in gorge's log, the paid forages (the fixture's only forage is the
// Forager's mana ability): the exile arm (three cards exiled as a forage cost) and the
// sacrifice arm (a Food), and the Forager's mana activations beside them.
type forageTally struct {
	exiled, sacrificed int
	ref                *GorgeSide
	at                 int
}

func (c *forageTally) scan(g *GorgeSide) *Divergence {
	evs := g.E.L.Events
	if c.ref != g || len(evs) < c.at {
		c.ref, c.at = g, 0 // the log is rewound under a lockstep game
	}
	for i := c.at; i < len(evs); i++ {
		if evs[i].Kind != events.ForageAction {
			continue
		}
		if i > 0 && evs[i-1].Kind == events.MoveZone && evs[i-1].Text == "exiled as a forage cost" {
			c.exiled++
		} else {
			c.sacrificed++
		}
	}
	c.at = len(evs)
	return nil
}

func (c forageTally) paid() int { return c.exiled + c.sacrificed }

// play plays one seed of the two decks on the full surface, the one that lists a play
// only a scripted prefix pays (the autopay surface drops such plain casts from both
// sides, gorge.go priorityVocabulary).
func (p foragePod) play(t *testing.T, seed uint64, maxTurns int32) (Result, *forageTally) {
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
	c := &forageTally{}
	rc.Check = func(g *GorgeSide, _ Side) *Divergence { return c.scan(g) }
	tbl, _, _, err := p.set.Matchup(p.decks[0], p.decks[1])
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
	return LockstepLed(m, seed, lead, k, forageManaPolicy(seed), rc), c
}

// forageManaPolicy plays a land, else casts, and leaves the other activations (the
// Forager's tutor, Foods) to the fallback's chance.
func forageManaPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		buf = buf[:0]
		land := false
		for _, a := range c.D.Options {
			k := a.Kind()
			if k == ActPlayLand {
				land = true
			}
			if k == ActCast || k == ActCastFace || k == ActPlayLand {
				buf = append(buf, a)
			}
		}
		if len(buf) == 0 {
			return rnd.Pick(c)
		}
		d := *c.D
		d.Options = buf
		if land {
			d.Options = d.Options[:0]
			for _, a := range buf {
				if a.Kind() == ActPlayLand {
					d.Options = append(d.Options, a)
				}
			}
		}
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

func forageManaFail(t *testing.T, seed uint64, res Result) {
	t.Helper()
	lo := max(res.Div.At-12, 0)
	t.Fatalf("seed %d: divergence at %d: %s %s %v stall=%q over=%v\nref %q\nker %q\n%s last %s\nlast actions %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Stall, res.Over,
		res.Div.RefOptions, res.Div.KerOptions, res.Div.Detail, res.Div.LastSourced, actionStrings(res.Actions[lo:res.Div.At]))
}

// forageManaSeeds are the pinned seeds of each deck pair, the Forager deck of the first pair stocking its
// graveyard (the exile arm: 3, 10, 20; 33 and 45 both arms: 10 funds a tutor with another Forager's forage, 45 a second forage after a Food was sacrificed) and the Food-rich one of the second beside Foods (6, 15 and 22 forage
// with both arms live): each pays at least one forage, measured.
var (
	forageManaSeeds     = []uint64{3, 10, 20, 33, 45}
	forageFoodSeeds     = []uint64{6, 15, 22}
	forageMutationSeeds = []uint64{6, 15}
)

// forageManaLockstep plays seeds of the pod's decks; each, when set, requires every seed to have
// paid a forage (a pinned seed).
func forageManaLockstep(t *testing.T, p foragePod, seeds []uint64, each bool) forageTally {
	var sum forageTally
	for _, seed := range seeds {
		res, c := p.play(t, seed, 40)
		if res.Div != nil {
			forageManaFail(t, seed, res)
		}
		if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("seed %d: kernel stopped: %s", seed, res.Stall)
		}
		if each && c.paid() == 0 {
			t.Fatalf("seed %d: forage-paid activations 0: the pinned seed no longer exercises the activation", seed)
		}
		sum.exiled += c.exiled
		sum.sacrificed += c.sacrificed
		t.Logf("seed %d: %d decisions, over %v %s; forages paid: %d by exile, %d by Food", seed, res.Decisions, res.Over, res.Stall, c.exiled, c.sacrificed)
	}
	return sum
}

// TestForageManaTwoSeatLockstep plays the fixture's two deck pairs on their pinned seeds: no divergence and no
// stall, every seed pays a forage, and both arms (exile three, sacrifice a Food) are paid over them.
func TestForageManaTwoSeatLockstep(t *testing.T) {
	a := forageManaLockstep(t, newForagePod(t, forageManaDecks), forageManaSeeds, true)
	b := forageManaLockstep(t, newForagePod(t, forageFoodDecks), forageFoodSeeds, true)
	if a.exiled+b.exiled == 0 || a.sacrificed+b.sacrificed == 0 {
		t.Fatalf("forages paid: %d by exile, %d by Food: want both arms exercised", a.exiled+b.exiled, a.sacrificed+b.sacrificed)
	}
}

// TestForageManaPickMutation: answering the forage ask with the last option first (DebugForagePickLast) pays a
// Food where gorge exiles three cards (the ask poses both once the graveyard holds three cards and a Food is out),
// so each pinned seed diverges with it set and is clean without it.
func TestForageManaPickMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	p := newForagePod(t, forageFoodDecks)
	forageManaLockstep(t, p, forageMutationSeeds, true) // the control: the unmutated kernel is clean on the same seeds
	dirac.DebugForagePickLast = true
	defer func() { dirac.DebugForagePickLast = false }()
	for _, seed := range forageMutationSeeds {
		res, _ := p.play(t, seed, 40)
		if res.Div == nil {
			t.Fatalf("seed %d: answering the forage ask with the last option diverged on no decision", seed)
		}
		t.Logf("seed %d: mutation diverged at %d: %s %s", seed, res.Div.At, res.Div.What, res.Div.Field)
	}
}
