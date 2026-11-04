package oracle

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// CollectEvidence<N> as a spell's OptionalCost part
// (CR 701.59a, 601.2b): Analyze the Pollen (8), Extract a Confession (6) and
// Behind the Mask (6). gorge offers the optional cast whatever the graveyard
// holds and poses the evidence ask after the targets (an unreachable N
// aborts the cast there); the spell reads "Collected" / "Collected$Amount"
// through the cost moves in its own window, which the kernel lowers to the
// kicked bit. Counterspell and Shock respond between the cast and the
// resolution (the case gorge anchored on the spell's own push).

// gorge's planner prices no OptionalCost card ("shape:optional_cost"), so a
// carrier's casts are offered only while the floating pool pays them (the
// enters-mana creatures: Iridescent Tiger's WUBRG, Burning-Tree Emissary's
// RG, Priest of Gix's BBB); the policy casts the carrier right after.
var koptevidDecks = [2][]cardset.Entry{
	{
		{Name: "Analyze the Pollen", Count: 4}, {Name: "Extract a Confession", Count: 4},
		{Name: "Thought Scour", Count: 4}, {Name: "Careful Study", Count: 4},
		{Name: "Iridescent Tiger", Count: 4}, {Name: "Burning-Tree Emissary", Count: 4}, {Name: "Priest of Gix", Count: 3},
		{Name: "Hill Giant", Count: 1}, {Name: "Forest", Count: 5}, {Name: "Swamp", Count: 4}, {Name: "Mountain", Count: 3},
	},
	{
		{Name: "Behind the Mask", Count: 4}, {Name: "Counterspell", Count: 3}, {Name: "Shock", Count: 4},
		{Name: "Thought Scour", Count: 3}, {Name: "Iridescent Tiger", Count: 4}, {Name: "Hill Giant", Count: 4},
		{Name: "Island", Count: 9}, {Name: "Mountain", Count: 9},
	},
}

var koptevidCarriers = map[string]bool{"Analyze the Pollen": true, "Extract a Confession": true, "Behind the Mask": true}

// koptevidStats is what the gorge-side event log says the games did.
type koptevidStats struct {
	collected int // optional casts that collected evidence (CollectEvidenceAction)
	resolved  int // ... of which resolved
	responded int // ... of which resolved after another object went on the stack
	plain     int // carrier spells that resolved without evidence
	offered   int // priority windows that offered an optional (kicked) cast
}

// koptevidTally folds the gorge event log into the stats.
type koptevidTally struct {
	evAt  int
	stack []state.ObjID // spells pushed and not yet resolved, bottom first
	live  map[state.ObjID]bool
	resp  map[state.ObjID]bool
	// evStep: evidence was collected since the last step change, turn change
	// or pool clear (see koptevidPolicy)
	evStep bool
}

func (p *koptevidTally) scan(ref *GorgeSide, st *koptevidStats) {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < p.evAt {
		p.evAt = 0
	}
	if p.live == nil {
		p.live, p.resp = map[state.ObjID]bool{}, map[state.ObjID]bool{}
	}
	for j := p.evAt; j < len(evs); j++ {
		e := &evs[j]
		switch e.Kind {
		case events.StepChange, events.TurnChange, events.ManaClear, events.PlayerLost:
			p.evStep = false
		case events.PutOnStack, events.AbilityPush:
			for o := range p.live {
				if o != e.Obj {
					p.resp[o] = true // a response went on the stack above the evidence spell
				}
			}
			if e.Kind == events.PutOnStack {
				p.stack = append(p.stack, e.Obj)
			}
		case events.CollectEvidenceAction:
			p.evStep = true
			if n := len(p.stack); n > 0 && koptevidCarriers[chName(g, p.stack[n-1])] {
				st.collected++
				p.live[p.stack[n-1]] = true
			}
		case events.Resolve:
			if !koptevidCarriers[chName(g, e.Obj)] {
				break
			}
			if p.live[e.Obj] {
				st.resolved++
				if p.resp[e.Obj] {
					st.responded++
				}
			} else {
				st.plain++
			}
			delete(p.live, e.Obj)
			delete(p.resp, e.Obj)
		}
	}
	p.evAt = len(evs)
}

// koptevidPolicy prefers an offered optional (kicked) cast, answers with
// Counterspell or Shock when it can, and plays the rest at random.
//
// It never takes a PLAIN cast of a carrier once evidence was collected in the
// same step: gorge's costMovesInWindow (rules/host_cast_ledger.go) runs its
// backward half (the ability branch: costs paid BEFORE the AbilityPush) for a
// spell too, so a plain Behind the Mask cast as a response to a kicked
// Extract a Confession reads the earlier spell's evidence moves as its own
// (optevid seed 85: the animate resolves 1/1, not 4/3). Forge keeps the
// collected cards on the spell ability that paid them; the kernel reads the
// spell's own paid flag. See the report's Gorge finding; drop this guard when
// the pin carries the fix.
func koptevidPolicy(seed uint64, st *koptevidStats) Policy {
	rnd := RandomPolicy(seed)
	r := rand.New(rand.NewPCG(seed, 0x6f707465)) // "opte"
	var tl koptevidTally
	var opt, resp, keep []Canon
	pick := func(c *PickCtx, from []Canon) Canon {
		d := *c.D
		d.Options = from
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	}
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			tl.scan(c.G, st)
		}
		if c.D.Kind != DPriority || c.G == nil {
			return rnd.Pick(c)
		}
		opt, resp, keep = opt[:0], resp[:0], keep[:0]
		stackBusy := len(c.G.E.G.Zone(state.ZStack, 0)) > 0
		for _, a := range c.D.Options {
			if a.Kind() != ActCast {
				keep = append(keep, a)
				continue
			}
			o, ok := c.G.GorgeOption(a)
			if !ok || o.Kind != "cast" {
				keep = append(keep, a)
				continue
			}
			name := c.G.E.G.Obj(o.Obj).Face().Name
			if koptevidCarriers[name] && o.Mode != "optionalcost" && tl.evStep {
				continue // the gorge window quirk above
			}
			switch {
			case o.Mode == "optionalcost" && koptevidCarriers[name]:
				opt = append(opt, a)
			case name == "Counterspell" || name == "Shock":
				if stackBusy {
					resp = append(resp, a)
				}
				continue // held for a response
			}
			keep = append(keep, a)
		}
		if len(opt) > 0 {
			st.offered++
		}
		switch {
		case len(resp) > 0 && r.IntN(4) != 0:
			return pick(c, resp)
		case len(opt) > 0 && r.IntN(4) != 0:
			return pick(c, opt)
		}
		return pick(c, keep)
	})
}

// runOptevid plays the matchup in both seatings for seeds 1..seeds each and
// returns the games, divergences, kernel stalls and the counts; mutate sets
// DebugOptCostNeverPaid (the mutation must diverge).
func runOptevid(t *testing.T, seeds int, mutate bool) (games, divs, stalls int, st koptevidStats) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("optevid")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugOptCostNeverPaid
	dirac.DebugOptCostNeverPaid = mutate
	defer func() { dirac.DebugOptCostNeverPaid = old }()
	for swap := 0; swap < 2; swap++ {
		md := koptevidDecks
		if swap == 1 {
			md = [2][]cardset.Entry{koptevidDecks[1], koptevidDecks[0]}
		}
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
				t.Fatalf("swap %d deck %d: %d cards", swap, s, len(decks[s]))
			}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= uint64(seeds); seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"koptevid-a", "koptevid-b"}, decks, 1)
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
			res := LockstepLed(m, seed, g, k, koptevidPolicy(seed, &st), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
			games++
			if d := res.Div; d != nil {
				divs++
				if !mutate {
					lo := d.At - 12
					if lo < 0 {
						lo = 0
					}
					t.Logf("swap %d seed %d last actions %v", swap, seed, actionStrings(res.Actions[lo:d.At]))
					t.Errorf("swap %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", swap, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				}
			}
			CloseSide(k)
			if strings.HasPrefix(res.Stall, KernelStallPrefix) {
				stalls++
				if !mutate {
					t.Errorf("swap %d seed %d: stalled: %s", swap, seed, res.Stall)
				}
			}
		}
	}
	return games, divs, stalls, st
}

func TestOptevidLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	// 96 seeds per seating: 192 games
	games, divs, stalls, st := runOptevid(t, 96, false)
	t.Logf("%d games, %d divergences, %d stalls, %+v", games, divs, stalls, st)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	if st.collected < 3 || st.responded < 3 || st.plain < 3 {
		t.Errorf("thin coverage: collected %d, resolved after a response %d, plain %d (want 3 each)", st.collected, st.responded, st.plain)
	}
}

// TestOptevidMutation: DebugOptCostNeverPaid reads every paid evidence cast
// as unpaid (the basic land search, the 4/3 animation), so the lockstep must
// diverge.
func TestOptevidMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, _, _ := runOptevid(t, 32, true)
	if divs == 0 {
		t.Fatalf("DebugOptCostNeverPaid: no divergence in %d games", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}
