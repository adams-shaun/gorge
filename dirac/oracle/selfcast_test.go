package oracle

import (
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The self cast trigger lockstep (selfcast): constructed matchups over the
// generated selfcast pool, whose carriers have a "when you cast this
// spell" trigger (CR 603.2: the spell itself, on the stack, is the trigger's
// source). Eldrazi ramp against a blue/black deck with cheap carriers, so a
// resolved carrier sits on the battlefield while other spells are cast
// (the battlefield copy must stay silent) and the cast triggers come up on
// both surfaces. The occurrences are counted on the gorge side (an ability
// on the stack whose source is a spell on the stack) and asserted.
var kselfcastDecks = [2][]cardset.Entry{
	{
		{Name: "Llanowar Elves", Count: 4}, {Name: "Elvish Mystic", Count: 3}, {Name: "Mind Stone", Count: 5},
		{Name: "Titans' Vanguard", Count: 3}, {Name: "Sowing Mycospawn", Count: 4}, {Name: "Hatchery Spider", Count: 2},
		{Name: "World Breaker", Count: 1}, {Name: "Desolation Twin", Count: 1}, {Name: "Bruna, the Fading Light", Count: 2},
		{Name: "Forest", Count: 9}, {Name: "Mountain", Count: 3}, {Name: "Plains", Count: 3},
	},
	{
		{Name: "Hope-Ender Coatl", Count: 3}, {Name: "Petrifying Meddler", Count: 3}, {Name: "Hydroid Krasis", Count: 3},
		{Name: "Old Stickfingers", Count: 3}, {Name: "Mind Stone", Count: 3},
		{Name: "Llanowar Elves", Count: 3}, {Name: "Dash Hopes", Count: 3}, {Name: "Phantasmagorian", Count: 2},
		{Name: "Island", Count: 7}, {Name: "Swamp", Count: 5}, {Name: "Forest", Count: 5},
	},
}

// selfCastCounter counts, through RunConfig.Check, every gorge stack
// ability whose source is a spell on the stack (a self cast trigger), by
// the source card's name.
type selfCastCounter struct {
	seen map[state.ObjID]bool
	n    map[string]int
	last *GorgeSide
	// kfix-cast: Sowing Mycospawn's kicked self cast trigger (the exile
	// one) on the stack, and gorge's any-player unless asks (ResumeKind
	// unless_pay posed for Dash Hopes / Phantasmagorian) by payer seat.
	kicked  int
	unless  [2]int
	lastSeq uint64
}

func (c *selfCastCounter) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	c.last = ref
	for _, id := range g.Stack {
		if c.seen[id] {
			continue
		}
		c.seen[id] = true
		o := g.Obj(id)
		if o == nil || o.Ability == nil || o.Source == 0 {
			continue
		}
		src := g.Obj(o.Source)
		if src == nil || src.Zone != state.ZStack || src.Card == nil || src.Ability != nil || len(src.Card.Faces) == 0 {
			continue
		}
		c.n[src.Card.Faces[0].Name]++
		if src.Card.Faces[0].Name == "Sowing Mycospawn" && strings.Contains(o.Ability.Line, "Destination$ Exile") {
			c.kicked++
		}
	}
	if fr := ref.fr; fr != nil && fr.d != nil && fr.seq != c.lastSeq && fr.d.ResumeKind == "unless_pay" {
		c.lastSeq = fr.seq
		if o := g.Obj(fr.d.Source); o != nil && o.Card != nil && len(o.Card.Faces) > 0 {
			if n := o.Card.Faces[0].Name; (n == "Dash Hopes" || n == "Phantasmagorian") && int(fr.d.Player) < 2 {
				c.unless[fr.d.Player]++
			}
		}
	}
	return nil
}

func TestSelfcastLockstep(t *testing.T) {
	t.Parallel()

	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("selfcast")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kselfcastDecks {
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
	tbl, _, _, err := set.Matchup(kselfcastDecks[0], kselfcastDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	seeds := uint64(256)
	if s := os.Getenv("KSELFCAST_SEEDS"); s != "" {
		seeds = 0
		for _, ch := range s {
			seeds = seeds*10 + uint64(ch-'0')
		}
	}
	holdKick := map[Ref]bool{} // deck A's Sowing Mycospawns, by card index
	for i, c := range decks[0] {
		if c.Faces[0].Name == "Sowing Mycospawn" {
			holdKick[Ref(i)] = true
		}
	}
	cnt := &selfCastCounter{n: map[string]int{}}
	total := 0
	div := 0
	for seed := uint64(1); seed <= seeds; seed++ {
		m, err := NewMatchup(reg, [2]string{"kselfcast-a", "kselfcast-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		cfg := Config{}
		if seed%4 == 0 {
			cfg = Config{GorgeAutopaySurface: true}
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		pol := castFirstPolicy(seed)
		if seed%4 == 1 {
			pol = kickHoldPolicy(pol, holdKick)
		}
		if seed%8 == 0 {
			pol = RandomPolicy(seed)
		}
		cnt.seen = map[state.ObjID]bool{}
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40, Check: cnt.check})
		CloseSide(k)
		if os.Getenv("KSELFCAST_TRACE") == strconv.FormatUint(seed, 10) {
			t.Logf("seed %d actions %v", seed, actionStrings(res.Actions))
		}
		if d := res.Div; d != nil {
			div++
			lo := d.At - 12
			if lo < 0 {
				lo = 0
			}
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	for _, n := range cnt.n {
		total += n
	}
	t.Logf("%d seeds, %d diverged; self cast triggers on the gorge stack: %d %v; kicked Mycospawn triggers %d; any-player unless asks %v", seeds, div, total, cnt.n, cnt.kicked, cnt.unless)
	if total < 20 {
		t.Errorf("self cast triggers: %d (want >= 20)", total)
	}
	if cnt.unless[0]+cnt.unless[1] < 20 || cnt.unless[0] == 0 || cnt.unless[1] == 0 {
		t.Errorf("any-player unless asks: %v (want >= 20, both seats)", cnt.unless)
	}
	if cnt.kicked < 10 {
		t.Errorf("kicked self cast fires: %d (want >= 10)", cnt.kicked)
	}
}
