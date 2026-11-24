package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Rick, Steadfast Leader (Greymond, Avacyn's Stalwart) in lockstep: the
// as-enters GenericChoice of three StoreSVar flags (first strike and
// vigilance, vigilance and lifelink, first strike and lifelink), the three
// CheckSVar$ statics that read them, the four-Humans +2/+2, and the leaves
// reset. Blink (Cloudshift) and removal (Doom Blade, Swords) make Rick leave
// and return, which must forget the choice and ask again; Elite Vanguard,
// Thraben Inspector and Militia Bugler are the Humans, Adanto Vanguard and the
// Bears the non-Humans that must not gain the keywords.
//
// Two seats only: dirac is a two-seat kernel and kgen refuses a pod card.
var rickDecks = [2][]cardset.Entry{
	{
		{Name: "Rick, Steadfast Leader", Count: 6}, {Name: "Elite Vanguard", Count: 6}, {Name: "Thraben Inspector", Count: 4},
		{Name: "Militia Bugler", Count: 4}, {Name: "Adanto Vanguard", Count: 2}, {Name: "Cloudshift", Count: 3},
		{Name: "Plains", Count: 15},
	},
	{
		{Name: "Doom Blade", Count: 3}, {Name: "Shock", Count: 4}, {Name: "Swords to Plowshares", Count: 3},
		{Name: "Unsummon", Count: 4}, {Name: "Grizzly Bears", Count: 6},
		{Name: "Swamp", Count: 7}, {Name: "Mountain", Count: 7}, {Name: "Plains", Count: 6},
	},
}

// TestRickLockstep: gorge and dirac play the pool identically, and every
// choice was made at least once.
func TestRickLockstep(t *testing.T) {
	for i := range dirac.SiegeModeByIndex {
		dirac.SiegeModeByIndex[i].Store(0)
	}
	var st dirac.NumStoreStats
	dirac.DebugNumStoreStats = &st
	defer func() { dirac.DebugNumStoreStats = nil }()
	lockstepSetSerial(t, "rick", rickDecks, 400, "KRICK_SEEDS")
	for i := 0; i < 3; i++ {
		if dirac.SiegeModeByIndex[i].Load() == 0 {
			t.Errorf("choice %d was never made: the pool does not exercise it", i)
		}
	}
	if st.Writes == 0 || st.Hits == 0 || st.Cleared == 0 {
		t.Errorf("the pool never reached the store: %+v", st)
	}
	t.Logf("choices %d/%d/%d; store %+v", dirac.SiegeModeByIndex[0].Load(), dirac.SiegeModeByIndex[1].Load(), dirac.SiegeModeByIndex[2].Load(), st)
}

// TestRickMutation: with ExtStoreNum writing nothing some seed diverges, so the
// lockstep observes the chosen flags. Test-only global: never parallel.
func TestRickMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugNumStoreNoWrite = true
	defer func() { dirac.DebugNumStoreNoWrite = false }()
	if !lockstepDiverges(t, "rick", rickDecks, 400) {
		t.Fatal("no seed diverged with the flag writes dropped: the pool does not read the chosen flags")
	}
}

// TestRickThresholdMutation: with the four-Humans bound narrowed to three some
// seed diverges, so the pool reaches exactly the count the +2/+2 turns on.
func TestRickThresholdMutation(t *testing.T) {
	t.Parallel()
	changed := 0
	n := lockstepRun(t, "rick", rickDecks, 400, false, func(tbl *kop.Table) {
		for i := range tbl.Conds {
			if c := &tbl.Conds[i]; c.Kind == kop.CondPresent && c.A.K == kop.VConst && c.A.N == 4 {
				c.A.N = 3
				changed++
			}
		}
	})
	if changed == 0 {
		t.Fatal("no four-or-more present condition in the table")
	}
	if n == 0 {
		t.Fatal("no seed diverged with the Humans bound at three: the pool never sits at exactly three Humans beside a ready Rick")
	}
}
