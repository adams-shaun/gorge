package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// kbfStats tallies the band formation lockstep.
type kbfStats struct {
	asks     int // CR 702.22c band declaration asks posed
	accepts  int // a candidate answered: a band formed
	bandTurn int // turns with a band in which a blocker was declared
}

// kbfProbe plays the pumpendcombat decks (kpecDecks) and accepts the first
// candidate of each band ask: a band of two forms, the re-ask is declined (the
// kernel blocks a band of two; a blocker's third block stalls).
type kbfProbe struct {
	st     *kbfStats
	base   Policy
	pj     Projection
	joined bool  // the previous pick was an accepted candidate (this ask is its re-ask)
	turn   int32 // the turn of the last accepted band (0 none)
	seen   int32 // the turn whose blockers were already counted
}

func (p *kbfProbe) Pick(c *PickCtx) Canon {
	if c.G != nil && p.turn != 0 && p.seen != p.turn {
		c.G.Project(&p.pj)
		if p.pj.Turn == p.turn {
			for i := 0; i < int(p.pj.NInst) && i < MaxInst; i++ {
				if p.pj.Cards[i].Zone == ZoneBattlefield && p.pj.Cards[i].Blocks != RefNone {
					p.st.bandTurn++
					p.seen = p.turn
					break
				}
			}
		}
	}
	code, _ := chooseCode("band")
	var decline, cand Canon
	var haveDecline, haveCand bool
	for _, a := range c.D.Options {
		if a.Kind() != ActChoose+CanonKind(code) {
			continue
		}
		if a.Src() == RefNone {
			decline, haveDecline = a, true
		} else if !haveCand {
			cand, haveCand = a, true
		}
	}
	if !haveDecline {
		p.joined = false
		return p.base.Pick(c)
	}
	p.st.asks++
	if haveCand && !p.joined {
		c.G.Project(&p.pj)
		p.st.accepts++
		p.joined = true
		p.turn = p.pj.Turn
		return cand
	}
	p.joined = false
	return decline
}

// kbfRun plays seeds first..first+seeds-1 of the pumpendcombat pool.
func kbfRun(t *testing.T, first, seeds uint64) (games, divs, stalls int, st kbfStats) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("pumpendcombat")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kpecDecks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				decks[s] = append(decks[s], c)
			}
		}
	}
	tbl, _, _, err := set.Matchup(kpecDecks[0], kpecDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := first; seed < first+seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kbf-a", "kbf-b"}, decks, 1)
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
		probe := &kbfProbe{st: &st, base: poisonhighPolicy(seed, map[string]int{}, map[string]int{})}
		res := LockstepLed(m, seed, g, k, probe, RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 24})
		CloseSide(k)
		games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			t.Errorf("seed %d: stalled: %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			divs++
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	return
}

// TestBandFormationLockstep: an accepted band ask plays (CR 702.22c-e, h): the
// kernel joins the candidate to the leader's band (dirac/combat_band.go),
// re-asks the leader, and a blocker declared against one member blocks the
// other. Battering Ram, which has banding until end of combat, attacks beside
// another creature and the probe accepts. Seeds beyond the decline probe's.
func TestBandFormationLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, stalls, st := kbfRun(t, 2001, 128)
	t.Logf("%d games, %+v, %d divergences, %d stalls", games, st, divs, stalls)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	if st.accepts < 5 {
		t.Errorf("only %d bands formed (want at least 5)", st.accepts)
	}
	if st.bandTurn < 2 {
		t.Errorf("only %d banded turns with a blocker (want at least 2)", st.bandTurn)
	}
}
