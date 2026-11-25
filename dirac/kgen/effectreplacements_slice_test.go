package kgen

import "testing"

// effectReplacementsSlice are the Effect-issued replacement carriers that
// lower whole: the stack-exile grant from an enters trigger or on a targeted
// graveyard card (lower_effectstackexile.go), the chosen-source shield's
// target, any-recipient, turn-long and gain-rider forms (lower_choosesource.go)
// and Gideon's prevent-all-damage-to-him Effect (lower_preventallto.go).
var effectReplacementsSlice = []string{
	"Dire Fleet Daredevil", "Sins of the Past",
	"Auriok Replica", "Burrenton Forge-Tender", "Charm Peddler", "Circle of Despair", "Martyr's Cause",
	"Pay No Heed", "Pilgrim of Justice", "Pilgrim of Virtue", "Prahv, Spires of Order", "Rith's Charm",
	"Samite Blessing", "Sanctum Guardian",
	"Cho-Arrim Alchemist", "Intervention Pact", "Reverse Damage",
	"Gideon, Ally of Zendikar", "Gideon, Champion of Justice", "Gideon, Martial Paragon",
}

func TestEffectReplacementsSlice(t *testing.T) {
	for _, name := range effectReplacementsSlice {
		if c := lowerNamedCard(t, name); len(c.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", name, c.Refusals)
		}
	}
}
