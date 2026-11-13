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

// The lockstep: RememberMoved$ on Surveil (Enlightened
// Confidant, Chandra, Chill of Compliance), RememberKept$ on Surveil (Starving
// Revenant), RememberDrawn$ on Draw (Trade
// Route Envoy) and RememberSacrificed$ on SacrificeAll (Bringer of the Last
// Gift). Deck A is a black-green ramp deck (Bringer, Envoy, mana creatures
// and rocks, counter sources), deck B a white-blue lifegain deck with
// Chandra, whose mana +1 floats a noncreature-only {U} (CR 106.6, 605.1a;
// mana_resolverestriction.go).

var kremw3Decks = [2][]cardset.Entry{
	{
		{Name: "Bringer of the Last Gift", Count: 4}, {Name: "Trade Route Envoy", Count: 4},
		{Name: "Llanowar Elves", Count: 3}, {Name: "Elvish Mystic", Count: 3}, {Name: "Mind Stone", Count: 2},
		{Name: "Sol Ring", Count: 1}, {Name: "Knight of Wundagore", Count: 4},
		{Name: "Starving Revenant", Count: 3}, {Name: "Giant Growth", Count: 2},
		{Name: "Forest", Count: 9}, {Name: "Swamp", Count: 5},
	},
	{
		{Name: "Enlightened Confidant", Count: 4},
		{Name: "Healer's Hawk", Count: 4}, {Name: "Infirmary Healer", Count: 4}, {Name: "Suntail Hawk", Count: 2},
		{Name: "Messenger Hawk", Count: 2}, {Name: "Cult Healer", Count: 3}, {Name: "Unsummon", Count: 2},
		{Name: "Counterspell", Count: 2}, {Name: "Chandra, Chill of Compliance", Count: 2},
		{Name: "Plains", Count: 8}, {Name: "Island", Count: 7},
	},
}

// kremw3Counts: what one game's gorge event log shows of the three writers.
type kremw3Counts struct {
	games                int
	surveilRem, returned int
	keptRem              int // Starving Revenant's surveils that kept a card on top
	envoyETB, envoyCtr   int
	bringerSweep, sacked int
	chandraRestr         int // Chandra's restricted {U} units added
}

// kremw3Scan reads gorge's event log: a "remembered" Choose from a
// Confidant (a surveil that remembered a card), a remembered card then moved
// graveyard -> hand (the return), each Trade Route Envoy trigger and the
// +1/+1 counters put on an Envoy (the no-draw branch; no other card here
// counters one), each Bringer trigger (a cast Bringer's sweep) and the
// permanents sacrificed other than the rocks' own.
func kremw3Scan(g *GorgeSide, c *kremw3Counts) {
	if g == nil {
		return
	}
	name := func(id state.ObjID) string {
		if o := g.E.G.Obj(id); o != nil && o.Face() != nil {
			return o.Face().Name
		}
		return ""
	}
	rem := map[state.ObjID]bool{}
	for _, ev := range g.E.L.Events {
		switch ev.Kind {
		case events.Choose:
			if ev.Counter == "remembered" && name(ev.Obj) == "Starving Revenant" {
				c.keptRem++
			}
			if ev.Counter == "remembered" && name(ev.Obj) == "Enlightened Confidant" {
				c.surveilRem++
				for _, id := range ev.IDs {
					rem[id] = true
				}
			}
		case events.MoveZone:
			if ev.From == state.ZGraveyard && ev.To == state.ZHand && rem[ev.Obj] {
				c.returned++
				delete(rem, ev.Obj)
			}
			if events.IsSacrifice(ev) && name(ev.Obj) != "Mind Stone" && name(ev.Obj) != "Sol Ring" {
				c.sacked++
			}
		case events.TriggerPush:
			switch name(ev.Obj) {
			case "Trade Route Envoy":
				c.envoyETB++
			case "Bringer of the Last Gift":
				c.bringerSweep++
			}
		case events.CounterChange:
			if ev.Counter == "P1P1" && name(ev.Obj) == "Trade Route Envoy" {
				c.envoyCtr++
			}
		case events.ManaAdd:
			if ev.Amount <= 0 {
				continue
			}
			if _, src, _, _, ok := events.ManaRestrictionFromText(restrSourceCutPersistent(ev.Text)); ok && name(src) == "Chandra, Chill of Compliance" {
				c.chandraRestr += int(ev.Amount)
			}
		}
	}
}

func kremw3Run(t *testing.T, seeds uint64, mutant bool) (c kremw3Counts, diverged bool, stalls map[string]int) {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("rememberwrites")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kremw3Decks {
		for _, e := range d {
			cd, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				decks[s] = append(decks[s], cd)
			}
		}
		if len(decks[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	tbl, _, _, err := set.Matchup(kremw3Decks[0], kremw3Decks[1])
	if err != nil {
		t.Fatal(err)
	}
	stalls = map[string]int{}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%4 == 0}
		m, err := NewMatchup(reg, [2]string{"kremw3-a", "kremw3-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		var gs *GorgeSide
		inner := castFirstPolicy(seed)
		if seed%8 == 0 {
			inner = RandomPolicy(seed)
		}
		pol := PolicyFunc(func(pc *PickCtx) Canon {
			if pc.G != nil {
				gs = pc.G
			}
			return inner.Pick(pc)
		})
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 14})
		CloseSide(k)
		c.games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls[res.Stall]++
		}
		kremw3Scan(gs, &c)
		if d := res.Div; d != nil {
			diverged = true
			if mutant {
				t.Logf("mutant: seed %d diverged at decision %d: %s %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.RefOptions, d.KerOptions)
				return
			}
			lo := max(d.At-12, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			return
		}
	}
	return
}

func TestRememberWritesLockstep(t *testing.T) {
	t.Parallel()
	c, _, stalls := kremw3Run(t, 400, false)
	t.Logf("%d games: %d surveils remembered a card, %d returned to hand, Starving Revenant kept a card %d times; Trade Route Envoy %d ETBs (%d drew, %d counter); Bringer %d sweeps sacrificing %d permanents; Chandra %d restricted {U}; kernel stalls %v",
		c.games, c.surveilRem, c.returned, c.keptRem, c.envoyETB, c.envoyETB-c.envoyCtr, c.envoyCtr, c.bringerSweep, c.sacked, c.chandraRestr, stalls)
	if c.surveilRem < 10 {
		t.Errorf("%d surveils remembered a card, want at least 10", c.surveilRem)
	}
	if c.keptRem < 5 {
		t.Errorf("%d Starving Revenant surveils kept a card, want at least 5", c.keptRem)
	}
	if c.returned < 5 {
		t.Errorf("%d remembered cards returned, want at least 5", c.returned)
	}
	if c.envoyETB < 10 || c.envoyETB-c.envoyCtr < 3 || c.envoyCtr < 3 {
		t.Errorf("Trade Route Envoy: %d ETBs, %d counters, want 10 ETBs, 3 on each branch", c.envoyETB, c.envoyCtr)
	}
	if c.bringerSweep < 3 || c.sacked < 3 {
		t.Errorf("%d Bringer sweeps sacrificing %d permanents, want at least 3 and 3", c.bringerSweep, c.sacked)
	}
	if c.chandraRestr < 1 {
		t.Errorf("Chandra added %d restricted {U}, want at least 1", c.chandraRestr)
	}
	if len(stalls) != 0 {
		t.Errorf("kernel capacity stalls %v", stalls)
	}
}

// TestRememberWritesMutation: with the three remember writes skipped the
// lockstep must diverge (gorge returns the surveilled card, skips the
// Envoy's counter after a draw, keeps the sacrificed creatures out of the
// Bringer's return).
func TestRememberWritesMutation(t *testing.T) {
	dirac.DebugNoRemWrites3 = true
	defer func() { dirac.DebugNoRemWrites3 = false }()
	if _, diverged, _ := kremw3Run(t, 128, true); !diverged {
		t.Fatal("no divergence in 128 seeds with the three remember writes skipped")
	}
}

// TestRememberWritesRestrictedManaMutation: with Chandra's restricted {U} pooled as
// plain mana the lockstep must diverge (the kernel offers a creature cast
// the restricted mana cannot pay; gorge withholds it).
func TestRememberWritesRestrictedManaMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugPlainLoyaltyMana = true
	defer func() { dirac.DebugPlainLoyaltyMana = false }()
	if _, diverged, _ := kremw3Run(t, 128, true); !diverged {
		t.Fatal("no divergence in 128 seeds with Chandra's restricted mana pooled plain")
	}
}
