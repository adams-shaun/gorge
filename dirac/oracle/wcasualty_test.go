package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The Casualty lockstep (CR 702.153a): constructed matchups over the
// generated wcasualty pool -- the nine printed Casualty N cards kgen lowers,
// vanilla creatures to sacrifice and basics -- so the expected answer at every
// decision is gorge's. The frozen FDN and BLB pair pools carry no casualty
// card. The test counts the mechanic itself: the casualty cast option chosen
// (ActCastCasualty) and the sacrifice election answered (the "casualty" choose
// kind), never a carrier's plain cast.

var kwCasualtyDecks = [][2][]cardset.Entry{{
	{
		{Name: "A Little Chat", Count: 4}, {Name: "Dig Up the Body", Count: 4},
		{Name: "Make Disappear", Count: 4}, {Name: "Rooftop Nuisance", Count: 4},
		{Name: "Cut Your Losses", Count: 2}, {Name: "Walking Corpse", Count: 4},
		{Name: "Vampire Cutthroat", Count: 4}, {Name: "Island", Count: 7}, {Name: "Swamp", Count: 7},
	},
	{
		{Name: "Light 'Em Up", Count: 4}, {Name: "Rob the Archives", Count: 4},
		{Name: "Cut of the Profits", Count: 3}, {Name: "Rooftop Nuisance", Count: 3},
		{Name: "Join the Maestros", Count: 2}, {Name: "Gray Ogre", Count: 4},
		{Name: "Hill Giant", Count: 4}, {Name: "Mountain", Count: 8}, {Name: "Island", Count: 4},
		{Name: "Swamp", Count: 4},
	},
}, {
	{
		{Name: "A Little Chat", Count: 4}, {Name: "Dig Up the Body", Count: 4},
		{Name: "Join the Maestros", Count: 3}, {Name: "Light 'Em Up", Count: 3},
		{Name: "Cut Your Losses", Count: 2}, {Name: "Walking Corpse", Count: 4},
		{Name: "Vampire Cutthroat", Count: 4}, {Name: "Island", Count: 8}, {Name: "Swamp", Count: 8},
	},
	{
		{Name: "Make Disappear", Count: 4}, {Name: "Rooftop Nuisance", Count: 4},
		{Name: "Rob the Archives", Count: 4}, {Name: "Cut of the Profits", Count: 3},
		{Name: "Gray Ogre", Count: 4}, {Name: "Hill Giant", Count: 3},
		{Name: "Walking Corpse", Count: 2}, {Name: "Mountain", Count: 6}, {Name: "Island", Count: 5},
		{Name: "Swamp", Count: 5},
	},
}}

// kwCasualtyPolicy prefers the casualty cast whenever it is offered and
// otherwise casts or activates when it can, so the games reach the mechanic;
// it counts the casualty cast and the election it poses.
func kwCasualtyPolicy(seed uint64, played map[string]int) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		buf = buf[:0]
		casualty := -1
		for i, a := range c.D.Options {
			switch a.Kind() {
			case ActCastCasualty:
				if casualty < 0 {
					casualty = i
				}
				buf = append(buf, a)
			case ActCast, ActActivate:
				buf = append(buf, a)
			}
		}
		var a Canon
		if casualty >= 0 {
			a = c.D.Options[casualty]
		} else if len(buf) == 0 {
			a = rnd.Pick(c)
		} else {
			d := *c.D
			d.Options = buf
			a = rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		}
		if a.Kind() == ActCastCasualty {
			played["cast"]++
		}
		if cp, ok := chooseCode("casualty"); ok && a.Kind() == ActChoose+CanonKind(cp) {
			played["election"]++
		}
		return a
	})
}

func TestWcasualtyLockstep(t *testing.T) {
	t.Parallel()

	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("wcasualty")
	if err != nil {
		t.Fatal(err)
	}
	games := 0
	played := map[string]int{}
	for mi, md := range kwCasualtyDecks {
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
			m, err := NewMatchup(reg, [2]string{"kwcasualty-a", "kwcasualty-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			pol := kwCasualtyPolicy(seed, played)
			if seed%4 == 0 {
				pol = RandomPolicy(seed)
			}
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
	t.Logf("%d games, casualty occurrences %v", games, played)
	for _, n := range []string{"cast", "election"} {
		if played[n] < 5 {
			t.Errorf("casualty %s chosen only %d times (want >= 5)", n, played[n])
		}
	}
}
