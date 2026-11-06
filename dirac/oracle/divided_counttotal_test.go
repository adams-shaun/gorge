package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The value-total divided-damage lockstep: Ureni, the Song Unending's enters
// trigger is "deals X damage divided as you choose among any number of target
// creatures and/or planeswalkers your opponents control, where X is the number
// of lands you control" (NumDmg$ X, TargetMax$ X, DividedAsYouChoose$ X). The
// total is a value, read where gorge puts the trigger on the stack
// (effects.DividedTotal / rules.divisionTotalFor, CR 603.3d, 601.2d); the
// division is fixed there and paid with at least 1 per target at resolution.
//
// The deck ramps with elves so Ureni enters with five lands (five damage) and
// opposing creatures to divide it among; a zero-target run takes the done
// option and resolves with no damage. The lockstep is decision by decision:
// before the lowering the card is not in the pool at all, so this test cannot
// run against it.

const kdctUreni = "Ureni, the Song Unending"

var kdctDecks = [2][]cardset.Entry{{
	{Name: "Ureni, the Song Unending", Count: 4},
	{Name: "Llanowar Elves", Count: 8},
	{Name: "Fyndhorn Elves", Count: 8},
	{Name: "Grizzly Bears", Count: 6},
	{Name: "Forest", Count: 4},
	{Name: "Taiga", Count: 4},
	{Name: "Tropical Island", Count: 4},
	{Name: "Volcanic Island", Count: 2},
}, {
	{Name: "Ureni, the Song Unending", Count: 4},
	{Name: "Llanowar Elves", Count: 8},
	{Name: "Fyndhorn Elves", Count: 8},
	{Name: "Grizzly Bears", Count: 6},
	{Name: "Forest", Count: 4},
	{Name: "Taiga", Count: 4},
	{Name: "Tropical Island", Count: 4},
	{Name: "Volcanic Island", Count: 2},
}}

type kdctShape struct {
	lands   int
	targets int
}

type kdctStats struct {
	games, divs, pushes, divisions int
	gone                           int // Ureni trigger abilities that left the stack
	five, three, zero              int // five lands; five lands and three chosen targets; zero targets chosen
}

// kdctSourceName names a stack object by its source permanent (an ability
// object has no face of its own).
func kdctSourceName(g *state.Game, id state.ObjID) string {
	o := g.Obj(id)
	for i := 0; o != nil && o.Face() == nil && o.Source != 0 && i < 4; i++ {
		o = g.Obj(o.Source)
	}
	if o != nil && o.Face() != nil {
		return o.Face().Name
	}
	return ""
}

func kdctLands(g *state.Game, p state.PlayerID) int {
	n := 0
	for _, id := range g.Zone(state.ZBattlefield, p) {
		if o := g.Obj(id); o != nil && o.Face() != nil && o.Face().IsLand() {
			n++
		}
	}
	return n
}

func kdctInHand(g *state.Game, p state.PlayerID, name string) bool {
	for _, id := range g.Zone(state.ZHand, p) {
		if o := g.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name {
			return true
		}
	}
	return false
}

// kdctUreniShape is the Ureni enters-trigger on the stack, with the lands its
// controller has and the targets chosen so far.
func kdctUreniShape(g *state.Game, id state.ObjID) (kdctShape, bool) {
	o := g.Obj(id)
	if o == nil || o.Ability == nil || kdctSourceName(g, id) != kdctUreni {
		return kdctShape{}, false
	}
	return kdctShape{lands: kdctLands(g, o.Controller), targets: len(o.Targets)}, true
}

func (st *kdctStats) final(sh kdctShape) {
	st.gone++
	if sh.targets == 0 {
		st.zero++
	}
	if sh.lands == 5 {
		st.five++
		if sh.targets == 3 {
			st.three++
		}
	}
}

// kdctCheck records Ureni triggers, their lands and their final target count,
// and the division events gorge announces.
func kdctCheck(st *kdctStats) (func(*GorgeSide, Side) *Divergence, func(*GorgeSide)) {
	seen := map[state.ObjID]kdctShape{}
	evAt := 0
	var cur *GorgeSide
	flush := func() {
		for id, sh := range seen {
			st.final(sh)
			delete(seen, id)
		}
	}
	return func(ref *GorgeSide, _ Side) *Divergence {
		if ref != cur {
			flush()
			cur, evAt = ref, 0
			st.games++
		}
		g := ref.E.G
		on := map[state.ObjID]bool{}
		for _, id := range g.Stack {
			if sh, ok := kdctUreniShape(g, id); ok {
				seen[id], on[id] = sh, true
			}
		}
		for id, sh := range seen {
			if !on[id] {
				st.final(sh)
				delete(seen, id)
			}
		}
		evs := ref.E.L.Events
		if len(evs) < evAt {
			evAt = 0
		}
		for ; evAt < len(evs); evAt++ {
			e := &evs[evAt]
			switch e.Kind {
			case events.TriggerPush:
				if kdctSourceName(g, e.Obj) == kdctUreni {
					st.pushes++
				}
			case events.Choose:
				if e.Counter == events.DivisionCounter && kdctSourceName(g, e.Obj) == kdctUreni {
					st.divisions++
				}
			}
		}
		return nil
	}, func(*GorgeSide) { flush() }
}

// kdctPolicy plays a land (up to five once Ureni is in hand), ramps with elves,
// casts Ureni, and answers Ureni's target and division asks. Every fifth seed
// takes zero targets; the others take three.
func kdctPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	zero := seed%5 == 0
	picks := 0
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G == nil {
			return rnd.Pick(c)
		}
		g := c.G.E.G
		pd := c.G.Pending()
		if pd != nil && pd.Kind == decision.KTarget && kdctSourceName(g, pd.Source) == kdctUreni {
			if zero {
				for _, a := range c.D.Options {
					if a.Kind() == ActDone {
						return a
					}
				}
			}
			if picks < 3 {
				for _, a := range c.D.Options {
					if a.Kind() == ActTarget {
						picks++
						return a
					}
				}
			}
			for _, a := range c.D.Options {
				if a.Kind() == ActDone {
					return a
				}
			}
		}
		if pd != nil && pd.Kind == decision.KChoose && strings.HasPrefix(pd.Prompt, "Divide among") {
			return rnd.Pick(c)
		}
		var land, uren, elf, bear Canon
		var haveLand, haveUren, haveElf, haveBear bool
		for _, a := range c.D.Options {
			o, ok := c.G.GorgeOption(a)
			if !ok || o.Obj == 0 {
				continue
			}
			ob := g.Obj(o.Obj)
			if ob == nil || ob.Face() == nil {
				continue
			}
			switch {
			case a.Kind() == ActPlayLand:
				land, haveLand = a, true
			case a.Kind() == ActCast && ob.Face().Name == kdctUreni:
				uren, haveUren = a, true
			case a.Kind() == ActCast && (ob.Face().Name == "Llanowar Elves" || ob.Face().Name == "Fyndhorn Elves"):
				elf, haveElf = a, true
			case a.Kind() == ActCast && ob.Face().Name == "Grizzly Bears":
				bear, haveBear = a, true
			}
		}
		lands := kdctLands(g, pd.Player)
		if haveLand && (lands < 5 || !kdctInHand(g, pd.Player, kdctUreni)) {
			return land
		}
		switch {
		case haveUren:
			return uren
		case haveElf:
			return elf
		case haveBear:
			return bear
		case haveLand && lands < 8:
			return land
		}
		return rnd.Pick(c)
	})
}

func runDividedCountTotal(t *testing.T, seeds uint64, mutate func()) kdctStats {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("dividedcount")
	if err != nil {
		t.Skip(err) // no tables yet: make dirac-tables
	}
	var decks [2][]*cards.Card
	for s, d := range kdctDecks {
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
	tbl, _, _, err := set.Matchup(kdctDecks[0], kdctDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	if mutate != nil {
		mutate()
	}
	var st kdctStats
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kdct-a", "kdct-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		check, finish := kdctCheck(&st)
		res := Lockstep(m, seed, k, kdctPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 30, Check: check, Finish: finish})
		CloseSide(k)
		if d := res.Div; d != nil {
			st.divs++
			if mutate == nil {
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
		if strings.HasPrefix(res.Stall, KernelStallPrefix) && mutate == nil {
			t.Errorf("seed %d: stalled: %s", seed, res.Stall)
		}
	}
	return st
}

func TestDividedCountTotalLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	st := runDividedCountTotal(t, 80, nil)
	t.Logf("%d games, %d divergences; Ureni triggers %d (%d left the stack): %d with five lands, %d with five lands and three targets, %d with zero targets; %d divisions",
		st.games, st.divs, st.pushes, st.gone, st.five, st.three, st.zero, st.divisions)
	if st.divs != 0 {
		t.Fatalf("%d divergences", st.divs)
	}
	if st.three < 2 {
		t.Errorf("only %d Ureni triggers with five lands and three chosen targets", st.three)
	}
	if st.zero < 2 {
		t.Errorf("only %d Ureni triggers with zero chosen targets", st.zero)
	}
	if st.five < 2 {
		t.Errorf("only %d Ureni triggers with five lands", st.five)
	}
	if st.divisions < 2 {
		t.Errorf("only %d Ureni divisions announced", st.divisions)
	}
}
