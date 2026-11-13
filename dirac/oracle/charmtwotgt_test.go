package oracle

import (
	"math/rand/v2"
	"strings"
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The two-declaration charm mode lockstep (CR 601.2c): Epic
// Fight, Go Nuts! and HULK SMASH! each carry a mode whose root and chain link
// both declare a target ("target creature you control fights target creature
// an opponent controls"). gorge's combined ask covers the chosen modes' ROOT
// declarations only; the chain link is announced after it (castSubAskLinks),
// and a single targeted root takes the ordinary sequential root-then-chain
// ask. The decks are G/R creatures of several powers on both sides so a
// fight or a power-damage resolution kills either way, plus Darksteel Relic
// (a noncreature artifact) for HULK SMASH!'s Destroy mode.
//
// The policy takes the teamworked cast (the cast-mode nibble full) half the
// time for the two Teamwork carriers, and answers the modes ask with Done one
// time in two; the tally reads gorge's decisions and event log.

var kcharmtwotgtDecks = [2][]cardset.Entry{
	{
		{Name: "Epic Fight", Count: 3}, {Name: "Go Nuts!", Count: 3}, {Name: "HULK SMASH!", Count: 2},
		{Name: "Iridescent Tiger", Count: 2}, {Name: "Burning-Tree Emissary", Count: 3},
		{Name: "Grizzly Bears", Count: 3}, {Name: "Hill Giant", Count: 3}, {Name: "Llanowar Elves", Count: 2},
		{Name: "Craw Wurm", Count: 1}, {Name: "Gray Ogre", Count: 1}, {Name: "Darksteel Relic", Count: 2},
		{Name: "Forest", Count: 8}, {Name: "Mountain", Count: 7},
	},
	{
		{Name: "Epic Fight", Count: 2}, {Name: "Go Nuts!", Count: 2}, {Name: "HULK SMASH!", Count: 3},
		{Name: "Iridescent Tiger", Count: 2}, {Name: "Burning-Tree Emissary", Count: 3},
		{Name: "Grizzly Bears", Count: 2}, {Name: "Hill Giant", Count: 3}, {Name: "Gray Ogre", Count: 2},
		{Name: "Earth Elemental", Count: 1}, {Name: "Craw Wurm", Count: 1}, {Name: "Lightning Bolt", Count: 1},
		{Name: "Darksteel Relic", Count: 2},
		{Name: "Forest", Count: 8}, {Name: "Mountain", Count: 8},
	},
}

var kcharmtwotgtCarrier = map[string]bool{"Epic Fight": true, "Go Nuts!": true, "HULK SMASH!": true}
var kcharmtwotgtTeamwork = map[string]bool{"Go Nuts!": true, "HULK SMASH!": true}

// kcharmtwotgtPay is the ETB mana creatures, cast first so the floating pool
// prices a teamworked cast.
var kcharmtwotgtPay = map[string]bool{"Iridescent Tiger": true, "Burning-Tree Emissary": true}

type kcharmtwotgtTally struct {
	bothChain int // both modes chosen: the combined ask, then the chain ask
	seqChain  int // only the two-declaration mode: root then chain, never combined
	oneTgt    int // only the one-target mode
	kills     int // a creature died during a carrier's resolution
	badShape  int // a cast whose ask shape did not match its modes (the lockstep catches the kernel; this is gorge's own)

	inElect bool
	// the carrier cast being walked, from its ModeChosen to the next priority
	cur        bool
	curTwo     bool
	curOther   bool
	curNTgt    int
	curCombine bool
	tSeq       uint64
	// kill window: from the carrier's Resolve to the next priority decision (the
	// state-based actions that follow it included)
	resolving state.ObjID
	pending   map[state.ObjID]bool
	evAt      int
}

func (c *kcharmtwotgtTally) reset() {
	*c = kcharmtwotgtTally{bothChain: c.bothChain, seqChain: c.seqChain, oneTgt: c.oneTgt, kills: c.kills, badShape: c.badShape,
		pending: map[state.ObjID]bool{}}
}

func (c *kcharmtwotgtTally) add(o *kcharmtwotgtTally) {
	c.bothChain += o.bothChain
	c.seqChain += o.seqChain
	c.oneTgt += o.oneTgt
	c.kills += o.kills
	c.badShape += o.badShape
}

// finish classifies the cast walked since its ModeChosen.
func (c *kcharmtwotgtTally) finish() {
	if !c.cur {
		return
	}
	c.cur = false
	switch {
	case c.curTwo && c.curOther:
		if c.curCombine && c.curNTgt >= 3 {
			c.bothChain++
		} else if c.curNTgt >= 3 || c.curCombine {
			c.badShape++
		}
	case c.curTwo:
		if !c.curCombine && c.curNTgt >= 2 {
			c.seqChain++
		}
	case c.curOther:
		if c.curNTgt >= 1 {
			c.oneTgt++
		}
	}
}

// kcharmtwotgtFights is the mode text of the two-declaration mode.
func kcharmtwotgtFights(text string) bool {
	return strings.Contains(text, "fights") || strings.Contains(text, "equal to its power")
}

// kcharmtwotgtOther is the mode text of the one-target mode.
func kcharmtwotgtOther(text string) bool {
	return strings.Contains(text, "Double target") || strings.Contains(text, "+1/+1 counter") || strings.Contains(text, "Destroy target")
}

func (c *kcharmtwotgtTally) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < c.evAt {
		c.evAt = 0
	}
	for j := c.evAt; j < len(evs); j++ {
		e := &evs[j]
		switch e.Kind {
		case events.ModeChosen:
			c.finish()
			if o := g.Obj(e.Obj); o != nil && o.Ability == nil && o.Face() != nil && kcharmtwotgtCarrier[o.Face().Name] {
				c.cur, c.curNTgt, c.curCombine = true, 0, false
				c.curTwo, c.curOther = kcharmtwotgtFights(e.Text), kcharmtwotgtOther(e.Text)
				c.pending[e.Obj] = true
			}
		case events.Resolve:
			if c.pending[e.Obj] {
				c.resolving = e.Obj
			}
		case events.MoveZone:
			if c.resolving != 0 && e.From == state.ZBattlefield && e.To == state.ZGraveyard {
				if o := g.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().IsCreature() {
					c.kills++
				}
			}
		}
	}
	c.evAt = len(evs)
	if fr := ref.fr; fr != nil && fr.d != nil {
		switch string(fr.d.Kind) {
		case "target":
			if c.cur && fr.d.Seq != c.tSeq {
				c.tSeq = fr.d.Seq
				if fr.d.ResumeKind == "charm_targets" {
					c.curCombine = true
					c.curNTgt += 2 // the combined ask announces both roots
				} else {
					c.curNTgt++
				}
			}
		case "priority":
			c.finish()
			c.resolving = 0 // the state-based actions after the resolution ran before this priority
		}
	}
	return nil
}

// kcharmtwotgtPolicy: the ETB mana creatures first, the teamworked cast of a
// Teamwork carrier half the time, the carriers before random plays, Done one
// time in two on the modes ask.
func kcharmtwotgtPolicy(seed uint64, c *kcharmtwotgtTally) Policy {
	rnd := RandomPolicy(seed)
	prng := rand.New(rand.NewPCG(seed, 0x636861726d32)) // "charm2"
	var buf []Canon
	return PolicyFunc(func(p *PickCtx) Canon {
		if p.D.Kind == DChoose {
			if code, ok := chooseCode("teamwork"); ok {
				var picks []Canon
				done := -1
				for i, a := range p.D.Options {
					switch {
					case a.Kind() == ActChoose+CanonKind(code):
						picks = append(picks, a)
					case a.Kind() == ActDone:
						done = i
					}
				}
				if len(picks) > 0 {
					first := !c.inElect
					c.inElect = true
					if done >= 0 && first && prng.IntN(4) == 0 {
						c.inElect = false
						return p.D.Options[done]
					}
					if done >= 0 && !first && prng.IntN(2) == 0 {
						c.inElect = false
						return p.D.Options[done]
					}
					return picks[prng.IntN(len(picks))]
				}
			}
		}
		c.inElect = false
		if p.D.Kind == DModes {
			done := -1
			buf = buf[:0]
			for i, a := range p.D.Options {
				switch a.Kind() {
				case ActMode:
					buf = append(buf, a)
				case ActDone:
					done = i
				}
			}
			if done >= 0 && (len(buf) == 0 || prng.IntN(2) == 0) {
				return p.D.Options[done]
			}
			if len(buf) > 0 {
				return buf[prng.IntN(len(buf))]
			}
		}
		if p.D.Kind == DAttackers {
			var atk []Canon
			for _, a := range p.D.Options {
				if a.Kind() == ActAttack {
					atk = append(atk, a)
				}
			}
			if len(atk) > 0 {
				return atk[prng.IntN(len(atk))]
			}
		}
		if p.D.Kind == DPriority {
			for i, a := range p.D.Options {
				if a.Kind() != ActCast || a.Mode()>>4 != 0 || a.Mode()&0xf != kteamworkCastOther ||
					!kcharmtwotgtTeamwork[refCardName(p.G, a.Src())] {
					continue
				}
				for _, b := range p.D.Options {
					if b.Kind() == ActCast && b.Mode() == 0 && b.Src() == a.Src() {
						if prng.IntN(2) == 0 {
							return b
						}
						break
					}
				}
				return p.D.Options[i]
			}
			for _, a := range p.D.Options {
				if a.Kind() == ActCast && kcharmtwotgtPay[refCardName(p.G, a.Src())] {
					return a
				}
			}
			buf = buf[:0]
			for _, a := range p.D.Options {
				if a.Kind() == ActCast && a.Mode() == 0 && kcharmtwotgtCarrier[refCardName(p.G, a.Src())] {
					buf = append(buf, a)
				}
			}
			if len(buf) > 0 && prng.IntN(4) != 0 {
				return buf[prng.IntN(len(buf))]
			}
			for _, a := range p.D.Options {
				if a.Kind() == ActPlayLand {
					return a
				}
			}
			buf = buf[:0]
			for _, a := range p.D.Options {
				switch a.Kind() {
				case ActCast, ActActivate, ActPlayLand:
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

type kcharmtwotgtEnv struct {
	reg   *cards.Registry
	set   cardset.Set
	decks [2][]*cards.Card
	tbl   *kop.Table
}

func kcharmtwotgtSetup(t *testing.T) *kcharmtwotgtEnv {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("charmtwotgt")
	if err != nil {
		t.Fatal(err)
	}
	env := &kcharmtwotgtEnv{reg: reg, set: set}
	for s, d := range kcharmtwotgtDecks {
		for _, e := range d {
			card, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("deck %d: %s not in the corpus", s, e.Name)
			}
			for i := 0; i < e.Count; i++ {
				env.decks[s] = append(env.decks[s], card)
			}
		}
		if len(env.decks[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(env.decks[s]))
		}
	}
	return env
}

func (env *kcharmtwotgtEnv) game(seed uint64, tbl *kop.Table, c *kcharmtwotgtTally) (Result, error) {
	cfg := Config{GorgeAutopaySurface: seed%2 == 0}
	m, err := NewMatchup(env.reg, [2]string{"kcharmtwotgt-a", "kcharmtwotgt-b"}, env.decks, 1)
	if err != nil {
		return Result{}, err
	}
	k, err := SetSide(env.set, tbl, cfg)(m, seed)
	if err != nil {
		return Result{}, err
	}
	c.reset()
	res := Lockstep(m, seed, k, kcharmtwotgtPolicy(seed, c),
		RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20, Check: c.check})
	CloseSide(k)
	return res, nil
}

func (env *kcharmtwotgtEnv) matchup() (*kop.Table, error) {
	tbl, _, _, err := env.set.Matchup(kcharmtwotgtDecks[0], kcharmtwotgtDecks[1])
	return tbl, err
}

func TestCharmtwotgtLockstep(t *testing.T) {
	t.Parallel()

	env := kcharmtwotgtSetup(t)
	const workers = 4
	tallies := make([]*kcharmtwotgtTally, workers)
	gameCounts := make([]int, workers)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			tbl, err := env.matchup()
			if err != nil {
				t.Errorf("worker %d: matchup: %v", w, err)
				return
			}
			c := &kcharmtwotgtTally{}
			for seed := uint64(w + 1); seed <= 256; seed += workers {
				res, err := env.game(seed, tbl, c)
				if err != nil {
					t.Errorf("seed %d: %v", seed, err)
					return
				}
				gameCounts[w]++
				if res.Div != nil {
					lo := res.Div.At - 12
					if lo < 0 {
						lo = 0
					}
					t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:res.Div.At]))
					t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v",
						seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Div.Detail,
						res.Div.RefOptions, res.Div.KerOptions)
				}
				if res.Stall != "" && res.Stall != "turns" && res.Stall != "decisions" {
					t.Errorf("seed %d: stalled: %s after %d turns", seed, res.Stall, res.Turns)
				}
			}
			tallies[w] = c
		}(w)
	}
	wg.Wait()
	c := &kcharmtwotgtTally{}
	games := 0
	for w, wc := range tallies {
		if wc != nil {
			c.add(wc)
		}
		games += gameCounts[w]
	}
	t.Logf("%d games: both modes (combined then chain) %d, only the two-declaration mode (root then chain) %d, only the one-target mode %d, kills during a carrier's resolution %d, mis-shaped asks %d",
		games, c.bothChain, c.seqChain, c.oneTgt, c.kills, c.badShape)
	for _, a := range []struct {
		name string
		got  int
		want int
	}{
		{"casts with both modes, combined ask then chain ask", c.bothChain, 20},
		{"casts with only the two-declaration mode, sequential", c.seqChain, 20},
		{"casts with only the one-target mode", c.oneTgt, 10},
		{"creature deaths in a carrier's resolution", c.kills, 10},
	} {
		if a.got < a.want {
			t.Errorf("%s: %d, want >= %d", a.name, a.got, a.want)
		}
	}
	if c.badShape != 0 {
		t.Errorf("%d casts whose gorge ask shape was neither combined+chain nor sequential", c.badShape)
	}
}

// TestCharmtwotgtMutation: with the chain link inside the combined ask
// (today's behaviour before the test) the lockstep must diverge.
func TestCharmtwotgtMutation(t *testing.T) {
	if isolate(t) {
		return
	}

	env := kcharmtwotgtSetup(t)
	tbl, err := env.matchup()
	if err != nil {
		t.Fatal(err)
	}
	dirac.DebugCharmTwoTgtAllSlots = true
	defer func() { dirac.DebugCharmTwoTgtAllSlots = false }()
	for seed := uint64(1); seed <= 64; seed++ {
		res, err := env.game(seed, tbl, &kcharmtwotgtTally{})
		if err != nil {
			t.Fatal(err)
		}
		if res.Div != nil {
			t.Logf("seed %d diverged at decision %d: %s %s", seed, res.Div.At, res.Div.What, res.Div.Field)
			return
		}
	}
	t.Errorf("DebugCharmTwoTgtAllSlots never diverged in 64 seeds")
}
