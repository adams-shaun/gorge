package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Will Riker, First in the Field lockstep: the beginning-of-combat trigger
// reveals until two lands, puts them onto the battlefield, puts the rest on
// the bottom in a random order and registers a delayed "sacrifice those lands
// at your next end step" trigger (ValidPlayer$ You). One matchup is land-rich
// (the dig finds two); the other is land-short (one Mountain and zero-cost
// Moxen) so the dig sometimes finds fewer than two. The lockstep compares the
// library's bottom order and the battlefield at every decision; wilrikerScan
// checks from gorge's events that both found-set sizes and the delayed
// sacrifice were actually reached.

var kwillrikerDecks = [][2][]cardset.Entry{
	{
		{
			{Name: "Will Riker, First in the Field", Count: 4},
			{Name: "Mountain", Count: 12}, {Name: "Forest", Count: 12}, {Name: "Island", Count: 12},
		},
		{
			{Name: "Will Riker, First in the Field", Count: 4},
			{Name: "Mountain", Count: 12}, {Name: "Forest", Count: 12}, {Name: "Island", Count: 12},
		},
	},
	{
		{
			{Name: "Will Riker, First in the Field", Count: 4},
			{Name: "Mountain", Count: 12}, {Name: "Forest", Count: 12}, {Name: "Island", Count: 12},
		},
		{
			{Name: "Will Riker, First in the Field", Count: 4},
			{Name: "Mountain", Count: 1},
			{Name: "Mox Ruby", Count: 7}, {Name: "Mox Jet", Count: 7}, {Name: "Mox Emerald", Count: 7},
			{Name: "Mox Sapphire", Count: 7}, {Name: "Mox Pearl", Count: 7},
		},
	},
}

// wilrikerScan counts, from gorge's event log, the delayed registrations by
// the number of lands the dig remembered (two, or fewer) and the lands the
// delayed trigger sacrificed at an end step.
func wilrikerScan(g *GorgeSide) (foundTwo, foundShort, sacked int) {
	if g == nil {
		return
	}
	reg := map[state.ObjID]bool{}
	for _, ev := range g.E.L.Events {
		switch ev.Kind {
		case events.DelayedRegister:
			if ev.Counter != "TrigSac" || !strings.Contains(ev.Text, "VP=You") {
				continue
			}
			if len(ev.IDs) >= 2 {
				foundTwo++
			} else {
				foundShort++
			}
			for _, id := range ev.IDs {
				reg[id] = true
			}
		case events.MoveZone:
			if ev.From != state.ZBattlefield || ev.To != state.ZGraveyard {
				continue
			}
			if reg[ev.Obj] {
				delete(reg, ev.Obj)
				sacked++
			}
		}
	}
	return
}

func runWillRiker(t *testing.T, seeds int, mutate bool) (games, divs, stalls, foundTwo, foundShort, sacked int) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("willriker")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugDelayYourEndStepOff
	dirac.DebugDelayYourEndStepOff = mutate
	defer func() { dirac.DebugDelayYourEndStepOff = old }()
	for mi, md := range kwillrikerDecks {
		var decks [2][]*cards.Card
		for s, d := range md {
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
				t.Fatalf("matchup %d deck %d: %d cards", mi, s, len(decks[s]))
			}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= uint64(seeds); seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"kwillriker-a", "kwillriker-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			pol := poisonhighPolicy(seed, map[string]int{}, map[string]int{})
			g, err := NewGorgeSide(m, seed, cfg, nil)
			if err != nil {
				t.Fatal(err)
			}
			res := LockstepLed(m, seed, g, k, pol, RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 20})
			CloseSide(k)
			games++
			two, short, sac := wilrikerScan(g)
			foundTwo += two
			foundShort += short
			sacked += sac
			if strings.HasPrefix(res.Stall, KernelStallPrefix) {
				stalls++
				if !mutate {
					t.Errorf("matchup %d seed %d: stalled: %s", mi, seed, res.Stall)
				}
			}
			if d := res.Div; d != nil {
				divs++
				if !mutate {
					t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				}
			}
		}
	}
	return
}

func TestWillRikerLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, stalls, foundTwo, foundShort, sacked := runWillRiker(t, 96, false)
	t.Logf("%d games, %d divergences, %d stalls; %d dig registrations found two lands, %d found fewer, %d lands sacrificed at the end step",
		games, divs, stalls, foundTwo, foundShort, sacked)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	if foundTwo == 0 || foundShort == 0 {
		t.Errorf("the lockstep did not reach both dig outcomes: %d found two, %d found fewer", foundTwo, foundShort)
	}
	if sacked == 0 {
		t.Errorf("the delayed sacrifice never fired in %d games", games)
	}
}

// TestWillRikerMutation drops the controller's-next-end-step program before it
// runs: the lands stay on the battlefield while gorge sacrifices them, so the
// lockstep must diverge. A green mutation means the test never reached the
// delayed trigger.
func TestWillRikerMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, _, _, _, _ := runWillRiker(t, 48, true)
	if divs == 0 {
		t.Fatalf("mutation dropped every controller's-end-step program in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}
