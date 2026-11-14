package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestMBCTriggerReferents5Lowers plays the MBC trigger-referents-5 family's
// spellings in lockstep against gorge on a table built for exactly their
// decks. Of the family's four spellings only one has a lockstep game today:
//
//   - the cast-provenance words on an other-object ETB replacement's entrant
//     filter (Venser, Visionary Traveler: "Each nontoken creature you control
//     that wasn't cast from your hand enters with two additional +1/+1
//     counters on it", K:ETBReplacement:Other with the filter
//     Creature.!token+YouCtrl+!wasCastFromYourHand). A creature reanimated by
//     Zombify while Venser is on the battlefield enters under the
//     replacement: its entry is not a hand cast, so it arrives with the two
//     counters. A creature cast from hand enters through the same program
//     and must not match the filter, so the kernel's entrant read has to
//     tell the two provenances apart (kop.PredWasCastSpell through the
//     resolving spell during the ReplOthersEnterProgram match) -- a
//     lowering that read either provenance wrong would diverge the moment a
//     hand-cast creature entered beside a reanimated one.
//
//   - Sahir, Visitor in Darkness's TriggerZones$ "Command, Battlefield"
//     Eminence phase trigger and its LifeLost ValidCause$
//     SpellAbility.YouCtrl trigger lower into the kernel, but the card still
//     refuses for the family's own out-of-scope blockers (the unread
//     param:trigger.DamageAmount / param:trigger.LifeAmount of the same
//     triggers), so there is no table to play it on yet.
//
//   - Yume, Chronicler of Valor's DB$ Draw Defined$
//     "You & TriggeredSpellAbility>ManaSpender.Other" lowers (the draw's
//     payer is the controller in both engines: the kernel's payments are
//     single-payer, so ManaSpender.Other is empty, and gorge's
//     ValidSAMatches fails closed on the two-field clause, so the
//     SpellCast trigger never fires there either), but the card still
//     refuses on ValidSA "Spell.ManaSpentBy Player.Other" and the
//     mana-abilities work -- out of scope.
func TestMBCTriggerReferents5Lowers(t *testing.T) {
	if isolate(t) {
		return
	}
	bears := []cardset.Entry{{Name: "Grizzly Bears", Count: 16}, {Name: "Forest", Count: 24}}
	decks := [2][]cardset.Entry{
		{
			{Name: "Venser, Visionary Traveler", Count: 4}, {Name: "Zombify", Count: 8},
			{Name: "Sarcomite Myr", Count: 12},
			{Name: "Island", Count: 8}, {Name: "Plains", Count: 4}, {Name: "Swamp", Count: 4},
		},
		bears,
	}
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	var names [2][]string
	var cs [2][]*cards.Card
	for s, d := range decks {
		for _, e := range d {
			names[s] = append(names[s], e.Name)
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for range e.Count {
				cs[s] = append(cs[s], c)
			}
		}
	}
	set := printedDelveSet(t, names)
	tbl, _, _, err := set.Matchup(decks[0], decks[1])
	if err != nil {
		t.Fatal(err)
	}
	reached := 0
	for seed := uint64(1); seed <= 16; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"trigrefs5-a", "trigrefs5-b"}, cs, 0)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		hit := false
		rc := RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40}
		rc.Check = func(ref *GorgeSide, _ Side) *Divergence {
			for _, id := range ref.E.G.Zone(state.ZBattlefield, 0) {
				o := ref.E.G.Obj(id)
				if o == nil || o.Face() == nil || o.Face().Name != "Sarcomite Myr" {
					continue
				}
				for _, c := range o.Counters {
					if c.Kind == "P1P1" && c.N >= 2 {
						hit = true
					}
				}
			}
			return nil
		}
		res := Lockstep(m, seed, k, RandomPolicy(seed), rc)
		CloseSide(k)
		if err := unmodelledStallError(t.Name(), &res); err != nil {
			t.Errorf("seed %d: %v", seed, err)
		}
		if d := res.Div; d != nil {
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v\nrecent %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions, res.RecentSources(m, 6))
		}
		if hit {
			reached++
		}
	}
	t.Logf("the mechanism reached in %d of 16 games", reached)
	if reached < 3 {
		t.Errorf("the mechanism reached in %d of 16 games, want at least 3", reached)
	}
}
