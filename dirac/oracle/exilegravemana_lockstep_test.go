package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Molt Tender's "{T}, Exile a card from your graveyard: Add one mana of any
// color." gorge pays the exile in its mana-activation walk (the "mana_exile"
// pick over the payer's graveyard, asked even with one candidate) and its
// planner prices no exile part, so the source is a census gap: a play only its
// mana pays is found by the prefix search, and the manual cast window lists it.
// The kernel models it as a source kind of the tap-another search
// (dirac/mana_exilegrave.go). The pods play the exilegravemana fixture: the
// Molt Tender decks stock their graveyards with self-mill creatures (Stitcher's
// Supplier, Armored Skaab) and cast spells (Hill Giant, Craw Wurm) and an activation
// (Jayemdae Tome's {4}, {T}) the extra mana pays, against plain decks.
func exileGraveManaDecks() [4][]cardset.Entry {
	tender := []cardset.Entry{
		{Name: "Molt Tender", Count: 6}, {Name: "Stitcher's Supplier", Count: 5}, {Name: "Armored Skaab", Count: 2},
		{Name: "Hill Giant", Count: 6}, {Name: "Craw Wurm", Count: 2}, {Name: "Jayemdae Tome", Count: 3},
		{Name: "Forest", Count: 7}, {Name: "Swamp", Count: 4}, {Name: "Mountain", Count: 3}, {Name: "Island", Count: 2},
	}
	plain := []cardset.Entry{
		{Name: "Grizzly Bears", Count: 10}, {Name: "Hill Giant", Count: 8}, {Name: "Forest", Count: 11}, {Name: "Mountain", Count: 11},
	}
	// the second Molt Tender deck swaps its Mountains and Islands for filter lands: the exile source beside a filter gap
	// runs the filter search's branch (mana_mixgaps.go tapXGap)
	filt := []cardset.Entry{
		{Name: "Molt Tender", Count: 6}, {Name: "Stitcher's Supplier", Count: 5}, {Name: "Armored Skaab", Count: 2},
		{Name: "Hill Giant", Count: 6}, {Name: "Craw Wurm", Count: 2}, {Name: "Jayemdae Tome", Count: 3},
		{Name: "Forest", Count: 7}, {Name: "Swamp", Count: 4}, {Name: "Cascade Bluffs", Count: 5},
	}
	return [4][]cardset.Entry{tender, plain, filt, plain}
}

type exileGravePod struct {
	reg   *cards.Registry
	set   cardset.Set
	decks [][]cardset.Entry
}

func newExileGravePod(t *testing.T, seats int) exileGravePod {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	set, err := cardset.RawByName("exilegravemana")
	if err != nil {
		t.Fatal(err)
	}
	all := exileGraveManaDecks()
	p := exileGravePod{reg: reg, set: set}
	for i := 0; i < seats; i++ {
		n := 0
		for _, e := range all[i] {
			n += e.Count
		}
		if n != 40 {
			t.Fatalf("deck %d: %d cards", i, n)
		}
		p.decks = append(p.decks, all[i])
	}
	return p
}

// exileGraveTally counts, in gorge's log, the Molt Tender activations that
// exiled a graveyard card (the fixture's only exile from a graveyard). scan reads the events a decision added (the log is
// rewound under a lockstep game, so the cursor resets when the log shrinks).
type exileGraveTally struct {
	tapped, exiled int
	tomePaid       int // Jayemdae Tome activations pushed within a few events of an exile (the activation window's route)
	lastExile      int
	ref            *GorgeSide
	at             int
}

func (c *exileGraveTally) scan(g *GorgeSide) *Divergence {
	evs := g.E.L.Events
	if c.ref != g || len(evs) < c.at {
		c.ref, c.at, c.lastExile = g, 0, -100
	}
	for i := c.at; i < len(evs); i++ {
		ev := &evs[i]
		if ev.Kind == events.Tap {
			if o := g.E.G.Obj(ev.Obj); o != nil && o.Face() != nil && o.Face().Name == "Molt Tender" {
				c.tapped++
			}
		}
		if ev.Kind == events.MoveZone && ev.From == state.ZGraveyard && ev.To == state.ZExile {
			c.exiled++ // (the fixture has no other exile from a graveyard)
			c.lastExile = i
		}
		if ev.Kind == events.AbilityPush && i-c.lastExile < 10 {
			if o := g.E.G.Obj(ev.Obj); o != nil && o.Face() != nil && o.Face().Name == "Jayemdae Tome" {
				c.tomePaid++
			}
		}
	}
	c.at = len(evs)
	return nil
}

// play plays one seed (four seats with four decks, two with two) on the full
// surface, the one that lists a play only a scripted prefix pays (the autopay
// surface drops such plain casts from both sides, gorge.go priorityVocabulary).
func (p exileGravePod) play(t *testing.T, seed uint64, maxTurns int32) (Result, *exileGraveTally) {
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
	if len(p.decks) == 4 {
		r.MoreDecks = []string{"x2", "x3"}
		r.MoreCards = [][]string{expand(p.decks[2]), expand(p.decks[3])}
	}
	m, err := r.Matchup(p.reg)
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.MaxDecisions, rc.MaxTurns = 8000, maxTurns
	c := &exileGraveTally{}
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
	return LockstepLed(m, seed, lead, k, exileGravePolicy(seed), rc), c
}

// exileGravePolicy plays a land, else casts, and leaves Molt Tender's mill
// ability (a tap that spends the mana ability's {T}) to the fallback's chance.
func exileGravePolicy(seed uint64) Policy {
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

func exileGraveFail(t *testing.T, seed uint64, res Result) {
	t.Helper()
	lo := max(res.Div.At-12, 0)
	t.Fatalf("seed %d: divergence at %d: %s %s %v stall=%q over=%v\nref %q\nker %q\n%s last %s\nlast actions %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Stall, res.Over,
		res.Div.RefOptions, res.Div.KerOptions, res.Div.Detail, res.Div.LastSourced, actionStrings(res.Actions[lo:res.Div.At]))
}

var exileGraveSeeds = []uint64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}

// exileGraveLockstep plays seeds of the pod's first seats decks; each, when set,
// requires every seed to have paid a Molt Tender exile (a pinned seed).
func exileGraveLockstep(t *testing.T, seats int, seeds []uint64, each bool) exileGraveTally {
	p := newExileGravePod(t, seats)
	var sum exileGraveTally
	for _, seed := range seeds {
		res, c := p.play(t, seed, 40)
		if res.Div != nil {
			exileGraveFail(t, seed, res)
		}
		if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("seed %d: kernel stopped: %s", seed, res.Stall)
		}
		if each && c.exiled == 0 {
			t.Fatalf("seed %d: Molt Tender taps %d, exile-paid activations 0: the pinned seed no longer exercises the activation", seed, c.tapped)
		}
		sum.tapped += c.tapped
		sum.exiled += c.exiled
		sum.tomePaid += c.tomePaid
		t.Logf("seed %d: %d decisions, over %v %s; Molt Tender taps %d, exile-paid %d, Tome activations after one %d", seed, res.Decisions, res.Over, res.Stall, c.tapped, c.exiled, c.tomePaid)
	}
	return sum
}

// TestExileGraveManaFourSeatLockstep plays four-seat pods of the fixture (one
// with filter lands): no divergence and no stall, and Molt Tender paid its exile.
func TestExileGraveManaFourSeatLockstep(t *testing.T) {
	if sum := exileGraveLockstep(t, 4, exileGraveSeeds, false); sum.exiled == 0 {
		t.Fatalf("Molt Tender taps %d, exile-paid activations %d: the activation was not exercised", sum.tapped, sum.exiled)
	}
}

// TestExileGraveManaTwoSeatLockstep is the same over the first two decks, on two
// pinned seeds: seed 15 pays three exiles (the scripted prefix and the cast
// window), seed 10 funds a Jayemdae Tome activation with one (the activation window).
func TestExileGraveManaTwoSeatLockstep(t *testing.T) {
	if sum := exileGraveLockstep(t, 2, []uint64{10, 15}, true); sum.tomePaid == 0 {
		t.Fatalf("no Jayemdae Tome activation followed a Molt Tender exile: the activation window was not exercised")
	}
}

// TestExileGraveManaPickMutation: answering the exile pick with the last
// graveyard card first (DebugExileGravePickLast) exiles another card than
// gorge's first, so the games must diverge.
func TestExileGraveManaPickMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	p := newExileGravePod(t, 4)
	dirac.DebugExileGravePickLast = true
	defer func() { dirac.DebugExileGravePickLast = false }()
	for _, seed := range exileGraveSeeds {
		if res, _ := p.play(t, seed, 40); res.Div != nil {
			t.Logf("mutation diverged at %d: %s %s", res.Div.At, res.Div.What, res.Div.Field)
			return
		}
	}
	t.Fatal("answering the exile pick with the last graveyard card diverged on no seed")
}
