package oracle

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The MBC api-params-7 lowerings in lockstep with gorge: each spelling plays
// its card from the fixture pool dirac/cardset/pools/fixtures/mbcapiparams7.
//
//   - other:Mobilize group of N (CR 702.181a): Dalkovan Packbeasts' Mobilize 3
//     mints three tapped and attacking Warriors that are sacrificed as one
//     batch at the next end step (the many-object delayed program, kop.DpMany,
//     with ExtSacDelayed C=1: dirac/kw_mobilize.go).
//   - other:return cost on a spell (CR 601.2b, 601.2h): Familiar's Ruse's
//     "Return<1/Creature>" additional cost is picked while casting, one
//     pick per permanent, and paid after the mana (dirac/cost_return.go).
//     The same cost on Deprive and Disappearing Act (Return<1/Land>,
//     Return<1/Permanent>: a returned land has already paid mana) and on
//     Devour in Flames (a sorcery) is played beside it.
//   - param:Draw.Reveal (Fa'adiyah Seer's "draw a card and reveal it") has no
//     MBC card that lowers whole yet, so dirac/kgen/draw_reveal_test.go holds it.

var mbcApiParams7Decks = []fixtureDecks{{
	{
		{Name: "Dalkovan Packbeasts", Count: 4}, {Name: "Familiar's Ruse", Count: 3}, {Name: "Deprive", Count: 3},
		{Name: "Disappearing Act", Count: 2}, {Name: "Merfolk of the Pearl Trident", Count: 4}, {Name: "Savannah Lions", Count: 3},
		{Name: "Elite Vanguard", Count: 3}, {Name: "Air Elemental", Count: 2},
		{Name: "Plains", Count: 7}, {Name: "Island", Count: 9},
	},
	{
		{Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 4}, {Name: "Raging Goblin", Count: 2},
		{Name: "Lightning Bolt", Count: 4}, {Name: "Shock", Count: 3}, {Name: "Devour in Flames", Count: 3},
		{Name: "Mountain", Count: 10}, {Name: "Forest", Count: 10},
	},
}, {
	{
		{Name: "Dalkovan Packbeasts", Count: 4}, {Name: "Familiar's Ruse", Count: 4}, {Name: "Merfolk of the Pearl Trident", Count: 4},
		{Name: "Savannah Lions", Count: 4}, {Name: "Elite Vanguard", Count: 4}, {Name: "Air Elemental", Count: 2},
		{Name: "Plains", Count: 9}, {Name: "Island", Count: 9},
	},
	{
		{Name: "Dalkovan Packbeasts", Count: 4}, {Name: "Familiar's Ruse", Count: 3}, {Name: "Deprive", Count: 3},
		{Name: "Merfolk of the Pearl Trident", Count: 4}, {Name: "Hill Giant", Count: 4}, {Name: "Shock", Count: 4},
		{Name: "Devour in Flames", Count: 3}, {Name: "Lightning Bolt", Count: 2},
		{Name: "Plains", Count: 5}, {Name: "Island", Count: 5}, {Name: "Mountain", Count: 3},
	},
}}

// mbcApiParams7Tally counts what the games showed, read from gorge's board
// between decisions.
type mbcApiParams7Tally struct {
	games, divs, stalls int
	ruse, packbeasts    int            // casts of Familiar's Ruse and Dalkovan Packbeasts
	returns             map[string]int // casts of the other Return-cost spells, by name
	minted3             int            // Warrior tokens minted three at a time
	sacked3             int            // three tokens sacrificed in one end step
	partial             int            // a group sacrificed short of three (a token died first)
}

// mbcApiParams7Policy plays randomly but casts Packbeasts and Ruse when offered
// (Ruse onto any stack, three times in four), attacks with Packbeasts and
// Shocks a Warrior token out of its group now and then.
func mbcApiParams7Policy(seed uint64, tal *mbcApiParams7Tally) Policy {
	rnd := RandomPolicy(seed)
	prng := rand.New(rand.NewPCG(seed, 0x6d626337)) // "mbc7"
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G == nil {
			return rnd.Pick(c)
		}
		switch c.D.Kind {
		case DAttackers:
			for _, a := range c.D.Options {
				if a.Kind() == ActAttack && refCardName(c.G, a.Src()) == "Dalkovan Packbeasts" {
					return a
				}
			}
		case DPriority:
			for _, a := range c.D.Options {
				if a.Kind() != ActCast {
					continue
				}
				switch refCardName(c.G, a.Src()) {
				case "Dalkovan Packbeasts":
					if prng.IntN(4) != 0 {
						tal.packbeasts++
						return a
					}
				case "Familiar's Ruse":
					if prng.IntN(4) != 0 {
						tal.ruse++
						return a
					}
				case "Deprive", "Disappearing Act", "Devour in Flames":
					if prng.IntN(4) != 0 {
						tal.returns[refCardName(c.G, a.Src())]++
						return a
					}
				}
			}
		}
		return rnd.Pick(c)
	})
}

// warriors counts the Warrior tokens on the battlefield.
func mbcApiParams7Warriors(g *state.Game) int {
	n := 0
	for p := state.PlayerID(0); p < 2; p++ {
		for _, id := range g.Zone(state.ZBattlefield, p) {
			if o := g.Obj(id); o != nil && o.IsToken && o.Face() != nil && strings.Contains(o.Face().Name, "Warrior") {
				n++
			}
		}
	}
	return n
}

func TestMBCApiParams7Lowers(t *testing.T) {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("mbcapiparams7")
	if err != nil {
		t.Fatal(err)
	}
	tal := mbcApiParams7Tally{returns: map[string]int{}}
	for mi, md := range mbcApiParams7Decks {
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
		for seed := uint64(1); seed <= 120; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"mbc7-a", "mbc7-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			prevW, prevStep := 0, state.StepUntap
			check := func(ref *GorgeSide, _ Side) *Divergence {
				w := mbcApiParams7Warriors(ref.E.G)
				if w-prevW == 3 {
					tal.minted3++
				}
				if prevStep == state.StepEnd && prevW-w >= 3 {
					tal.sacked3++
				} else if prevStep == state.StepEnd && prevW > w {
					tal.partial++
				}
				prevW, prevStep = w, ref.E.G.Step
				return nil
			}
			res := Lockstep(m, seed, k, mbcApiParams7Policy(seed, &tal), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 30, Check: check})
			CloseSide(k)
			tal.games++
			if res.Stall != "" && res.Stall != "turns" {
				tal.stalls++
				t.Errorf("matchup %d seed %d: stalled: %s", mi, seed, res.Stall)
			}
			if d := res.Div; d != nil {
				tal.divs++
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				if tal.divs >= 3 {
					t.FailNow()
				}
			}
		}
	}
	t.Logf("%d games, %d divergences, %d stalls: Packbeasts cast %d, groups of three minted %d and sacrificed whole %d, partial %d; Ruse cast %d, others %v",
		tal.games, tal.divs, tal.stalls, tal.packbeasts, tal.minted3, tal.sacked3, tal.partial, tal.ruse, tal.returns)
	for _, c := range []struct {
		name string
		n    int
	}{{"Packbeasts casts", tal.packbeasts}, {"groups of three minted", tal.minted3}, {"groups of three sacrificed", tal.sacked3}, {"Ruse casts", tal.ruse},
		{"Deprive casts", tal.returns["Deprive"]}, {"Devour in Flames casts", tal.returns["Devour in Flames"]}, {"Disappearing Act casts", tal.returns["Disappearing Act"]}} {
		if c.n < 10 {
			t.Errorf("%s: %d, want at least 10", c.name, c.n)
		}
	}
}
