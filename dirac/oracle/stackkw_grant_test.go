package oracle

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The Improvise and Casualty:1 grants of the AffectedZone$ Stack keyword
// static (follow-up): Ironheart, Clever Champion and
// Inspiring Statuary grant improvise, Silverquill, the Disputant grants
// casualty 1. gorge offers a tap-paid cast (its potential planner refuses
// the shape) and the casualty mode only against the real floating pool, and
// no mana ability is an action, so the decks run the two ETB mana creatures
// (Iridescent Tiger adds one of each colour, Burning-Tree Emissary {R}{G}):
// the policy casts them first and the pool they leave pays the coloured part
// of the next cast, the artifacts the generic part (improvise) or a creature
// is sacrificed (casualty).

var kstackkwGrantDecks = [2][2][]cardset.Entry{{kstackkwGrantImp, kstackkwGrantCas}, {kstackkwGrantCas, kstackkwGrantImp}}

var kstackkwGrantImp = []cardset.Entry{
	{Name: "Ironheart, Clever Champion", Count: 3}, {Name: "Inspiring Statuary", Count: 3}, {Name: "Iridescent Tiger", Count: 4},
	{Name: "Mind Stone", Count: 2}, {Name: "Sol Ring", Count: 1}, {Name: "Bonesplitter", Count: 2}, {Name: "Hill Giant", Count: 1}, {Name: "Grizzly Bears", Count: 2},
	{Name: "Opt", Count: 2}, {Name: "Impulse", Count: 3}, {Name: "Brainstorm", Count: 1},
	{Name: "Island", Count: 7}, {Name: "Mountain", Count: 3}, {Name: "Plains", Count: 2}, {Name: "Swamp", Count: 2}, {Name: "Forest", Count: 2},
}

var kstackkwGrantCas = []cardset.Entry{
	{Name: "Silverquill, the Disputant", Count: 3}, {Name: "Iridescent Tiger", Count: 3}, {Name: "Burning-Tree Emissary", Count: 4},
	{Name: "Shock", Count: 3}, {Name: "Lightning Bolt", Count: 2}, {Name: "Opt", Count: 2}, {Name: "Impulse", Count: 2}, {Name: "Brainstorm", Count: 1},
	{Name: "Grizzly Bears", Count: 2}, {Name: "Walking Corpse", Count: 2},
	{Name: "Plains", Count: 4}, {Name: "Swamp", Count: 4}, {Name: "Mountain", Count: 4}, {Name: "Forest", Count: 3}, {Name: "Island", Count: 1},
}

var kstackkwGrantPay = map[string]bool{"Iridescent Tiger": true, "Burning-Tree Emissary": true}

// kstackkwGrantCreature: the imp deck's creature spells (nonartifact, so only Inspiring Statuary grants them improvise).
var kstackkwGrantCreature = map[string]bool{"Iridescent Tiger": true, "Grizzly Bears": true, "Hill Giant": true}

// kstackkwGrantPolicy casts the ETB mana first, then prefers the casualty
// cast and the improvise_generic picks, and counts them: every improvise
// pick not on Ironheart itself (its own improvise is printed) and every
// casualty cast (no card in the pool prints casualty) is a granted play.
func kstackkwGrantPolicy(seed uint64, cnt map[string]int) Policy {
	rnd := RandomPolicy(seed)
	prng := rand.New(rand.NewPCG(seed, 0x6772616e74)) // "grant"
	lastCast := ""
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		note := func(a Canon) Canon {
			if c.G != nil && (a.Kind() == ActCast || a.Kind() == ActCastCasualty) {
				lastCast = refCardName(c.G, a.Src())
			}
			if a.Kind() == ActCastCasualty {
				cnt["casualty"]++
			}
			return a
		}
		if c.G == nil {
			return rnd.Pick(c)
		}
		if cp, ok := chooseCode("convoke"); ok && c.D.Kind == DChoose {
			for _, a := range c.D.Options {
				if o, ok := c.G.GorgeOption(a); ok && a.Kind() == ActChoose+CanonKind(cp) && o.Kind == "improvise_generic" && prng.IntN(4) != 0 {
					if lastCast != "Ironheart, Clever Champion" {
						cnt["improvise"]++
						if kstackkwGrantCreature[lastCast] {
							cnt["improvise-creature"]++
						}
					}
					return a
				}
			}
		}
		if c.D.Kind == DPriority && len(c.G.E.G.Stack) == 0 {
			for _, a := range c.D.Options {
				if a.Kind() == ActCast && kstackkwGrantPay[refCardName(c.G, a.Src())] {
					return note(a)
				}
			}
			for _, a := range c.D.Options {
				if a.Kind() == ActCastCasualty && prng.IntN(4) != 0 {
					return note(a)
				}
			}
			buf = buf[:0]
			for _, a := range c.D.Options {
				if k := a.Kind(); k == ActCast || k == ActPlayLand {
					buf = append(buf, a)
				}
			}
			if len(buf) > 0 {
				return note(buf[prng.IntN(len(buf))])
			}
		}
		return note(rnd.Pick(c))
	})
}

// runStackkwGrant plays seeds 1..seeds of the grant decks, alternating the
// two seatings, and returns the games, divergences, capacity stalls and the
// granted-play counts.
func runStackkwGrant(t *testing.T, seeds int, report bool) (games, divs, stalls int, cnt map[string]int) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("stackkw")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][2][]*cards.Card
	var tbls [2]*kop.Table
	for mi, md := range kstackkwGrantDecks {
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
	cnt = map[string]int{}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		mi := int(seed/2) % 2
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kstackkw-grant-a", "kstackkw-grant-b"}, decks[mi], 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbls[mi], cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		g, err := NewGorgeSide(m, seed, cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		res := LockstepLed(m, seed, g, k, kstackkwGrantPolicy(seed, cnt), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 30})
		CloseSide(k)
		games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			if report {
				t.Errorf("seed %d: kernel capacity stall %q", seed, res.Stall)
			}
		}
		if d := res.Div; d != nil {
			divs++
			if report {
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
			if !report || divs >= 3 {
				break
			}
		}
	}
	return games, divs, stalls, cnt
}

func TestStackkwGrantLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, stalls, cnt := runStackkwGrant(t, 256, true)
	t.Logf("%d games, %d divergences, %d stalls, granted plays %v", games, divs, stalls, cnt)
	for _, w := range []string{"improvise", "improvise-creature", "casualty"} {
		want := 5
		if w == "improvise-creature" {
			want = 1
		}
		if cnt[w] < want {
			t.Errorf("only %d granted %s plays (want >= %d)", cnt[w], w, want)
		}
	}
}

// TestStackkwGrantMutation makes the kernel ignore the keyword grants: the
// granted improvise picks and casualty casts vanish, so the lockstep must
// diverge.
func TestStackkwGrantMutation(t *testing.T) {
	old := dirac.DebugStackKwOff
	dirac.DebugStackKwOff = true
	defer func() { dirac.DebugStackKwOff = old }()
	games, divs, _, _ := runStackkwGrant(t, 64, false)
	if divs == 0 {
		t.Fatalf("grants ignored in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}
