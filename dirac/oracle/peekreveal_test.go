package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
)

// The optional PeekAndReveal lockstep (CR 701.20a,
// 701.20e, 608.2c): `PeekAndReveal | PeekAmount$ 1 | RevealValid$ <spec> |
// RevealOptional$ True | RememberRevealed$ True`. gorge asks the library's
// owner yes/no only when the top card matches the spec; a yes remembers the
// card for the chained move. Deck 0 carries all three optional PeekAndReveal
// carriers, so the taken `Defined$ Remembered | Origin$ Library |
// Destination$ Hand` fetch is compared on Traveling Botanist and Gathering
// Stone. A plain reveal-and-take owes no shuffle in gorge or the kernel
// (`zone_search.go:741-752`; the kernel copies the `Origin$ Library` gate,
// `BncFromLibrary`), so the lockstep also holds the
// library order and RNGDraws across that move.
var kpeekrevealDecks = [2][]cardset.Entry{
	{
		{Name: "Quest for Ula's Temple", Count: 4}, {Name: "Grizzly Bears", Count: 8},
		{Name: "Hill Giant", Count: 4}, {Name: "Ornithopter", Count: 4},
		{Name: "Bucolic Ranch", Count: 3}, {Name: "Gathering Stone", Count: 3},
		{Name: "Traveling Botanist", Count: 3},
		{Name: "Forest", Count: 5}, {Name: "Island", Count: 6},
	},
	{
		{Name: "Quest for Ula's Temple", Count: 3}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Hill Giant", Count: 3}, {Name: "Ornithopter", Count: 2},
		{Name: "Forest", Count: 14}, {Name: "Island", Count: 14},
	},
}

// kpeekrevealType is the creature type the policy elects at a creature-type
// ask. Gathering Stone offers its controller gorge's whole subtype vocabulary
// (362 options), so a uniform pick almost never matches the top card;
// electing a type plentiful in the deck keeps its optional reveal reachable
// deterministically. Grizzly Bears (8 copies) is the deck's most numerous
// creature.
const kpeekrevealType = "Bear"

// kpeekrevealChooseType elects want at a creature-type ask, in the exact
// canonical form of the gorge option (optionAction). ok false: no such ask.
func kpeekrevealChooseType(c *PickCtx, want string) (Canon, bool) {
	if c.G == nil {
		return 0, false
	}
	d := c.G.Pending()
	if d == nil || d.Kind != decision.KChoose {
		return 0, false
	}
	for i := range d.Options {
		o := &d.Options[i]
		if (o.Kind != "creature_type" && o.Kind != "type") || o.Label != want {
			continue
		}
		a, ok := c.G.optionAction(d, o)
		if !ok {
			continue
		}
		for _, opt := range c.D.Options {
			if opt == a {
				return a, true
			}
		}
	}
	return 0, false
}

// kpeekrevealPolicy plays a land, else casts, and answers the may-reveal ask
// (a yes/no whose yes names the top card) yes three times in four; the rest
// is random. It counts the asks it answered in aggregate and by source card.
func kpeekrevealPolicy(seed uint64, yes, no *int, bySource map[string][2]int) Policy {
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
		case DTriggerOptional:
			// take the upkeep trigger so the may-reveal is reached
			for _, a := range c.D.Options {
				if a.Kind() == ActYes {
					return a
				}
			}
		case DChoose:
			if a, ok := kpeekrevealChooseType(c, kpeekrevealType); ok {
				return a
			}
			if len(c.D.Options) == 2 && c.D.Options[0].Kind() == ActChoose+CanonKind(yesK) &&
				c.D.Options[1].Kind() == ActChoose+CanonKind(noK) && c.D.Options[0].Src() != RefNone {
				// the may-reveal ask: the yes option names the top card
				take := c.Seq%4 != 0
				if take {
					*yes++
				} else {
					*no++
				}
				// Count it by the ask's source card too: the aggregate is
				// dominated by Quest for Ula's Temple, and the optional
				// carriers' taken library fetch needs its own coverage.
				if c.G != nil {
					if d := c.G.Pending(); d != nil && d.ResumeKind == "reveal_optional" {
						if ob := c.G.E.G.Obj(d.Source); ob != nil && ob.Face() != nil {
							n, y := bySource[ob.Face().Name][0], bySource[ob.Face().Name][1]
							if take {
								y++
							} else {
								n++
							}
							bySource[ob.Face().Name] = [2]int{n, y}
						}
					}
				}
				if take {
					return c.D.Options[0]
				}
				return c.D.Options[1]
			}
		}
		return rnd.Pick(c)
	})
}

func TestPeekrevealLockstep(t *testing.T) {
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
	for s, d := range kpeekrevealDecks {
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
	tbl, _, _, err := set.Matchup(kpeekrevealDecks[0], kpeekrevealDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	var st dirac.PeekRevealStats
	dirac.DebugPeekRevealStats = &st
	defer func() { dirac.DebugPeekRevealStats = nil }()
	games, divs, stalls, yes, no := 0, 0, 0, 0, 0
	bySource := map[string][2]int{} // may-reveal answers [no, yes] by the ask's source card
	for seed := uint64(1); seed <= 256; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kpr-a", "kpr-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, kpeekrevealPolicy(seed, &yes, &no, bySource), RunConfig{Config: cfg, MaxDecisions: 6000, MaxTurns: 20})
		CloseSide(k)
		games++
		// "turns" is the MaxTurns cap on a game neither side finished, not a
		// capacity stall: both sides agreed up to it
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
	t.Logf("%d games, %d divergences, %d stalls; may-reveal asks answered yes %d, no %d, by source %v; kernel asks %d, top card failed the spec %d, empty library %d",
		games, divs, stalls, yes, no, bySource, st.Asks, st.Skips, st.Empty)
	if yes < 30 {
		t.Errorf("%d asks answered yes, want at least 30", yes)
	}
	if no < 10 {
		t.Errorf("%d asks answered no, want at least 10", no)
	}
	if st.Skips < 20 {
		t.Errorf("%d peeks with a non-matching top card, want at least 20", st.Skips)
	}
	if bySource["Traveling Botanist"][1] < 20 {
		t.Errorf("%d may-reveal asks from Traveling Botanist answered yes, want at least 20", bySource["Traveling Botanist"][1])
	}
	if bySource["Traveling Botanist"][0] < 10 {
		t.Errorf("%d may-reveal asks from Traveling Botanist answered no, want at least 10", bySource["Traveling Botanist"][0])
	}
	if gs := bySource["Gathering Stone"]; gs[0]+gs[1] < 1 {
		t.Errorf("%d may-reveal asks from Gathering Stone, want at least 1", gs[0]+gs[1])
	}
	if st.Asks != yes+no {
		t.Errorf("the kernel posed %d asks, the policy answered %d", st.Asks, yes+no)
	}
}
