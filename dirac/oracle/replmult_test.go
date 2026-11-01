package oracle

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The replacement-multiplier lockstep: the filtered
// damage doublers (Gratuitous Violence: a creature you control on the
// battlefield; Neriv: a creature you control that entered this turn; Trance
// Kuja: a Wizard you control) beside Twinflame Tyrant, so two doublers pose
// gorge's CR 616.1 order ask, and the token multipliers (Exalted Sunborn x2
// on every token, Ojer Taq x3 on creature tokens only) beside creature and
// non-creature token makers. The kernel's cold dblDmgMatch / replTokensMult
// (dirac/repl_multipliers.go) decide every amount the way gorge's
// damageReplacementMatches and per-token ValidToken$ match do.

var kreplmultDecks = [2][]cardset.Entry{
	// A: the doublers and the token multipliers
	{
		{Name: "Gratuitous Violence", Count: 3}, {Name: "Neriv, Heart of the Storm", Count: 2},
		{Name: "Kuja, Genome Sorcerer", Count: 2}, {Name: "Twinflame Tyrant", Count: 2},
		{Name: "Exalted Sunborn", Count: 2}, {Name: "Ojer Taq, Deepest Foundation", Count: 2},
		{Name: "Raging Goblin", Count: 1}, {Name: "Ghitu Lavarunner", Count: 2}, {Name: "Dragon Fodder", Count: 2},
		{Name: "Raise the Alarm", Count: 2}, {Name: "Thraben Inspector", Count: 2}, {Name: "Strike It Rich", Count: 2},
		{Name: "Mountain", Count: 9}, {Name: "Plains", Count: 7},
	},
	// B: the same shell with burn (noncombat damage to creatures and players)
	{
		{Name: "Gratuitous Violence", Count: 2}, {Name: "Twinflame Tyrant", Count: 2}, {Name: "Kuja, Genome Sorcerer", Count: 2},
		{Name: "Neriv, Heart of the Storm", Count: 1}, {Name: "Ojer Taq, Deepest Foundation", Count: 2},
		{Name: "Exalted Sunborn", Count: 1}, {Name: "Shock", Count: 3}, {Name: "Burst Lightning", Count: 2},
		{Name: "Lightning Strike", Count: 2}, {Name: "Goblin Instigator", Count: 2}, {Name: "Raging Goblin", Count: 1},
		{Name: "Dragon Fodder", Count: 1}, {Name: "Thraben Inspector", Count: 2}, {Name: "Strike It Rich", Count: 1},
		{Name: "Mountain", Count: 9}, {Name: "Plains", Count: 7},
	},
}

var kreplmultDoublers = map[string]bool{"Gratuitous Violence": true, "Neriv, Heart of the Storm": true,
	"Trance Kuja, Fate Defied": true, "Twinflame Tyrant": true}

// kreplmultTally counts the coverage from gorge's log (read at each agreed
// decision; the battlefield is read then, so the counts are approximate).
type kreplmultTally struct {
	ref                                        *GorgeSide
	evAt                                       int
	doubled, noMatch, asks, multTok, ojerNoTok int
}

func hasTypeWord(o *state.Object, w string) bool {
	for _, t := range o.Face().Types {
		if t == w {
			return true
		}
	}
	return false
}

func (p *kreplmultTally) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	evs := ref.E.L.Events
	if p.ref != ref || len(evs) < p.evAt {
		p.ref, p.evAt = ref, 0
	}
	ctrls := func(pl state.PlayerID, name string) bool {
		for _, id := range g.Zone(state.ZBattlefield, pl) {
			if o := g.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name {
				return true
			}
		}
		return false
	}
	for j := p.evAt; j < len(evs); j++ {
		e := &evs[j]
		switch e.Kind {
		case events.DamageProvenance:
			src := g.Obj(e.Obj)
			if src == nil || src.Face() == nil || e.Amount <= 0 || len(e.IDs) == 0 {
				continue
			}
			c := src.Controller
			onBf := src.Zone == state.ZBattlefield
			crt := onBf && src.EffectiveIsCreature()
			opp := false
			if pl, ok := e.IDs[0].PlayerRef(); ok {
				opp = pl != c
			} else if r := g.Obj(e.IDs[0]); r != nil {
				opp = r.Controller != c
			}
			m := 0
			if opp && ctrls(c, "Twinflame Tyrant") {
				m++
			}
			if ctrls(c, "Gratuitous Violence") {
				if crt {
					m++
				} else {
					p.noMatch++
				}
			}
			if ctrls(c, "Neriv, Heart of the Storm") {
				if crt && src.EnteredThisTurn {
					m++
				} else {
					p.noMatch++
				}
			}
			if ctrls(c, "Trance Kuja, Fate Defied") && hasTypeWord(src, "Wizard") {
				m++
			}
			if m > 0 {
				p.doubled++
			}
		case events.TokenCreate:
			def := g.Tokens[e.Text]
			if def == nil || len(def.Faces) == 0 {
				continue
			}
			if ctrls(e.Player, "Exalted Sunborn") {
				p.multTok++
			}
			if ctrls(e.Player, "Ojer Taq, Deepest Foundation") {
				if def.Faces[0].IsCreature() {
					p.multTok++
				} else {
					p.ojerNoTok++
				}
			}
		}
	}
	p.evAt = len(evs)
	return nil
}

// kreplmultPolicy casts or activates whenever it can (so the games reach
// the doublers), else random; it counts the order asks among two or more
// doublers the gorge side poses.
func kreplmultPolicy(seed uint64, tl *kreplmultTally) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			n := 0
			for _, a := range c.D.Options {
				if o, ok := c.G.GorgeOption(a); ok && a.Kind() >= ActChoose {
					if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil && kreplmultDoublers[ob.Face().Name] {
						n++
					}
				}
			}
			if n >= 2 {
				tl.asks++
			}
		}
		if seed%4 == 0 {
			return rnd.Pick(c)
		}
		buf = buf[:0]
		for _, a := range c.D.Options {
			if k := a.Kind(); k == ActCast || k == ActActivate {
				buf = append(buf, a)
			}
		}
		if len(buf) == 0 {
			return rnd.Pick(c)
		}
		d := *c.D
		d.Options = buf
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

// runReplmult plays the constructed matchup and returns the games, the
// divergences, the stalls and the tally. mutate ignores the doublers'
// filters (the mutation must diverge).
func runReplmult(t *testing.T, seeds int, mutate bool) (games, divs, stalls int, tl *kreplmultTally) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("replmult")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugReplMultNoFilter
	dirac.DebugReplMultNoFilter = mutate
	defer func() { dirac.DebugReplMultNoFilter = old }()
	md := kreplmultDecks
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
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	tbl, _, _, err := set.Matchup(md[0], md[1])
	if err != nil {
		t.Fatal(err)
	}
	tl = &kreplmultTally{}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		if v := os.Getenv("KREPLMULT_ONLY"); v != "" && v != fmt.Sprint(seed) {
			continue
		}
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kreplmult-a", "kreplmult-b"}, decks, 1)
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
		res := LockstepLed(m, seed, g, k, kreplmultPolicy(seed, tl),
			RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20, Check: tl.check})
		CloseSide(k)
		games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			if !mutate {
				t.Errorf("seed %d: stalled: %s", seed, res.Stall)
			}
		}
		if d := res.Div; d != nil {
			divs++
			if os.Getenv("KREPLMULT_ONLY") != "" {
				// debugging one seed: the battlefield, the refs and gorge's log tail
				gg := g.E.G
				for pl := state.PlayerID(0); pl < 2; pl++ {
					for _, id := range gg.Zone(state.ZBattlefield, pl) {
						if o := gg.Obj(id); o != nil && o.Face() != nil {
							t.Logf("p%d bf %d %s entered=%v", pl, id, o.Face().Name, o.EnteredThisTurn)
						}
					}
				}
				for r := 0; r < 10 && r < len(g.id.obj); r++ {
					if ob := gg.Obj(g.id.obj[r]); ob != nil && ob.Face() != nil {
						t.Logf("ref #%d: obj %d %s", r, g.id.obj[r], ob.Face().Name)
					}
				}
				evs := g.E.L.Events
				for j := max(0, len(evs)-60); j < len(evs); j++ {
					e := evs[j]
					nm := ""
					if o := gg.Obj(e.Obj); o != nil && o.Face() != nil {
						nm = o.Face().Name
					}
					t.Logf("%v obj=%d(%s) pl=%d amt=%d ids=%v text=%q", e.Kind, e.Obj, nm, e.Player, e.Amount, e.IDs, e.Text)
				}
			}
			if !mutate {
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	return games, divs, stalls, tl
}

func TestReplmultLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, stalls, tl := runReplmult(t, 256, false)
	t.Logf("%d games, %d divergences, %d stalls, %d doubled, %d filtered no-match, %d two-doubler asks, %d multiplied tokens, %d non-creature tokens under Ojer Taq",
		games, divs, stalls, tl.doubled, tl.noMatch, tl.asks, tl.multTok, tl.ojerNoTok)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	for _, c := range []struct {
		what    string
		got, lo int
	}{{"doubled damage events", tl.doubled, 100}, {"filtered no-match damage events", tl.noMatch, 30},
		{"two-doubler order asks", tl.asks, 10}, {"multiplied tokens", tl.multTok, 50},
		{"non-creature tokens under Ojer Taq", tl.ojerNoTok, 20}} {
		if c.got < c.lo {
			t.Errorf("%s: %d, want at least %d", c.what, c.got, c.lo)
		}
	}
}

// TestReplmultMutation ignores the doublers' filters (every doubler
// applies to every source its controller controls): Gratuitous Violence's
// non-creature sources and Neriv's earlier creatures are doubled, so the
// games must diverge.
func TestReplmultMutation(t *testing.T) {
	games, divs, _, _ := runReplmult(t, 64, true)
	if divs == 0 {
		t.Fatalf("mutation ignored the doubler filters in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}
