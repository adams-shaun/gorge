package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The First Contact lockstep: "if this spell is the first spell you've cast
// this game, you gain 2 life", stored at cast by a static SpellCast trigger
// (ExFirstCast). Both seats cast it, beside other spells to cast before it,
// Twincast (a copy spell, itself a cast) and counterspells to cast in
// response, so the flag is fixed at cast: a response resolving first does not
// take it away, an earlier cast does. The expected answer at every decision is
// gorge's.
var kfirstContactDecks = [2][]cardset.Entry{
	{
		{Name: "First Contact", Count: 4}, {Name: "Twincast", Count: 3},
		{Name: "Divination", Count: 3}, {Name: "Counterspell", Count: 2},
		{Name: "Grizzly Bears", Count: 3}, {Name: "Llanowar Elves", Count: 3},
		{Name: "Forest", Count: 11}, {Name: "Island", Count: 11},
	},
	{
		{Name: "First Contact", Count: 3}, {Name: "Counterspell", Count: 4},
		{Name: "Shock", Count: 4}, {Name: "Lightning Bolt", Count: 4},
		{Name: "Hill Giant", Count: 3}, {Name: "Island", Count: 8},
		{Name: "Mountain", Count: 7}, {Name: "Forest", Count: 7},
	},
}

func TestFirstContactLockstep(t *testing.T) {
	t.Parallel()
	reg, set, decks, tbl := p3Setup(t, "firstcontact", kfirstContactDecks)
	st := &dirac.FirstCastStats
	s0, l0, r0 := st.Stamped.Load(), st.Later.Load(), st.ReadTrue.Load()
	p3Lockstep(t, "firstcontact", reg, set, decks, tbl, 600)
	stamped, later, read := st.Stamped.Load()-s0, st.Later.Load()-l0, st.ReadTrue.Load()-r0
	t.Logf("%d first casts stamped, %d later casts, %d resolutions read the stamp", stamped, later, read)
	if stamped == 0 || later == 0 || read == 0 {
		t.Fatal("the pool does not exercise the flag: need first casts, later casts and a resolution reading the stamp")
	}
}

// TestFirstContactMutation proves the pool exercises the stored-at-cast
// property: with the flag read at resolution instead (a spell cast in
// response takes it from the earlier spell), a game that responds to a first
// First Contact diverges from gorge.
func TestFirstContactMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	reg, set, decks, tbl := p3Setup(t, "firstcontact", kfirstContactDecks)
	old := dirac.DebugFirstCastLate
	dirac.DebugFirstCastLate = true
	defer func() { dirac.DebugFirstCastLate = old }()
	for seed := uint64(1); seed <= 600; seed++ {
		m, err := NewMatchup(reg, [2]string{"firstcontact-a", "firstcontact-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		cfg := Config{}
		if seed%4 == 0 {
			cfg = Config{GorgeAutopaySurface: true}
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, castFirstPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		CloseSide(k)
		if res.Div != nil {
			t.Logf("seed %d diverges with the flag read at resolution (decision %d: %s)", seed, res.Div.At, res.Div.What)
			return
		}
	}
	t.Fatal("no seed diverged with the flag read at resolution: the pool does not exercise a response to a first cast")
}
