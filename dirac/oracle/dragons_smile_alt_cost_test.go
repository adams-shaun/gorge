package oracle

import (
	"fmt"
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestDragonsSmileAltCostLowers plays Dragon's Smile's non-granting static
// ("Once during each of your turns, you may pay {0} rather than pay the mana
// cost for a spell you cast from exile": MayPlayAltManaCost$ 0 |
// MayPlayDontGrantZonePermissions$ True | AffectedZone$ Exile, a FreeOnly
// static on Exile in the kernel, dirac/cast_mayzeroalt.go) in lockstep with
// gorge over the dragonsmile fixture pool. Its combat-damage trigger exiles
// the damaged player's top cards with an Effect letting its controller play
// them this turn; gorge lists each such nonland cast at AltCostIndex 0 (the
// printed cost) and, while the static's turn is unspent on its controller's
// turn, the "mayplay" option at AltCostIndex 1 paying {0} (rules mayplay.go
// mayPlayCastAlts). The policy crews and attacks with the Vehicle and takes
// both casts; the preconditions are that it connected, that both the plain
// and the {0} cast were taken, and that a second exiled card was offered
// with the alternative spent (the once-per-turn key). The mutation drops the
// static from the table and must diverge.
func TestDragonsSmileAltCostLowers(t *testing.T) {
	if isolate(t) {
		return
	}
	t.Run("lockstep", func(t *testing.T) {
		run, st := runDragonsSmile(t, 24, true, false)
		t.Logf("%d games, %d divergences, %d stalls; %+v", run.games, run.divs, run.stalls, st)
		if st.connected < 2 {
			t.Errorf("Dragon's Smile's trigger went on the stack %d times, want at least 2", st.connected)
		}
		if st.altCasts < 2 {
			t.Errorf("an exiled card was cast at the {0} alternative %d times, want at least 2", st.altCasts)
		}
		if st.plainCasts < 2 {
			t.Errorf("an exiled card was cast at its printed cost %d times, want at least 2", st.plainCasts)
		}
		if st.spentOffers < 1 {
			t.Errorf("an exiled card was offered with the turn's alternative spent %d times, want at least 1", st.spentOffers)
		}
	})
	t.Run("mutation-ZeroAlt", func(t *testing.T) {
		run, _ := runDragonsSmile(t, 24, false, true)
		run.requireDivergence(t, "Dragon's Smile without its {0} alternative")
	})
}

var dragonsSmileDeck = []cardset.Entry{
	{Name: "Dragon's Smile", Count: 6}, {Name: "Hill Giant", Count: 8}, {Name: "Raging Goblin", Count: 6},
	{Name: "Shock", Count: 4}, {Name: "Mountain", Count: 16},
}

// dragonsSmileStats are the preconditions measured from gorge's side.
type dragonsSmileStats struct {
	connected   int // Dragon's Smile's combat-damage trigger pushed
	altCasts    int // exiled cards cast at AltCostIndex 1 (the {0} alternative)
	plainCasts  int // exiled cards cast at AltCostIndex 0 beside it
	spentOffers int // priority decisions offering a plain cast from exile with no alternative after one was cast this turn
}

// dragonsSmilePolicy plays lands, casts Dragon's Smile, crews it before
// combat and attacks with it; an exiled card's cast is taken whenever one is
// offered, the {0} alternative and the printed cost by turns. Else random.
func dragonsSmilePolicy(seed uint64, st *dragonsSmileStats, altTurn *int32) Policy {
	rnd := RandomPolicy(seed)
	prng := rand.New(rand.NewPCG(seed, 0x736d696c)) // "smil"
	mayplay, _ := castModeCode("mayplay")
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G == nil {
			return rnd.Pick(c)
		}
		g := c.G.E.G
		var plain, alt []Canon
		for _, a := range c.D.Options {
			if a.Kind() == ActCast && a.Mode()&0xF == mayplay {
				if o, ok := c.G.GorgeOption(a); ok && g.Obj(o.Obj) != nil && g.Obj(o.Obj).Zone == state.ZExile {
					if a.Mode()>>4 == 1 {
						alt = append(alt, a)
					} else {
						plain = append(plain, a)
					}
				}
			}
		}
		if c.D.Kind == DPriority && len(plain) > 0 && len(alt) == 0 && *altTurn == g.Turn {
			st.spentOffers++
		}
		switch {
		case len(alt) > 0 && (len(plain) == 0 || prng.IntN(2) == 0):
			st.altCasts++
			*altTurn = g.Turn
			return alt[prng.IntN(len(alt))]
		case len(plain) > 0:
			st.plainCasts++
			return plain[prng.IntN(len(plain))]
		}
		for _, a := range c.D.Options {
			n := refCardName(c.G, a.Src())
			switch {
			case a.Kind() == ActPlayLand:
				return a
			case a.Kind() == ActCast && n == "Dragon's Smile" && len(g.Stack) == 0:
				return a
			case a.Kind() == ActActivate && n == "Dragon's Smile" && g.Step == state.StepMain1 && len(g.Stack) == 0:
				return a // Crew 3
			case a.Kind() == ActAttack && n == "Dragon's Smile":
				return a
			}
		}
		return rnd.Pick(c)
	})
}

// dragonsSmileObserve counts Dragon's Smile's trigger pushes since *seen.
func dragonsSmileObserve(ref *GorgeSide, seen *int, st *dragonsSmileStats) {
	g := ref.E.G
	evs := ref.E.L.Events
	for ; *seen < len(evs); *seen++ {
		if e := evs[*seen]; e.Kind == events.TriggerPush {
			if o := g.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == "Dragon's Smile" {
				st.connected++
			}
		}
	}
}

// runDragonsSmile plays seeds 1..seeds of the dragonsmile mirror, gorge
// leading, alternating the option surface by seed. mutate turns the static
// off in the table (StatNone): the alternative is never offered.
func runDragonsSmile(t *testing.T, seeds int, report, mutate bool) (fixtureRun, dragonsSmileStats) {
	t.Helper()
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("dragonsmile")
	if err != nil {
		t.Fatal(err)
	}
	var deck []*cards.Card
	for _, e := range dragonsSmileDeck {
		c, ok := reg.Lookup(e.Name)
		if !ok {
			t.Fatalf("%s not in the corpus", e.Name)
		}
		for range e.Count {
			deck = append(deck, c)
		}
	}
	if len(deck) != 40 {
		t.Fatalf("deck: %d cards", len(deck))
	}
	tbl, _, _, err := set.Matchup(dragonsSmileDeck, dragonsSmileDeck)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for i := range tbl.Cards {
		cd := &tbl.Cards[i]
		if cd.Name != "Dragon's Smile" {
			continue
		}
		cd.Abilities = append([]kop.Ability(nil), cd.Abilities...) // a copy: the set's table is shared
		for j := range cd.Abilities {
			if a := &cd.Abilities[j]; a.Kind == kop.AbStatic && a.Static.Kind == kop.StatMayPlayFrom &&
				a.Static.StFlags&kop.StFreeOnly != 0 && a.Static.Zone == kop.ZoneExile {
				found = true
				if mutate {
					a.Static.Kind = kop.StatNone
				}
			}
		}
	}
	if !found {
		t.Fatal("Dragon's Smile has no FreeOnly MayPlayFrom static on Exile in the table")
	}
	run := fixtureRun{casts: map[string]int{}}
	var st dragonsSmileStats
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"smile-a", "smile-b"}, [2][]*cards.Card{deck, deck}, 1)
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
		seen, altTurn := 0, int32(-1)
		check := func(ref *GorgeSide, _ Side) *Divergence {
			dragonsSmileObserve(ref, &seen, &st)
			return nil
		}
		res := LockstepLed(m, seed, g, k, dragonsSmilePolicy(seed, &st, &altTurn), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 30, Check: check})
		CloseSide(k)
		run.games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			run.stalls++
			if report {
				t.Errorf("seed %d: kernel capacity stall %q", seed, res.Stall)
			}
		}
		if d := res.Div; d != nil {
			run.divs++
			msg := fmt.Sprintf("seed %d decision %d: %s", seed, d.At, strings.TrimSpace(d.What+" "+d.Field+" "+d.Detail))
			if run.first == "" {
				run.first = msg
			}
			if report {
				t.Errorf("seed %d: diverged at decision %d: %s %v\nref %v\nker %v", seed, d.At, msg, d.Diff, d.RefOptions, d.KerOptions)
			}
			if !report || run.divs >= 3 {
				break
			}
		}
	}
	return run, st
}
