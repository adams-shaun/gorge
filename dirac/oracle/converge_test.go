package oracle

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The lockstep: constructed decks over the generated converge
// pool, whose carriers read the colours of mana spent to cast a spell
// (Count$Converge: the spell chain, K:etbCounter, a self-ETB trigger;
// TriggeredCard$Converge: Magmablood Archaic's SpellCast pump). A
// five-colour deck of basics and gain lands makes 1-5 colour casts;
// Twincast copies a converge spell (the copy reads 0) and removal can kill
// an Archaic with its enters trigger on the stack (gorge reads the source
// live: 0).

// Together as One never faces Twincast: a copy of a spell whose chain
// announces a second target diverges in the copy's target record
// (a copy gap outside converge).
var convergeA = []cardset.Entry{ // five colours
	{Name: "Arcane Omens", Count: 2}, {Name: "Archaic's Agony", Count: 2}, {Name: "Snarl Song", Count: 2},
	{Name: "Rancorous Archaic", Count: 2}, {Name: "Sundering Archaic", Count: 3},
	{Name: "Steel Exemplar", Count: 2}, {Name: "Grizzly Bears", Count: 1}, {Name: "Opt", Count: 1}, {Name: "Shock", Count: 2},
	{Name: "Plains", Count: 3}, {Name: "Island", Count: 3}, {Name: "Swamp", Count: 3}, {Name: "Mountain", Count: 3}, {Name: "Forest", Count: 3},
	{Name: "Scoured Barrens", Count: 1}, {Name: "Dismal Backwater", Count: 1}, {Name: "Bloodfell Caves", Count: 1}, {Name: "Rugged Highlands", Count: 1},
	{Name: "Jungle Hollow", Count: 1}, {Name: "Thornwood Falls", Count: 1}, {Name: "Wind-Scarred Crag", Count: 1}, {Name: "Swiftwater Cliffs", Count: 1},
}

var convergeB = []cardset.Entry{ // U/B/R: Magmablood, removal
	{Name: "Magmablood Archaic", Count: 3}, {Name: "Priest of Urabrask", Count: 3}, {Name: "Sundering Archaic", Count: 1},
	{Name: "Lightning Bolt", Count: 3}, {Name: "Shock", Count: 2}, {Name: "Murder", Count: 3}, {Name: "Cancel", Count: 1},
	{Name: "Opt", Count: 1}, {Name: "Steel Exemplar", Count: 2}, {Name: "Archaic's Agony", Count: 1}, {Name: "Arcane Omens", Count: 1},
	{Name: "Mountain", Count: 6}, {Name: "Island", Count: 4}, {Name: "Swamp", Count: 3},
	{Name: "Swiftwater Cliffs", Count: 2}, {Name: "Bloodfell Caves", Count: 2}, {Name: "Dismal Backwater", Count: 2},
}

// with replaces deck d's entries named in r (a swap of equal counts).
func convergeWith(d []cardset.Entry, r map[string]cardset.Entry) []cardset.Entry {
	out := make([]cardset.Entry, len(d))
	for i, e := range d {
		if x, ok := r[e.Name]; ok {
			e = x
		}
		out[i] = e
	}
	return out
}

var convergeDecks = [][2][]cardset.Entry{
	{ // Together as One (a Rancorous slot) against the removal deck
		convergeWith(convergeA, map[string]cardset.Entry{"Rancorous Archaic": {Name: "Together as One", Count: 2}}),
		convergeB,
	},
	{ // Twincast (the removal deck's Archaic's Agony and Cancel slots) copies a converge spell
		convergeA,
		convergeWith(convergeB, map[string]cardset.Entry{"Archaic's Agony": {Name: "Twincast", Count: 1}, "Cancel": {Name: "Twincast", Count: 1}}),
	},
}

type convergeKey struct {
	card dirac.CardID
	ab   uint8
	obj  dirac.Obj
	kind uint8
}

// convergeProbe counts, at every decision, the converge reads newly on the
// kernel's stack: a converge spell (or a copy of one), a Magmablood pump,
// a self-ETB trigger (and one whose source has left).
type convergeProbe struct {
	n    map[string]int
	base Policy
	rs   *realSide
	seen map[convergeKey]bool
	left map[convergeKey]bool
	r    *rand.Rand
	hunt bool // removal was cast in answer to an enters trigger: target its source
}

func (p *convergeProbe) scan() {
	s := &p.rs.s
	for i := 0; i < s.StackLen(); i++ {
		n, kind, ok := s.ConvergeRead(i)
		if !ok {
			continue
		}
		_, src, card, _ := s.StackEntry(i)
		cd := &p.rs.tbl.Cards[card]
		if (kind == dirac.ConvergeSpell || kind == dirac.ConvergeCopy) && !cd.SpendRead {
			continue // not a converge reader
		}
		key := convergeKey{card, s.StackAbility(i), src, kind}
		if kind == dirac.ConvergeETBLeft {
			if !p.left[key] && n > 0 {
				p.left[key] = true
				p.n["ETB read after the source left"]++
			}
			continue
		}
		if p.seen[key] {
			continue
		}
		p.seen[key] = true
		p.n["reads"]++
		if n >= 3 {
			p.n["reads of 3+ colours"]++
		}
		if cd.Name == "Steel Exemplar" {
			if n >= 2 {
				p.n["Steel Exemplar 2+ colours"]++
			} else {
				p.n["Steel Exemplar under 2 colours"]++
			}
		}
		switch kind {
		case dirac.ConvergeCopy:
			p.n["copy reads"]++
			if n != 0 {
				p.n["copy reads nonzero"]++
			}
		case dirac.ConvergeCastTrig:
			p.n["Magmablood pumps"]++
		case dirac.ConvergeETB:
			p.n["ETB reads"]++
		}
	}
}

// name is the gorge card name of option x's object ("" if none).
func (p *convergeProbe) name(c *PickCtx, x Canon) string {
	o, ok := c.G.GorgeOption(x)
	if !ok || o.Obj == 0 {
		return ""
	}
	if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil {
		return ob.Face().Name
	}
	return ""
}

// etbOut: an opponent of pl has a converge-bound enters trigger on the
// kernel's stack whose source is still on the battlefield.
func (p *convergeProbe) etbOut(pl uint8) bool {
	s := &p.rs.s
	for i := 0; i < s.StackLen(); i++ {
		if _, kind, ok := s.ConvergeRead(i); ok && kind == dirac.ConvergeETB {
			if _, _, _, ctrl := s.StackEntry(i); ctrl != pl {
				return true
			}
		}
	}
	return false
}

// pick biases the base policy toward the converge branches: Magmablood
// Archaic is cast whenever offered (gorge's planner declines its hybrid
// cost, so it is offered only once Priest of Urabrask's {R}{R}{R} floats),
// the Priest is cast often, and removal is mostly held for an Archaic's
// enters trigger, then aimed at it (its source gone, the trigger reads 0).
func (p *convergeProbe) pick(c *PickCtx) Canon {
	p.scan()
	a := p.base.Pick(c)
	if c.G == nil {
		return a
	}
	var magma, priest, removal, hunt []Canon
	for _, x := range c.D.Options {
		switch n := p.name(c, x); {
		case x.Kind() == ActCast && n == "Magmablood Archaic":
			magma = append(magma, x)
		case x.Kind() == ActCast && n == "Priest of Urabrask":
			priest = append(priest, x)
		case x.Kind() == ActCast && (n == "Lightning Bolt" || n == "Murder"):
			removal = append(removal, x)
		case x.Kind() == ActTarget && p.hunt && n == "Sundering Archaic":
			hunt = append(hunt, x)
		}
	}
	switch {
	case len(hunt) > 0:
		a, p.hunt = hunt[0], false
	case len(magma) > 0 && p.r.IntN(8) != 0:
		a = magma[0]
	case len(removal) > 0 && p.etbOut(uint8(c.D.Player)) && p.r.IntN(2) == 0:
		a, p.hunt = removal[p.r.IntN(len(removal))], true
	case len(priest) > 0 && p.r.IntN(2) == 0:
		a = priest[0]
	case a.Kind() == ActCast && p.r.IntN(4) != 0 && len(removal) > 0 && c.D.Options[0].Kind() == ActPass:
		for _, x := range removal {
			if x == a {
				a = c.D.Options[0] // hold the removal for an enters trigger
				break
			}
		}
	}
	return a
}

func convergeSetup(t *testing.T) (*cards.Registry, cardset.Set) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("converge")
	if err != nil {
		t.Fatal(err)
	}
	return reg, set
}

// convergeRun plays seeds 1..seeds; it returns whether a game diverged
// (report logs the first as a test error), the kernel stalls and the
// probe's tallies.
func convergeRun(t *testing.T, seeds uint64, report bool) (diverged bool, stalls int, n map[string]int) {
	t.Helper()
	reg, set := convergeSetup(t)
	n = map[string]int{}
	for mi, md := range convergeDecks {
		decks := kcastDeck(t, reg, md)
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= seeds; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"converge-a", "converge-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			base := Policy(HeuristicPolicy(seed))
			if seed%3 == 0 {
				base = RandomPolicy(seed)
			}
			probe := &convergeProbe{n: n, base: base, rs: k.(*realSide), seen: map[convergeKey]bool{}, left: map[convergeKey]bool{},
				r: rand.New(rand.NewPCG(seed, 0x636f6e76))}
			res := Lockstep(m, seed, k, PolicyFunc(probe.pick), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
			CloseSide(k)
			if strings.HasPrefix(res.Stall, KernelStallPrefix) {
				stalls++
				if report {
					t.Errorf("matchup %d seed %d: kernel stall %s", mi, seed, res.Stall)
				}
			}
			if d := res.Div; d != nil {
				diverged = true
				if report {
					lo := max(d.At-12, 0)
					t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
					t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				}
				return
			}
		}
	}
	return
}

func TestConvergeLockstep(t *testing.T) {
	t.Parallel()
	div, stalls, n := convergeRun(t, 256, true)
	t.Logf("256 seeds per matchup, counts %v, %d divergences, %d stalls", n, b2i(div), stalls)
	if div {
		return
	}
	if n["copy reads nonzero"] != 0 {
		t.Errorf("%d copies read a nonzero converge count", n["copy reads nonzero"])
	}
	for _, c := range []struct {
		s   string
		min int
	}{
		{"reads", 100},
		{"reads of 3+ colours", 20},
		{"Magmablood pumps", 10},
		{"ETB read after the source left", 3},
		{"Steel Exemplar 2+ colours", 10},
		{"Steel Exemplar under 2 colours", 5},
	} {
		if n[c.s] < c.min {
			t.Errorf("%s: %d, want at least %d", c.s, n[c.s], c.min)
		}
	}
}

// TestConvergeMutation: a generic pip paid from a coloured unit not
// counted toward its colour must make the lockstep diverge.
func TestConvergeMutation(t *testing.T) {
	dirac.DebugConvergeGenericColourless = true
	defer func() { dirac.DebugConvergeGenericColourless = false }()
	if diverged, _, _ := convergeRun(t, 64, false); !diverged {
		t.Fatal("DebugConvergeGenericColourless: no divergence in 64 seeds per matchup")
	}
}
