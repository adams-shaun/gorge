package oracle

import (
	"math/rand/v2"
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The multi-pick charm sequential-ask lockstep (CR
// 601.2c): Trystan's Command, Atlantis Attacks and Amazing Acrobatics each
// carry a mode whose slot is "one or two targets" (1..2). gorge's
// askCharmModeTargets poses the single combined KTarget only when every
// chosen mode's slot is 1..1; a 1..2 slot beside another targeted mode takes
// askCharmSeqSlot instead: one KTarget per target-bearing mode, in chosen-mode
// order, each with its slot's own Min..Max clamped to its candidate count.
//
// Deck A (B/G) casts Trystan's Command over Elves, creatures that die into the
// graveyard; deck B (U/G) holds Amazing Acrobatics (a spell to counter, the
// creatures to tap) and Atlantis Attacks. The
// policy casts carriers first, answers the modes ask with Done one time in
// two, and takes a second target in a 1..2 slot three times in four; the tally
// reads gorge's decisions and event log.
//
// gorge at the pin never offers Atlantis Attacks' teamworked cast in these
// games, so its X is 1 (one mode): the card runs the ordinary 1..2 ask only.

var kcharmseqDecks = [2][]cardset.Entry{
	{
		{Name: "Trystan's Command", Count: 4}, {Name: "Llanowar Elves", Count: 4}, {Name: "Cylian Elf", Count: 2},
		{Name: "Treetop Warden", Count: 2}, {Name: "Willow Elf", Count: 2}, {Name: "Walking Corpse", Count: 3},
		{Name: "Grizzly Bears", Count: 3}, {Name: "Trained Armodon", Count: 2}, {Name: "Craw Wurm", Count: 1},
		{Name: "Forest", Count: 9}, {Name: "Swamp", Count: 8},
	},
	{
		{Name: "Amazing Acrobatics", Count: 4}, {Name: "Atlantis Attacks", Count: 3}, {Name: "Llanowar Elves", Count: 4},
		{Name: "Merfolk of the Pearl Trident", Count: 3}, {Name: "Coral Merfolk", Count: 3},
		{Name: "Trained Armodon", Count: 2}, {Name: "Grizzly Bears", Count: 2}, {Name: "Water Elemental", Count: 2},
		{Name: "Island", Count: 9}, {Name: "Forest", Count: 8},
	},
}

var kcharmseqCarrier = map[string]bool{"Trystan's Command": true, "Atlantis Attacks": true, "Amazing Acrobatics": true}

type kcharmseqTally struct {
	seqCasts int    // casts whose targets were asked one mode at a time
	twoPicks int    // sequential asks of a 1..2 slot that took two targets
	onePicks int    // sequential asks of a 1..2 slot that took one
	byCard   [3]int // seqCasts by carrier: Trystan's Command, Atlantis Attacks, Amazing Acrobatics
	modes    [3]int // mode announcements by carrier, the seqCasts' ModeChosen included

	// the sequential cast being walked
	seqSrc state.ObjID
	// the 1..2 ask in flight: its answer's TargetsChosen events are counted
	// until the next decision
	win    bool
	winSeq uint64
	winN   int
	tSeq   uint64
	evAt   int
}

func (c *kcharmseqTally) reset() {
	*c = kcharmseqTally{seqCasts: c.seqCasts, twoPicks: c.twoPicks, onePicks: c.onePicks, byCard: c.byCard, modes: c.modes}
}

func (c *kcharmseqTally) add(o *kcharmseqTally) {
	c.seqCasts += o.seqCasts
	c.twoPicks += o.twoPicks
	c.onePicks += o.onePicks
	for i := range c.byCard {
		c.byCard[i] += o.byCard[i]
		c.modes[i] += o.modes[i]
	}
}

func (c *kcharmseqTally) closeWindow() {
	if !c.win {
		return
	}
	c.win = false
	if c.winN >= 2 {
		c.twoPicks++
	} else if c.winN == 1 {
		c.onePicks++
	}
}

func (c *kcharmseqTally) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < c.evAt {
		c.evAt = 0
	}
	for j := c.evAt; j < len(evs); j++ {
		if c.win && evs[j].Kind == events.TargetsChosen {
			c.winN++
		}
		if evs[j].Kind == events.ModeChosen {
			if o := g.Obj(evs[j].Obj); o != nil && o.Ability == nil && o.Face() != nil {
				switch o.Face().Name {
				case "Trystan's Command":
					c.modes[0]++
				case "Atlantis Attacks":
					c.modes[1]++
				case "Amazing Acrobatics":
					c.modes[2]++
				}
			}
		}
	}
	c.evAt = len(evs)
	fr := ref.fr
	if c.win && (fr == nil || fr.d == nil || fr.d.Seq != c.winSeq) {
		c.closeWindow()
	}
	if fr == nil || fr.d == nil {
		return nil
	}
	switch string(fr.d.Kind) {
	case "target":
		if fr.d.Seq == c.tSeq {
			break
		}
		c.tSeq = fr.d.Seq
		if fr.d.ResumeKind == "charm_mode_seq" {
			if c.seqSrc != fr.d.Source {
				c.seqSrc = fr.d.Source
				c.seqCasts++
				if o := g.Obj(fr.d.Source); o != nil && o.Face() != nil {
					switch o.Face().Name {
					case "Trystan's Command":
						c.byCard[0]++
					case "Atlantis Attacks":
						c.byCard[1]++
					case "Amazing Acrobatics":
						c.byCard[2]++
					}
				}
			}
			if fr.d.Max == 2 {
				c.win, c.winSeq, c.winN = true, fr.d.Seq, 0
			}
		}
	case "priority":
		c.seqSrc = 0
	}
	return nil
}

// kcharmseqPolicy: the carriers before random plays (Amazing Acrobatics only
// with a spell on the stack to counter), Done one time in two on the modes
// ask, a second target in a multi-pick ask three times in four.
func kcharmseqPolicy(seed uint64, c *kcharmseqTally) Policy {
	rnd := RandomPolicy(seed)
	prng := rand.New(rand.NewPCG(seed, 0x636861726d73)) // "charms"
	var buf []Canon
	return PolicyFunc(func(p *PickCtx) Canon {
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
		if p.D.Kind == DTarget {
			done := -1
			buf = buf[:0]
			for i, a := range p.D.Options {
				switch a.Kind() {
				case ActTarget:
					buf = append(buf, a)
				case ActDone:
					done = i
				}
			}
			if done >= 0 && len(buf) > 0 {
				if prng.IntN(4) != 0 {
					return buf[prng.IntN(len(buf))]
				}
				return p.D.Options[done]
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
			// Amazing Acrobatics is held for a spell to counter
			held := func(a Canon) bool {
				return len(p.G.E.G.Stack) == 0 && refCardName(p.G, a.Src()) == "Amazing Acrobatics"
			}
			buf = buf[:0]
			for _, a := range p.D.Options {
				if a.Kind() == ActCast && a.Mode() == 0 && kcharmseqCarrier[refCardName(p.G, a.Src())] && !held(a) {
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
				case ActCast:
					if !held(a) {
						buf = append(buf, a)
					}
				case ActActivate, ActPlayLand:
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

type kcharmseqEnv struct {
	reg   *cards.Registry
	set   cardset.Set
	decks [2][]*cards.Card
}

func kcharmseqSetup(t *testing.T) *kcharmseqEnv {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("charmseq")
	if err != nil {
		t.Fatal(err)
	}
	env := &kcharmseqEnv{reg: reg, set: set}
	for s, d := range kcharmseqDecks {
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

func (env *kcharmseqEnv) game(seed uint64, tbl *kop.Table, c *kcharmseqTally) (Result, error) {
	cfg := Config{GorgeAutopaySurface: seed%2 == 0}
	m, err := NewMatchup(env.reg, [2]string{"kcharmseq-a", "kcharmseq-b"}, env.decks, 1)
	if err != nil {
		return Result{}, err
	}
	k, err := SetSide(env.set, tbl, cfg)(m, seed)
	if err != nil {
		return Result{}, err
	}
	c.reset()
	res := Lockstep(m, seed, k, kcharmseqPolicy(seed, c),
		RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 25, Check: c.check})
	c.closeWindow()
	CloseSide(k)
	return res, nil
}

func (env *kcharmseqEnv) matchup() (*kop.Table, error) {
	tbl, _, _, err := env.set.Matchup(kcharmseqDecks[0], kcharmseqDecks[1])
	return tbl, err
}

func TestCharmseqLockstep(t *testing.T) {
	t.Parallel()

	env := kcharmseqSetup(t)
	const workers = 4
	tallies := make([]*kcharmseqTally, workers)
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
			c := &kcharmseqTally{}
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
	c := &kcharmseqTally{}
	games := 0
	for w, wc := range tallies {
		if wc != nil {
			c.add(wc)
		}
		games += gameCounts[w]
	}
	t.Logf("%d games: sequential-ask casts %d (Trystan's Command %d, Atlantis Attacks %d, Amazing Acrobatics %d), 1..2 asks that took two targets %d, one target %d; mode announcements %v",
		games, c.seqCasts, c.byCard[0], c.byCard[1], c.byCard[2], c.twoPicks, c.onePicks, c.modes)
	if c.seqCasts < 20 {
		t.Errorf("casts asked one mode at a time: %d, want >= 20", c.seqCasts)
	}
	if c.twoPicks < 10 {
		t.Errorf("1..2 asks that took two targets: %d, want >= 10", c.twoPicks)
	}
}

// TestCharmseqMutation: with the combined ask posed over a 1..2 slot (one
// pick per mode) the lockstep must diverge.
func TestCharmseqMutation(t *testing.T) {
	if isolate(t) {
		return
	}

	env := kcharmseqSetup(t)
	tbl, err := env.matchup()
	if err != nil {
		t.Fatal(err)
	}
	dirac.DebugCharmSeqCombined = true
	defer func() { dirac.DebugCharmSeqCombined = false }()
	for seed := uint64(1); seed <= 128; seed++ {
		res, err := env.game(seed, tbl, &kcharmseqTally{})
		if err != nil {
			t.Fatal(err)
		}
		if res.Div != nil {
			t.Logf("seed %d diverged at decision %d: %s %s", seed, res.Div.At, res.Div.What, res.Div.Field)
			return
		}
	}
	t.Errorf("DebugCharmSeqCombined never diverged in 128 seeds")
}
