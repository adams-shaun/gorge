package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The sneaked-mark lockstep (CR 702.190a): Karai, Future of the
// Foot (ConditionPresent$ Card.ThisTurnEntered+sneaked on its combat-damage
// trigger), Leonardo, Leader in Blue (ValidCard$ Card.Self+sneaked on its ETB)
// and Turncoat Kunoichi (ConditionPresent$ Card.sneaked on its ETB body) read
// a battlefield permanent's sneak provenance, which the kernel keeps as the
// arena's mechSneaked bit. Deck A (W/B) carries the three, played both from
// the hand and for the sneak cost (the {X} instants float the mana that makes
// gorge offer it, as in sneak_test.go); deck B is blockers and creatures for
// Kunoichi to exile.
var sneakMarkDecks = [2][]cardset.Entry{
	{
		{Name: "Karai, Future of the Foot", Count: 4}, {Name: "Leonardo, Leader in Blue", Count: 4},
		{Name: "Turncoat Kunoichi", Count: 4}, {Name: "Slice from the Shadows", Count: 10},
		{Name: "Savannah Lions", Count: 4},
		{Name: "Plains", Count: 7}, {Name: "Swamp", Count: 7},
	},
	{
		{Name: "Gnarlid Colony", Count: 6}, {Name: "Goblin Boarders", Count: 6},
		{Name: "Courageous Goblin", Count: 4}, {Name: "Fanatical Firebrand", Count: 4},
		{Name: "Torch Breath", Count: 4}, {Name: "Mountain", Count: 8}, {Name: "Forest", Count: 8},
	},
}

var sneakMarkCarriers = map[string]bool{
	"Karai, Future of the Foot": true, "Leonardo, Leader in Blue": true, "Turncoat Kunoichi": true,
}

// sneakMarkTally counts, from gorge's event log, the carriers that resolve
// onto the battlefield cast for their sneak cost and from the hand.
type sneakMarkTally struct {
	evAt    int
	sneaked map[state.ObjID]bool
	sneak   int
	hard    int
}

func (p *sneakMarkTally) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < p.evAt {
		p.evAt = 0
		p.sneaked = map[state.ObjID]bool{}
	}
	for j := p.evAt; j < len(evs); j++ {
		e := &evs[j]
		switch e.Kind {
		case events.CastInfo:
			for _, f := range strings.Split(e.Counter, ",") {
				if strings.TrimSpace(f) == "sneaked" {
					p.sneaked[e.Obj] = true
				}
			}
		case events.MoveZone:
			if e.From != state.ZStack {
				continue
			}
			sn := p.sneaked[e.Obj]
			delete(p.sneaked, e.Obj)
			if e.To != state.ZBattlefield {
				continue
			}
			if o := g.Obj(e.Obj); o != nil && o.Face() != nil && sneakMarkCarriers[o.Face().Name] {
				if sn {
					p.sneak++
				} else {
					p.hard++
				}
			}
		}
	}
	p.evAt = len(evs)
	return nil
}

func sneakMarkSetup(t *testing.T) (*cards.Registry, cardset.Set, [][2][]*cards.Card, []*kop.Table) {
	t.Helper()
	cardsettest.Require(t, "sneakmark")
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("sneakmark")
	if err != nil {
		t.Fatal(err)
	}
	matchups := [][2][]cardset.Entry{{sneakMarkDecks[0], sneakMarkDecks[1]}, {sneakMarkDecks[1], sneakMarkDecks[0]}}
	decks := make([][2][]*cards.Card, len(matchups))
	tbls := make([]*kop.Table, len(matchups))
	for mi, md := range matchups {
		for s, d := range md {
			for _, e := range d {
				c, ok := reg.Lookup(e.Name)
				if !ok {
					t.Fatalf("%s not in the corpus", e.Name)
				}
				for i := 0; i < e.Count; i++ {
					decks[mi][s] = append(decks[mi][s], c)
				}
			}
			if len(decks[mi][s]) != 40 {
				t.Fatalf("matchup %d deck %d: %d cards", mi, s, len(decks[mi][s]))
			}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		tbls[mi] = tbl
	}
	return reg, set, decks, tbls
}

func sneakMarkRun(t *testing.T, reg *cards.Registry, set cardset.Set, decks [2][]*cards.Card, tbl *kop.Table, seed uint64, check func(*GorgeSide, Side) *Divergence) Result {
	t.Helper()
	cfg := Config{GorgeAutopaySurface: seed%2 == 0}
	m, err := NewMatchup(reg, [2]string{"sneakmark-a", "sneakmark-b"}, decks, 1)
	if err != nil {
		t.Fatal(err)
	}
	k, err := SetSide(set, tbl, cfg)(m, seed)
	if err != nil {
		t.Fatal(err)
	}
	// sneakPolicy (sneak_test.go) is the random policy plus the float of a
	// small-X instant and the always-taken sneak offer; seed%4 == 0 is the
	// plain random policy
	pol := sneakPolicy(seed)
	if seed%4 == 0 {
		pol = RandomPolicy(seed)
	}
	if seed%3 == 1 {
		pol = sneakMarkHold(pol)
	}
	res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20, Check: check})
	CloseSide(k)
	return res
}

// sneakMarkHold keeps the carriers in hand: the policy greedily hard-casts a
// carrier the moment its cheap cost is payable, so without it the 4-5 mana a
// sneak cast needs is almost never still untapped in the declare-blockers step.
// The carrier is withheld from the policy's options (the hard cast stays legal
// and offered; this game's policy just does not pick it) whenever any other
// option remains. Other seeds hard-cast freely.
func sneakMarkHold(inner Policy) Policy {
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G == nil {
			return inner.Pick(c)
		}
		buf = buf[:0]
		for _, a := range c.D.Options {
			if a.Kind() == ActCast && int(a.Src()) < len(c.G.id.obj) {
				if o := c.G.E.G.Obj(c.G.id.obj[a.Src()]); o != nil && o.Face() != nil && sneakMarkCarriers[o.Face().Name] {
					continue
				}
			}
			buf = append(buf, a)
		}
		if len(buf) == 0 || len(buf) == len(c.D.Options) {
			return inner.Pick(c)
		}
		d := *c.D
		d.Options = buf
		return inner.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

func TestSneakmarkLockstep(t *testing.T) {
	t.Parallel()
	reg, set, decks, tbls := sneakMarkSetup(t)
	var p sneakMarkTally
	games, divs, stalls, capped := 0, 0, 0, 0
	for mi := range decks {
		for seed := uint64(1); seed <= 256; seed++ {
			p.evAt, p.sneaked = 0, map[state.ObjID]bool{}
			res := sneakMarkRun(t, reg, set, decks[mi], tbls[mi], seed, p.check)
			games++
			if res.Stall == "turns" {
				capped++ // the MaxTurns 20 cap, not a kernel stall
			} else if res.Stall != "" {
				stalls++
				t.Errorf("matchup %d seed %d: stalled: %s", mi, seed, res.Stall)
			}
			if d := res.Div; d != nil {
				divs++
				lo := d.At - 12
				if lo < 0 {
					lo = 0
				}
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	t.Logf("%d games: %d divergences, %d stalls (%d more reached the 20-turn cap); carriers resolved: %d cast for the sneak cost, %d hard cast", games, divs, stalls, capped, p.sneak, p.hard)
	if p.sneak < 15 {
		t.Errorf("%d sneak casts of the carriers resolved, want at least 15", p.sneak)
	}
	if p.hard < 15 {
		t.Errorf("%d hard casts of the carriers resolved, want at least 15", p.hard)
	}
}

// TestSneakmarkMutation: every spell's permanent marked sneaked (a hard-cast
// Leonardo pumps the team, a hard-cast Kunoichi exiles for good) must diverge
// the lockstep.
func TestSneakmarkMutation(t *testing.T) {
	reg, set, decks, tbls := sneakMarkSetup(t)
	dirac.DebugSneakMarkAll = true
	defer func() { dirac.DebugSneakMarkAll = false }()
	for seed := uint64(1); seed <= 64; seed++ {
		for mi := range decks {
			res := sneakMarkRun(t, reg, set, decks[mi], tbls[mi], seed, nil)
			if d := res.Div; d != nil {
				t.Logf("mutation diverged: matchup %d seed %d at decision %d: %s %s", mi, seed, d.At, d.What, d.Field)
				return
			}
		}
	}
	t.Fatal("DebugSneakMarkAll did not diverge the lockstep in 64 seeds")
}
