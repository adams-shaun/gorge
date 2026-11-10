package kgen

import "testing"

// grantedManaFeeForms pins the two Station lands whose threshold grants a
// coloured-fee, dynamic-amount mana ability to themselves. The kernel prices a
// filter only with a generic fee and an any-colour (or combination) product
// (dirac filterTapMana), and pays its fee generically from the pool
// (filtDo), so a {G}, {T} ability making one G per creature (or {U} per
// artifact) is outside it: both lands stay refused, by the grant and by the
// second mana ability.
var grantedManaFeeForms = []formCase{
	{card: "Evendo, Waking Haven", form: "a Station-granted {G}, {T}: Add {G} per creature", refused: "granted mana ability with a mana fee"},
	{card: "Uthros, Titanic Godcore", form: "a Station-granted {U}, {T}: Add {U} per artifact", refused: "granted mana ability with a mana fee"},
	{card: "Evendo, Waking Haven", form: "the granted ability read as a second printed one", refused: "2 mana abilities: mana ability 1"},
	{card: "Uthros, Titanic Godcore", form: "the granted ability read as a second printed one", refused: "2 mana abilities: mana ability 1"},
}

func TestGrantedManaFee(t *testing.T) { checkForms(t, grantedManaFeeForms) }
