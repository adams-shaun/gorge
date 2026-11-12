package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The echo and phasing lockstep. Phasing changes no control (CR 702.26d), so
// an echo creature (CR 702.30a) phased out after it came under its
// controller's control, and phased back in during their untap step (CR
// 702.26a), still owes echo at the upkeep that follows: gorge's acquisition
// stamp ignores phasing, and dirac's gate (trig_echo.go echoSnap) reads the
// phased-out permanent's sickness before phaseUntap clears it. Vodalian
// Illusionist and Reality Ripple phase out creatures on both sides.

var kechoPhaseDeck = []cardset.Entry{
	{Name: "Raven Familiar", Count: 3}, {Name: "Pouncing Jaguar", Count: 4}, {Name: "Cradle Guard", Count: 3},
	{Name: "Vodalian Illusionist", Count: 4}, {Name: "Reality Ripple", Count: 4}, {Name: "Cloudskate", Count: 2},
	{Name: "Forest", Count: 10}, {Name: "Island", Count: 10},
}

type echoPhaseStats struct {
	games, divs, stalls int
	phasedOut           int // observations of an echo creature phased out
	phasedOwed          int // gorge's echo asks for a creature seen phased out since its controller's previous turn
}

var echoPhaseNames = map[string]bool{"Raven Familiar": true, "Pouncing Jaguar": true, "Cradle Guard": true}

// echoPhaseCheck records the turn each echo creature was last seen phased out
// and counts an echo ask (gorge's echo_sac option) for one seen phased out on
// one of the two turns before this one, once per creature and turn.
func echoPhaseCheck(ref *GorgeSide, phasedAt, asked map[state.ObjID]int32, st *echoPhaseStats) {
	g := ref.E.G
	for pl := range g.Players {
		for _, id := range g.Zone(state.ZBattlefield, state.PlayerID(pl)) {
			if o := g.Obj(id); o != nil && o.PhasedOut && o.Face() != nil && echoPhaseNames[o.Face().Name] {
				st.phasedOut++
				phasedAt[id] = g.Turn
			}
		}
	}
	d := ref.E.Pending()
	if d == nil {
		return
	}
	for _, op := range d.Options {
		if op.Kind != "echo_sac" {
			continue
		}
		if t, ok := phasedAt[op.Obj]; ok && t < g.Turn && g.Turn-t <= 2 && asked[op.Obj] != g.Turn {
			asked[op.Obj] = g.Turn
			st.phasedOwed++
		}
	}
}

func runEchoPhasing(t *testing.T, seeds int) (st echoPhaseStats) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("echophasing")
	if err != nil {
		t.Fatal(err)
	}
	var deck []*cards.Card
	for _, e := range kechoPhaseDeck {
		c, ok := reg.Lookup(e.Name)
		if !ok {
			t.Fatalf("%s not in the corpus", e.Name)
		}
		for i := 0; i < e.Count; i++ {
			deck = append(deck, c)
		}
	}
	if len(deck) != 40 {
		t.Fatalf("deck: %d cards", len(deck))
	}
	tbl, _, _, err := set.Matchup(kechoPhaseDeck, kechoPhaseDeck)
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kechophase-a", "kechophase-b"}, [2][]*cards.Card{deck, deck}, 1)
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
		phasedAt := map[state.ObjID]int32{}
		asked := map[state.ObjID]int32{}
		res := LockstepLed(m, seed, g, k, castFirstPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 24,
			Check: func(ref *GorgeSide, _ Side) *Divergence {
				echoPhaseCheck(ref, phasedAt, asked, &st)
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

func TestEchoPhasingLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	st := runEchoPhasing(t, 64)
	t.Logf("%+v", st)
	if st.divs != 0 || st.stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", st.divs, st.stalls)
	}
	// precondition: an echo creature phased out since its controller's
	// previous turn phased back in and was asked for its echo
	if st.phasedOut == 0 || st.phasedOwed == 0 {
		t.Fatalf("the games never asked echo of a creature that had phased out: %+v", st)
	}
}
