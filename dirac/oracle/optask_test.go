package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
)

// The optional-ask lockstep: constructed matchups over the generated optask
// pool -- Optional$ True on Sacrifice and Discard in every gorge shape (the
// may-sacrifice pick, the may-election discards, AnyNumber$'s Min 0 pick,
// the target-player asks and chooseMode's revealed-hand picks) -- so the
// expected answer at every decision is gorge's. The frozen FDN and BLB pair
// pools cast none of these arms.

var optaskDecks = [][2][]cardset.Entry{{
	{
		{Name: "Desecration Demon", Count: 4}, {Name: "Crovax the Cursed", Count: 3}, {Name: "Extortion", Count: 4},
		{Name: "Kinetic Augur", Count: 3}, {Name: "Discordant Dirge", Count: 2}, {Name: "Grizzly Bears", Count: 2},
		{Name: "Ornithopter", Count: 2}, {Name: "Mountain", Count: 8}, {Name: "Swamp", Count: 12},
	},
	{
		{Name: "Coral Net", Count: 4}, {Name: "Extract the Truth", Count: 4}, {Name: "Illuminate History", Count: 3},
		{Name: "Mind Maggots", Count: 3}, {Name: "Curse Artifact", Count: 2}, {Name: "Hill Giant", Count: 4},
		{Name: "Island", Count: 4}, {Name: "Mountain", Count: 4}, {Name: "Swamp", Count: 12},
	},
}, {
	{
		{Name: "Midgar, City of Mako", Count: 4}, {Name: "Desecration Demon", Count: 3}, {Name: "Extortion", Count: 4},
		{Name: "Crovax the Cursed", Count: 2}, {Name: "Gray Ogre", Count: 4}, {Name: "Grizzly Bears", Count: 3},
		{Name: "Swamp", Count: 10}, {Name: "Forest", Count: 4}, {Name: "Mountain", Count: 6},
	},
	{
		{Name: "Illuminate History", Count: 4}, {Name: "Discordant Dirge", Count: 3}, {Name: "Last Rites", Count: 3},
		{Name: "Mind Maggots", Count: 3}, {Name: "Coral Net", Count: 3}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Island", Count: 2}, {Name: "Mountain", Count: 5}, {Name: "Swamp", Count: 12}, {Name: "Forest", Count: 1},
	},
}, {
	// The land-front Adventure: Midgar's Reactor Raid is the test's
	// named lift. The {X} spells float the mana a face cast needs (the
	// faces pool's matchup 3 shape), and the controller keeps a
	// creature/artifact to sacrifice for the Optional$ ask.
	{
		{Name: "Blaze", Count: 4}, {Name: "Stream of Life", Count: 2}, {Name: "Midgar, City of Mako", Count: 4},
		{Name: "Grizzly Bears", Count: 2}, {Name: "Hill Giant", Count: 2}, {Name: "Gray Ogre", Count: 2},
		{Name: "Ornithopter", Count: 2}, {Name: "Mountain", Count: 10}, {Name: "Forest", Count: 8}, {Name: "Swamp", Count: 4},
	},
	{
		{Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 4}, {Name: "Gray Ogre", Count: 4},
		{Name: "Ornithopter", Count: 4}, {Name: "Forest", Count: 8}, {Name: "Mountain", Count: 8}, {Name: "Island", Count: 8},
	},
}, {
	// Oildeep Gearhulk's ETB: ValidTgts$ Player lowers to both seats, and
	// the controller chooses from the TARGET's hand (gorge discardChooser
	// returns the controller for RevealYouChoose), so it exercises both the
	// self-target and opponent-target arms. The deck is half lands so the
	// 4-drop is castable in a short random game.
	{
		{Name: "Oildeep Gearhulk", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 4},
		{Name: "Ornithopter", Count: 4}, {Name: "Island", Count: 12}, {Name: "Swamp", Count: 12},
	},
	{
		{Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 4}, {Name: "Gray Ogre", Count: 4},
		{Name: "Ornithopter", Count: 4}, {Name: "Forest", Count: 8}, {Name: "Mountain", Count: 8}, {Name: "Island", Count: 8},
	},
}}

// optaskCarriers names the cards whose casts and activations the test
// counts (each must be played at least once).
var optaskCarriers = []string{"Desecration Demon", "Crovax the Cursed", "Extortion", "Kinetic Augur", "Discordant Dirge",
	"Coral Net", "Extract the Truth", "Illuminate History", "Mind Maggots", "Curse Artifact", "Last Rites", "Midgar,",
	"Oildeep Gearhulk"}

// carrierOf maps a played face name to its optaskCarriers entry ("" when
// the face is not a carrier); "Midgar," matches any Midgar face.
func carrierOf(name string) string {
	for _, n := range optaskCarriers {
		if n == "Midgar," {
			if strings.HasPrefix(name, "Midgar") || name == "Reactor Raid" {
				return n
			}
			continue
		}
		if n == name {
			return n
		}
	}
	return ""
}

// askSourceName names the object a decision resolves for ("" when the
// decision has no source), so the test can attribute an optional ask to
// Midgar's Reactor Raid rather than counting it only in the global tally.
func askSourceName(c *PickCtx, d *decision.Decision) string {
	if c.G == nil || d == nil || d.Source == 0 {
		return ""
	}
	ob := c.G.E.G.Obj(d.Source)
	if ob == nil || ob.Face() == nil {
		return ""
	}
	return ob.Face().Name
}

// optaskPolicy plays randomly, but answers the optional asks in a fixed
// decline/accept alternation (a decline is Done on a Min 0 pick or the
// election's "no"; an accept is any object pick or the "yes") and counts
// what it answered -- the gorge side's asks drive the classification
// (effects/cardflow.go "discard_may", zone_bulk.go "sacrifice" Min 0,
// cardflow.go "discard" Min 0).
func optaskPolicy(seed uint64, counts map[string]int, played map[string]int) Policy {
	rnd := RandomPolicy(seed)
	yes, yOK := chooseCode("yes")
	no, nOK := chooseCode("no")
	if !yOK || !nOK {
		panic("oracle: chooseCode lost yes/no/done")
	}
	flip := 0
	return PolicyFunc(func(c *PickCtx) Canon {
		var d *decision.Decision
		if c.G != nil {
			d = c.G.Pending()
		}
		if d != nil {
			src := askSourceName(c, d)
			midgar := strings.HasPrefix(src, "Midgar") || src == "Reactor Raid"
			if midgar {
				counts["midgar_flip"]++
			}
			if d.Kind == decision.KTarget && strings.Contains(d.Prompt, "Oildeep Gearhulk") {
				// gorge discardChooser asks the controller, whoever the target
				// is; alternate self/opponent so both arms are played. A
				// trigger target decision's Source names the trigger source,
				// not the card, so match the prompt.
				ctrl := int(d.Player) // the controller chooses the trigger's target
				counts["oil_targets"]++
				want := ctrl
				if counts["oil_targets"]%2 == 0 {
					want = 1 - ctrl
				}
				if want == ctrl {
					counts["oil_self"]++
				} else {
					counts["oil_opp"]++
				}
				for _, a := range c.D.Options {
					if a.Kind() == ActTarget && int(a.Tgt()) == int(RefPlayer)+want {
						return a
					}
				}
			}
			decline := func() bool {
				if midgar {
					return counts["midgar_flip"]%2 == 0
				}
				return flip%2 == 0
			}
			switch {
			case d.ResumeKind == "discard_may": // the may-election: "no"/"yes"
				flip++
				kk := CanonKind(no)
				if !decline() {
					kk = CanonKind(yes)
				}
				for _, a := range c.D.Options {
					if a.Kind() == ActChoose+kk {
						if kk == CanonKind(no) {
							counts["declined"]++
							if midgar {
								counts["midgar_declined"]++
							}
						} else {
							counts["accepted"]++
							if midgar {
								counts["midgar_accepted"]++
							}
						}
						return a
					}
				}
			case (d.ResumeKind == "sacrifice" || d.ResumeKind == "discard") && d.Min == 0:
				flip++
				if decline() {
					for _, a := range c.D.Options {
						if a.Kind() == ActDone { // decline: take nothing
							counts["declined"]++
							if midgar {
								counts["midgar_declined"]++
							}
							return a
						}
					}
				}
				for _, a := range c.D.Options {
					if a.Kind() != ActDone { // accept: pick the first candidate
						counts["accepted"]++
						if midgar {
							counts["midgar_accepted"]++
						}
						return a
					}
				}
			}
		}
		a := rnd.Pick(c)
		// ratchet: prefer a legal cast / land play / activation of a
		// carrier not yet played, so every carrier shows up in some game
		// (the random policy otherwise leaves the rarer ones unplayed).
		if c.G != nil {
			for _, o := range c.D.Options {
				k := o.Kind()
				if k == ActCastFace { // Midgar's Adventure: cast it whenever offered
					counts["face_casts"]++
					a = o
					break
				}
				if k != ActCast && k != ActPlayLand && k != ActActivate {
					continue
				}
				gop, ok := c.G.GorgeOption(o)
				if !ok {
					continue
				}
				ob := c.G.E.G.Obj(gop.Obj)
				if ob == nil || ob.Face() == nil {
					continue
				}
				name := ob.Face().Name
				if carrierOf(name) != "" && played[name] == 0 {
					a = o
					break
				}
			}
		}
		if k := a.Kind(); (k == ActCast || k == ActActivate || k == ActPlayLand || k == ActCastFace) && c.G != nil {
			if o, ok := c.G.GorgeOption(a); ok {
				if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil {
					played[ob.Face().Name]++
				}
			}
		}
		return a
	})
}

func TestOptaskLockstep(t *testing.T) {
	t.Parallel()

	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("optask")
	if err != nil {
		t.Fatal(err)
	}
	games := 0
	counts, played := map[string]int{}, map[string]int{}
	for mi, md := range optaskDecks {
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
		seeds := uint64(48)
		if mi == 2 {
			seeds = 96 // a land-front Adventure face cast needs floating mana: rarer
		}
		if mi == 3 {
			seeds = 96 // Oildeep's 4-drop needs enough games to enter and target
		}
		for seed := uint64(1); seed <= seeds; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			if mi == 2 {
				// the autopay surface drops plain casts its planner cannot
				// price, leaving no floating mana for a face cast (faces_test.go)
				cfg = Config{}
			}
			m, err := NewMatchup(reg, [2]string{"optask-a", "optask-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			pol := optaskPolicy(seed, counts, played)
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
				t.Logf("table: %v", m.Table)
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	t.Logf("%d games, declined %d accepted %d (Midgar declined %d accepted %d), Oildeep targets %d (self %d, opponent %d), carrier plays %v", games,
		counts["declined"], counts["accepted"], counts["midgar_declined"], counts["midgar_accepted"],
		counts["oil_targets"], counts["oil_self"], counts["oil_opp"], played)
	for _, n := range optaskCarriers {
		ok := false
		for name, cnt := range played {
			if (n == "Midgar," && strings.HasPrefix(name, "Midgar") || n == name) && cnt > 0 {
				ok = true
				break
			}
		}
		if !ok {
			t.Errorf("carrier %s never played", n)
		}
	}
	if counts["declined"] < 20 {
		t.Errorf("only %d declined optional asks", counts["declined"])
	}
	if counts["accepted"] < 20 {
		t.Errorf("only %d accepted optional asks", counts["accepted"])
	}
	if counts["face_casts"] == 0 {
		t.Errorf("Midgar's Reactor Raid Adventure never cast")
	}
	if counts["midgar_declined"] == 0 || counts["midgar_accepted"] == 0 {
		t.Errorf("Midgar's optional sacrifice never %s (declined %d, accepted %d)",
			"exercised both ways", counts["midgar_declined"], counts["midgar_accepted"])
	}
	if counts["oil_self"] == 0 || counts["oil_opp"] == 0 {
		t.Errorf("Oildeep never targeted %s (targets %d, self %d, opponent %d)",
			"both the controller and the opponent", counts["oil_targets"], counts["oil_self"], counts["oil_opp"])
	}
}
