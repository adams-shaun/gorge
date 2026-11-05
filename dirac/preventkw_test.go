package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// newPreventKWScen builds a scenario over the preventkw fixture pool: the
// printed K:Prevent carriers and the creatures and burn the tests play
// against them. The pool is loaded raw (lowered, unverified included): the
// keyword's card-def facts are what is under test.
func newPreventKWScen(t *testing.T, deck []cardset.Entry) (*scen, map[string]CardID) {
	t.Helper()
	set, err := cardset.RawByName("preventkw")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range deck {
		if _, ok := set.Lookup(e.Name); !ok {
			t.Fatalf("%s not in the preventkw pool", e.Name)
		}
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	return newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true}), id
}

// TestPreventKeywordCombatToAndBy: Fog Bank ("Prevent all combat damage that
// would be dealt to and dealt by CARDNAME.") blocks a 3/3. The blocker is
// dealt nothing, and it deals nothing back: assignment still happens (the
// attacker assigns lethal and the blocker assigns its power), then the whole
// event is prevented.
func TestPreventKeywordCombatToAndBy(t *testing.T) {
	sc, id := newPreventKWScen(t, []cardset.Entry{
		{Name: "Fog Bank", Count: 1}, {Name: "Hill Giant", Count: 1}, {Name: "Plains", Count: 8},
	})
	fb := sc.perm(id["Fog Bank"], 1)
	giant := sc.perm(id["Hill Giant"], 0)
	sc.combat(t, []Obj{giant}, [][2]Obj{{fb, giant}})
	if d := sc.s.perms[sc.slot(fb)].damage; d != 0 {
		t.Fatalf("Fog Bank damage = %d, want 0 (combat damage to it is prevented)", d)
	}
	if d := sc.s.perms[sc.slot(giant)].damage; d != 0 {
		t.Fatalf("Hill Giant damage = %d, want 0 (combat damage by Fog Bank is prevented)", d)
	}
	if !sc.onBF(fb) || !sc.onBF(giant) {
		t.Fatalf("Fog Bank alive=%v, Hill Giant alive=%v; want both alive (no damage was dealt)", sc.onBF(fb), sc.onBF(giant))
	}
}

// TestPreventKeywordCombatToOnly: Guard Gomazoa ("Prevent all combat damage
// that would be dealt to CARDNAME.") blocks the 3/3. It is dealt nothing but
// deals its 1 power back: the sentence names only the "to" direction.
func TestPreventKeywordCombatToOnly(t *testing.T) {
	sc, id := newPreventKWScen(t, []cardset.Entry{
		{Name: "Guard Gomazoa", Count: 1}, {Name: "Hill Giant", Count: 1}, {Name: "Plains", Count: 8},
	})
	gom := sc.perm(id["Guard Gomazoa"], 1)
	giant := sc.perm(id["Hill Giant"], 0)
	sc.combat(t, []Obj{giant}, [][2]Obj{{gom, giant}})
	if d := sc.s.perms[sc.slot(gom)].damage; d != 0 {
		t.Fatalf("Guard Gomazoa damage = %d, want 0 (combat damage to it is prevented)", d)
	}
	if d := sc.s.perms[sc.slot(giant)].damage; d != 1 {
		t.Fatalf("Hill Giant damage = %d, want 1 (Guard Gomazoa's 1 power is dealt back)", d)
	}
}

// TestPreventKeywordAllDamageTo: Dawn Elemental ("Prevent all damage that
// would be dealt to CARDNAME.") takes no noncombat damage either, and the
// same burn still lands on another creature. Fog Bank's combat-only sentence
// keeps a noncombat hit, which pins the two scopes apart.
func TestPreventKeywordAllDamageTo(t *testing.T) {
	sc, id := newPreventKWScen(t, []cardset.Entry{
		{Name: "Dawn Elemental", Count: 1}, {Name: "Fog Bank", Count: 1}, {Name: "Grizzly Bears", Count: 1},
		{Name: "Shock", Count: 1}, {Name: "Plains", Count: 8},
	})
	dawn := sc.perm(id["Dawn Elemental"], 0)
	fog := sc.perm(id["Fog Bank"], 0)
	bear := sc.perm(id["Grizzly Bears"], 0)
	shock := id["Shock"]
	if n := sc.s.replDamage(sc.k, 0, NoSlot, shock, uint8(dawn), false, 3); n != 0 {
		t.Fatalf("Dawn Elemental took %d of a 3-damage burn, want 0 (all damage to it is prevented)", n)
	}
	if n := sc.s.replDamage(sc.k, 0, NoSlot, shock, uint8(fog), false, 3); n != 3 {
		t.Fatalf("Fog Bank took %d of a 3-damage burn, want 3 (its sentence is combat-scoped)", n)
	}
	if n := sc.s.replDamage(sc.k, 0, NoSlot, shock, uint8(bear), false, 3); n != 3 {
		t.Fatalf("Grizzly Bears took %d of a 3-damage burn, want 3 (no prevention)", n)
	}
}
