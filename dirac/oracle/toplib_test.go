package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The Defined$ TopOfLibrary lockstep (CR 608.2c, 608.2d,
// 701.17a): the else branch after an optional PeekAndReveal, `ChangeZone |
// Defined$ TopOfLibrary | Origin$ Library | Optional$ True | ConditionDefined$
// Remembered | ConditionPresent$ Card | ConditionCompare$ EQ0`, to the
// graveyard (Gathering Stone, Traveling Botanist) or the bottom of the library
// (Bucolic Ranch). gorge's moveDefinedLibraryObjects asks "defined_library_
// optional" once, none on an empty library; a yes moves the top card unshuffled.
//
// A TAKEN reveal on the DBToHand sub (`Defined$ Remembered Origin$ Library
// Destination$ Hand`) is a fetch, not a search (CR 701.23a): the pinned gorge
// (c4f1ece62) shuffles only a library destination, a `NoLooking$` placement or
// a deferred search shuffle (its implicitShuffle), so it shuffles nothing here,
// as the Oracle text of Traveling Botanist and Gathering Stone says. The
// kernel follows (BncShuffleOwed / RetShuffleOwed), so the policy
// takes the may-reveal three times in four. Gathering Stone's chosen-type
// reads (the may-reveal RevealValid$ and the ReduceCost static) are exact and
// it is in the decks.
var ktoplibDecks = [2][]cardset.Entry{
	{
		{Name: "Traveling Botanist", Count: 8}, {Name: "Bucolic Ranch", Count: 4},
		{Name: "Gathering Stone", Count: 4}, {Name: "Grizzly Bears", Count: 2},
		{Name: "Hill Giant", Count: 2}, {Name: "Forest", Count: 10}, {Name: "Island", Count: 10},
	},
	{
		{Name: "Traveling Botanist", Count: 8}, {Name: "Bucolic Ranch", Count: 4},
		{Name: "Gathering Stone", Count: 4}, {Name: "Grizzly Bears", Count: 2},
		{Name: "Hill Giant", Count: 2}, {Name: "Forest", Count: 10}, {Name: "Island", Count: 10},
	},
}

// ktoplibStats counts what the policy saw gorge pose.
type ktoplibStats struct {
	reveal   [2]int         // may-reveal asks answered no, yes
	ask      [2]int         // defined_library_optional asks answered no, yes
	bySource map[string]int // those asks by the source card's name
}

// ktoplibPolicy plays a land, else casts, takes optional triggers, activates
// abilities one time in two, takes the may-reveal ask and answers the
// defined_library_optional ask yes three times in four each; the rest is random.
func ktoplibPolicy(seed uint64, st *ktoplibStats) Policy {
	rnd := RandomPolicy(seed)
	yesK, _ := chooseCode("yes")
	noK, _ := chooseCode("no")
	return PolicyFunc(func(c *PickCtx) Canon {
		switch c.D.Kind {
		case DPriority:
			for _, k := range [...]CanonKind{ActPlayLand, ActCast} {
				for _, a := range c.D.Options {
					if a.Kind() == k {
						return a
					}
				}
			}
			if c.Seq%2 == 0 {
				for _, a := range c.D.Options {
					if a.Kind() == ActActivate {
						return a
					}
				}
			}
		case DTriggerOptional:
			for _, a := range c.D.Options {
				if a.Kind() == ActYes {
					return a
				}
			}
		case DChoose:
			if len(c.D.Options) != 2 || c.D.Options[0].Kind() != ActChoose+CanonKind(yesK) ||
				c.D.Options[1].Kind() != ActChoose+CanonKind(noK) {
				break
			}
			if c.D.Options[0].Src() != RefNone {
				// the may-reveal ask (its yes option names the top card)
				i, pick := 0, c.D.Options[1]
				if c.Seq%4 != 0 {
					i, pick = 1, c.D.Options[0]
				}
				st.reveal[i]++
				return pick
			}
			i, pick := 0, c.D.Options[1]
			if c.Seq%4 != 0 {
				i, pick = 1, c.D.Options[0]
			}
			if c.G != nil {
				if d := c.G.Pending(); d != nil && d.ResumeKind == "defined_library_optional" {
					st.ask[i]++
					if ob := c.G.E.G.Obj(d.Source); ob != nil && ob.Face() != nil {
						st.bySource[ob.Face().Name]++
					}
					return pick
				}
			}
		}
		return rnd.Pick(c)
	})
}

func TestToplibLockstep(t *testing.T) {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("peekreveal")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range ktoplibDecks {
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
	tbl, _, _, err := set.Matchup(ktoplibDecks[0], ktoplibDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	st := &ktoplibStats{bySource: map[string]int{}}
	games, divs, stalls := 0, 0, 0
	for seed := uint64(1); seed <= 256; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"ktl-a", "ktl-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, ktoplibPolicy(seed, st), RunConfig{Config: cfg, MaxDecisions: 6000, MaxTurns: 20})
		CloseSide(k)
		games++
		if res.Stall != "" && res.Stall != "turns" {
			stalls++
			t.Errorf("seed %d: stalled: %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			divs++
			lo := max(d.At-12, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	t.Logf("%d games, %d divergences, %d stalls; may-reveal asks answered yes %d, no %d; defined_library_optional asks answered yes %d, no %d, by source %v",
		games, divs, stalls, st.reveal[1], st.reveal[0], st.ask[1], st.ask[0], st.bySource)
	if st.reveal[1] < 30 {
		t.Errorf("%d may-reveal asks answered yes, want at least 30", st.reveal[1])
	}
	if st.ask[1] < 20 {
		t.Errorf("%d defined_library_optional asks answered yes, want at least 20", st.ask[1])
	}
	if st.ask[0] < 10 {
		t.Errorf("%d defined_library_optional asks answered no, want at least 10", st.ask[0])
	}
	for _, name := range [...]string{"Traveling Botanist", "Bucolic Ranch"} {
		if st.bySource[name] < 20 {
			t.Errorf("%d defined_library_optional asks from %s, want at least 20", st.bySource[name], name)
		}
	}
}
