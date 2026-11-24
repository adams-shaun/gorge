package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The gated-Play lockstep (kgen lower_zone.go play's gate keys, kernel
// OpSkipUnless over the CastFree/Hideaway play): a Play node's Condition*
// gate is the resolution's SkipIf, skipped (with the ask) when the gate
// denies and the chain continues to its SubAbility. The carriers:
// Cemetery Tampering, Rabble Rousing and Wiretapping (Defined$ ExiledWith
// behind a present-zone count — twenty owned cards in graveyards, ten
// creatures you control, nine cards in hand), Howltooth Hollow (the same,
// both hands empty), Coveted Prize (Count$Party EQ4 over a hand play) and
// Hidetsugu and Kairi (Remembered$Valid Instant,Sorcery EQ1 over the
// remembered play). Flawless Forgery, Herald of Amity and Lieutenant Nyota
// Uhura ride the round's P1 clause shapes; Dance with Calamity, Portent of
// Calamity and West Coast Expansion share the gate keys but keep another
// blocker each, so no deck plays them.
var kplaygatesDecks = [2][]cardset.Entry{
	// the hideaway trio behind a gate, over a self-milling deck (Cemetery
	// Tampering's twenty owned cards in graveyards), Flawless Forgery's
	// copied casts and Herald of Amity's dug auras
	{
		{Name: "Cemetery Tampering", Count: 3}, {Name: "Rabble Rousing", Count: 4}, {Name: "Wiretapping", Count: 2},
		{Name: "Howltooth Hollow", Count: 2}, {Name: "Flawless Forgery", Count: 2}, {Name: "Herald of Amity", Count: 2},
		{Name: "Hedron Crab", Count: 3}, {Name: "Tome Scour", Count: 3}, {Name: "Armored Skaab", Count: 1},
		{Name: "Windfall", Count: 2}, {Name: "Raise the Alarm", Count: 2}, {Name: "Memnite", Count: 1},
		{Name: "Unholy Strength", Count: 2}, {Name: "Swamp", Count: 6}, {Name: "Island", Count: 5},
	},
	// Coveted Prize's party of four, Hidetsugu and Kairi's remembered
	// instant or sorcery, and Lieutenant Nyota Uhura's dug instants and
	// sorceries
	{
		{Name: "Hidetsugu and Kairi", Count: 4}, {Name: "Coveted Prize", Count: 4}, {Name: "Eleven, the Mage", Count: 2},
		{Name: "Lieutenant Nyota Uhura", Count: 2}, {Name: "Accorder Paladin", Count: 4}, {Name: "Cleric of Life's Bond", Count: 4},
		{Name: "Lightning Bolt", Count: 5}, {Name: "Divination", Count: 3}, {Name: "Tome Scour", Count: 3},
		{Name: "Plains", Count: 3}, {Name: "Island", Count: 3}, {Name: "Mountain", Count: 2}, {Name: "Swamp", Count: 1},
	},
}

// gatePlayPolicy answers at random and counts gorge's pending play asks
// whose SA carries one of the gate keys this slice lowered (playShapesCount's
// shape, playshapes_lockstep_test.go): the pool must pose them, or the
// lockstep proves nothing about the SkipIf.
func gatePlayPolicy(t *pgTally) func(uint64) Policy {
	return func(seed uint64) Policy {
		rnd := RandomPolicy(seed)
		return PolicyFunc(func(c *PickCtx) Canon {
			a := rnd.Pick(c)
			if c.G == nil {
				return a
			}
			pd := c.G.Pending()
			if pd == nil || pd.ResumeKind != "play" || pd.ResumeSA == nil {
				return a
			}
			for _, k := range []cards.ParamKey{cards.PKConditionPresent, cards.PKConditionCheckSVar} {
				if pd.ResumeSA.ParamStr(k) != "" {
					t.gated++
					return a
				}
			}
			return a
		})
	}
}

// pgTally counts the gated play asks the pool posed.
type pgTally struct{ gated int }

func TestPlayGatesLockstep(t *testing.T) {
	t.Parallel()
	tally := new(pgTally)
	lockstepRunPolicy(t, "playgates", kplaygatesDecks, 200, true, nil, nil, gatePlayPolicy(tally))
	if tally.gated == 0 {
		t.Error("no gated play ask was posed: the pool does not exercise the SkipIf")
	}
	t.Logf("gated play asks posed: %d", tally.gated)
}
