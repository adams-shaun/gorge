package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Dauthi Voidwalker lockstep: an opponent's card is exiled with a VOID
// counter, its activated ability's ChooseCard over exile finds it, and the
// free end-of-turn play lowers to a permission the carrier can cast. The check
// proves each half happened, so a run that never lowers the rider or the
// reader cannot pass vacuously.
var dauthiVoidwalkerDecks = [2][]cardset.Entry{
	{
		{Name: "Dauthi Voidwalker", Count: 4}, {Name: "Doom Blade", Count: 4},
		{Name: "Grizzly Bears", Count: 10}, {Name: "Swamp", Count: 22},
	},
	{
		{Name: "Grizzly Bears", Count: 12}, {Name: "Doom Blade", Count: 4},
		{Name: "Swamp", Count: 8}, {Name: "Forest", Count: 16},
	},
}

func TestDauthiVoidwalkerLockstep(t *testing.T) {
	t.Parallel()
	reg, set, gd := lockstepPool(t, "exileonleavectr", dauthiVoidwalkerDecks)
	tbl, _, _, err := set.Matchup(dauthiVoidwalkerDecks[0], dauthiVoidwalkerDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	seen, playedSeeds := 0, 0
	for seed := uint64(1); seed <= 60; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"dauthi-a", "dauthi-b"}, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		saw, played := false, false
		prev := map[state.ObjID]*cards.Card{}
		rc := RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 30}
		rc.Check = func(ref *GorgeSide, _ Side) *Divergence {
			g := ref.E.G
			cur := map[state.ObjID]*cards.Card{}
			for p := state.PlayerID(0); p < 2; p++ {
				for _, o := range g.Zone(state.ZExile, p) {
					if g.Obj(o).Counter("VOID") > 0 {
						cur[o], saw = g.Obj(o).Card, true
					}
				}
			}
			// A carrier that left exile and now has the same card on the stack
			// or the battlefield was played by Dauthi's grant: the grant is the
			// only permission to cast an opponent's exiled card, and the cast
			// moves it out of exile.
			for o, card := range prev {
				if _, still := cur[o]; still {
					continue
				}
				for p := state.PlayerID(0); p < 2; p++ {
					for _, so := range g.Zone(state.ZStack, p) {
						if g.Obj(so).Card == card {
							played = true
						}
					}
					for _, so := range g.Zone(state.ZBattlefield, p) {
						if g.Obj(so).Card == card {
							played = true
						}
					}
				}
			}
			prev = cur
			return nil
		}
		res := Lockstep(m, seed, k, castFirstPolicy(seed), rc)
		CloseSide(k)
		if d := res.Div; d != nil {
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s", seed, d.At, d.What, d.Field, d.Diff, d.Detail)
		}
		if saw {
			seen++
		}
		if played {
			playedSeeds++
		}
	}
	if seen == 0 {
		t.Fatal("no seed reached an exiled VOID counter")
	}
	if playedSeeds == 0 {
		t.Fatal("no seed activated Dauthi Voidwalker to play the exiled card")
	}
	t.Logf("%d of 60 seeds reached an exiled VOID counter, %d played it", seen, playedSeeds)
}
