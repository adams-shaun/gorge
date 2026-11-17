package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The self "if this would be put into a graveyard ..." lockstep: Possessed
// Skaab exiles itself instead of dying, Gravebane Zombie goes on top of its
// owner's library and Nissa's Chosen on the bottom. The kernel's
// ReplExileOnLeave self filter (kgen lower_repl.go selfExile) and its
// ReplGraveToLibrary position bits (GraveToLibrary) must move the card where
// gorge's replacementMatches and ChangeZone bodies do.
//
// Both decks hold all three carriers plus Murder, so every removal spell has
// a carrier to target and the replacement is reached; the policy prefers the
// carriers and Murder and counts both casts, so a run where nothing died
// fails instead of passing vacuously. The carriers are not in the mbc pool,
// so the table is built from the decks' own cards (printedDelveSet).
func selfGraveReplDecks() [2][]cardset.Entry {
	d := []cardset.Entry{
		{Name: "Possessed Skaab", Count: 4},
		{Name: "Gravebane Zombie", Count: 4},
		{Name: "Nissa's Chosen", Count: 4},
		{Name: "Murder", Count: 8},
		{Name: "Swamp", Count: 10},
		{Name: "Forest", Count: 6},
		{Name: "Island", Count: 4},
	}
	return [2][]cardset.Entry{append([]cardset.Entry(nil), d...), append([]cardset.Entry(nil), d...)}
}

// selfGraveReplPolicy casts the carriers and Murder whenever offered (the
// removal's target is picked at random among the carriers, the decks' only
// creatures) and otherwise plays at random.
func selfGraveReplPolicy(st *map[string]int) func(uint64) Policy {
	return func(seed uint64) Policy {
		rnd := RandomPolicy(seed)
		return PolicyFunc(func(c *PickCtx) Canon {
			if c.G != nil && c.D.Kind == DPriority {
				var carriers, removal []Canon
				for _, a := range c.D.Options {
					if k := a.Kind(); k != ActCast && k != ActCastFace {
						continue
					}
					switch optionFaceName(c, a) {
					case "Possessed Skaab", "Gravebane Zombie", "Nissa's Chosen":
						carriers = append(carriers, a)
					case "Murder":
						removal = append(removal, a)
					}
				}
				if len(removal) > 0 {
					(*st)["removal"]++
					d := *c.D
					d.Options = removal
					return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
				}
				if len(carriers) > 0 {
					(*st)["carriers"]++
					d := *c.D
					d.Options = carriers
					return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
				}
			}
			return rnd.Pick(c)
		})
	}
}

func TestSelfGraveReplLockstep(t *testing.T) {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	decks := selfGraveReplDecks()
	var names [2][]string
	var gd [2][]*cards.Card
	for s, d := range decks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				names[s] = append(names[s], e.Name)
				gd[s] = append(gd[s], c)
			}
		}
		if len(gd[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(gd[s]))
		}
	}
	set := printedDelveSet(t, names)
	st := &map[string]int{}
	clean, casts, div := lockstepSeeds(t, reg, set, "selfgraverepl", decks, gd, 1, 80, true, nil, nil, selfGraveReplPolicy(st))
	t.Logf("%d clean seeds, %d casts/activations, %d divergent games; carrier casts %d, Murder casts %d",
		clean, casts, div, (*st)["carriers"], (*st)["removal"])
	if div != 0 {
		t.Fatalf("%d divergences", div)
	}
	if (*st)["carriers"] == 0 || (*st)["removal"] == 0 {
		t.Fatalf("carriers cast %d, Murder cast %d: the games never reached the replacement", (*st)["carriers"], (*st)["removal"])
	}
}
