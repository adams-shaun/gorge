package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The cast-from-graveyard lockstep: the in-scope Standard
// carriers of the resolving spell's own graveyard-origin provenance --
// ConditionPresent$ Card.wasCastFromGraveyard (Ruthless Negotiation,
// Esper Origins, Retrieve the Esper, Recursive Recruitment) and Count$wasCastFromGraveyard.<yes>.<no> (Plunder the
// Trollshaws, The Final Days) -- beside discard/self-mill enablers that put
// them in a graveyard, so the kernel's cold ExCastFromGrave reader decides at
// every decision the way gorge's state.ObjectWasCastFromGraveyard does. The frozen
// FDN and BLB pair pools exercise none of these arms.

// kcastgraveDecks is one 40-card matchup: deck A casts the black/red/blue
// carriers and loots them into the graveyard, deck B the blue/green one, with
// discard and self-mill on both sides.
var kcastgraveDecks = [2][]cardset.Entry{
	// A: Ruthless Negotiation, Plunder the Trollshaws and The Final Days,
	// with Faithless Looting / Mind Rot to fill the graveyard
	{
		{Name: "Ruthless Negotiation", Count: 3},
		{Name: "Plunder the Trollshaws", Count: 3}, {Name: "The Final Days", Count: 3},
		{Name: "Faithless Looting", Count: 4}, {Name: "Mind Rot", Count: 4},
		{Name: "Walking Corpse", Count: 4}, {Name: "Raging Goblin", Count: 3}, {Name: "Hill Giant", Count: 2},
		{Name: "Swamp", Count: 5}, {Name: "Mountain", Count: 5}, {Name: "Island", Count: 4},
	},
	// B: Esper Origins and Retrieve the Esper, with Thought Scour / Mind Rot / Broodspinner to
	// fill the graveyard
	{
		{Name: "Esper Origins", Count: 3}, {Name: "Retrieve the Esper", Count: 3},
		{Name: "Thought Scour", Count: 4}, {Name: "Mind Rot", Count: 2}, {Name: "Broodspinner", Count: 2},
		{Name: "Recursive Recruitment", Count: 3},
		{Name: "Grizzly Bears", Count: 3}, {Name: "Llanowar Elves", Count: 3},
		{Name: "Forest", Count: 6}, {Name: "Island", Count: 6}, {Name: "Swamp", Count: 3}, {Name: "Plains", Count: 2},
	},
}

// kcastgraveCarriers names the cards whose resolutions the test counts.
var kcastgraveCarriers = []string{"Ruthless Negotiation", "Plunder the Trollshaws",
	"The Final Days", "Esper Origins", "Retrieve the Esper", "Recursive Recruitment"}

// kcastgraveCast is one carrier cast being followed to its resolution.
type kcastgraveCast struct {
	obj     state.ObjID
	name    string
	flash   bool
	onStack bool
}

// kcastgraveObs is what the games reached: carrier resolutions by cast route.
type kcastgraveObs struct {
	hand, flash         int
	handBy, flashByName map[string]int
	pending             []kcastgraveCast
}

func (o *kcastgraveObs) note(name string, flash bool) {
	if flash {
		o.flash++
		o.flashByName[name]++
		return
	}
	o.hand++
	o.handBy[name]++
}

// kcastgravePolicy casts or activates whenever it can (a flashback cast
// first, so the graveyard arm is reached), else random, and records the
// carriers' resolutions from the gorge side. A resolution is a cast that
// reached the stack and then left it for a zone other than its origin, so a
// reversed cast is not counted.
func kcastgravePolicy(seed uint64, obs *kcastgraveObs) Policy {
	rnd := RandomPolicy(seed)
	var castBuf, flashBuf, restBuf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		if g := c.G; g != nil {
			live := obs.pending[:0]
			for _, p := range obs.pending {
				ob := g.E.G.Obj(p.obj)
				if ob == nil || ob.Zone == state.ZStack {
					p.onStack = p.onStack || ob != nil
					live = append(live, p)
					continue
				}
				if !p.onStack {
					live = append(live, p) // a decision before the cast reached the stack
					continue
				}
				if p.flash && ob.Zone == state.ZGraveyard || !p.flash && ob.Zone == state.ZHand {
					continue // reversed, not resolved
				}
				if p.flash {
					obs.note(p.name, true)
				} else {
					obs.note(p.name, false)
				}
			}
			obs.pending = live
		}
		castBuf, flashBuf, restBuf = castBuf[:0], flashBuf[:0], restBuf[:0]
		for _, a := range c.D.Options {
			if a.Kind() != ActCast {
				restBuf = append(restBuf, a)
				if a.Kind() == ActActivate {
					castBuf = append(castBuf, a)
				}
				continue
			}
			flash := CastModeName(a.Mode()&15) == "flashback"
			if c.G != nil {
				if o, ok := c.G.GorgeOption(a); ok && o.Kind == "cast" && o.Mode == "flashback" {
					flash = true // a planner-paid cast's canonical mode
				}
			}
			restBuf = append(restBuf, a)
			castBuf = append(castBuf, a)
			if flash {
				flashBuf = append(flashBuf, a)
			}
		}
		pick := func(opts []Canon) Canon {
			d := *c.D
			d.Options = opts
			return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		}
		var a Canon
		switch {
		case len(flashBuf) > 0:
			a = pick(flashBuf)
		case len(castBuf) > 0:
			a = pick(castBuf)
		case len(restBuf) > 0:
			a = pick(restBuf)
		default:
			a = rnd.Pick(c)
		}
		// Record a carrier cast after the action is chosen (the pick above is
		// the policy's only stateful choice and never reads the observation).
		if a.Kind() == ActCast && c.G != nil {
			if o, ok := c.G.GorgeOption(a); ok && o.Kind == "cast" {
				if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil && carrierName(ob.Face().Name) {
					flash := o.Mode == "flashback" || CastModeName(a.Mode()&15) == "flashback"
					obs.pending = append(obs.pending, kcastgraveCast{obj: o.Obj, name: ob.Face().Name, flash: flash})
				}
			}
		}
		return a
	})
}

func carrierName(name string) bool {
	for _, n := range kcastgraveCarriers {
		if n == name {
			return true
		}
	}
	return false
}

// runCastgrave plays the matchup and returns the games, divergences, kernel
// capacity stalls, turn/decision-capped games and the carrier resolutions.
// mutate suppresses the per-divergence error (the mutation test wants them).
func runCastgrave(t *testing.T, seeds int, mutate bool) (games, divs, stalls, capped int, obs *kcastgraveObs) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("castgrave")
	if err != nil {
		t.Fatal(err)
	}
	obs = &kcastgraveObs{handBy: map[string]int{}, flashByName: map[string]int{}}
	for mi, md := range [][2][]cardset.Entry{kcastgraveDecks} {
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
				t.Fatalf("matchup %d deck %d: %d cards", mi, s, len(decks[s]))
			}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= uint64(seeds); seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"kcastgrave-a", "kcastgrave-b"}, decks, 1)
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
			res := LockstepLed(m, seed, g, k, kcastgravePolicy(seed, obs), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
			CloseSide(k)
			games++
			if d := res.Div; d != nil {
				divs++
				if !mutate {
					t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				}
			}
			switch {
			case res.Stall == "":
			case strings.HasPrefix(res.Stall, KernelStallPrefix):
				stalls++
				t.Errorf("matchup %d seed %d: kernel capacity stall %q", mi, seed, res.Stall)
			default:
				capped++ // the MaxDecisions / MaxTurns cap, not a capacity stall
			}
		}
	}
	return games, divs, stalls, capped, obs
}

func TestCastgraveLockstep(t *testing.T) {
	if isolate(t) {
		return
	}

	games, divs, stalls, capped, obs := runCastgrave(t, 256, false)
	t.Logf("%d games, %d divergences, %d kernel capacity stalls, %d capped; carrier resolutions from hand %d, with flashback %d",
		games, divs, stalls, capped, obs.hand, obs.flash)
	t.Logf("carrier resolutions by route: hand %v; flashback %v", obs.handBy, obs.flashByName)
	if divs != 0 {
		t.Fatalf("%d divergences", divs)
	}
	if stalls != 0 {
		t.Fatalf("%d kernel capacity stalls", stalls)
	}
	if obs.hand < 20 {
		t.Errorf("only %d carrier resolutions cast from hand, want at least 20", obs.hand)
	}
	if obs.flash < 20 {
		t.Errorf("only %d carrier resolutions cast with flashback, want at least 20", obs.flash)
	}
	if obs.handBy["Recursive Recruitment"] == 0 {
		t.Errorf("Recursive Recruitment never resolved cast from hand")
	}
	if obs.flashByName["Recursive Recruitment"] == 0 {
		t.Errorf("Recursive Recruitment never resolved cast with flashback")
	}
}

// TestCastgraveMutation makes the kernel read the cast-from-graveyard flag
// as never set: gorge still reads flashback, so the games must diverge. A
// green mutation (no divergence) means the lockstep never reaches a
// resolving carrier.
func TestCastgraveMutation(t *testing.T) {
	old := dirac.DebugCastGraveNever
	dirac.DebugCastGraveNever = true
	defer func() { dirac.DebugCastGraveNever = old }()
	games, divs, _, _, _ := runCastgrave(t, 32, true)
	if divs == 0 {
		t.Fatalf("mutation read no graveyard cast in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}
