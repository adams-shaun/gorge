package oracle

import (
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The until-your-next-turn and zone-extreme lockstep (CR 611.2a, 611.2b):
// The One Ring grants its controller protection from everything until their
// next turn and reads its burden counters, Mu Yanling, Sky Dancer and Fear of
// Falling lower a creature -2/-0 and lose flying until that turn, and
// Windfall, Jace's Archivist and Visions of Beyond read a
// PlayerCount$HighestValid zone extreme. Nautiloid Ship's enter trigger and
// Nightmares and Daydreams' chapters target a player, so the protection's
// target refusal is exercised at every seat. Both seats play the same deck,
// so every card is cast by both engines.

var kOneRingDeck = []cardset.Entry{
	{Name: "The One Ring", Count: 2},
	{Name: "Mu Yanling, Sky Dancer", Count: 2},
	{Name: "Fear of Falling", Count: 2},
	{Name: "Windfall", Count: 2},
	{Name: "Jace's Archivist", Count: 2},
	{Name: "Visions of Beyond", Count: 3},
	{Name: "Nightmares and Daydreams", Count: 2},
	{Name: "Nautiloid Ship", Count: 2},
	{Name: "Grizzly Bears", Count: 2},
	{Name: "Llanowar Elves", Count: 2},
	{Name: "Lightning Bolt", Count: 3},
	{Name: "Island", Count: 8},
	{Name: "Mountain", Count: 8},
}

// oneRingCards are the pool's cards whose cast the scan counts; oneRingActs
// the activated abilities' sources.
var oneRingCards = map[string]bool{
	"The One Ring": true, "Mu Yanling, Sky Dancer": true, "Fear of Falling": true,
	"Windfall": true, "Jace's Archivist": true, "Visions of Beyond": true,
	"Nightmares and Daydreams": true, "Nautiloid Ship": true, "Lightning Bolt": true,
}
var oneRingActs = map[string]bool{
	"The One Ring": true, "Mu Yanling, Sky Dancer": true, "Jace's Archivist": true,
}

// oneRingScan counts, in gorge's event log, each carrier's cast from the
// hand, each activation of the cards with one, every life loss and every
// burden counter placed.
func oneRingScan(g *GorgeSide, n map[string]int) {
	if g == nil {
		return
	}
	for _, ev := range g.E.L.Events {
		switch ev.Kind {
		case events.PutOnStack:
			if ob := g.E.G.Obj(ev.Obj); ob != nil && ob.Face() != nil && ev.From == state.ZHand && oneRingCards[ob.Face().Name] {
				n[ob.Face().Name]++
			}
		case events.AbilityPush:
			if ob := g.E.G.Obj(ev.Obj); ob != nil && ob.Face() != nil && oneRingActs[ob.Face().Name] {
				n[ob.Face().Name+" act"]++
			}
		case events.LifeChange:
			if ev.Amount < 0 {
				n["life lost"]++
			}
		case events.CounterChange:
			if ev.Counter == "BURDEN" {
				n["burden"]++
			}
		}
	}
}

// oneRingProbe biases priority toward casts and activations (a random policy
// otherwise mostly passes) and keeps the gorge side for the scan.
type oneRingProbe struct {
	base Policy
	r    *rand.Rand
	gs   *GorgeSide
}

func (p *oneRingProbe) pick(c *PickCtx) Canon {
	if c.G != nil {
		p.gs = c.G
	}
	var casts, acts []Canon
	for _, a := range c.D.Options {
		switch a.Kind() {
		case ActCast:
			casts = append(casts, a)
		case ActActivate:
			acts = append(acts, a)
		}
	}
	if len(casts) > 0 && p.r.IntN(3) != 0 {
		return casts[p.r.IntN(len(casts))]
	}
	if len(acts) > 0 && p.r.IntN(2) == 0 {
		return acts[p.r.IntN(len(acts))]
	}
	return p.base.Pick(c)
}

// runOneRing plays the seeded games; report logs the first divergence as a
// test error (the mutation runs expect one and stay quiet).
func runOneRing(t *testing.T, seeds uint64, report bool) (diverged bool, st map[string]int) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("onering")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s := range decks {
		for _, e := range kOneRingDeck {
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
	tbl, _, _, err := set.Matchup(kOneRingDeck, kOneRingDeck)
	if err != nil {
		t.Fatal(err)
	}
	st = map[string]int{}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"konering-a", "konering-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		probe := &oneRingProbe{base: RandomPolicy(seed), r: rand.New(rand.NewPCG(seed, 0x72696e67))}
		res := Lockstep(m, seed, k, PolicyFunc(probe.pick), RunConfig{Config: cfg, MaxDecisions: 2500, MaxTurns: 25})
		CloseSide(k)
		oneRingScan(probe.gs, st)
		if d := res.Div; d != nil {
			diverged = true
			if report {
				lo := d.At - 12
				if lo < 0 {
					lo = 0
				}
				t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
			return
		}
	}
	return
}

func TestOneRingLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	div, st := runOneRing(t, 256, true)
	t.Logf("256 games, %v", st)
	if div {
		t.Fatalf("diverged")
	}
	for name, want := range map[string]int{
		"The One Ring":               8,
		"The One Ring act":           4,
		"Mu Yanling, Sky Dancer":     8,
		"Mu Yanling, Sky Dancer act": 4,
		"Fear of Falling":            4,
		"Windfall":                   4,
		"Jace's Archivist":           4,
		"Jace's Archivist act":       2,
		"Visions of Beyond":          16,
		"Nightmares and Daydreams":   4,
		"Nautiloid Ship":             4,
		"Lightning Bolt":             16,
		"life lost":                  32,
		"burden":                     8,
	} {
		if st[name] < want {
			t.Errorf("%s %d, want at least %d", name, st[name], want)
		}
	}
}

// TestOneRingPlProtAllMutation: without the kernel's protection static its
// damage and target paths let through what gorge refuses, so the lockstep
// must diverge.
func TestOneRingPlProtAllMutation(t *testing.T) {
	dirac.DebugPlProtAllOff = true
	defer func() { dirac.DebugPlProtAllOff = false }()
	if div, _ := runOneRing(t, 64, false); !div {
		t.Fatal("DebugPlProtAllOff: no divergence in 64 seeds")
	}
}

// TestOneRingExtremeMutation: with the zone extreme reading zero the
// Windfall family draws a different count than gorge, so the lockstep must
// diverge.
func TestOneRingExtremeMutation(t *testing.T) {
	dirac.DebugNoPlExtreme = true
	defer func() { dirac.DebugNoPlExtreme = false }()
	if div, _ := runOneRing(t, 64, false); !div {
		t.Fatal("DebugNoPlExtreme: no divergence in 64 seeds")
	}
}

// TestOneRingNTEOTMutation: a -2/-0 until your next turn that ends at end of
// turn instead must make the lockstep diverge.
func TestOneRingNTEOTMutation(t *testing.T) {
	dirac.DebugPumpNTEndsEOT = true
	defer func() { dirac.DebugPumpNTEndsEOT = false }()
	if div, _ := runOneRing(t, 64, false); !div {
		t.Fatal("DebugPumpNTEndsEOT: no divergence in 64 seeds")
	}
}
