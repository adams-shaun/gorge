package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The granted-horsemanship lockstep (CR 702.31b): Riding Red Hare's Pump KW$
// Horsemanship is an until-end-of-turn ExtKwMark, and Riding the Dilu Horse's
// Duration$ Permanent one an override record's Horsemanship; the kernel reads
// both beside the printed CardDef.Evasion bit, as the attacker's restriction and as a
// blocker's exception. The pool is dirac/cardset/pools/fixtures/
// horsemanshipgrant: Red Hare beside a printed horsemanship creature (Wu
// Scout) and creatures without it. The kernel's GrantedHorse count proves a
// granted horsemanship decided a block, so a green game is not one where the
// mark never mattered.
const horsemanshipGrantPool = "horsemanshipgrant"

var horsemanshipGrantDecks = [2][]cardset.Entry{
	{
		{Name: "Riding Red Hare", Count: 6}, {Name: "Riding the Dilu Horse", Count: 4}, {Name: "Glory Seeker", Count: 8}, {Name: "Wu Scout", Count: 2}, {Name: "Plains", Count: 12}, {Name: "Forest", Count: 8},
	},
	{
		{Name: "Wu Scout", Count: 6}, {Name: "Grizzly Bears", Count: 8}, {Name: "Glory Seeker", Count: 6}, {Name: "Plains", Count: 12}, {Name: "Forest", Count: 8},
	},
}

// TestHorsemanshipGrantLockstep: no divergence and no kernel stall over the
// seeds, and across them a granted horsemanship forbade a block.
func TestHorsemanshipGrantLockstep(t *testing.T) {
	reg, set, gd := lockstepPool(t, horsemanshipGrantPool, horsemanshipGrantDecks)
	st := &dirac.LandwalkStats{}
	dirac.DebugLandwalkStats = st
	defer func() { dirac.DebugLandwalkStats = nil }()
	clean, _, divs := lockstepSeedsMode(t, reg, set, horsemanshipGrantPool, horsemanshipGrantDecks, gd, 1, 24, lockstepFailEach, nil, nil, castFirstPolicy)
	if len(divs) > 0 {
		t.Errorf("%d divergent games", len(divs))
	}
	if st.GrantedHorse == 0 {
		t.Error("no game restricted a block through a granted horsemanship")
	}
	t.Logf("%d clean games; %+v", clean, *st)
}
