package oracle

import (
	"strings"
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The TriggeredAttackerLKICopy condition lockstep (gorge
// ): the five Theros Ordeals ("put a +1/+1 counter on it. Then if it
// has three or more +1/+1 counters on it, sacrifice CARDNAME") and Viv Vision
// ("draw a card if her power is 4 or greater"). The condition reads the single
// attacker the Attacks trigger captured, as it stands after the Ordeal's own
// counter (CR 608.2c) and after any pump. Bone Sabres (four counters) gets
// Viv to power 4.
var ordealDecks = [][2][]cardset.Entry{
	{
		{
			{Name: "Ordeal of Heliod", Count: 4}, {Name: "Ordeal of Erebos", Count: 4},
			{Name: "Viv Vision, Teen Synthezoid", Count: 4}, {Name: "Bone Sabres", Count: 2},
			{Name: "Grizzly Bears", Count: 4}, {Name: "Gray Ogre", Count: 4},
			{Name: "Plains", Count: 9}, {Name: "Swamp", Count: 9},
		},
		{
			{Name: "Ordeal of Nylea", Count: 4}, {Name: "Ordeal of Thassa", Count: 4},
			{Name: "Viv Vision, Teen Synthezoid", Count: 4}, {Name: "Bone Sabres", Count: 2},
			{Name: "Centaur Courser", Count: 4}, {Name: "Hill Giant", Count: 4},
			{Name: "Forest", Count: 9}, {Name: "Island", Count: 9},
		},
	},
	{
		{
			{Name: "Ordeal of Purphoros", Count: 4}, {Name: "Ordeal of Nylea", Count: 4},
			{Name: "Viv Vision, Teen Synthezoid", Count: 4}, {Name: "Bone Sabres", Count: 2},
			{Name: "Raging Goblin", Count: 4}, {Name: "Goblin Piker", Count: 4},
			{Name: "Mountain", Count: 9}, {Name: "Forest", Count: 9},
		},
		{
			{Name: "Ordeal of Heliod", Count: 4}, {Name: "Ordeal of Thassa", Count: 4},
			{Name: "Viv Vision, Teen Synthezoid", Count: 4}, {Name: "Bone Sabres", Count: 2},
			{Name: "Grizzly Bears", Count: 4}, {Name: "Gray Ogre", Count: 4},
			{Name: "Plains", Count: 9}, {Name: "Island", Count: 9},
		},
	},
}

type ordealStats struct {
	sacrificed int // an Ordeal left with its creature at three or more +1/+1 counters
	notYet     int // an Ordeal's trigger added a counter and the Ordeal stayed
	vivDraw    int // Viv attacked at power 4+ and its controller drew
	vivNoDraw  int // Viv attacked below power 4 and nothing was drawn
}

type ordealSeen struct {
	att state.ObjID
	ctr int32
}

// ordealPolicy plays cast/activate/land/attack options at random (passing
// when none is legal) and counts the condition's two outcomes on each carrier
// from the gorge side's state between decisions.
func ordealPolicy(seed uint64, st *ordealStats) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	ordeals := map[state.ObjID]ordealSeen{}
	var viv state.ObjID
	vivWait := false
	var vivLib int
	prevAtk := false
	prevLib := 0
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			g := c.G.E.G
			live := map[state.ObjID]bool{}
			for p := state.PlayerID(0); p < 2; p++ {
				for _, id := range g.Zone(state.ZBattlefield, p) {
					ob := g.Obj(id)
					if ob == nil || ob.Face() == nil {
						continue
					}
					live[id] = true
					switch n := ob.Face().Name; {
					case strings.HasPrefix(n, "Ordeal of"):
						cur := ordealSeen{att: ob.AttachedTo}
						if cur.att != 0 {
							if a := g.Obj(cur.att); a != nil {
								cur.ctr = a.Counter("P1P1")
							}
						}
						if old, ok := ordeals[id]; ok && old.att == cur.att && cur.ctr > old.ctr {
							st.notYet++
						}
						ordeals[id] = cur
					case strings.HasPrefix(n, "Viv Vision"):
						viv = id
					}
				}
			}
			for id, old := range ordeals {
				if live[id] {
					continue
				}
				if a := g.Obj(old.att); a != nil && a.Zone == state.ZBattlefield && a.Counter("P1P1") >= 3 {
					st.sacrificed++
				}
				delete(ordeals, id)
			}
			if vo := g.Obj(viv); viv != 0 && live[viv] && vo != nil {
				lib := len(g.Zone(state.ZLibrary, vo.Controller))
				switch {
				case vo.IsAttacking && !prevAtk:
					vivWait, vivLib = true, prevLib
				case vo.IsAttacking && vivWait && len(g.Stack) == 0:
					pw := c.G.E.Power(viv)
					switch {
					case lib < vivLib && pw >= 4:
						st.vivDraw++
					case lib == vivLib && pw < 4:
						st.vivNoDraw++
					}
					vivWait = false
				case !vo.IsAttacking:
					vivWait = false
				}
				prevAtk, prevLib = vo.IsAttacking, lib
			} else {
				vivWait, prevAtk = false, false
			}
		}
		buf = buf[:0]
		for _, a := range c.D.Options {
			switch a.Kind() {
			case ActCast, ActActivate, ActPlayLand, ActAttack:
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

type ordealPrep struct {
	decks [2][]*cards.Card
	tbl   *kop.Table
}

func prepareOrdeal(t *testing.T, reg *cards.Registry, set cardset.Set) []ordealPrep {
	t.Helper()
	out := make([]ordealPrep, len(ordealDecks))
	for mi, md := range ordealDecks {
		for s, d := range md {
			for _, e := range d {
				c, ok := reg.Lookup(e.Name)
				if !ok {
					t.Fatalf("%s not in the corpus", e.Name)
				}
				for j := 0; j < e.Count; j++ {
					out[mi].decks[s] = append(out[mi].decks[s], c)
				}
			}
			if len(out[mi].decks[s]) != 40 {
				t.Fatalf("matchup %d deck %d: %d cards", mi, s, len(out[mi].decks[s]))
			}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		out[mi].tbl = tbl
	}
	return out
}

// runOrdeal plays seeds [1, seeds] of every matchup and returns the first
// divergence's description ("" when none) with the summed statistics.
func runOrdeal(t *testing.T, reg *cards.Registry, set cardset.Set, prep []ordealPrep, seeds int, stopAtDiv bool) (string, ordealStats) {
	t.Helper()
	var mu sync.Mutex
	var wg sync.WaitGroup
	var total ordealStats
	div := ""
	for mi := range prep {
		wg.Add(1)
		go func(mi int) {
			defer wg.Done()
			var st ordealStats
			p := prep[mi]
			for seed := uint64(1); seed <= uint64(seeds); seed++ {
				cfg := Config{GorgeAutopaySurface: seed%2 == 0}
				m, err := NewMatchup(reg, [2]string{"ordeal-a", "ordeal-b"}, p.decks, 1)
				if err != nil {
					t.Errorf("matchup %d seed %d: %v", mi, seed, err)
					return
				}
				k, err := SetSide(set, p.tbl, cfg)(m, seed)
				if err != nil {
					t.Errorf("matchup %d seed %d: %v", mi, seed, err)
					return
				}
				res := Lockstep(m, seed, k, ordealPolicy(seed, &st), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
				CloseSide(k)
				if d := res.Div; d != nil {
					lo := d.At - 12
					if lo < 0 {
						lo = 0
					}
					mu.Lock()
					if div == "" {
						div = strings.TrimSpace(d.What + " " + d.Field)
					}
					mu.Unlock()
					if !stopAtDiv {
						t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
						t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
					}
					return
				}
			}
			mu.Lock()
			total.sacrificed += st.sacrificed
			total.notYet += st.notYet
			total.vivDraw += st.vivDraw
			total.vivNoDraw += st.vivNoDraw
			mu.Unlock()
		}(mi)
	}
	wg.Wait()
	return div, total
}

func ordealSetup(t *testing.T) (*cards.Registry, cardset.Set) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("ordeal")
	if err != nil {
		t.Fatal(err)
	}
	return reg, set
}

func TestOrdealLockstep(t *testing.T) {
	t.Parallel()

	reg, set := ordealSetup(t)
	prep := prepareOrdeal(t, reg, set)
	_, st := runOrdeal(t, reg, set, prep, 512, false)
	t.Logf("1024 games: sacrifice-at-three %d, not-yet %d, Viv draw %d, Viv no-draw %d", st.sacrificed, st.notYet, st.vivDraw, st.vivNoDraw)
	if st.sacrificed < 10 {
		t.Errorf("Ordeal sacrificed at three counters %d times, want at least 10", st.sacrificed)
	}
	if st.notYet < 10 {
		t.Errorf("Ordeal trigger below three counters %d times, want at least 10", st.notYet)
	}
	if st.vivDraw < 10 {
		t.Errorf("Viv Vision drew at power 4 %d times, want at least 10", st.vivDraw)
	}
	if st.vivNoDraw < 10 {
		t.Errorf("Viv Vision attacked below power 4 without drawing %d times, want at least 10", st.vivNoDraw)
	}
}

// TestOrdealMutation: with every RefMatches condition of the table inverted
// (the Ordeal's gate and Viv's power check both answer the other way) the
// same games diverge.
func TestOrdealMutation(t *testing.T) {
	t.Parallel()

	reg, set := ordealSetup(t)
	prep := prepareOrdeal(t, reg, set)
	flipped := 0
	for i := range prep {
		cs := append([]kop.Cond(nil), prep[i].tbl.Conds...)
		for j := range cs {
			if cs[j].Kind == kop.CondRefMatches {
				cs[j].Neg = !cs[j].Neg
				flipped++
			}
		}
		tbl := *prep[i].tbl
		tbl.Conds = cs
		prep[i].tbl = &tbl
	}
	if flipped == 0 {
		t.Fatal("no CondRefMatches condition in the ordeal table")
	}
	div, _ := runOrdeal(t, reg, set, prep, 16, true)
	if div == "" {
		t.Fatal("inverting the TriggeredAttackerLKICopy condition produced no divergence")
	}
	t.Logf("mutation diverged: %s", div)
}
