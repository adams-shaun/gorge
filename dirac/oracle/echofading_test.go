package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The echo and fading lockstep. Echo (CR 702.30a; Cradle Guard, Mogg War
// Marshal, Avalanche Riders) asks its controller, at the first upkeep after the
// creature came under their control, to pay the echo cost or sacrifice it
// (gorge's echo_pay / echo_sac); Act of Treason hands a creature to the other
// player for a turn, so the gate (the acquisition against the controller's last
// upkeep) is read for a creature that changed hands twice. Fading (CR 702.32a;
// Blastoderm) enters with three fade counters, removes one each upkeep and is
// sacrificed at the upkeep that finds none. kgen lowers echo to a
// CondEchoOwed-gated trigger over ExtCumulative B=1 and an UnlessPaid, and
// fading to ordinary counter ops; both engines decide every pay, decline, cast
// and attack together.

var kechoDecks = [2][]cardset.Entry{
	{
		{Name: "Cradle Guard", Count: 4}, {Name: "Mogg War Marshal", Count: 4}, {Name: "Avalanche Riders", Count: 3},
		{Name: "Blastoderm", Count: 3}, {Name: "Act of Treason", Count: 3}, {Name: "Shock", Count: 3},
		{Name: "Hill Giant", Count: 4}, {Name: "Forest", Count: 8}, {Name: "Mountain", Count: 8},
	},
	{
		{Name: "Cradle Guard", Count: 4}, {Name: "Mogg War Marshal", Count: 4}, {Name: "Avalanche Riders", Count: 3},
		{Name: "Blastoderm", Count: 3}, {Name: "Act of Treason", Count: 3}, {Name: "Shock", Count: 3},
		{Name: "Hill Giant", Count: 4}, {Name: "Forest", Count: 8}, {Name: "Mountain", Count: 8},
	},
}

type echoStats struct {
	games, divs, stalls int
	echoPaid            int // an echo creature still on the battlefield two turns after it first appeared
	echoGone            int // an echo creature in a graveyard two turns after it first appeared
	fade3, fade2, fade1 int // Blastoderm observations with that many fade counters
	fade0               int // Blastoderm on the battlefield with none left
	fadeSacrificed      int // a Blastoderm that held fade counters, in a graveyard
	stolen              int // an echo creature under its owner's opponent's control (Act of Treason)
}

func echoCheck(ref *GorgeSide, first map[state.ObjID]int32, seenFade map[state.ObjID]bool, st *echoStats) {
	turn := ref.E.G.Turn
	for _, pl := range []state.PlayerID{0, 1} {
		for _, id := range ref.E.G.Zone(state.ZBattlefield, pl) {
			o := ref.E.G.Obj(id)
			if o == nil || o.Face() == nil {
				continue
			}
			switch o.Face().Name {
			case "Cradle Guard", "Mogg War Marshal", "Avalanche Riders":
				if o.Controller != o.Owner {
					st.stolen++
				}
				if t, ok := first[id]; !ok {
					first[id] = turn
				} else if turn >= t+2 && turn == t+2 {
					st.echoPaid++
				}
			case "Blastoderm":
				seenFade[id] = true
				switch o.Counter("FADE") {
				case 3:
					st.fade3++
				case 2:
					st.fade2++
				case 1:
					st.fade1++
				case 0:
					st.fade0++
				}
			}
		}
		for _, id := range ref.E.G.Zone(state.ZGraveyard, pl) {
			o := ref.E.G.Obj(id)
			if o == nil || o.Face() == nil {
				continue
			}
			if t, ok := first[id]; ok && turn == t+2 {
				st.echoGone++
				delete(first, id)
			}
			if o.Face().Name == "Blastoderm" && seenFade[id] {
				st.fadeSacrificed++
				delete(seenFade, id)
			}
		}
	}
}

func runEchoFading(t *testing.T, seeds int) (st echoStats) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("echofading")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kechoDecks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				decks[s] = append(decks[s], c)
			}
		}
		if len(decks[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	tbl, _, _, err := set.Matchup(kechoDecks[0], kechoDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kecho-a", "kecho-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		g, err := NewGorgeSide(m, seed, cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		first := map[state.ObjID]int32{}
		seenFade := map[state.ObjID]bool{}
		res := LockstepLed(m, seed, g, k, castFirstPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 24,
			Check: func(ref *GorgeSide, _ Side) *Divergence {
				echoCheck(ref, first, seenFade, &st)
				return nil
			}})
		CloseSide(k)
		st.games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			st.stalls++
			t.Errorf("seed %d: stalled: %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			st.divs++
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	return st
}

func TestEchoFadingLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	st := runEchoFading(t, 64)
	t.Logf("%+v", st)
	if st.divs != 0 || st.stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", st.divs, st.stalls)
	}
	// preconditions: the games reached paid and unpaid echoes and a Blastoderm's whole fade
	if st.echoPaid == 0 || st.echoGone == 0 || st.fade3 == 0 || st.fade1 == 0 || st.fadeSacrificed == 0 || st.stolen == 0 {
		t.Fatalf("the games never reached the echo and fading outcomes: %+v", st)
	}
}
