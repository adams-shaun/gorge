package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// countBoundsCase is one four-seat pod holding a carrier whose lowering now
// reads the live player count.
type countBoundsCase struct {
	name      string
	witnesses []string // every card the pod's decks name; the pool is searched
	carrier   string
	mode      uint8 // the charm/tiered mode to force (-1: none)
	decks     [4][]string
}

// countBoundsProbe answers a four-seat game with castActivatePlayedPolicy and,
// for a carrier's target ask, picks every target the ask offers before Done.
// It records the most targets picked in one ask and how often the carrier was
// cast or activated.
type countBoundsProbe struct {
	base     Policy
	carrier  string
	mode     uint8
	last     string
	pending  int
	maxTgt   int
	used     int
	seenMode bool
}

func (p *countBoundsProbe) Pick(c *PickCtx) Canon {
	a := p.base.Pick(c)
	if c.G == nil {
		return a
	}
	if c.D.Kind == DPriority {
		p.last = ""
		for _, x := range c.D.Options {
			if k := x.Kind(); (k == ActCast || k == ActActivate) && refCardName(c.G, x.Src()) == p.carrier {
				a = x
				p.last = p.carrier
				break
			}
		}
	}
	src := p.source(c)
	if c.D.Kind == DModes && src == p.carrier && p.mode != 255 && !p.seenMode {
		for _, x := range c.D.Options {
			if x.Kind() == ActMode && x.Mode() == p.mode {
				p.seenMode = true
				a = x
				break
			}
		}
	}
	if c.D.Kind == DTarget && src == p.carrier {
		// Take two targets, then close: enough to prove the live bound is not
		// the two-seat constant one, without exercising a four-object effect.
		if p.pending < 2 {
			for _, x := range c.D.Options {
				if x.Kind() == ActTarget {
					a = x
					p.pending++
					break
				}
			}
		} else {
			for _, x := range c.D.Options {
				if x.Kind() == ActDone {
					a = x
					break
				}
			}
		}
		if p.pending > p.maxTgt {
			p.maxTgt = p.pending
		}
	} else if c.D.Kind != DTarget {
		p.pending = 0
	}
	if k := a.Kind(); k == ActCast || k == ActActivate {
		if n := refCardName(c.G, a.Src()); n == p.carrier {
			p.used++
		}
	}
	return a
}

// source names the stack top's source card for a triggered ability, else the
// carrier cast or activated just before this decision. A spell's target ask
// is posed before the spell reaches the stack, so last is its source.
func (p *countBoundsProbe) source(c *PickCtx) string {
	g := c.G.E.G
	if len(g.Stack) > 0 {
		if top := g.Obj(g.Stack[len(g.Stack)-1]); top != nil && top.Ability != nil {
			if src := g.Obj(top.Source); src != nil && src.Face() != nil {
				return src.Face().Name
			}
		}
	}
	return p.last
}

// countBoundsPlay builds the four-seat pod and returns a seed player whose
// probe records the carrier's target asks.
func countBoundsPlay(t *testing.T, tc countBoundsCase) func(seed uint64) (Result, *countBoundsProbe) {
	t.Helper()
	reg := synthReg(t)
	pool, set := synthPoolWith(t, tc.witnesses...)
	t.Logf("%s: carrier pool %s", tc.name, pool)
	var lists [][]cardset.Entry
	for _, d := range tc.decks {
		es := make([]cardset.Entry, len(d))
		for i, n := range d {
			es[i] = cardset.Entry{Name: n, Count: 1}
		}
		lists = append(lists, es)
	}
	tbl, _, err := set.MatchupN(lists)
	if err != nil {
		t.Fatalf("%s: %v", tc.name, err)
	}
	return func(seed uint64) (Result, *countBoundsProbe) {
		r := &Repro{Format: reproFormat, Decks: [2]string{"x0", "x1"}, Cards: [2][]string{tc.decks[0], tc.decks[1]},
			MoreDecks: []string{"x2", "x3"}, MoreCards: [][]string{tc.decks[2], tc.decks[3]}, Seed: seed, PassElision: true, Surface: "full"}
		m, err := r.Matchup(reg)
		if err != nil {
			t.Fatal(err)
		}
		rc := r.RunConfig()
		rc.MaxDecisions, rc.MaxTurns = 3000, 30
		k, err := SetSide(set, tbl, rc.Config)(m, seed)
		if err != nil {
			t.Fatalf("%s seed %d: %v", tc.name, seed, err)
		}
		defer CloseSide(k)
		played := map[string]int{}
		p := &countBoundsProbe{base: castActivatePlayedPolicy(seed, played), carrier: tc.carrier, mode: tc.mode}
		res := Lockstep(m, seed, k, p, rc)
		return res, p
	}
}

// countBoundsPods are the four-seat pods: each deck carries the carrier, a
// cheap creature of the same pool to target, and lands to cast them. A pod
// names the cards it needs, not a pool: buildSynthList splits a tail-* bucket
// when a newly lowered card pushes its table over a size limit, and a pinned
// name would then fail an unrelated landing, so countBoundsPlay searches the
// synthetic pools (synthPoolWith) for one that holds every witness. Rot Hulk
// (never cast by the 40-card pod) is not here; see the round report's Issues.
func countBoundsPods() []countBoundsCase {
	return []countBoundsCase{
		{name: "age-of-ultron", witnesses: []string{"Age of Ultron", "Ana Disciple", "Swamp", "Forest"}, carrier: "Age of Ultron", mode: 255,
			decks: [4][]string{
				deckOf(12, "Age of Ultron", deckOf(8, "Ana Disciple", deckOf(10, "Swamp", deckOf(10, "Forest")...)...)...),
				deckOf(12, "Age of Ultron", deckOf(8, "Ana Disciple", deckOf(10, "Swamp", deckOf(10, "Forest")...)...)...),
				deckOf(12, "Age of Ultron", deckOf(8, "Ana Disciple", deckOf(10, "Swamp", deckOf(10, "Forest")...)...)...),
				deckOf(12, "Age of Ultron", deckOf(8, "Ana Disciple", deckOf(10, "Swamp", deckOf(10, "Forest")...)...)...),
			}},
		{name: "hollow-marauder", witnesses: []string{"Hollow Marauder", "Swamp"}, carrier: "Hollow Marauder", mode: 255,
			decks: [4][]string{
				deckOf(12, "Hollow Marauder", deckOf(28, "Swamp")...),
				deckOf(12, "Hollow Marauder", deckOf(28, "Swamp")...),
				deckOf(12, "Hollow Marauder", deckOf(28, "Swamp")...),
				deckOf(12, "Hollow Marauder", deckOf(28, "Swamp")...),
			}},
		{name: "battle-at-the-helvault", witnesses: []string{"Battle at the Helvault", "Balduvian Bears", "Plains", "Forest"}, carrier: "Battle at the Helvault", mode: 255,
			decks: [4][]string{
				deckOf(12, "Battle at the Helvault", deckOf(8, "Balduvian Bears", deckOf(10, "Plains", deckOf(10, "Forest")...)...)...),
				deckOf(12, "Battle at the Helvault", deckOf(8, "Balduvian Bears", deckOf(10, "Plains", deckOf(10, "Forest")...)...)...),
				deckOf(12, "Battle at the Helvault", deckOf(8, "Balduvian Bears", deckOf(10, "Plains", deckOf(10, "Forest")...)...)...),
				deckOf(12, "Battle at the Helvault", deckOf(8, "Balduvian Bears", deckOf(10, "Plains", deckOf(10, "Forest")...)...)...),
			}},
		{name: "nils", witnesses: []string{"Nils, Discipline Enforcer", "Affa Guard Hound", "Plains"}, carrier: "Nils, Discipline Enforcer", mode: 255,
			decks: [4][]string{
				deckOf(12, "Nils, Discipline Enforcer", deckOf(8, "Affa Guard Hound", deckOf(20, "Plains")...)...),
				deckOf(12, "Nils, Discipline Enforcer", deckOf(8, "Affa Guard Hound", deckOf(20, "Plains")...)...),
				deckOf(12, "Nils, Discipline Enforcer", deckOf(8, "Affa Guard Hound", deckOf(20, "Plains")...)...),
				deckOf(12, "Nils, Discipline Enforcer", deckOf(8, "Affa Guard Hound", deckOf(20, "Plains")...)...),
			}},
		{name: "thraben-charm", witnesses: []string{"Thraben Charm", "Cloudcrown Oak", "Plains", "Forest"}, carrier: "Thraben Charm", mode: 2,
			decks: [4][]string{
				deckOf(12, "Thraben Charm", deckOf(8, "Cloudcrown Oak", deckOf(10, "Plains", deckOf(10, "Forest")...)...)...),
				deckOf(12, "Thraben Charm", deckOf(8, "Cloudcrown Oak", deckOf(10, "Plains", deckOf(10, "Forest")...)...)...),
				deckOf(12, "Thraben Charm", deckOf(8, "Cloudcrown Oak", deckOf(10, "Plains", deckOf(10, "Forest")...)...)...),
				deckOf(12, "Thraben Charm", deckOf(8, "Cloudcrown Oak", deckOf(10, "Plains", deckOf(10, "Forest")...)...)...),
			}},
		{name: "araumi", witnesses: []string{"Araumi of the Dead Tide", "Fell Stinger", "Island", "Swamp"}, carrier: "Araumi of the Dead Tide", mode: 255,
			decks: [4][]string{
				deckOf(12, "Araumi of the Dead Tide", deckOf(8, "Fell Stinger", deckOf(10, "Island", deckOf(10, "Swamp")...)...)...),
				deckOf(12, "Araumi of the Dead Tide", deckOf(8, "Fell Stinger", deckOf(10, "Island", deckOf(10, "Swamp")...)...)...),
				deckOf(12, "Araumi of the Dead Tide", deckOf(8, "Fell Stinger", deckOf(10, "Island", deckOf(10, "Swamp")...)...)...),
				deckOf(12, "Araumi of the Dead Tide", deckOf(8, "Fell Stinger", deckOf(10, "Island", deckOf(10, "Swamp")...)...)...),
			}},
		{name: "clouds-limit-break", witnesses: []string{"Cloud's Limit Break", "Sacred Cat", "Plains"}, carrier: "Cloud's Limit Break", mode: 1,
			decks: [4][]string{
				deckOf(12, "Cloud's Limit Break", deckOf(16, "Sacred Cat", deckOf(12, "Plains")...)...),
				deckOf(12, "Cloud's Limit Break", deckOf(16, "Sacred Cat", deckOf(12, "Plains")...)...),
				deckOf(12, "Cloud's Limit Break", deckOf(16, "Sacred Cat", deckOf(12, "Plains")...)...),
				deckOf(12, "Cloud's Limit Break", deckOf(16, "Sacred Cat", deckOf(12, "Plains")...)...),
			}},
		{name: "lagrella", witnesses: []string{"Lagrella, the Magpie", "Plains", "Forest", "Island"}, carrier: "Lagrella, the Magpie", mode: 255,
			decks: [4][]string{
				deckOf(12, "Lagrella, the Magpie", deckOf(10, "Plains", deckOf(10, "Forest", deckOf(8, "Island")...)...)...),
				deckOf(12, "Lagrella, the Magpie", deckOf(10, "Plains", deckOf(10, "Forest", deckOf(8, "Island")...)...)...),
				deckOf(12, "Lagrella, the Magpie", deckOf(10, "Plains", deckOf(10, "Forest", deckOf(8, "Island")...)...)...),
				deckOf(12, "Lagrella, the Magpie", deckOf(10, "Plains", deckOf(10, "Forest", deckOf(8, "Island")...)...)...),
			}},
		{name: "communal-brewing", witnesses: []string{"Communal Brewing", "Dawnhart Disciple", "Forest"}, carrier: "Communal Brewing", mode: 255,
			decks: [4][]string{
				deckOf(12, "Communal Brewing", deckOf(8, "Dawnhart Disciple", deckOf(20, "Forest")...)...),
				deckOf(12, "Communal Brewing", deckOf(8, "Dawnhart Disciple", deckOf(20, "Forest")...)...),
				deckOf(12, "Communal Brewing", deckOf(8, "Dawnhart Disciple", deckOf(20, "Forest")...)...),
				deckOf(12, "Communal Brewing", deckOf(8, "Dawnhart Disciple", deckOf(20, "Forest")...)...),
			}},
	}
}

// TestFourSeatCountBoundsLockstep plays four-seat pods holding each carrier
// whose target bound or exile cost now reads the live opponents or players,
// gorge against gorge with a policy that picks every target the carrier's ask
// offers. Every game must reach Lockstep with no divergence or unmodelled
// stop, and the carrier must be cast, activated or given a target ask; the
// carriers whose four-seat ask is two or more targets must actually offer two
// or more, which the old two-seat constant cannot.
func TestFourSeatCountBoundsLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	const seeds = 3
	for _, tc := range countBoundsPods() {
		t.Run(tc.name, func(t *testing.T) {
			play := countBoundsPlay(t, tc)
			exercised, maxTgt := 0, 0
			for seed := uint64(1); seed <= seeds; seed++ {
				res, p := play(seed)
				if res.Div != nil {
					t.Fatalf("seed %d: divergence at %d: %s %s %v (last %s; ref options %v; ker options %v)", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Div.LastSourced, res.Div.RefOptions, res.Div.KerOptions)
				}
				if res.StallUnmodelled {
					t.Fatalf("seed %d: kernel stopped unmodelled: %s", seed, res.Stall)
				}
				if p.used > 0 || p.maxTgt > 0 {
					exercised++
				}
				if p.maxTgt > maxTgt {
					maxTgt = p.maxTgt
				}
				t.Logf("seed %d: %d decisions, over %v, %s, casts %d, max targets %d", seed, res.Decisions, res.Over, res.Stall, p.used, p.maxTgt)
			}
			if exercised == 0 {
				t.Fatalf("%s was never cast, activated or asked for a target in %d games; the lockstep proves nothing", tc.carrier, seeds)
			}
			if tc.name == "araumi" {
				if maxTgt != 1 {
					t.Errorf("%s: the pump's one-target ask took %d targets, want 1", tc.carrier, maxTgt)
				}
			} else if maxTgt < 2 {
				t.Errorf("%s: the most targets one ask took was %d over %d games, want at least 2 at four seats", tc.carrier, maxTgt, seeds)
			}
		})
	}
}

// TestFourSeatCountBoundsMutation: the kernel emulating either old fold - the
// opponents' count read as one (dirac.DebugOppCountOne) or the per-player
// diff-controllers ask capped at one (dirac.DebugOneEachOne) - diverges on at
// least one seed of a representative pod.
func TestFourSeatCountBoundsMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	const seeds = 6
	byName := map[string]countBoundsCase{}
	for _, tc := range countBoundsPods() {
		byName[tc.name] = tc
	}
	for _, tc := range []struct {
		name string
		flag *bool
	}{
		{"hollow-marauder", &dirac.DebugOppCountOne},
		{"nils", &dirac.DebugOneEachOne},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := byName[tc.name]
			play := countBoundsPlay(t, c)
			*tc.flag = true
			defer func() { *tc.flag = false }()
			for seed := uint64(1); seed <= seeds; seed++ {
				res, _ := play(seed)
				if res.Div != nil {
					t.Logf("mutation: seed %d diverges at %d (%s %s)", seed, res.Div.At, res.Div.What, res.Div.Field)
					return
				}
			}
			t.Fatalf("%s: the old fold changed no seed over %d games", c.carrier, seeds)
		})
	}
}
