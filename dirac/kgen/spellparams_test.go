package kgen

import (
	"strings"
	"testing"
)

// The spell-level shapes: a destroy filter with three negated subtypes, the
// split-destination search's NoLooking$ fetch, a spell shuffling itself into
// its owner's library, and a spell that adds mana.
var spellParamForms = []formCase{
	{card: "Victim of Night", form: "three negated subtypes in one clause", lir: "notsub=Vampire notsub=Werewolf notsub=Zombie"},
	{card: "Nissa's Pilgrimage", form: "NoLooking$ on the Defined$ fetch of the split search", lir: "Bounce obj=Remembered CS=from=Library|ShuffleOwed"},
	{card: "Blue Sun's Zenith", form: "Origin$ Stack Defined$ Parent shuffle", lir: "SpellToLib"},
	{card: "Seething Song", form: "SP$ Mana adds its mana as the spell resolves", lir: "AddMana obj=You A=5 CS=R"},
	{card: "Manamorphose", form: "SP$ Mana over a colour choice, then a draw", lir: "ManaChoice A=2 CS=WUBRG"},
}

func TestSpellParamForms(t *testing.T) { checkForms(t, spellParamForms) }

// TestNotSubtypeLimit: a clause keeps up to five distinct negated subtypes
// (NotSub and NotSubs); a repeated word adds nothing and a sixth refuses.
func TestNotSubtypeLimit(t *testing.T) {
	known := func(string) bool { return true }
	c, err := parseClause("Creature.nonVampire+nonWerewolf+nonZombie+nonHuman+nonBear+nonVampire", known)
	if err != nil {
		t.Fatalf("five distinct negations: %v", err)
	}
	if c.NotSubtype != "Vampire" || c.MoreNotSubtype != [4]string{"Werewolf", "Zombie", "Human", "Bear"} {
		t.Fatalf("negations = %q %v", c.NotSubtype, c.MoreNotSubtype)
	}
	_, err = parseClause("Creature.nonVampire+nonWerewolf+nonZombie+nonHuman+nonBear+nonSpirit", known)
	if err == nil || !strings.Contains(err.Error(), "six negated subtypes") {
		t.Fatalf("six negations: got %v", err)
	}
}
