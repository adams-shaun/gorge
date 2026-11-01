package oracle

import (
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// CopyPermanent WithCountersType$ / WithCountersAmount$:
// Ochre Jelly dies with two or more +1/+1 counters and, at the next end
// step, its delayed program makes a token copy that enters with half that
// many counters, rounded down (CR 122.6, 614.1c, 608.2h, 707.2). The Jelly
// is cast for the largest payable X and sacrificed to Carrion Feeder and
// Village Rites; the token copy is a Jelly too, so it splits again.

func ochreJellyOpp() []cardset.Entry { return klkidelayedOpp }

func ochreJellyDeck(jelly string) []cardset.Entry {
	return []cardset.Entry{
		{Name: jelly, Count: 4}, {Name: "Carrion Feeder", Count: 4}, {Name: "Village Rites", Count: 4},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Llanowar Elves", Count: 4}, {Name: "Shock", Count: 2},
		{Name: "Forest", Count: 10}, {Name: "Swamp", Count: 8},
	}
}

// ochreJellyStats is what the gorge-side event log says the Jelly did.
type ochreJellyStats struct {
	splits   int // a token copy of an Ochre Jelly was created
	checked  int // splits whose source's counters were seen and whose token is still on the battlefield
	wrong    int // checked tokens whose +1/+1 counters are not half the source's, rounded down
	positive int // tokens that entered with at least one counter
	lastErr  string
}

type ochreJellyTally struct {
	evAt   int
	seen   map[state.ObjID]int32 // an Ochre Jelly on the battlefield -> its +1/+1 counters at the last decision
	src    state.ObjID           // the source of a CopyToken whose entering MoveZone has not come yet
	tokens []ochreTok
}

type ochreTok struct {
	id   state.ObjID
	half int32 // floor(the source's counters / 2); -1 when never seen
}

func isJelly(n string) bool { return n == "Ochre Jelly" || n == "A-Ochre Jelly" }

func (p *ochreJellyTally) scan(ref *GorgeSide, st *ochreJellyStats) {
	g := ref.E.G
	if p.seen == nil {
		p.seen = map[state.ObjID]int32{}
	}
	evs := ref.E.L.Events
	if len(evs) < p.evAt {
		p.evAt, p.src, p.tokens = 0, 0, nil
		clear(p.seen)
	}
	for j := p.evAt; j < len(evs); j++ {
		e := &evs[j]
		switch {
		case e.Kind == events.CopyToken && isJelly(chName(g, e.Obj)):
			p.src = e.Obj
			st.splits++
		case e.Kind == events.MoveZone && p.src != 0 && e.To == state.ZBattlefield && isJelly(chName(g, e.Obj)) && e.Obj != p.src:
			half := int32(-1)
			if n, ok := p.seen[p.src]; ok {
				half = n / 2
			}
			p.tokens = append(p.tokens, ochreTok{e.Obj, half})
			p.src = 0
		}
	}
	p.evAt = len(evs)
	// the tokens still in play now: their counters are the entry's
	for _, tk := range p.tokens {
		o := g.Obj(tk.id)
		if o == nil || o.Zone != state.ZBattlefield {
			continue // died before it was read; its split is checked as a source
		}
		n := o.Counter("P1P1")
		if n > 0 {
			st.positive++
		}
		if tk.half >= 0 {
			st.checked++
			if n != tk.half {
				st.wrong++
				st.lastErr = fmt.Sprintf("token %d has %d +1/+1 counters, its source had %d", tk.id, n, 2*tk.half)
			}
		}
	}
	p.tokens = p.tokens[:0]
	// observe the Jellies in play for the next split
	for pl := state.PlayerID(0); pl < 2; pl++ {
		for _, id := range g.Zone(state.ZBattlefield, pl) {
			if isJelly(chName(g, id)) {
				p.seen[id] = g.Obj(id).Counter("P1P1")
			}
		}
	}
}

// ochreJellyPolicy casts a Jelly for the largest payable X, feeds Jellies
// that hold two or more counters to the sacrifice outlets, and otherwise
// plays at random among casting, activating and playing lands.
func ochreJellyPolicy(seed uint64, st *ochreJellyStats) Policy {
	rnd := RandomPolicy(seed)
	prng := rand.New(rand.NewPCG(seed, 0x6f6368726a)) // "ochrj"
	var buf []Canon
	var tl ochreJellyTally
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			tl.scan(c.G, st)
		}
		var g *state.Game
		if c.G != nil {
			g = c.G.E.G
		}
		if c.G != nil && c.D.Kind != DPriority && c.D.Kind != DAttackers {
			best, bestN := Canon(0), -1
			for _, a := range c.D.Options {
				oa, ok := c.G.GorgeOption(a)
				if !ok {
					continue
				}
				switch {
				case oa.Kind == "x" && oa.Amount > bestN: // the largest payable X
					best, bestN = a, oa.Amount
				case oa.Obj != 0 && isJelly(chName(g, oa.Obj)) && g.Obj(oa.Obj).Counter("P1P1") >= 2 && bestN < 0 && oa.Kind != "x":
					best, bestN = a, 0 // sacrifice the Jelly
				}
			}
			if bestN >= 0 {
				return best
			}
		}
		switch c.D.Kind {
		case DAttackers:
			buf = buf[:0]
			for _, a := range c.D.Options {
				if a.Kind() == ActAttack {
					buf = append(buf, a)
				}
			}
			if len(buf) > 0 && prng.IntN(2) == 0 {
				return buf[prng.IntN(len(buf))]
			}
		case DPriority:
			if c.G != nil && prng.IntN(4) != 0 {
				buf = buf[:0]
				for _, a := range c.D.Options {
					if k := a.Kind(); k != ActCast && k != ActActivate {
						continue
					}
					oa, ok := c.G.GorgeOption(a)
					if !ok || oa.Obj == 0 {
						continue
					}
					switch n := chName(g, oa.Obj); {
					case isJelly(n), n == "Carrion Feeder" || n == "Village Rites":
						buf = append(buf, a)
					}
				}
				if len(buf) > 0 {
					return buf[prng.IntN(len(buf))]
				}
			}
			buf = buf[:0]
			for _, a := range c.D.Options {
				switch a.Kind() {
				case ActCast, ActActivate, ActPlayLand:
					buf = append(buf, a)
				}
			}
			if len(buf) > 0 {
				d := *c.D
				d.Options = buf
				return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
			}
		}
		return rnd.Pick(c)
	})
}

func runOchreJelly(t *testing.T, seeds int) (games, divs, stalls int, st ochreJellyStats) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("lkidelayed")
	if err != nil {
		t.Fatal(err)
	}
	for mi, jelly := range []string{"Ochre Jelly", "A-Ochre Jelly"} {
		md0 := [2][]cardset.Entry{ochreJellyDeck(jelly), ochreJellyOpp()}
		for swap := 0; swap < 2; swap++ {
			md := md0
			if swap == 1 {
				md = [2][]cardset.Entry{md0[1], md0[0]}
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
					t.Fatalf("matchup %d deck %d: %d cards", mi, s, len(decks[s]))
				}
			}
			tbl, _, _, err := set.Matchup(md[0], md[1])
			if err != nil {
				t.Fatal(err)
			}
			for seed := uint64(1); seed <= uint64(seeds); seed++ {
				cfg := Config{GorgeAutopaySurface: seed%2 == 0}
				m, err := NewMatchup(reg, [2]string{"ochrejelly-a", "ochrejelly-b"}, decks, 1)
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
				res := LockstepLed(m, seed, g, k, ochreJellyPolicy(seed, &st), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
				CloseSide(k)
				games++
				if len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
					stalls++
					t.Errorf("%s swap %d seed %d: kernel stall %q", jelly, swap, seed, res.Stall)
				}
				if d := res.Div; d != nil {
					divs++
					t.Errorf("%s swap %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", jelly, swap, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				}
			}
		}
	}
	return games, divs, stalls, st
}

func TestOchreJellyLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	// 48 seeds per seating, two Jelly names: 96 games each
	games, divs, stalls, st := runOchreJelly(t, 48)
	t.Logf("%d games (Ochre Jelly and A-Ochre Jelly, 48 seeds per seating), %d divergences, %d stalls", games, divs, stalls)
	t.Logf("%+v", st)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	if st.splits < 3 {
		t.Errorf("Ochre Jelly token copies: %d (want >= 3)", st.splits)
	}
	if st.positive < 3 {
		t.Errorf("tokens that entered with counters: %d (want >= 3)", st.positive)
	}
	if st.checked < 3 {
		t.Errorf("tokens whose counters were checked against the source's: %d (want >= 3)", st.checked)
	}
	if st.wrong != 0 {
		t.Errorf("%d tokens with the wrong counters, e.g. %s", st.wrong, st.lastErr)
	}
}
