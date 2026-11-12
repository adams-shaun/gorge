package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestGrantedManaAbilityFiresNoAbilityCast: Imprisoned in the Moon grants
// "{T}: Add {C}" at its own ability index. The planner's view of the
// imprisoned creature carries that index, which on a creature whose printed
// ability 0 is an exhaust mana ability (Loot, the Pathfinder) names the
// exhaust one. Tapping the grant is no exhaust activation, so a listener
// ("whenever you activate an exhaust ability", Rangers' Aetherhive) queues
// nothing; the same view of an unimprisoned creature queues it (CR 605.1a).
// No pool holds both cards, so the FDN table is given the two printed lines.
func TestGrantedManaAbilityFiresNoAbilityCast(t *testing.T) {
	deck := []cardset.Entry{{Name: "Imprisoned in the Moon", Count: 1}, {Name: "Eager Trufflesnout", Count: 2}, {Name: "Island", Count: 10}}
	tbl, da, _, err := cardsettest.Synth(t, "std-fdn").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	// Loot's `{G}, {T}, Exhaust: add three mana of any one colour`, first on the creature.
	loot := kop.Ability{Kind: kop.AbMana, Exhaust: true, ManaColors: kop.ColAny, ManaN: 3, Cost: kop.Cost{Flags: kop.CostTap}}
	loot.Cost.Mana.Pips[kop.G] = 1
	cr := &tbl.Cards[id["Eager Trufflesnout"]]
	cr.Abilities = append([]kop.Ability{loot}, cr.Abilities...)
	// Rangers' Aetherhive's listener, on the Moon after its grant.
	moonCard := &tbl.Cards[id["Imprisoned in the Moon"]]
	moonCard.Abilities = append(moonCard.Abilities, kop.Ability{Kind: kop.AbTriggered, Trig: kop.Trigger{Event: kop.TrigAbilityCast, Whose: kop.WhoseYou, N: kop.TrigAbCastExhaust}})

	sc := newScenTable(t, tbl, Config{NoElide: true})
	s, k := sc.s, sc.k
	if k.gen == nil || !k.gen.anyAbilityCast || len(k.gen.grantPC) == 0 {
		t.Fatal("precondition: the table has no AbilityCast listener or no granted mana ability")
	}
	free := sc.perm(id["Eager Trufflesnout"], 0)
	prisoner := sc.perm(id["Eager Trufflesnout"], 0)
	moon := sc.perm(id["Imprisoned in the Moon"], 0)
	s.perms[sc.slot(moon)].attached = sc.slot(prisoner)
	s.loseAllOnset(k, sc.slot(moon), sc.slot(prisoner))
	if s.manaGranted(k, sc.slot(free)) || !s.manaGranted(k, sc.slot(prisoner)) {
		t.Fatal("precondition: the imprisoned creature is the only one read through the grant")
	}

	before := s.npend
	s.fireViewAbilityCast(k, 0, sc.slot(free), 0)
	if s.npend == before {
		t.Fatal("precondition: the creature's own exhaust mana activation queued no listener trigger")
	}
	before = s.npend
	s.fireViewAbilityCast(k, 0, sc.slot(prisoner), s.manaPCGen(k, sc.slot(prisoner)).manaAb)
	if s.npend != before {
		t.Fatalf("the granted {T}: Add {C} queued %d AbilityCast trigger(s), want none", s.npend-before)
	}
}
