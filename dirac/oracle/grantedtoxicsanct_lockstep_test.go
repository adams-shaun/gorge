package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// A granted toxic beside Personal Sanctuary's prevention (no printed toxic or
// infect in the pool): the table is not an infect table for the replacement
// ask and beside a Saga (a lore counter trigger), since toxic is a trigger (CR 702.164b) and nothing orders. 64 seeds
// play without a divergence or a stall, and the grants placed poison, so a
// green game is not one where no grant mattered.
const grantedToxicSanctPool = "grantedtoxicsanct"

var grantedToxicSanctDecks = [2][]cardset.Entry{
	{
		{Name: "Necrogen Communion", Count: 4}, {Name: "Muck Rats", Count: 6}, {Name: "Prosthetic Injector", Count: 4}, {Name: "Personal Sanctuary", Count: 4},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Plains", Count: 8}, {Name: "Swamp", Count: 6}, {Name: "Forest", Count: 4},
	},
	{
		{Name: "Aspirant's Ascent", Count: 6}, {Name: "Grizzly Bears", Count: 6}, {Name: "Glory Seeker", Count: 6}, {Name: "Personal Sanctuary", Count: 4},
		{Name: "Prosthetic Injector", Count: 2}, {Name: "History of Benalia", Count: 4}, {Name: "Island", Count: 4}, {Name: "Plains", Count: 4}, {Name: "Forest", Count: 4},
	},
}

func TestGrantedToxicSanctuaryLockstep(t *testing.T) {
	reg, set, gd := lockstepPool(t, grantedToxicSanctPool, grantedToxicSanctDecks)
	st := &dirac.ToxicStats{}
	dirac.DebugToxicStats = st
	defer func() { dirac.DebugToxicStats = nil }()
	c, _, d := lockstepSeedsMode(t, reg, set, grantedToxicSanctPool, grantedToxicSanctDecks, gd, 1, 64, lockstepFailEach, nil, nil, castFirstPolicy)
	if len(d) > 0 {
		t.Errorf("%d divergent games", len(d))
	}
	if st.GrantedPoison == 0 {
		t.Error("no game placed poison through a granted toxic")
	}
	t.Logf("%d clean games; %+v", c, *st)
}
