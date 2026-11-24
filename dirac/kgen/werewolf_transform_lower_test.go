package kgen

import "testing"

// werewolfCarriers carry Phase$ Upkeep triggers gated by
// WerewolfTransformCondition$ (front faces: "if no spells were cast last
// turn, transform") or WerewolfUntransformCondition$ (back faces, and Call of
// the Full Moon: "if a player cast two or more spells last turn"). Each lowers
// to a CondValue over kop.ExSpellsCastLastTurn (value_spellslastturn.go).
var werewolfCarriers = []string{
	"Breakneck Rider", "Call of the Full Moon", "Convicted Killer", "Daybreak Ranger", "Duskwatch Recruiter",
	"Gatstaf Arsonists", "Gatstaf Shepherd", "Grizzled Outcasts", "Hanweir Watchkeep", "Hermit of the Natterknolls",
	"Hinterland Hermit", "Hinterland Logger", "Huntmaster of the Fells", "Instigator Gang", "Kessig Forgemaster",
	"Kruin Outlaw", "Lambholt Elder", "Lambholt Pacifist", "Mayor of Avabruck", "Mondronen Shaman",
	"Reckless Waif", "Scorned Villager", "Solitary Hunter", "Tormented Pariah", "Ulrich of the Krallenhorde",
	"Ulvenwald Mystics", "Village Ironsmith", "Village Messenger", "Villagers of Estwald", "Wolfbitten Captive",
}

func TestWerewolfTransformLowers(t *testing.T) {
	for _, name := range werewolfCarriers {
		if c := lowerNamedCard(t, name); len(c.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", name, c.Refusals)
		}
	}
}
