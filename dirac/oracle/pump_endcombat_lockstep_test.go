package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The Pump Duration$ UntilEndOfCombat lockstep: Murk Dwellers' +2/+0 when it
// attacks and isn't blocked, Flummoxed Cyclops' and Forgestoker Dragon's
// "can't block this combat". The kernel's override record (kop.Override.
// UntilEndCombat) must show gorge's powers at every decision: applied from
// creation and gone once the end-of-combat step ends.
//
// Battering Ram (granted Banding, kop.Override.Banding) is in the decks too:
// gorge poses the CR 702.22c band declaration ask for a declared attacker with
// banding beside another attacker, and the kernel mirrors it (dirac/
// combat_band.go; the grant is the record dirac/kw_banding.go reads). Band
// formation is not modelled (an accepted band stalls, StallBand), so the
// probe's policy declines every band ask: neither engine ever forms a band.

var kpecDecks = [2][]cardset.Entry{
	{
		{Name: "Murk Dwellers", Count: 6},
		{Name: "Flummoxed Cyclops", Count: 4},
		{Name: "Forgestoker Dragon", Count: 4},
		{Name: "Battering Ram", Count: 4},
		{Name: "Swamp", Count: 11},
		{Name: "Mountain", Count: 11},
	},
	{
		{Name: "Murk Dwellers", Count: 4},
		{Name: "Flummoxed Cyclops", Count: 6},
		{Name: "Forgestoker Dragon", Count: 4},
		{Name: "Battering Ram", Count: 4},
		{Name: "Grizzly Bears", Count: 4},
		{Name: "Swamp", Count: 9},
		{Name: "Mountain", Count: 9},
	},
}

type kpecStats struct {
	pumped   int // a Murk Dwellers first seen above its printed power
	expired  int // a pumped Murk Dwellers seen at its printed power again
	bandAsks int // a CR 702.22c band declaration ask posed (the probe declines it)
}

// kpecDeclineBand is the band ask's decline (option 0, naming no object) when
// d is a band declaration: the kernel does not model band formation.
func kpecDeclineBand(d *CDecision) (Canon, bool) {
	code, _ := chooseCode("band")
	for _, a := range d.Options {
		if a.Kind() == ActChoose+CanonKind(code) && a.Src() == RefNone {
			return a, true
		}
	}
	return 0, false
}

// kpecProbe wraps the casting policy and reads the powers through gorge's
// projection at every decision (gorge leads, so PickCtx.G is set).
type kpecProbe struct {
	st     *kpecStats
	base   Policy
	tbl    *kop.Table
	pj     Projection
	pumped [MaxInst]bool
}

func (p *kpecProbe) Pick(c *PickCtx) Canon {
	if c.G != nil {
		c.G.Project(&p.pj)
		dwellers := nameHash("Murk Dwellers")
		for i := 0; i < int(p.pj.NInst) && i < MaxInst; i++ {
			o := &p.pj.Cards[i]
			if o.Zone != ZoneBattlefield || o.NameHash != dwellers || o.Card == NoCardIndex || int(o.Card) >= len(p.tbl.Cards) {
				p.pumped[i] = false
				continue
			}
			switch printed := int16(p.tbl.Cards[o.Card].Power); {
			case o.Power > printed && !p.pumped[i]:
				p.pumped[i] = true
				p.st.pumped++
			case o.Power == printed && p.pumped[i]:
				p.pumped[i] = false
				p.st.expired++
			}
		}
	}
	if a, ok := kpecDeclineBand(c.D); ok {
		p.st.bandAsks++
		return a
	}
	return p.base.Pick(c)
}

// kpecRun plays seeds 1..seeds and returns the games, the divergences and
// stalls (reported as test errors when report is set) and the probe tallies.
func kpecRun(t *testing.T, seeds uint64, report bool) (games, divs, stalls int, st kpecStats) {
	t.Helper()
	return kpecRunFrom(t, 1, seeds, report)
}

// kpecRunFrom plays seeds first..first+seeds-1.
func kpecRunFrom(t *testing.T, first, seeds uint64, report bool) (games, divs, stalls int, st kpecStats) {
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
		if len(decks[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	tbl, _, _, err := set.Matchup(kpecDecks[0], kpecDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := first; seed < first+seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kpec-a", "kpec-b"}, decks, 1)
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
		probe := &kpecProbe{st: &st, base: poisonhighPolicy(seed, map[string]int{}, map[string]int{}), tbl: tbl}
		res := LockstepLed(m, seed, g, k, probe, RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 24})
		CloseSide(k)
		games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			if report {
				t.Errorf("seed %d: stalled: %s", seed, res.Stall)
			}
		}
		if d := res.Div; d != nil {
			divs++
			if report {
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	return
}

func TestPumpEndCombatLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, stalls, st := kpecRun(t, 128, true)
	t.Logf("%d games, %+v, %d divergences, %d stalls", games, st, divs, stalls)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	if st.pumped < 5 {
		t.Errorf("only %d Murk Dwellers pumps seen (want at least 5)", st.pumped)
	}
	if st.expired < 5 {
		t.Errorf("only %d pumps seen to end (want at least 5)", st.expired)
	}
}

// TestPumpEndCombatMutation: a record that outlives the end-of-combat step
// must make the lockstep diverge.
func TestPumpEndCombatMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugNoOvEndCombat = true
	defer func() { dirac.DebugNoOvEndCombat = false }()
	games, divs, _, _ := kpecRun(t, 32, false)
	if divs == 0 {
		t.Fatalf("mutation kept every record in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}
