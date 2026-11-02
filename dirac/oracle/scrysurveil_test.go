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

// The scry/surveil trigger lockstep: the six Standard
// "whenever you scry or surveil" carriers (Mode$ Scry / Mode$ Surveil,
// ValidPlayer$ You; two of them ActivationLimit$ 1 per line) in a U/W shell
// of scry and surveil spells, enter triggers and lands. Both decks hold the
// carriers, so each side's scries and surveils reach the other side's
// ValidPlayer$ You lines, which must stay silent. Basics only: gorge's
// resolution-time pay ask (rules respay.go resPayNeedsMana) prices a
// trigger's "you may pay {2}" (Proft) against PotentialMana's over-bound,
// which counts both colours of a dual land, so a lone untapped Temple of
// Enlightenment is offered a pay the kernel (correctly) withholds -- a gorge
// finding, kept out of this pool.

var kscrysurveilShell = []cardset.Entry{
	{Name: "Matoya, Archon Elder", Count: 2}, {Name: "Denzilore Fatehold", Count: 2}, {Name: "Diviner of Victory", Count: 2},
	{Name: "Proft, Consulting Detective", Count: 2}, {Name: "Prudent Fateseer", Count: 2}, {Name: "Saheeli, Consul of Oversight", Count: 2},
	{Name: "Opt", Count: 3}, {Name: "Consider", Count: 2}, {Name: "Faerie Seer", Count: 2}, {Name: "Augury Owl", Count: 2},
	{Name: "Dimir Informant", Count: 2},
}

var kscrysurveilDecks = [2][]cardset.Entry{
	append(append([]cardset.Entry(nil), kscrysurveilShell...),
		cardset.Entry{Name: "Dazzling Lights", Count: 2},
		cardset.Entry{Name: "Island", Count: 10}, cardset.Entry{Name: "Plains", Count: 5}),
	append(append([]cardset.Entry(nil), kscrysurveilShell...),
		cardset.Entry{Name: "Omen of the Sea", Count: 2},
		cardset.Entry{Name: "Island", Count: 10}, cardset.Entry{Name: "Plains", Count: 5}),
}

var kscrysurveilCarriers = map[string]bool{"Matoya, Archon Elder": true, "Denzilore Fatehold": true,
	"Diviner of Victory": true, "Proft, Consulting Detective": true, "Prudent Fateseer": true,
	"Saheeli, Consul of Oversight": true}

// kscrysurveilTally counts, from gorge's event log, the carrier triggers
// pushed after a scry (scryFires) or a surveil (survFires) marker, and the
// seat-1 scries and surveils while seat 0 controlled a carrier (oppEvents).
type kscrysurveilTally struct {
	scryFires, survFires, oppEvents int
}

// kscrysurveilPolicy plays randomly and scans the gorge log as it goes.
func kscrysurveilPolicy(seed uint64, tl *kscrysurveilTally) Policy {
	rnd := RandomPolicy(seed)
	at := 0
	var last events.Kind
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
				case events.Scry, events.Surveil:
					last = e.Kind
					if e.Player == 1 && kscrysurveilHolds(g, 0) {
						tl.oppEvents++
					}
				case events.TriggerPush:
					if o := g.Obj(e.Obj); o != nil && o.Face() != nil && kscrysurveilCarriers[o.Face().Name] {
						if last == events.Scry {
							tl.scryFires++
						} else if last == events.Surveil {
							tl.survFires++
						}
					}
				}
			}
			at = len(evs)
		}
		return rnd.Pick(c)
	})
}

// kscrysurveilHolds reports whether seat p controls a carrier on the
// battlefield.
func kscrysurveilHolds(g *state.Game, p state.PlayerID) bool {
	for _, id := range g.Zone(state.ZBattlefield, p) {
		if o := g.Obj(id); o != nil && o.Controller == p && o.Face() != nil && kscrysurveilCarriers[o.Face().Name] {
			return true
		}
	}
	return false
}

func runScrysurveil(t *testing.T, seeds int, mutate bool, tune func(*Config)) (games, divs, stalls int, tl kscrysurveilTally) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("scrysurveil")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugScrySurveilAnyPlayer
	dirac.DebugScrySurveilAnyPlayer = mutate
	defer func() { dirac.DebugScrySurveilAnyPlayer = old }()
	var decks [2][]*cards.Card
	for s, d := range kscrysurveilDecks {
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
	tbl, _, _, err := set.Matchup(kscrysurveilDecks[0], kscrysurveilDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		if tune != nil {
			tune(&cfg)
		}
		m, err := NewMatchup(reg, [2]string{"kscrysurveil-a", "kscrysurveil-b"}, decks, 1)
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
		res := LockstepLed(m, seed, g, k, kscrysurveilPolicy(seed, &tl), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
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

func TestScrysurveilLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, stalls, tl := runScrysurveil(t, 256, false, nil)
	t.Logf("%d games, %d divergences, %d stalls, carrier triggers after a scry %d, after a surveil %d, opponent scries/surveils while A held a carrier %d",
		games, divs, stalls, tl.scryFires, tl.survFires, tl.oppEvents)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	if tl.scryFires < 100 {
		t.Errorf("only %d scry-trigger firings (need 100)", tl.scryFires)
	}
	if tl.survFires < 100 {
		t.Errorf("only %d surveil-trigger firings (need 100)", tl.survFires)
	}
	if tl.oppEvents < 20 {
		t.Errorf("only %d opponent scries/surveils while A held a carrier (need 20)", tl.oppEvents)
	}
}

// TestScrysurveilArrangeRestOrder runs the same lockstep with the pile-B
// phase on for both sides: every scry/surveil that leaves two or more cards
// in pile B orders them, and the shadow's Intent.Rest must match the kernel's
// Rest order at every decision and in the projection (CR 404.3/701.22a).
func TestScrysurveilArrangeRestOrder(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, stalls, tl := runScrysurveil(t, 32, false, func(c *Config) { c.ArrangeRestOrder = true })
	t.Logf("%d games, %d divergences, %d stalls, carrier triggers after a scry %d, after a surveil %d",
		games, divs, stalls, tl.scryFires, tl.survFires)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	if tl.scryFires+tl.survFires == 0 {
		t.Fatal("no scry or surveil fired: the pile-B phase was never exercised")
	}
}

// TestScrysurveilMutation fires the lines for either player's scry or
// surveil: a ValidPlayer$ You line then answers the opponent's too, so the
// games must diverge.
func TestScrysurveilMutation(t *testing.T) {
	games, divs, _, _ := runScrysurveil(t, 32, true, nil)
	if divs == 0 {
		t.Fatalf("any-player mutation in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}
