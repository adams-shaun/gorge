package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The shuffle-choice lockstep: constructed matchups over the generated
// oblige pool -- the multi-object graveyard-to-library shuffles, mandatory
// and may-shuffle, self-owned and parent-player-restricted -- so the
// expected answer at every decision is gorge's.

var obligeDecks = [][2][]cardset.Entry{{
	{
		{Name: "Krosan Reclamation", Count: 3}, {Name: "Oracle of Tragedy", Count: 3},
		{Name: "Put Away", Count: 3}, {Name: "Witness the Future", Count: 3},
		{Name: "Wand of Vertebrae", Count: 3}, {Name: "Cathartic Parting", Count: 2},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Island", Count: 10},
		{Name: "Forest", Count: 9},
	},
	{
		{Name: "Krosan Reclamation", Count: 3}, {Name: "Oracle of Tragedy", Count: 3},
		{Name: "Put Away", Count: 3}, {Name: "Witness the Future", Count: 3},
		{Name: "Wand of Vertebrae", Count: 3}, {Name: "Cathartic Parting", Count: 2},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Island", Count: 10},
		{Name: "Forest", Count: 9},
	},
}}

// obligeDecksDeclined is the same matchup without Cathartic Parting: its
// zero-target Put Away decline is the separate proof TestObligeDeclinedZeroTargets
// needs, and the extra may-shuffle carrier changes the draws enough to hide it.
var obligeDecksDeclined = [][2][]cardset.Entry{{
	{
		{Name: "Krosan Reclamation", Count: 3}, {Name: "Oracle of Tragedy", Count: 3},
		{Name: "Put Away", Count: 3}, {Name: "Witness the Future", Count: 3},
		{Name: "Wand of Vertebrae", Count: 3}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Island", Count: 11}, {Name: "Forest", Count: 10},
	},
	{
		{Name: "Krosan Reclamation", Count: 3}, {Name: "Oracle of Tragedy", Count: 3},
		{Name: "Put Away", Count: 3}, {Name: "Witness the Future", Count: 3},
		{Name: "Wand of Vertebrae", Count: 3}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Island", Count: 11}, {Name: "Forest", Count: 10},
	},
}}

// obligeCarriers names the cards whose casts and activations the test counts
// (each must be played at least once).
var obligeCarriers = []string{"Krosan Reclamation", "Oracle of Tragedy", "Put Away", "Witness the Future", "Wand of Vertebrae", "Cathartic Parting"}

type obligeCounts struct {
	played      map[string]int
	shuffleYes  int
	shuffleNo   int
	shuffleAsks int
	// targetCards counts completed target decisions by the number of card
	// targets chosen (0 is an "up to N" declined at zero). In this pool the
	// only multi-card target decisions are the shuffle ChangeZones.
	targetCards map[int]int
	// zeroTargetNo counts a may-shuffle election declined after zero card
	// targets were chosen: the whole move, and its shuffle, must be a no-op.
	zeroTargetNo int
}

// obligePolicy casts or activates whenever it can (so the games reach the
// carriers), else random. It forces the may-shuffle election to one side so
// both the move-and-shuffle and the decline paths run, and counts the
// carrier plays the gorge side poses.
func obligePolicy(seed uint64, counts *obligeCounts, forceYes bool) Policy {
	rnd := RandomPolicy(seed)
	yesCode, _ := chooseCode("yes")
	noCode, _ := chooseCode("no")
	var buf []Canon
	inTarget := false
	curCards := 0
	lastTargetCards := -1
	flushTarget := func() {
		if inTarget {
			counts.targetCards[curCards]++
			lastTargetCards = curCards
			inTarget = false
			curCards = 0
		}
	}
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.D.Kind != DTarget {
			flushTarget()
		}
		var yes, no Canon
		hasYes, hasNo := false, false
		for _, a := range c.D.Options {
			switch a.Kind() {
			case ActChoose + CanonKind(yesCode):
				yes, hasYes = a, true
			case ActChoose + CanonKind(noCode):
				no, hasNo = a, true
			}
		}
		if hasYes && hasNo {
			counts.shuffleAsks++
			if lastTargetCards == 0 && !forceYes {
				counts.zeroTargetNo++
			}
			if forceYes {
				counts.shuffleYes++
				return yes
			}
			counts.shuffleNo++
			return no
		}
		buf = buf[:0]
		for _, a := range c.D.Options {
			if k := a.Kind(); k == ActCast || k == ActActivate {
				buf = append(buf, a)
			}
		}
		var a Canon
		if len(buf) == 0 {
			a = rnd.Pick(c)
		} else {
			d := *c.D
			d.Options = buf
			a = rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		}
		if c.D.Kind == DTarget {
			switch {
			case a.Kind() == ActTarget && a.Tgt() < RefStack:
				inTarget = true
				curCards++
			case a.Kind() == ActDone:
				counts.targetCards[curCards]++
				lastTargetCards = curCards
				inTarget = false
				curCards = 0
			}
		}
		if k := a.Kind(); (k == ActCast || k == ActActivate) && c.G != nil {
			if o, ok := c.G.GorgeOption(a); ok {
				if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil {
					counts.played[ob.Face().Name]++
				}
			}
		}
		return a
	})
}

func TestObligeLockstep(t *testing.T) {
	t.Parallel()

	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("oblige")
	if err != nil {
		t.Fatal(err)
	}
	games := 0
	counts := &obligeCounts{played: map[string]int{}, targetCards: map[int]int{}}
	for mi, md := range obligeDecks {
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
		for seed := uint64(1); seed <= 48; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"oblige-a", "oblige-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			pol := obligePolicy(seed, counts, seed%2 == 1)
			res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
			CloseSide(k)
			games++
			if d := res.Div; d != nil {
				lo := d.At - 12
				if lo < 0 {
					lo = 0
				}
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	t.Logf("%d games, carrier plays %v, shuffle asks %d (yes %d, no %d), target-card counts %v, declined zero-target %d", games, counts.played, counts.shuffleAsks, counts.shuffleYes, counts.shuffleNo, counts.targetCards, counts.zeroTargetNo)
	for _, n := range obligeCarriers {
		if counts.played[n] == 0 {
			t.Errorf("carrier %s never played", n)
		}
	}
	if counts.shuffleYes == 0 {
		t.Error("may-shuffle election never accepted")
	}
	if counts.shuffleNo == 0 {
		t.Error("may-shuffle election never declined")
	}
	for _, n := range []int{2, 4} {
		if counts.targetCards[n] == 0 {
			t.Errorf("no target decision with %d card targets (N=%d carrier never fired)", n, n)
		}
	}
}

// obligeMutateSide flips the kernel's may-shuffle "yes" to "no" behind the
// canonical surface. The lockstep must report a divergence: the mutation
// test proves the shuffle-election comparison is live, not vacuous.
type obligeMutateSide struct {
	*realSide
	flips int
}

func (m *obligeMutateSide) Apply(c Canon) {
	yesCode, _ := chooseCode("yes")
	noCode, _ := chooseCode("no")
	if m.realSide.d.Kind == dirac.DkChoose && m.realSide.d.Sub == dirac.ChGenConfirm &&
		c.Kind() == ActChoose+CanonKind(yesCode) {
		m.flips++
		c = Pack(ActChoose+CanonKind(noCode), RefNone, RefNone, 0)
	}
	m.realSide.Apply(c)
}

func TestObligeMutationDiverges(t *testing.T) {
	t.Parallel()

	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("oblige")
	if err != nil {
		t.Fatal(err)
	}
	md := obligeDecks[0]
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
	}
	tbl, _, _, err := set.Matchup(md[0], md[1])
	if err != nil {
		t.Fatal(err)
	}
	counts := &obligeCounts{played: map[string]int{}, targetCards: map[int]int{}}
	for seed := uint64(1); seed <= 48; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"oblige-a", "oblige-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		side, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		mut := &obligeMutateSide{realSide: side.(*realSide)}
		pol := obligePolicy(seed, counts, seed%2 == 1)
		res := Lockstep(m, seed, mut, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		CloseSide(mut)
		if mut.flips == 0 {
			continue
		}
		if res.Div == nil {
			t.Fatalf("seed %d: flipped %d may-shuffle answers but no divergence", seed, mut.flips)
		}
		t.Logf("seed %d: flipped %d may-shuffle answer(s), diverged at decision %d (%s/%s)", seed, mut.flips, res.Div.At, res.Div.What, res.Div.Field)
		return
	}
	t.Fatal("no seed flipped a may-shuffle answer")
}

// TestObligeDeclinedZeroTargets is the separate proof of the may-shuffle
// decline: Put Away's "up to one" ChangeZone elects zero card targets, then
// the may-shuffle ask is declined. gorge's objectPathMayShuffle returns
// before any move, so the whole resolution must be a no-op; the lockstep's
// projection comparison is the proof that the kernel did nothing either.
func TestObligeDeclinedZeroTargets(t *testing.T) {
	t.Parallel()

	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("oblige")
	if err != nil {
		t.Fatal(err)
	}
	md := obligeDecksDeclined[0]
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
	}
	tbl, _, _, err := set.Matchup(md[0], md[1])
	if err != nil {
		t.Fatal(err)
	}
	counts := &obligeCounts{played: map[string]int{}, targetCards: map[int]int{}}
	for seed := uint64(1); seed <= 48; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"oblige-a", "oblige-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		pol := obligePolicy(seed, counts, false)
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		CloseSide(k)
		if d := res.Div; d != nil {
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s", seed, d.At, d.What, d.Field, d.Diff, d.Detail)
		}
	}
	if counts.zeroTargetNo == 0 {
		t.Fatal("no declined zero-target may-shuffle")
	}
	t.Logf("declined zero-target may-shuffles %d, target-card counts %v", counts.zeroTargetNo, counts.targetCards)
}
