package kgen

import "testing"

// countConditionsLowered are the gap-count-conditions cards this slice
// lowers: the two-colour devotion gods and March of the Canonized
// (Count$DevotionDual, the Devotion head over a union mask), Decommission
// (bare Condition$ Revolt) and Poison the Cup (bare Condition$ Foretold); then
// Teachings of the Archaics and Anchor to Reality (a Count$Compare bound that
// names an SVar), Nasty End (Sacrificed$Valid Card.Legendary), the four
// Count$MostProminentCreatureType cards with a Creature.YouCtrl spec (Kindred
// Summons reads it only in an AI SVar), and Gonti's Machinations (the bare
// AFLifeLost amount after a LoseLife); then three Masteries
// (Count$AltCost.1.0: the stack entry's trigPlAltCost mark, ExAltCost).
var countConditionsLowered = []string{
	"Athreos, God of Passage", "Iroas, God of Victory", "Karametra, God of Harvests",
	"Klothys, God of Destiny", "Mogis, God of Slaughter", "Phenax, God of Deception",
	"Xenagos, God of Revels", "March of the Canonized", "Decommission", "Poison the Cup",
	"Teachings of the Archaics", "Anchor to Reality", "Nasty End",
	"White Lotus Tile", "Basalt Ravager", "Littjara Kinseekers", "Skemfar Shadowsage",
	"Gonti's Machinations", "Kindred Summons",
	"Baleful Mastery", "Fervent Mastery", "Ingenious Mastery",
}

// countConditionsRefused are the cards the slice still refuses, each a class of
// its own with a kernel read (or a gorge model) the slice does not carry:
// Hellbent and Surge (gorge fails both closed: conditionUnmodelledBare), the
// loyalty/equip/power-up activation kinds (the activation ledger records no
// kind), Graxiplon's DefenderCtrl read in a CantBlockBy static, the kicker-{X}
// Count$xPaid forms, Devastating and Verdant Mastery (their Count$AltCost
// lowers; a ChangeZone to or under the ChosenPlayer does not), Ugin's
// Labyrinth's ExiledWithSource count, the TargetedByTarget$ and
// TargetedController$CardsInGraveyard heads, Galloping Lizrog's "any number"
// RemoveCounter and Sanctum Prelate's CantBeCast bound "Chosen". A card that
// lowers must move to countConditionsLowered with its lockstep evidence.
var countConditionsRefused = []string{
	"Twinstrike", "Crush of Tentacles",
	"Kiora of Salt and Sand", "Bruenor Battlehammer", "Advancing the Spirit",
	"Graxiplon",
	"Devastating Mastery", "Verdant Mastery",
	"Verdeloth the Ancient", "Kangee, Aerie Keeper", "Plumb the Forbidden", "Relentless Dead",
	"Ugin's Labyrinth", "Not of This World", "Bane's Contingency", "Lullmage's Domination",
	"Galloping Lizrog", "Sanctum Prelate",
}

func TestCountConditionsStillRefused(t *testing.T) {
	for _, name := range countConditionsRefused {
		if c := lowerNamedCard(t, name); len(c.Refusals) == 0 {
			t.Errorf("%s lowers now: move it to countConditionsLowered with its lockstep evidence", name)
		}
	}
}

func TestCountConditionsSlice(t *testing.T) {
	for _, name := range countConditionsLowered {
		if c := lowerNamedCard(t, name); len(c.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", name, c.Refusals)
		}
	}
}

// TestDevotionDualNeedsTwoColours: gorge fails a DevotionDual head without
// exactly two readable colours closed, so kgen refuses it rather than lowering
// a one-colour devotion.
func TestDevotionDualNeedsTwoColours(t *testing.T) {
	for _, rest := range []string{"DevotionDual.White", "DevotionDual.White.Blue.Red", "DevotionDual."} {
		if _, ok, err := devotionHead(rest); !ok || err == nil {
			t.Errorf("%s: ok=%v err=%v, want a refusal", rest, ok, err)
		}
	}
	for _, rest := range []string{"DevotionDual.White.Blue", "Devotion.White", "Devotion.White.Blue"} {
		if _, ok, err := devotionHead(rest); !ok || err != nil {
			t.Errorf("%s: ok=%v err=%v, want it lowered", rest, ok, err)
		}
	}
}
