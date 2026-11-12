package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The Silas Renn lockstep: his combat-damage trigger chooses target artifact
// card in a graveyard and lets its controller cast it this turn. The
// artifact lands (Seat of the Synod, Vault of Whispers) are legal targets
// that must get no permission (a land is never cast), where the artifact
// spells do. Against Goblin attackers, Shock and Hill Giant as blockers.
// The fixture pool is dirac/cardset/pools/fixtures/silasgrave.
var ksilasgraveDecks = [2][]cardset.Entry{
	{
		{Name: "Silas Renn, Seeker Adept", Count: 6}, {Name: "Thought Scour", Count: 10},
		{Name: "Ornithopter", Count: 4}, {Name: "Mind Stone", Count: 6},
		{Name: "Seat of the Synod", Count: 5}, {Name: "Vault of Whispers", Count: 5},
		{Name: "Island", Count: 2}, {Name: "Swamp", Count: 2},
	},
	{
		{Name: "Shock", Count: 4}, {Name: "Mountain", Count: 28},
		{Name: "Raging Goblin", Count: 8},
	},
}

func TestSilasGraveLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "silasgrave", ksilasgraveDecks, 300, "KSILASGRAVE_SEEDS")
}

// TestSilasGraveLockstepMutation proves the pool reaches the check: with
// DebugMayPlayTargetLands the kernel grants the targeted artifact land a
// permission, and the lockstep diverges. The preamble asserts both values
// the check compares exist: Silas's grant carries MayPlayNonLand over
// target slot 0, and Seat of the Synod is a land in the pool.
func TestSilasGraveLockstepMutation(t *testing.T) {
	set, err := cardset.RawByName("silasgrave")
	if err != nil {
		t.Fatal(err)
	}
	tbl, _, _, err := set.Matchup(ksilasgraveDecks[0], ksilasgraveDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	land := false
	grant := false
	for i := range tbl.Cards {
		cd := &tbl.Cards[i]
		if cd.Name == "Seat of the Synod" {
			land = cd.Types&kop.TLand != 0
		}
		if cd.Name != "Silas Renn, Seeker Adept" {
			continue
		}
		for j := range cd.Abilities {
			for _, in := range cd.Abilities[j].Prog {
				if in.Op == kop.OpMayPlay && in.Obj == kop.RefTarget0 && in.B.N&kop.MayPlayNonLand != 0 {
					grant = true
				}
			}
		}
	}
	if !land {
		t.Fatal("Seat of the Synod is not a land in the silasgrave table: the type check never fires")
	}
	if !grant {
		t.Fatal("Silas Renn has no OpMayPlay over target 0 with MayPlayNonLand: the pool does not reach the check")
	}
	old := dirac.DebugMayPlayTargetLands
	dirac.DebugMayPlayTargetLands = true
	defer func() { dirac.DebugMayPlayTargetLands = old }()
	if !lockstepDiverges(t, "silasgrave", ksilasgraveDecks, 300) {
		t.Fatal("no seed diverged with targeted lands granted a permission: the pool never targets an artifact land")
	}
}
