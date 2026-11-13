package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The cumulative upkeep and phasing lockstep. A permanent phased out in
// response to its cumulative upkeep trigger (CR 702.24a) is not affected by it
// (CR 702.26b): no age counter, no pay-or-sacrifice ask, no sacrifice (gorge's
// cumulative.go reads the source through pay.ExistsOnBattlefield; dirac's
// cumulativeStart skips a phased-out source). Vodalian Illusionist and
// Reality Ripple phase out creatures, artifacts and lands, so the table's
// subject is Revered Unicorn, the cheapest plain cumulative upkeep creature;
// Mystic Remora (an enchantment) is covered by gorge's rules test
// TestCumulativeUpkeepPhasedOutSourceAsksNothing and cannot be phased out
// by either card, and Clever Concealment, the only one that can, is convoke
// and so unpriced by gorge's planner ("unsupported shape:contribution").

var kremoraPhaseDeck = []cardset.Entry{
	{Name: "Revered Unicorn", Count: 6}, {Name: "Vodalian Illusionist", Count: 4}, {Name: "Reality Ripple", Count: 4},
	{Name: "Island", Count: 13}, {Name: "Plains", Count: 13},
}

type remoraPhaseStats struct {
	games, divs, stalls int
	phasedOut           int // observations of a phased-out Revered Unicorn
}

func remoraPhaseCheck(ref *GorgeSide, st *remoraPhaseStats) {
	g := ref.E.G
	for pl := range g.Players {
		for _, id := range g.Zone(state.ZBattlefield, state.PlayerID(pl)) {
			if o := g.Obj(id); o != nil && o.PhasedOut && o.Face() != nil && o.Face().Name == "Revered Unicorn" {
				st.phasedOut++
			}
		}
	}
}

// remoraPhasePolicy pays every cumulative upkeep (a Unicorn that stays gives
// the table time to hold the mana), phases out the Unicorn (Reality Ripple's
// cast, Vodalian Illusionist's activation, the Unicorn as the target) only
// in response to a cumulative upkeep trigger on the stack, and otherwise
// casts, activates and plays lands first without spending either.
func remoraPhasePolicy(seed uint64) Policy {
	base := castFirstPolicy(seed)
	pay, _ := chooseCode("trigger_cost_pay")
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G == nil {
			return base.Pick(c)
		}
		switch c.D.Kind {
		case DChoose:
			for _, o := range c.D.Options {
				if o.Kind() == ActChoose+CanonKind(pay) {
					return o
				}
			}
		case DTarget:
			for _, o := range c.D.Options {
				if gorgeOptionName(c, o) == "Revered Unicorn" {
					return o
				}
			}
		case DPriority:
			g := c.G.E.G
			response := false
			if n := len(g.Stack); n > 0 {
				if top := g.Obj(g.Stack[n-1]); top != nil && top.Ability != nil && top.Ability.API == "CumulativeUpkeep" {
					response = true
				}
			}
			buf = buf[:0]
			for _, o := range c.D.Options {
				if nm := gorgeOptionName(c, o); nm == "Reality Ripple" || nm == "Vodalian Illusionist" && o.Kind() == ActActivate {
					if response {
						return o
					}
					continue
				}
				buf = append(buf, o)
			}
			d := *c.D
			d.Options = buf
			return base.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		}
		return base.Pick(c)
	})
}

func runRemoraPhasing(t *testing.T, seeds int) (st remoraPhaseStats) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("remoraphasing")
	if err != nil {
		t.Fatal(err)
	}
	var deck []*cards.Card
	for _, e := range kremoraPhaseDeck {
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
	tbl, _, _, err := set.Matchup(kremoraPhaseDeck, kremoraPhaseDeck)
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kremoraphase-a", "kremoraphase-b"}, [2][]*cards.Card{deck, deck}, 1)
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
		res := LockstepLed(m, seed, g, k, remoraPhasePolicy(seed), RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 24,
			Check: func(ref *GorgeSide, _ Side) *Divergence {
				remoraPhaseCheck(ref, &st)
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

func TestRemoraPhasingLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	st := runRemoraPhasing(t, 64)
	t.Logf("%+v", st)
	if st.divs != 0 || st.stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", st.divs, st.stalls)
	}
	if st.phasedOut == 0 {
		t.Fatalf("the games never phased out a cumulative upkeep creature in response: %+v", st)
	}
}
