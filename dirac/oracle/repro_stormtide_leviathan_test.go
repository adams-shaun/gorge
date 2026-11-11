package oracle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Stormtide Leviathan's "Creatures without flying or islandwalk can't
// attack" is a CantAttack static whose ValidCard$ reads the negated exact
// keyword !hasKeywordLandwalk:Island over the derived keywords: kgen lowers it
// to PredWord7 / W7LacksIslandwalk, and dirac reads a printed landwalk or a
// Continuous static's granted one (Lord of Atlantis's islandwalk for the
// other Merfolk), gorge's keywordPredicateMatches.
//
// The recorded game: the first player casts Lord of Atlantis, Merfolk of the
// Pearl Trident and Giant Octopus, attacks with everything it may, and later
// casts Stormtide Leviathan. Before the Leviathan the Octopuses attack; once it
// is out the Octopuses may not (no flying, no islandwalk) while a Merfolk (islandwalk
// from the Lord) and the Leviathan (printed islandwalk) may.
var stormtideCase = effectLongCase{
	name:   "repro_stormtide_leviathan_cant_attack_without_islandwalk",
	prefer: []string{"Lord of Atlantis", "Merfolk of the Pearl Trident", "Giant Octopus", "Stormtide Leviathan"},
	decks: [2][]string{
		append(append(append(append(deckOf(3, "Stormtide Leviathan"), deckOf(2, "Lord of Atlantis")...),
			deckOf(2, "Merfolk of the Pearl Trident")...), deckOf(3, "Giant Octopus")...), deckOf(14, "Island")...),
		append(deckOf(16, "Island"), deckOf(8, "Giant Octopus")...),
	},
	surface: "autopay",
	reached: stormtideAttack,
}

// stormtideAttack reads the finished gorge log: the first player controls a
// Lord of Atlantis and some Giant Octopuses that were there at its previous
// attack (so none is summoning sick); once its Leviathan is out it attacks
// (Player is the defender) with the Leviathan and a Merfolk and with none of
// those Octopuses.
func stormtideAttack(g *GorgeSide) bool {
	name := func(id state.ObjID) string {
		if o := g.E.G.Obj(id); o != nil && o.Face() != nil {
			return o.Face().Name
		}
		return ""
	}
	leviathan := false
	alive := map[state.ObjID]bool{} // the first player's battlefield permanents of the three names
	seasoned := map[state.ObjID]bool{}
	for i := range g.E.L.Events {
		e := &g.E.L.Events[i]
		switch {
		case e.Kind == events.MoveZone && e.To == state.ZBattlefield:
			if o := g.E.G.Obj(e.Obj); o != nil && o.Controller == 0 {
				switch name(e.Obj) {
				case "Stormtide Leviathan":
					leviathan = true
					alive[e.Obj] = true
				case "Lord of Atlantis", "Giant Octopus":
					alive[e.Obj] = true
				}
			}
		case e.Kind == events.MoveZone && e.From == state.ZBattlefield:
			delete(alive, e.Obj)
			delete(seasoned, e.Obj)
		case e.Kind == events.DeclareAttackers && e.Player == 1:
			var lev, merfolk bool
			for _, id := range e.IDs {
				switch name(id) {
				case "Stormtide Leviathan":
					lev = true
				case "Merfolk of the Pearl Trident":
					merfolk = true
				case "Giant Octopus":
					if leviathan {
						return false // the gate failed to hold an Octopus back
					}
				}
			}
			lord, octopus := false, false
			for id := range alive {
				lord = lord || name(id) == "Lord of Atlantis"
				octopus = octopus || name(id) == "Giant Octopus" && seasoned[id]
			}
			if leviathan && lev && merfolk && lord && octopus {
				return true
			}
			for id := range alive {
				seasoned[id] = true
			}
		}
	}
	return false
}

// stormtidePolicy casts in the preferred order, plays lands and attacks with
// every creature it is offered; anything else is the seed's random pick.
func stormtidePolicy(m *Matchup, seed uint64) Policy {
	base := effectLongPolicy(m, stormtideCase, seed)
	return PolicyFunc(func(p *PickCtx) Canon {
		if p.D.Kind == DAttackers && p.D.Player == 0 {
			for _, o := range p.D.Options {
				if o.Kind() == ActAttack {
					return o
				}
			}
		}
		return base.Pick(p)
	})
}

// TestReproStormtideLeviathanReach replays the recorded game: no divergence,
// every scripted action offered, and the attack the log reads above reached.
func TestReproStormtideLeviathanReach(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	r, err := ReadRepro(filepath.Join("testdata", "repro", stormtideCase.name+".repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	script := &ScriptPolicy{Actions: r.Actions}
	res, reached := effectLongPlay(t, reg, stormtideCase, r, printedDelveSet(t, r.Cards), script)
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if script.Skipped != 0 {
		t.Errorf("%d scripted actions were not offered", script.Skipped)
	}
	if !reached {
		t.Fatal("the recorded game never reaches the attack under Stormtide Leviathan")
	}
}

// TestReproStormtideLeviathanRecord plays seeds until one reaches the attack
// and writes the repro. It runs only with STORMTIDE_RECORD=1.
func TestReproStormtideLeviathanRecord(t *testing.T) {
	if os.Getenv("STORMTIDE_RECORD") == "" {
		return // a recording tool, not coverage
	}
	reg := synthReg(t)
	c := stormtideCase
	for seed := uint64(1); seed <= 400; seed++ {
		r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"},
			Cards: c.decks, Seed: seed, PassElision: true, Surface: c.surface, Kernel: "custom"}
		set := reproSet(t, "custom", r)
		m, err := r.Matchup(reg)
		if err != nil {
			t.Fatal(err)
		}
		res, reached := effectLongPlay(t, reg, c, r, set, stormtidePolicy(m, seed))
		if res.Div != nil {
			t.Fatalf("seed %d: divergence at %d: %s %s %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
		}
		if !reached {
			continue
		}
		r.setActions(res.Actions)
		if err := r.Write(filepath.Join("testdata", "repro", c.name+".repro.json")); err != nil {
			t.Fatal(err)
		}
		t.Logf("seed %d: %d decisions", seed, len(res.Actions))
		return
	}
	t.Fatal("no seed reached the attack under Stormtide Leviathan")
}
