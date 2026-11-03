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

// The Explores / CollectEvidence trigger lockstep:
// Merfolk Cave-Diver and Nicanzil (a creature you control explores; Nicanzil's
// two lines read the revealed card), Evidence Examiner and Surveillance
// Monitor (you collect evidence) with explorers, evidence payers and burn to
// fill the graveyards. Both decks hold the carriers, so each side's explores
// and evidence reach the other side's ValidCard$ Creature.YouCtrl / ValidPlayer$
// You lines, which must stay silent. No ward-evidence card here: the ward
// payment's CollectEvidenceAction is pinned by TestStdSetReprosLockstep.

var ktrigexplevShell = []cardset.Entry{
	{Name: "Merfolk Cave-Diver", Count: 2}, {Name: "Nicanzil, Current Conductor", Count: 2},
	{Name: "Evidence Examiner", Count: 2}, {Name: "Surveillance Monitor", Count: 2},
	{Name: "Cenote Scout", Count: 3}, {Name: "Jadelight Ranger", Count: 2}, {Name: "Merfolk Branchwalker", Count: 3},
	{Name: "Forensic Researcher", Count: 2}, {Name: "Sample Collector", Count: 2},
	{Name: "Shock", Count: 2}, {Name: "Lightning Bolt", Count: 2},
}

var ktrigexplevDecks = [2][]cardset.Entry{
	append(append([]cardset.Entry(nil), ktrigexplevShell...),
		cardset.Entry{Name: "Forest", Count: 7}, cardset.Entry{Name: "Island", Count: 6}, cardset.Entry{Name: "Mountain", Count: 3}),
	append(append([]cardset.Entry(nil), ktrigexplevShell...),
		cardset.Entry{Name: "Forest", Count: 6}, cardset.Entry{Name: "Island", Count: 7}, cardset.Entry{Name: "Mountain", Count: 3}),
}

// ktrigexplevTally counts, from gorge's event log: the carrier triggers
// pushed after an explore (explFires; landReveal / nonlandReveal by the
// revealed card; nicLand / nicNon Nicanzil's two lines) or after a collect
// (evFires), and seat-1 collects while seat 0 controlled an evidence carrier
// (oppEvents).
type ktrigexplevTally struct {
	explFires, landReveal, nonlandReveal, nicLand, nicNon int
	evFires, oppEvents                                    int
}

var ktrigexplevExplCarriers = map[string]bool{"Merfolk Cave-Diver": true, "Nicanzil, Current Conductor": true}
var ktrigexplevEvCarriers = map[string]bool{"Evidence Examiner": true, "Surveillance Monitor": true}

// ktrigexplevPolicy plays randomly and scans the gorge log as it goes.
func ktrigexplevPolicy(seed uint64, tl *ktrigexplevTally) Policy {
	rnd := RandomPolicy(seed)
	at := 0
	var last events.Kind
	var lastLand bool
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			g := c.G.E.G
			evs := c.G.E.L.Events
			if len(evs) < at {
				at = 0
			}
			for j := at; j < len(evs); j++ {
				e := &evs[j]
				switch e.Kind {
				case events.Explore:
					last = e.Kind
					lastLand = false
					if len(e.IDs) > 0 {
						if o := g.Obj(e.IDs[0]); o != nil && o.Face() != nil {
							lastLand = o.Face().IsLand()
						}
					}
				case events.CollectEvidenceAction:
					last = e.Kind
					if e.Player == 1 && ktrigexplevHolds(g, 0, ktrigexplevEvCarriers) {
						tl.oppEvents++
					}
				case events.TriggerPush:
					o := g.Obj(e.Obj)
					if o == nil || o.Face() == nil {
						continue
					}
					name := o.Face().Name
					if last == events.Explore && ktrigexplevExplCarriers[name] {
						tl.explFires++
						if lastLand {
							tl.landReveal++
						} else {
							tl.nonlandReveal++
						}
						if name == "Nicanzil, Current Conductor" {
							if lastLand {
								tl.nicLand++
							} else {
								tl.nicNon++
							}
						}
					} else if last == events.CollectEvidenceAction && ktrigexplevEvCarriers[name] {
						tl.evFires++
					}
				}
			}
			at = len(evs)
		}
		return rnd.Pick(c)
	})
}

// ktrigexplevHolds reports whether seat p controls a card of set on the
// battlefield.
func ktrigexplevHolds(g *state.Game, p state.PlayerID, set map[string]bool) bool {
	for _, id := range g.Zone(state.ZBattlefield, p) {
		if o := g.Obj(id); o != nil && o.Controller == p && o.Face() != nil && set[o.Face().Name] {
			return true
		}
	}
	return false
}

func runTrigexplev(t *testing.T, seeds int, mutate bool) (games, divs, stalls int, tl ktrigexplevTally) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("trigexplev")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugExplEvAnyPlayer
	dirac.DebugExplEvAnyPlayer = mutate
	defer func() { dirac.DebugExplEvAnyPlayer = old }()
	var decks [2][]*cards.Card
	for s, d := range ktrigexplevDecks {
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
	tbl, _, _, err := set.Matchup(ktrigexplevDecks[0], ktrigexplevDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"ktrigexplev-a", "ktrigexplev-b"}, decks, 1)
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
		res := LockstepLed(m, seed, g, k, ktrigexplevPolicy(seed, &tl), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 25})
		CloseSide(k)
		games++
		if d := res.Div; d != nil {
			divs++
			if !mutate {
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			t.Logf("seed %d: %s", seed, res.Stall)
		}
	}
	return games, divs, stalls, tl
}

func TestTrigexplevLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, stalls, tl := runTrigexplev(t, 256, false)
	t.Logf("%d games, %d divergences, %d stalls; carrier triggers after an explore %d (land reveals %d, nonland %d; Nicanzil land line %d, nonland line %d), after a collect %d; opponent collects while A held an evidence carrier %d",
		games, divs, stalls, tl.explFires, tl.landReveal, tl.nonlandReveal, tl.nicLand, tl.nicNon, tl.evFires, tl.oppEvents)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	if tl.explFires < 50 {
		t.Errorf("only %d explore-trigger firings (need 50)", tl.explFires)
	}
	if tl.landReveal < 10 || tl.nonlandReveal < 10 {
		t.Errorf("land reveals %d, nonland %d (need 10 each)", tl.landReveal, tl.nonlandReveal)
	}
	if tl.nicLand < 5 || tl.nicNon < 5 {
		t.Errorf("Nicanzil land line %d, nonland line %d (need 5 each)", tl.nicLand, tl.nicNon)
	}
	if tl.evFires < 30 {
		t.Errorf("only %d collect-evidence firings (need 30)", tl.evFires)
	}
	if tl.oppEvents < 5 {
		t.Errorf("only %d opponent collects while A held an evidence carrier (need 5)", tl.oppEvents)
	}
}

// TestTrigexplevMutation fires CollectEvidence lines for either player's
// evidence and ignores an Explores line's revealed-card filter: the games
// must diverge.
func TestTrigexplevMutation(t *testing.T) {
	games, divs, _, _ := runTrigexplev(t, 32, true)
	if divs == 0 {
		t.Fatalf("any-player mutation in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}
