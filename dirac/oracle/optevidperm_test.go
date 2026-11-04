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

// CollectEvidence<N> as a permanent spell's OptionalCost part and the
// CastSA>Collected reads (CR 701.59a, 601.2b):
// Crimestopper Sprite (tap target creature, a stun counter if evidence was
// collected) and Vitu-Ghazi Inspector (an intervening-if +1/+1 counter and 2
// life). gorge reads the group off the permanent's own cast window
// (castCollectedGroup -> castWindowMoves), so a response to the
// creature spell does not erase it (Counterspell and Shock between the cast and
// the resolution) and a source that left the battlefield before its ETB
// resolves reads NOT collected (Shock and Lightning Bolt in response to the
// trigger). The kernel reads the permanent's kicked provenance (F2Kicked).
//
// gorge's planner prices no OptionalCost card, so the optional cast is offered
// only while the floating pool pays it (Iridescent Tiger's WUBRG,
// Burning-Tree Emissary's RG); the policy casts the carrier right after.
var koptevidpermDecks = [2][]cardset.Entry{
	{
		{Name: "Crimestopper Sprite", Count: 4}, {Name: "Vitu-Ghazi Inspector", Count: 4},
		{Name: "Thought Scour", Count: 4}, {Name: "Careful Study", Count: 4}, {Name: "Faithless Looting", Count: 1},
		{Name: "Iridescent Tiger", Count: 4}, {Name: "Burning-Tree Emissary", Count: 4}, {Name: "Hill Giant", Count: 1},
		{Name: "Island", Count: 6}, {Name: "Forest", Count: 5}, {Name: "Mountain", Count: 3},
	},
	{
		{Name: "Counterspell", Count: 3}, {Name: "Shock", Count: 8}, {Name: "Lightning Bolt", Count: 8},
		{Name: "Island", Count: 7}, {Name: "Mountain", Count: 14},
	},
}

var koptevidpermCarriers = map[string]bool{"Crimestopper Sprite": true, "Vitu-Ghazi Inspector": true}

// koptevidpermStats is what the gorge-side event log says the games did.
type koptevidpermStats struct {
	collected  int // optional casts that collected evidence (CollectEvidenceAction)
	resolved   int // ... of which resolved (the permanent entered)
	responded  int // ... of which resolved after another object went on the stack
	sourceGone int // ... whose ETB trigger was still on the stack when the permanent left the battlefield
	plain      int // carrier spells that resolved without evidence
	offered    int // priority windows that offered an optional (kicked) cast
}

// koptevidpermTally folds the gorge event log into the stats.
type koptevidpermTally struct {
	evAt    int
	stack   []state.ObjID // spells pushed and not yet resolved, bottom first
	live    map[state.ObjID]bool
	resp    map[state.ObjID]bool
	spellOn map[state.ObjID]bool // spells on the stack, by card
	entered map[state.ObjID]bool // collected permanents that entered and whose ETB trigger has not been queued
	pending map[state.ObjID]bool // collected permanents whose ETB trigger has not resolved
}

func (p *koptevidpermTally) scan(ref *GorgeSide, st *koptevidpermStats) {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < p.evAt {
		p.evAt = 0
	}
	if p.live == nil {
		p.spellOn, p.live, p.resp, p.entered, p.pending = map[state.ObjID]bool{}, map[state.ObjID]bool{}, map[state.ObjID]bool{}, map[state.ObjID]bool{}, map[state.ObjID]bool{}
	}
	for j := p.evAt; j < len(evs); j++ {
		e := &evs[j]
		switch e.Kind {
		case events.PutOnStack, events.AbilityPush, events.TriggerPush:
			for o := range p.live {
				if o != e.Obj {
					p.resp[o] = true // a response went on the stack above the evidence spell
				}
			}
			switch e.Kind {
			case events.PutOnStack:
				p.stack = append(p.stack, e.Obj)
				p.spellOn[e.Obj] = true
			case events.TriggerPush:
				if p.entered[e.Obj] {
					p.pending[e.Obj] = true
					delete(p.entered, e.Obj)
				}
			}
		case events.CollectEvidenceAction:
			if n := len(p.stack); n > 0 && koptevidpermCarriers[chName(g, p.stack[n-1])] {
				st.collected++
				p.live[p.stack[n-1]] = true
			}
		case events.MoveZone:
			if e.From == state.ZStack && e.To == state.ZGraveyard {
				delete(p.spellOn, e.Obj) // countered
				delete(p.live, e.Obj)
				delete(p.resp, e.Obj)
			}
			if e.From == state.ZBattlefield && p.pending[e.Obj] {
				st.sourceGone++
				delete(p.pending, e.Obj)
			}
			if e.From == state.ZBattlefield {
				delete(p.entered, e.Obj)
			}
		case events.Resolve:
			// a spell's resolution and its ETB trigger's both name the card
			if !p.spellOn[e.Obj] {
				if o := g.Obj(e.Obj); o != nil {
					delete(p.pending, o.Source) // the ETB trigger resolved, its source still there
				}
				break
			}
			delete(p.spellOn, e.Obj)
			if !koptevidpermCarriers[chName(g, e.Obj)] {
				break
			}
			if p.live[e.Obj] {
				st.resolved++
				if p.resp[e.Obj] {
					st.responded++
				}
				p.entered[e.Obj] = true
			} else {
				st.plain++
			}
			delete(p.live, e.Obj)
			delete(p.resp, e.Obj)
		}
	}
	p.evAt = len(evs)
}

// koptevidpermPolicy prefers an offered optional (kicked) cast, answers with
// Counterspell, Shock or Lightning Bolt when it can (a response to the
// creature spell or to its ETB trigger), and plays the rest at random.
func koptevidpermPolicy(seed uint64, st *koptevidpermStats) Policy {
	rnd := RandomPolicy(seed)
	r := rand.New(rand.NewPCG(seed, 0x6f707470)) // "optp"
	var tl koptevidpermTally
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
		if c.G != nil && c.D.Kind == DTarget && len(tl.pending) > 0 && r.IntN(4) != 0 {
			// aim at a collected permanent whose ETB trigger is on the stack
			// (Shock or Lightning Bolt in response: the source-gone read)
			for _, a := range c.D.Options {
				if o, ok := c.G.GorgeOption(a); ok && a.Kind() == ActTarget && tl.pending[o.Obj] {
					return a
				}
			}
		}
		if c.D.Kind != DPriority || c.G == nil {
			return rnd.Pick(c)
		}
		opt, resp, keep = opt[:0], resp[:0], keep[:0]
		// hold the responders for a collected carrier: its spell on the stack, or its ETB trigger
		stackBusy := len(tl.pending) > 0 || len(tl.live) > 0 && r.IntN(3) == 0
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
			switch {
			case o.Mode == "optionalcost" && koptevidpermCarriers[name]:
				opt = append(opt, a)
			case name == "Counterspell" || name == "Shock" || name == "Lightning Bolt":
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
		if len(resp) == 0 && len(opt) == 0 {
			// play lands so the responders have mana (the random pick passes the turn at once)
			var lands []Canon
			for _, a := range keep {
				if a.Kind() == ActPlayLand {
					lands = append(lands, a)
				}
			}
			if len(lands) > 0 {
				return pick(c, lands)
			}
		}
		switch {
		case len(resp) > 0:
			return pick(c, resp)
		case len(opt) > 0 && r.IntN(4) != 0:
			return pick(c, opt)
		}
		return pick(c, keep)
	})
}

// runOptevidperm plays the matchup in both seatings for seeds 1..seeds each
// and returns the games, divergences, kernel stalls and the counts; mutate sets
// DebugCastCollectedNever (the mutation must diverge).
func runOptevidperm(t *testing.T, seeds int, mutate bool) (games, divs, stalls int, st koptevidpermStats) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("optevidperm")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugCastCollectedNever
	dirac.DebugCastCollectedNever = mutate
	defer func() { dirac.DebugCastCollectedNever = old }()
	for swap := 0; swap < 2; swap++ {
		md := koptevidpermDecks
		if swap == 1 {
			md = [2][]cardset.Entry{koptevidpermDecks[1], koptevidpermDecks[0]}
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
			m, err := NewMatchup(reg, [2]string{"koptevidperm-a", "koptevidperm-b"}, decks, 1)
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
			res := LockstepLed(m, seed, g, k, koptevidpermPolicy(seed, &st), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
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

func TestOptevidpermLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	// 96 seeds per seating: 192 games
	games, divs, stalls, st := runOptevidperm(t, 96, false)
	t.Logf("%d games, %d divergences, %d stalls, %+v", games, divs, stalls, st)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	if st.collected < 3 || st.responded < 3 || st.sourceGone < 3 || st.plain < 3 {
		t.Errorf("thin coverage: collected %d, resolved after a response %d, source gone %d, plain %d (want 3 each)", st.collected, st.responded, st.sourceGone, st.plain)
	}
}

// TestOptevidpermMutation: DebugCastCollectedNever reads every paid evidence
// permanent as not collected (no stun counter, no Inspector counter or life),
// so the lockstep must diverge.
func TestOptevidpermMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, _, _ := runOptevidperm(t, 32, true)
	if divs == 0 {
		t.Fatalf("DebugCastCollectedNever: no divergence in %d games", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}
