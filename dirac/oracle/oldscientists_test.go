package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Those Old Scientists lockstep: Chapter III asks for up to two target
// cards in graveyards and each card's owner shuffles it into their library
// (gorge objectPathShuffleOwners: the moved cards go on top in pick order,
// then each distinct owner's library shuffles once, in referent order). The
// deck is its own copies (so both graveyards fill from the Chapter I
// discards) and Islands (so Chapter II, which needs an artifact, instant or
// sorcery in your graveyard, never offers a second target ask).
var koldscientistsDecks = [][2][]cardset.Entry{{
	{
		{Name: "Those Old Scientists", Count: 4}, {Name: "Island", Count: 36},
	},
	{
		{Name: "Those Old Scientists", Count: 4}, {Name: "Island", Count: 36},
	},
}}

// oldscientistsCounts tallies the completed Chapter III target asks by the
// shape the policy chose: zero, two cards with one owner, two cards with two
// owners.
type oldscientistsCounts struct {
	asks      int
	zero      int
	oneOwner  int
	twoOwners int
}

func distinctOwners(owners []state.PlayerID) int {
	n := 0
	for i := range owners {
		seen := false
		for j := 0; j < i; j++ {
			if owners[j] == owners[i] {
				seen = true
				break
			}
		}
		if !seen {
			n++
		}
	}
	return n
}

// oldscientistsPolicy casts when it can (the base policy), and at a Chapter
// III target ask (KTarget, Max 2) forces the shape seed%3 selects: a card
// from each graveyard, two cards from one graveyard, or zero. It tallies the
// completed ask so the test proves each shape was reached.
func oldscientistsPolicy(seed uint64, counts *oldscientistsCounts) Policy {
	base := poisonhighPolicy(seed, map[string]int{}, map[string]int{})
	mode := seed % 3
	var (
		active bool
		src    state.ObjID
		owners []state.PlayerID
	)
	finish := func() {
		if !active {
			return
		}
		switch {
		case len(owners) == 0:
			counts.zero++
		case len(owners) == 2 && distinctOwners(owners) == 1:
			counts.oneOwner++
		case len(owners) == 2 && distinctOwners(owners) == 2:
			counts.twoOwners++
		}
		active = false
		src = 0
		owners = owners[:0]
	}
	type cardOpt struct {
		a     Canon
		owner state.PlayerID
	}
	return PolicyFunc(func(c *PickCtx) Canon {
		var d *decision.Decision
		if c.G != nil {
			d = c.G.Pending()
		}
		if c.D.Kind != DTarget || d == nil || d.Kind != decision.KTarget || d.Max != 2 {
			finish()
			return base.Pick(c)
		}
		if active && d.Source != src {
			finish()
		}
		if !active {
			active = true
			src = d.Source
			owners = owners[:0]
			counts.asks++
		}
		var cards []cardOpt
		var done Canon
		hasDone := false
		for _, a := range c.D.Options {
			switch a.Kind() {
			case ActDone:
				done, hasDone = a, true
			case ActTarget:
				o, ok := c.G.GorgeOption(a)
				if !ok || o.Obj == 0 {
					continue
				}
				if ob := c.G.E.G.Obj(o.Obj); ob != nil {
					cards = append(cards, cardOpt{a, ob.Owner})
				}
			}
		}
		if mode == 2 {
			if hasDone {
				finish()
				return done
			}
			return base.Pick(c)
		}
		switch len(owners) {
		case 0:
			// A card from the opponent's graveyard first for the two-owner
			// shape; for the one-owner shape, an owner with two cards offered
			// (the controller's graveyard first).
			pick := -1
			if mode == 0 {
				for i := range cards {
					if cards[i].owner != d.Player {
						pick = i
						break
					}
				}
			} else {
				want := d.Player
				for i := range cards {
					if cards[i].owner == want {
						for j := i + 1; j < len(cards); j++ {
							if cards[j].owner == want {
								pick = i
								break
							}
						}
					}
					if pick >= 0 {
						break
					}
				}
				if pick < 0 {
					for i := range cards {
						for j := i + 1; j < len(cards); j++ {
							if cards[i].owner == cards[j].owner {
								pick = i
								break
							}
						}
						if pick >= 0 {
							break
						}
					}
				}
			}
			if pick >= 0 {
				owners = append(owners, cards[pick].owner)
				return cards[pick].a
			}
		case 1:
			pick := -1
			for i := range cards {
				if mode == 0 && cards[i].owner != owners[0] {
					pick = i
					break
				}
				if mode == 1 && cards[i].owner == owners[0] {
					pick = i
					break
				}
			}
			if pick >= 0 {
				owners = append(owners, cards[pick].owner)
				return cards[pick].a
			}
		}
		if hasDone {
			finish()
			return done
		}
		return base.Pick(c)
	})
}

// runOldScientists plays the mirror matchup for seeds 1..seeds. mutate turns
// on DebugShuffleOwnOnly, which must make the lockstep diverge.
func runOldScientists(t *testing.T, seeds int, mutate bool) (games, divs, stalls int, counts *oldscientistsCounts) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("oldscientists")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugShuffleOwnOnly
	dirac.DebugShuffleOwnOnly = mutate
	defer func() { dirac.DebugShuffleOwnOnly = old }()
	counts = &oldscientistsCounts{}
	md := koldscientistsDecks[0]
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
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"oldscientists-a", "oldscientists-b"}, decks, 1)
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
		pol := oldscientistsPolicy(seed, counts)
		res := LockstepLed(m, seed, g, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 30})
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
			if !mutate {
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	return
}

func TestThoseOldScientistsLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, stalls, counts := runOldScientists(t, 24, false)
	t.Logf("%d games, %d divergences, %d stalls; Chapter III asks %d (zero %d, one owner %d, two owners %d)",
		games, divs, stalls, counts.asks, counts.zero, counts.oneOwner, counts.twoOwners)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	if counts.twoOwners == 0 {
		t.Error("no Chapter III ask chose a card from each graveyard")
	}
	if counts.oneOwner == 0 {
		t.Error("no Chapter III ask chose two cards owned by one player")
	}
	if counts.zero == 0 {
		t.Error("no Chapter III ask declined both targets")
	}
}

// TestThoseOldScientistsMutation shuffles only the effect controller's
// library: a Chapter III ask that chose a card from the opponent's graveyard
// must then diverge (the opponent's library order, and every later draw,
// differs from gorge's).
func TestThoseOldScientistsMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, _, counts := runOldScientists(t, 24, true)
	if divs == 0 {
		t.Fatalf("mutation shuffled only the controller's library in %d games with no divergence (asks %d)", games, counts.asks)
	}
	t.Logf("mutation diverged in %d of %d games (asks %d, two owners %d)", divs, games, counts.asks, counts.twoOwners)
}
