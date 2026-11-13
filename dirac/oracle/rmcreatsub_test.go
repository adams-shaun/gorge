package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The RemoveCreatureTypes lockstep: the four
// Auras that make the enchanted creature a Citizen / Noggle / Turtle and
// strip its other creature types (gorge applyTypeEffect, RemoveCreatureTypes$
// without RemoveCardTypes$) beside the tribal lords and anthems that read a
// creature type, so an enchanted Elf / Goblin / Merfolk / Knight leaves the
// lord's count, and an animated Vehicle keeps its Vehicle subtype.

var krmcreatsubDecks = [][2][]cardset.Entry{{
	// U/W Auras and tribal creatures, against the tribal G/R deck
	{
		{Name: "Noggle the Mind", Count: 3}, {Name: "Spider-Man No More", Count: 3},
		{Name: "Honest Work", Count: 3}, {Name: "Retro-Mutation", Count: 3},
		{Name: "Merfolk Sovereign", Count: 2}, {Name: "Merfolk of the Pearl Trident", Count: 2},
		{Name: "Knight Exemplar", Count: 2}, {Name: "Elite Vanguard", Count: 2},
		{Name: "Savannah Lions", Count: 2}, {Name: "Smuggler's Copter", Count: 2},
		{Name: "Island", Count: 10}, {Name: "Plains", Count: 6},
	},
	{
		{Name: "Elvish Archdruid", Count: 3}, {Name: "Llanowar Elves", Count: 4}, {Name: "Elvish Mystic", Count: 3},
		{Name: "Elvish Visionary", Count: 3}, {Name: "Imperious Perfect", Count: 3}, {Name: "Goblin Chieftain", Count: 3},
		{Name: "Raging Goblin", Count: 2}, {Name: "Goblin Piker", Count: 3},
		{Name: "Forest", Count: 9}, {Name: "Mountain", Count: 7},
	},
}, {
	// the Aura deck against itself, and a tribal deck with Auras on its own side
	{
		{Name: "Noggle the Mind", Count: 3}, {Name: "Spider-Man No More", Count: 3},
		{Name: "Honest Work", Count: 3}, {Name: "Retro-Mutation", Count: 3},
		{Name: "Merfolk Sovereign", Count: 2}, {Name: "Merfolk of the Pearl Trident", Count: 2},
		{Name: "Knight Exemplar", Count: 2}, {Name: "Elite Vanguard", Count: 2},
		{Name: "Savannah Lions", Count: 2}, {Name: "Smuggler's Copter", Count: 2},
		{Name: "Island", Count: 10}, {Name: "Plains", Count: 6},
	},
	{
		{Name: "Noggle the Mind", Count: 3}, {Name: "Retro-Mutation", Count: 3},
		{Name: "Elvish Archdruid", Count: 3}, {Name: "Llanowar Elves", Count: 3}, {Name: "Elvish Mystic", Count: 3},
		{Name: "Imperious Perfect", Count: 3}, {Name: "Goblin Chieftain", Count: 3}, {Name: "Raging Goblin", Count: 2},
		{Name: "Silvercoat Lion", Count: 1}, {Name: "Grizzly Bears", Count: 1},
		{Name: "Forest", Count: 7}, {Name: "Island", Count: 8},
	},
}}

var (
	krmcreatsubAuras = map[string]bool{"Noggle the Mind": true, "Spider-Man No More": true, "Honest Work": true, "Retro-Mutation": true}
	krmcreatsubLords = map[string]bool{"Merfolk Sovereign": true, "Knight Exemplar": true, "Elvish Archdruid": true,
		"Imperious Perfect": true, "Goblin Chieftain": true}
)

// krmcreatsubStats counts, over a run, the distinct Auras attached to an
// opposing creature and the decisions where a creature-type lord is in play
// beside an enchanted creature.
type krmcreatsubStats struct{ onOpp, typeReads int }

// krmcreatsubPolicy casts whenever it can (so the Auras land), else random;
// it counts the gorge side's board at each decision.
func krmcreatsubPolicy(seed uint64, st *krmcreatsubStats, seen map[state.ObjID]bool) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			g := c.G.E.G
			lord, enchanted := false, false
			for i := range g.Objs {
				o := &g.Objs[i]
				if o.Zone != state.ZBattlefield || o.Face() == nil {
					continue
				}
				name := o.Face().Name
				if krmcreatsubLords[name] {
					lord = true
				}
				if !krmcreatsubAuras[name] || o.AttachedTo == 0 {
					continue
				}
				enchanted = true
				if t := g.Obj(o.AttachedTo); t != nil && t.Controller != o.Controller && !seen[o.ID] {
					seen[o.ID] = true
					st.onOpp++
				}
			}
			if lord && enchanted {
				st.typeReads++
			}
		}
		buf = buf[:0]
		for _, a := range c.D.Options {
			if k := a.Kind(); k == ActCast || k == ActActivate {
				buf = append(buf, a)
			}
		}
		if len(buf) == 0 {
			return rnd.Pick(c)
		}
		d := *c.D
		d.Options = buf
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

// runRmcreatsub plays the constructed matchups and returns the games, the
// divergences, the stalls and the stats. keep skips the creature-type strip
// (the mutation must diverge).
func runRmcreatsub(t *testing.T, seeds int, keep bool) (games, divs, stalls int, st krmcreatsubStats) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("rmcreatsub")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugRmCreatSubKeep
	dirac.DebugRmCreatSubKeep = keep
	defer func() { dirac.DebugRmCreatSubKeep = old }()
	for mi, md := range krmcreatsubDecks {
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
		for seed := uint64(1); seed <= uint64(seeds); seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"krmcreatsub-a", "krmcreatsub-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			pol := krmcreatsubPolicy(seed, &st, map[state.ObjID]bool{})
			g, err := NewGorgeSide(m, seed, cfg, nil)
			if err != nil {
				t.Fatal(err)
			}
			res := LockstepLed(m, seed, g, k, pol, RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
			CloseSide(k)
			games++
			if strings.HasPrefix(res.Stall, KernelStallPrefix) {
				stalls++
			}
			if d := res.Div; d != nil {
				divs++
				if !keep {
					t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				}
			}
		}
	}
	return games, divs, stalls, st
}

func TestRmcreatsubLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, stalls, st := runRmcreatsub(t, 256, false)
	t.Logf("%d games, %d divergences, %d stalls, %d Auras on an opposing creature, %d type-reading decisions with an enchanted creature",
		games, divs, stalls, st.onOpp, st.typeReads)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	if st.onOpp < 30 {
		t.Errorf("only %d Auras resolved on an opposing creature", st.onOpp)
	}
	if st.typeReads < 10 {
		t.Errorf("only %d decisions with a lord beside an enchanted creature", st.typeReads)
	}
}

// TestRmcreatsubMutation skips the creature-type strip (the add-only
// reading): an enchanted creature keeps its Elf / Goblin / Merfolk types
// and the lords still count it, so the games must diverge.
func TestRmcreatsubMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, _, _ := runRmcreatsub(t, 64, true)
	if divs == 0 {
		t.Fatalf("mutation kept the creature types in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}

// An animated Treasure keeps its Treasure subtype (CR 205.1a): Mimic's Animate
// strips creature types only, so Gold Rush still counts it.
func TestAnimateRemoveCreatureTypesKeepsTreasure(t *testing.T) {
	t.Parallel()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("rmcreatsub")
	if err != nil {
		t.Fatal(err)
	}
	deck := []cardset.Entry{
		{Name: "Mimic", Count: 10}, {Name: "Gold Rush", Count: 8}, {Name: "Grizzly Bears", Count: 6},
		{Name: "Forest", Count: 16},
	}
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	var cards40 []*cards.Card
	for _, e := range deck {
		c, ok := reg.Lookup(e.Name)
		if !ok {
			t.Fatalf("%s not in the corpus", e.Name)
		}
		for i := 0; i < e.Count; i++ {
			cards40 = append(cards40, c)
		}
	}
	animated := 0
	for seed := uint64(1); seed <= 40; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"mimic-a", "mimic-b"}, [2][]*cards.Card{cards40, cards40}, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		rnd := krmcreatsubPolicy(seed, new(krmcreatsubStats), map[state.ObjID]bool{})
		pol := PolicyFunc(func(c *PickCtx) Canon {
			if c.G != nil {
				for i := range c.G.E.G.Objs {
					o := &c.G.E.G.Objs[i]
					if o.Zone == state.ZBattlefield && o.Face() != nil && o.Face().Name == "Mimic" && c.G.E.IsCreature(o.ID) {
						animated++
					}
				}
			}
			return rnd.Pick(c)
		})
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
		CloseSide(k)
		if d := res.Div; d != nil {
			t.Errorf("seed %d: diverged at decision %d: %s %s %v", seed, d.At, d.What, d.Field, d.Diff)
		}
	}
	if animated == 0 {
		t.Error("no game animated a Mimic")
	}
}
