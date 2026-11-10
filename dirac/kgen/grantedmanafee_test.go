package kgen

import "testing"

// grantedManaFeeForms pins the two Station lands whose threshold grants a
// coloured-fee, dynamic-amount mana ability to themselves. The grant lowers as
// the card's own second mana ability gated on the CHARGE count
// (lower_stationgrant.go stationGrantMana), which the kernel plays as a
// counters-gated one-pip fixed-colour filter beside the land's free {T}
// (dirac filterTapMana, mana_filterpip.go).
var grantedManaFeeForms = []formCase{
	{card: "Evendo, Waking Haven", form: "a Station-granted {G}, {T}: Add {G} per creature", lir: "ab[1] Mana  cost={0G} T"},
	{card: "Uthros, Titanic Godcore", form: "a Station-granted {U}, {T}: Add {U} per artifact", lir: "ab[1] Mana  cost={0U} T"},
	{card: "Evendo, Waking Haven", form: "the grant is gated on the source's 12 charge counters", lir: "cond=Present({types=0x0 Self cmp(3 1 12  CHARGE)},op1,1)"},
	{card: "Uthros, Titanic Godcore", form: "the product is the fee's colour, one per artifact", lir: "mana U xCount({types=0x4 ctrl=1})"},
}

func TestGrantedManaFee(t *testing.T) { checkForms(t, grantedManaFeeForms) }
