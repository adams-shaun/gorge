package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
)

// The reveal lands' as-enters election lockstep: "As this enters, you may reveal
// a <type> card from your hand. If you don't, it enters tapped." gorge offers pay
// only with a candidate, takes a lone candidate unasked and asks "revealcost" over
// several, on a land play and on a land put onto the battlefield mid-resolution
// (Growth Spiral).
var krevealDecks = [2][]cardset.Entry{
	{
		{Name: "Choked Estuary", Count: 3}, {Name: "Port Town", Count: 3}, {Name: "Vineglimmer Snarl", Count: 3},
		{Name: "Frostboil Snarl", Count: 3}, {Name: "Growth Spiral", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Shock", Count: 4}, {Name: "Island", Count: 6}, {Name: "Forest", Count: 4}, {Name: "Swamp", Count: 3},
		{Name: "Plains", Count: 3},
	},
	{
		{Name: "Gilt-Leaf Palace", Count: 4}, {Name: "Foreboding Ruins", Count: 3}, {Name: "Fortified Village", Count: 3},
		{Name: "Furycalm Snarl", Count: 2}, {Name: "Shineshadow Snarl", Count: 2}, {Name: "Llanowar Elves", Count: 6},
		{Name: "Changeling Outcast", Count: 4}, {Name: "Growth Spiral", Count: 2}, {Name: "Forest", Count: 5},
		{Name: "Swamp", Count: 4}, {Name: "Mountain", Count: 3}, {Name: "Plains", Count: 2},
	},
}

type krevealTally struct {
	games, stalls, paid, declined, declineOnly, picks, mid int
}

// krevealPolicy plays a land whenever it can and reveals with probability 1/2;
// everything else is random.
func krevealPolicy(seed uint64, tal *krevealTally) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G == nil {
			return rnd.Pick(c)
		}
		d := c.G.E.Pending()
		if d != nil && d.Kind == decision.KChoose && len(d.Options) > 0 && d.Options[0].Kind == "revealcost" {
			tal.picks++
			return rnd.Pick(c)
		}
		if d != nil && d.ResumeKind == "unless_pay" {
			pay, decline := -1, -1
			for i := range d.Options {
				switch d.Options[i].Mode {
				case decision.ModeUnlessPay:
					pay = i
				case decision.ModeUnlessDecline:
					decline = i
				}
			}
			if len(c.G.E.G.Stack) > 0 {
				tal.mid++
			}
			if pay < 0 {
				tal.declineOnly++
			}
			want := decline
			if pay >= 0 && (seed+uint64(c.Seq))%2 == 0 {
				want = pay
				tal.paid++
			} else {
				tal.declined++
			}
			return c.D.Options[want]
		}
		for _, a := range c.D.Options {
			if a.Kind() == ActPlayLand {
				return a
			}
		}
		return rnd.Pick(c)
	})
}

func TestRevealLandLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("revealland")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range krevealDecks {
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
	tbl, _, _, err := set.Matchup(krevealDecks[0], krevealDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	var tal krevealTally
	for seed := uint64(1); seed <= 256; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kreveal-a", "kreveal-b"}, decks, 1)
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
		res := LockstepLed(m, seed, g, k, krevealPolicy(seed, &tal), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 120})
		CloseSide(k)
		tal.games++
		if res.Stall != "" {
			tal.stalls++
			t.Logf("seed %d: stall %s after %d decisions", seed, res.Stall, res.Decisions)
		}
		if d := res.Div; d != nil {
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v",
				seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	t.Logf("%+v", tal)
	if tal.stalls != 0 {
		t.Fatalf("%d stalls", tal.stalls)
	}
	if tal.paid < 30 || tal.declined < 30 || tal.declineOnly < 5 || tal.picks < 10 || tal.mid < 5 {
		t.Errorf("thin coverage: %+v", tal)
	}
}
