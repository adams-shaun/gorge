package oracle

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The TwoPiles lockstep over the remembered reveal (gorge effTwoPiles'
// DefinedCards$ Remembered arm, CR 700.3): the top cards are revealed and
// remembered, one player separates them into two piles, the other picks one;
// the chosen pile goes to the hand, the other to the graveyard, each pile in
// its own order (pile A as the separator picked it). An opponent separates and
// you pick for Fact or Fiction, Sphinx of Uthuun, Unesh, Criosphinx Sovereign
// and Sphinx of Clear Skies (kop.TpOppSeparates); you separate and an
// opponent picks for Steam Augury and Epiphany at the Drownyard. Every carrier
// moves its unchosen pile with ChangeZone Defined$ Remembered, so an unchosen
// pile A reaches the graveyard in pick order (kop.TpUnchosenPileOrder). Both
// seats play the same deck over the generated twopiles pool.

var ktpDeck = []cardset.Entry{
	{Name: "Fact or Fiction", Count: 4}, {Name: "Steam Augury", Count: 4},
	{Name: "Epiphany at the Drownyard", Count: 3}, {Name: "Sphinx of Uthuun", Count: 3},
	{Name: "Unesh, Criosphinx Sovereign", Count: 2}, {Name: "Sphinx of Clear Skies", Count: 3},
	{Name: "Grizzly Bears", Count: 2},
	{Name: "Island", Count: 12}, {Name: "Mountain", Count: 4},
	{Name: "Plains", Count: 1}, {Name: "Swamp", Count: 1}, {Name: "Forest", Count: 1},
}

// ktpOppSeparates names each carrier and whether an opponent separates.
var ktpOppSeparates = map[string]bool{
	"Fact or Fiction": true, "Sphinx of Uthuun": true, "Unesh, Criosphinx Sovereign": true,
	"Sphinx of Clear Skies": true, "Steam Augury": false, "Epiphany at the Drownyard": false,
}

const (
	ktpMin   = 4  // pile picks each carrier must reach
	ktpSeeds = 80 // games of the lockstep run
)

// ktpTally counts, from gorge's pending asks, each carrier's splits and pile
// picks, the pile B picks, and the unchosen piles A of two or more cards (the
// TpUnchosenPileOrder path). A gorge ask posed to the wrong seat is recorded in
// bad: the CR says who separates and who picks, whatever both engines agree on.
type ktpTally struct {
	n     map[string]int
	pileA int // the open split's picks so far
	bad   []string
}

// ktpPolicy plays a land when it can, then casts eagerly (eagerPolicy), and
// tallies the TwoPiles asks it answers.
func ktpPolicy(seed uint64, played map[string]int, tl *ktpTally) Policy {
	inner := eagerPolicy(seed, played)
	split, _ := chooseCode("twopiles")
	pileB, _ := chooseCode("pile-b")
	return PolicyFunc(func(c *PickCtx) Canon {
		for _, o := range c.D.Options {
			if o.Kind() == ActPlayLand {
				return o
			}
		}
		a := inner.Pick(c)
		if c.G == nil {
			return a
		}
		d := c.G.Pending()
		if d == nil || d.ResumeKind != "twopiles_split" && d.ResumeKind != "twopiles_pick" {
			return a
		}
		src := c.G.E.G.Obj(d.Source)
		if src == nil || src.Face() == nil {
			return a
		}
		name := src.Face().Name
		oppSep, ok := ktpOppSeparates[name]
		if !ok {
			tl.bad = append(tl.bad, "a TwoPiles ask from "+name)
			return a
		}
		ctrl := src.Controller
		opp := 1 - ctrl
		if d.ResumeKind == "twopiles_split" {
			want := ctrl
			if oppSep {
				want = opp
			}
			if d.Player != want {
				tl.bad = append(tl.bad, fmt.Sprintf("%s: the split asked p%d, want p%d", name, d.Player, want))
			}
			if !c.G.MidPick() {
				tl.pileA = 0
				tl.n[name+"/split"]++
			}
			if a.Kind() == ActChoose+CanonKind(split) {
				tl.pileA++
			}
			return a
		}
		want := opp
		if oppSep {
			want = ctrl
		}
		if d.Player != want {
			tl.bad = append(tl.bad, fmt.Sprintf("%s: the pile pick asked p%d, want p%d", name, d.Player, want))
		}
		tl.n[name]++
		if a.Kind() == ActChoose+CanonKind(pileB) {
			tl.n[name+"/b"]++
			if tl.pileA >= 2 {
				tl.n["unchosen pile A of 2+"]++
			}
		}
		tl.pileA = 0
		return a
	})
}

// runTwoPilesRemembered plays the seeded games. mutate, when set, clears that
// OpTwoPiles flag in the table and stops at the first divergence.
func runTwoPilesRemembered(t *testing.T, seeds uint64, mutate int16) (games, divs int, tl *ktpTally) {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("twopiles")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	var ents [2][]cardset.Entry
	for s := range decks {
		ents[s] = ktpDeck
		for _, e := range ktpDeck {
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
	tbl, _, _, err := set.Matchup(ents[0], ents[1])
	if err != nil {
		t.Fatal(err)
	}
	piles, hit := 0, 0
	for ci := range tbl.Cards {
		for ai := range tbl.Cards[ci].Abilities {
			for pi := range tbl.Cards[ci].Abilities[ai].Prog {
				in := &tbl.Cards[ci].Abilities[ai].Prog[pi]
				if in.Op != kop.OpTwoPiles {
					continue
				}
				piles++
				if mutate != 0 && in.B.N&mutate != 0 {
					in.B.N &^= mutate
					hit++
				}
			}
		}
	}
	if piles != len(ktpOppSeparates) {
		t.Fatalf("%d OpTwoPiles in the table, want one per carrier (%d)", piles, len(ktpOppSeparates))
	}
	if mutate != 0 && hit == 0 {
		t.Fatalf("mutation found no OpTwoPiles with flag %d", mutate)
	}
	tl = &ktpTally{n: map[string]int{}}
	played := map[string]int{}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"ktp-a", "ktp-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, ktpPolicy(seed, played, tl), RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 30})
		CloseSide(k)
		games++
		if err := unmodelledStallError(t.Name(), &res); err != nil {
			t.Errorf("seed %d: %v", seed, err)
		}
		if d := res.Div; d != nil {
			divs++
			if mutate != 0 {
				return
			}
			lo := max(d.At-12, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			return
		}
	}
	return
}

// TestTwoPilesRememberedLockstep plays every carrier's split and pile pick in
// lockstep, both piles chosen, an unchosen pile A of two or more cards among
// them, and checks gorge asks the seat the card names.
func TestTwoPilesRememberedLockstep(t *testing.T) {
	t.Parallel()

	games, divs, tl := runTwoPilesRemembered(t, ktpSeeds, 0)
	keys := make([]string, 0, len(tl.n))
	for k := range tl.n {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var bs []string
	for _, k := range keys {
		bs = append(bs, fmt.Sprintf("%s=%d", k, tl.n[k]))
	}
	t.Logf("%d games, %d divergences; %s", games, divs, strings.Join(bs, " "))
	for _, b := range tl.bad {
		t.Error(b)
	}
	names := make([]string, 0, len(ktpOppSeparates))
	for n := range ktpOppSeparates {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		if tl.n[n] < ktpMin {
			t.Errorf("%s: %d pile picks, want at least %d", n, tl.n[n], ktpMin)
		}
		if tl.n[n+"/split"] == 0 {
			t.Errorf("%s: never split", n)
		}
	}
	if tl.n["unchosen pile A of 2+"] < ktpMin {
		t.Errorf("%d unchosen piles A of two or more cards, want at least %d", tl.n["unchosen pile A of 2+"], ktpMin)
	}
}

// TestTwoPilesRememberedSeparatorMutation: with the separator and the chooser
// swapped back (an opponent's pick of your split), some game diverges.
func TestTwoPilesRememberedSeparatorMutation(t *testing.T) {
	t.Parallel()

	games, divs, _ := runTwoPilesRemembered(t, ktpSeeds, kop.TpOppSeparates)
	if divs == 0 {
		t.Fatalf("mutation in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in game %d", games)
}

// TestTwoPilesRememberedOrderMutation: with an unchosen pile A moved in the
// revealed order instead of its pick order, some game diverges.
func TestTwoPilesRememberedOrderMutation(t *testing.T) {
	t.Parallel()

	games, divs, _ := runTwoPilesRemembered(t, ktpSeeds, kop.TpUnchosenPileOrder)
	if divs == 0 {
		t.Fatalf("mutation in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in game %d", games)
}
