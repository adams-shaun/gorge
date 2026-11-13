package oracle

import (
	"fmt"
	"math/rand/v2"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The spell-shaped cost statics: the Bargain
// trio's "costs {N} less if it's bargained" (Hamlet Glutton, Ice Out,
// Johann's Stopgap), Brush Off's Color$ 1 U reduction when it targets an
// instant or sorcery spell, and Dragon's Prey's {2} raise when it targets a
// Dragon. Two constructed matchups over the generated reducespell pool:
//
//	M1 the trio plus artifact and token makers (the sacrifice candidates),
//	   floating-mana creatures (the pool the surface prices a bargained cast
//	   from) against a deck that casts creatures and burn;
//	M2 Brush Off and Dragon's Prey against Dragons, other creatures and
//	   instants, so both the reduced and the full price, the raised and the
//	   plain one, are cast.
//
// The counters read the gorge side: what the policy chose, and what the live
// stack shows (Brush Off's and Dragon's Prey's chosen targets).

type krsMatch struct {
	name  string
	decks [2][]cardset.Entry
}

var krsMatches = []krsMatch{
	{"M1", [2][]cardset.Entry{
		{
			{Name: "Hamlet Glutton", Count: 3}, {Name: "Ice Out", Count: 3}, {Name: "Johann's Stopgap", Count: 4},
			{Name: "Ornithopter", Count: 3}, {Name: "Curious Pair", Count: 3}, {Name: "Bumbleflower's Sharepot", Count: 2},
			{Name: "Iridescent Tiger", Count: 3}, {Name: "Shock", Count: 3}, {Name: "Lightning Bolt", Count: 2},
			{Name: "Island", Count: 6}, {Name: "Forest", Count: 5}, {Name: "Mountain", Count: 3},
		},
		{
			{Name: "Grizzly Bears", Count: 8}, {Name: "Llanowar Elves", Count: 4},
			{Name: "Lightning Bolt", Count: 3}, {Name: "Shock", Count: 3},
			{Name: "Ornithopter", Count: 2}, {Name: "Curious Pair", Count: 2},
			{Name: "Forest", Count: 9}, {Name: "Mountain", Count: 7}, {Name: "Island", Count: 1}, {Name: "Swamp", Count: 1},
		},
	}},
	{"M2", [2][]cardset.Entry{
		{
			{Name: "Brush Off", Count: 4}, {Name: "Dragon's Prey", Count: 4}, {Name: "Cancel", Count: 2},
			{Name: "Ornithopter", Count: 3}, {Name: "Grizzly Bears", Count: 5}, {Name: "Llanowar Elves", Count: 4},
			{Name: "Island", Count: 6}, {Name: "Swamp", Count: 6}, {Name: "Forest", Count: 6},
		},
		{
			{Name: "Dragon Hatchling", Count: 4}, {Name: "Furnace Whelp", Count: 4},
			{Name: "Grizzly Bears", Count: 4}, {Name: "Llanowar Elves", Count: 3},
			{Name: "Lightning Bolt", Count: 4}, {Name: "Shock", Count: 4}, {Name: "Ornithopter", Count: 1},
			{Name: "Mountain", Count: 9}, {Name: "Forest", Count: 7},
		},
	}},
}

var krsTrio = map[string]bool{"Hamlet Glutton": true, "Ice Out": true, "Johann's Stopgap": true}
var krsDragon = map[string]bool{"Dragon Hatchling": true, "Furnace Whelp": true}
var krsPay = map[string]bool{"Iridescent Tiger": true, "Burning-Tree Emissary": true, "Llanowar Elves": true}

type krsTally struct {
	bargained     map[string]int // the trio cast bargained, by card
	discountOnly  int            // a bargained offer with no plain cast of the card beside it
	plainTrio     int            // a trio card cast plain while the bargained cast was offered
	brushReduced  int            // Brush Off cast at an instant or sorcery
	brushFull     int            // Brush Off cast at a creature spell
	preyDragon    int            // Dragon's Prey cast at a Dragon (raised)
	preyOther     int            // Dragon's Prey cast at a non-Dragon
	dragonDropped int            // a Dragon left out of Dragon's Prey's target menu
	seen          map[state.ObjID]bool
	ref           *GorgeSide
}

func newKrsTally() *krsTally {
	return &krsTally{bargained: map[string]int{}, seen: map[state.ObjID]bool{}}
}

func (c *krsTally) add(o *krsTally) {
	for k, v := range o.bargained {
		c.bargained[k] += v
	}
	c.discountOnly += o.discountOnly
	c.plainTrio += o.plainTrio
	c.brushReduced += o.brushReduced
	c.brushFull += o.brushFull
	c.preyDragon += o.preyDragon
	c.preyOther += o.preyOther
	c.dragonDropped += o.dragonDropped
}

func (c *krsTally) bargainedTotal() int {
	n := 0
	for _, v := range c.bargained {
		n += v
	}
	return n
}

func krsObj(g *GorgeSide, ref Ref) *state.Object {
	if g == nil || ref >= RefStack || int(ref) >= len(g.id.obj) {
		return nil
	}
	return g.E.G.Obj(g.id.obj[ref])
}

func krsObjName(o *state.Object) string {
	if o == nil || o.Card == nil || len(o.Card.Faces) == 0 {
		return ""
	}
	return o.Card.Faces[0].Name
}

// krsPolicy prefers, in order: the bargain election (a token half the time),
// the floating-mana creatures, an artifact or Food maker, a Brush Off or
// Dragon's Prey cast, a bargained cast of a trio card (three times in four),
// a land, then a random cast, activation or land.
func krsPolicy(seed uint64, c *krsTally) Policy {
	rnd := RandomPolicy(seed)
	prng := rand.New(rand.NewPCG(seed, 0x726564756365)) // "reduce"
	var buf []Canon
	return PolicyFunc(func(p *PickCtx) Canon {
		switch p.D.Kind {
		case DChoose:
			if code, ok := chooseCode("bargain"); ok {
				var picks []Canon
				for _, a := range p.D.Options {
					if a.Kind() == ActChoose+CanonKind(code) {
						picks = append(picks, a)
					}
				}
				if len(picks) > 0 {
					if prng.IntN(3) != 0 {
						for _, a := range picks {
							if kbargainIsToken(p.G, a.Src()) {
								return a
							}
						}
					}
					return picks[prng.IntN(len(picks))]
				}
			}
		case DAttackers:
			var atk []Canon
			for _, a := range p.D.Options {
				if a.Kind() == ActAttack {
					atk = append(atk, a)
				}
			}
			if len(atk) > 0 {
				return atk[prng.IntN(len(atk))]
			}
		case DTarget:
			c.noteTargets(p)
		case DPriority:
			for _, a := range p.D.Options {
				if a.Kind() == ActCast && krsPay[refCardName(p.G, a.Src())] {
					return a
				}
			}
			for _, a := range p.D.Options {
				if a.Kind() == ActCastFace && a.Mode()&0xf == faceCodeAdventure && refCardName(p.G, a.Src()) == "Curious Pair" {
					return a
				}
			}
			for _, a := range p.D.Options {
				if a.Kind() == ActCast && refCardName(p.G, a.Src()) == "Ornithopter" {
					return a
				}
			}
			for _, a := range p.D.Options {
				if n := refCardName(p.G, a.Src()); a.Kind() == ActCast && a.Mode() == 0 && (n == "Brush Off" || n == "Dragon's Prey") {
					return a
				}
			}
			if i := kbargainCastWhere(p.G, p.D, func(n string) bool { return krsTrio[n] }); i >= 0 {
				src := p.D.Options[i].Src()
				plain := -1
				for j, a := range p.D.Options {
					if a.Kind() == ActCast && a.Mode() == 0 && a.Src() == src {
						plain = j
						break
					}
				}
				if plain < 0 {
					c.discountOnly++
				} else if prng.IntN(2) == 0 {
					// spend a little of the pool first: a cheap spell leaves
					// the pool short of the plain price but not the bargained
					for _, a := range p.D.Options {
						if n := refCardName(p.G, a.Src()); a.Kind() == ActCast && a.Mode() == 0 && (n == "Shock" || n == "Lightning Bolt") {
							return a
						}
					}
				}
				name := refCardName(p.G, src)
				if plain < 0 || prng.IntN(4) != 0 {
					c.bargained[name]++
					return p.D.Options[i]
				}
				c.plainTrio++
				return p.D.Options[plain]
			}
			for _, a := range p.D.Options {
				if a.Kind() == ActPlayLand {
					return a
				}
			}
			buf = buf[:0]
			for _, a := range p.D.Options {
				switch a.Kind() {
				case ActCast, ActCastCasualty, ActActivate, ActPlayLand:
					buf = append(buf, a)
				}
			}
			if len(buf) > 0 {
				d := *p.D
				d.Options = buf
				return rnd.Pick(&PickCtx{D: &d, G: p.G, Seq: p.Seq})
			}
		}
		return rnd.Pick(p)
	})
}

// noteTargets counts a Dragon the target menu of the Dragon's Prey being
// announced leaves out while it is on the battlefield (the raised cost is
// unaffordable for it).
func (c *krsTally) noteTargets(p *PickCtx) {
	g := p.G
	if g == nil || len(g.E.G.Stack) == 0 {
		return
	}
	if krsObjName(g.E.G.Obj(g.E.G.Stack[len(g.E.G.Stack)-1])) != "Dragon's Prey" {
		return
	}
	offered := map[state.ObjID]bool{}
	for _, a := range p.D.Options {
		if a.Kind() == ActTarget {
			if o := krsObj(g, a.Tgt()); o != nil {
				offered[o.ID] = true
			}
		}
	}
	for pl := range 2 {
		for _, id := range g.E.G.Zone(state.ZBattlefield, state.PlayerID(pl)) {
			if krsDragon[krsObjName(g.E.G.Obj(id))] && !offered[id] {
				c.dragonDropped++
			}
		}
	}
}

// check reads the live gorge stack: the chosen target of each Brush Off and
// Dragon's Prey, counted once when the spell is first seen with a target.
func (c *krsTally) check(ref *GorgeSide, _ Side) *Divergence {
	c.ref = ref
	g := ref.E.G
	for _, id := range g.Stack {
		if c.seen[id] {
			continue
		}
		o := g.Obj(id)
		if o == nil || o.Ability != nil || len(o.Targets) == 0 {
			continue
		}
		t := g.Obj(o.Targets[0].Obj)
		if o.Targets[0].IsPlayer || t == nil {
			continue
		}
		switch krsObjName(o) {
		case "Brush Off":
			c.seen[id] = true
			if f := t.Face(); f != nil && (f.IsInstant() || f.IsSorcery()) {
				c.brushReduced++
			} else {
				c.brushFull++
			}
		case "Dragon's Prey":
			c.seen[id] = true
			if krsDragon[krsObjName(t)] {
				c.preyDragon++
			} else {
				c.preyOther++
			}
		}
	}
	return nil
}

type krsResult struct {
	games, divs, stalls int
	tally               *krsTally
	first               string
}

// krsRun plays seeds 1..n of match m in four workers.
func krsRun(t *testing.T, m krsMatch, n int) krsResult {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("reducespell")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range m.decks {
		for _, e := range d {
			card, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s deck %d: %s not in the corpus", m.name, s, e.Name)
			}
			for i := 0; i < e.Count; i++ {
				decks[s] = append(decks[s], card)
			}
		}
		if len(decks[s]) != 40 {
			t.Fatalf("%s deck %d: %d cards", m.name, s, len(decks[s]))
		}
	}
	const workers = 4
	var (
		mu  sync.Mutex
		wg  sync.WaitGroup
		res = krsResult{tally: newKrsTally()}
	)
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			tbl, _, _, err := set.Matchup(m.decks[0], m.decks[1])
			if err != nil {
				t.Errorf("worker %d: matchup: %v", w, err)
				return
			}
			c := newKrsTally()
			var games, divs, stalls int
			first := ""
			for seed := uint64(w + 1); seed <= uint64(n); seed += workers {
				cfg := Config{GorgeAutopaySurface: seed%2 == 0}
				mu2, err := NewMatchup(reg, [2]string{"krs-a", "krs-b"}, decks, 1)
				if err != nil {
					t.Errorf("seed %d: matchup: %v", seed, err)
					return
				}
				k, err := SetSide(set, tbl, cfg)(mu2, seed)
				if err != nil {
					t.Errorf("seed %d: side: %v", seed, err)
					return
				}
				c.seen = map[state.ObjID]bool{}
				r := Lockstep(mu2, seed, k, krsPolicy(seed, c),
					RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 60, Check: c.check})
				CloseSide(k)
				games++
				if r.Div != nil {
					divs++
					if first == "" {
						lo := max(r.Div.At-12, 0)
						first = formatDiv(seed, r, actionStrings(r.Actions[lo:r.Div.At])) + "\n" + c.optionNames(r.Div.KerOptions) + c.boardDump()
					}
				}
				if r.Stall != "" {
					stalls++
					if first == "" {
						first = "seed stalled: " + r.Stall
					}
				}
			}
			mu.Lock()
			res.games += games
			res.divs += divs
			res.stalls += stalls
			res.tally.add(c)
			if res.first == "" {
				res.first = first
			}
			mu.Unlock()
		}(w)
	}
	wg.Wait()
	return res
}

func formatDiv(seed uint64, r Result, last []string) string {
	return fmt.Sprintf("seed %d diverged at decision %d: %s %s %v %s; last actions %v\nref %v\nker %v",
		seed, r.Div.At, r.Div.What, r.Div.Field, r.Div.Diff, r.Div.Detail, last, r.Div.RefOptions, r.Div.KerOptions)
}

func TestReducespellLockstep(t *testing.T) {
	t.Parallel()
	for _, m := range krsMatches {
		res := krsRun(t, m, 192)
		c := res.tally
		t.Logf("%s: %d games, %d divergences, %d stalls; %+v", m.name, res.games, res.divs, res.stalls, *c)
		if res.divs != 0 || res.stalls != 0 {
			t.Errorf("%s: %d divergences, %d stalls; first: %s", m.name, res.divs, res.stalls, res.first)
		}
		var want []struct {
			name string
			got  int
		}
		if m.name == "M1" {
			// Hamlet Glutton ({3}{G}{G} bargained) and Ice Out ({U}{U}) cannot be
			// paid from the floating pool this surface reaches (an ETB-mana
			// creature floats one of each colour; the policy cannot tap lands
			// into the pool), and a bargained offer with no plain cast beside
			// it needs the lands tapped out too: TestReducespellBargainPrice
			// (package kernel) drives all three with an exact pool.
			want = append(want, struct {
				name string
				got  int
			}{"bargained cast of Johann's Stopgap", c.bargained["Johann's Stopgap"]})
		} else {
			want = []struct {
				name string
				got  int
			}{
				{"Brush Off at an instant or sorcery (reduced)", c.brushReduced},
				{"Brush Off at a creature spell (full price)", c.brushFull},
				{"Dragon's Prey at a Dragon (raised)", c.preyDragon},
				{"Dragon's Prey at a non-Dragon", c.preyOther},
				{"Dragon dropped from the Prey menu as unaffordable", c.dragonDropped},
			}
		}
		for _, a := range want {
			if a.got < 3 {
				t.Errorf("%s: %s: %d, want >= 3", m.name, a.name, a.got)
			}
		}
	}
}

// TestReducespellMutation: with the discount, the colour reduction and the
// Dragon raise all dropped, each matchup must diverge.
func TestReducespellMutation(t *testing.T) {
	dirac.DebugReduceSpellFlip = true
	defer func() { dirac.DebugReduceSpellFlip = false }()
	for _, m := range krsMatches {
		res := krsRun(t, m, 64)
		t.Logf("%s: %d games, %d divergences, %d stalls", m.name, res.games, res.divs, res.stalls)
		if res.divs == 0 {
			t.Errorf("%s: the mutation produced no divergence in %d games", m.name, res.games)
		}
	}
}

// optionNames names the objects a kernel option list mentions ("cast(#4,...").
func (c *krsTally) optionNames(opts []string) string {
	out := ""
	for _, o := range opts {
		if i := strings.Index(o, "#"); i >= 0 {
			if n, err := strconv.Atoi(strings.FieldsFunc(o[i+1:], func(r rune) bool { return r < '0' || r > '9' })[0]); err == nil {
				out += fmt.Sprintf(" %s=%s", o, krsObjName(krsObj(c.ref, Ref(n))))
			}
		}
	}
	return out
}

func (c *krsTally) boardDump() string {
	g := c.ref.E.G
	out := fmt.Sprintf("\nstack=%d", len(g.Stack))
	for pl := range 2 {
		out += fmt.Sprintf("\n p%d life=%d pool=%v:", pl, g.Players[pl].Life, g.Players[pl].Pool.Total())
		for _, id := range g.Zone(state.ZBattlefield, state.PlayerID(pl)) {
			o := g.Obj(id)
			out += fmt.Sprintf(" %s(tap=%v)", krsObjName(o), o.Tapped)
		}
		out += " HAND:"
		for _, id := range g.Zone(state.ZHand, state.PlayerID(pl)) {
			out += " " + krsObjName(g.Obj(id))
		}
	}
	return out
}
