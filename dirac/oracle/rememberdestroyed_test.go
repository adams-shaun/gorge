package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The lockstep: constructed matchups over the generated
// rememberdestroyed pool. Its carriers read the remember params of a single Destroy and
// PutCounter:
//   - Shredder's Technique: Destroy RememberDestroyed$, then LoseLife 2 when
//     the remembered (destroyed) object is an enchantment. gorge remembers
//     only a target the destroy put into a graveyard: an indestructible
//     (Darksteel Myr), regenerated (Drudge Skeletons) or
//     replaced-away (Leyline of the Void exiles the opponent's cards) target
//     is not remembered.
//   - Wishing Well: PutCounter COIN RememberAmount$, then an ImmediateTrigger
//     whose TriggerAmount$ is Count$RememberedNumber (a free cast from the
//     graveyard of an instant/sorcery with MV equal to the coin counters).
//   - Earthbender Ascension: a landfall PutCounter QUEST RememberAmount$,
//     then an ImmediateTrigger gated on four or more quest counters
//     (IsPresent$ equal to ConditionPresent$).
//
// Each branch is counted from gorge's event log: a Shredder resolution's
// segment (its Resolve event up to the next Priority or Resolve), and the
// reflexive trigger's DelayedPush (Counter names its Execute$ SVar).

var krem2Decks = [2][]cardset.Entry{
	// U/B: Shredder's Technique and Wishing Well, one Leyline of the Void (a
	// destroy of the opponent's permanent is replaced by exile; two would
	// pose gorge's replacement-order choice, which the kernel stalls on), a
	// regenerating and an indestructible creature, and cheap instants and
	// sorceries of MV 1-3 for Wishing Well's free cast
	{
		{Name: "Shredder's Technique", Count: 4}, {Name: "Wishing Well", Count: 4},
		{Name: "Leyline of the Void", Count: 1}, {Name: "Drudge Skeletons", Count: 2},
		{Name: "Darksteel Myr", Count: 1}, {Name: "Walking Corpse", Count: 3},
		{Name: "Coral Merfolk", Count: 2}, {Name: "Opt", Count: 3},
		{Name: "Think Twice", Count: 2}, {Name: "Divination", Count: 2},
		{Name: "Sign in Blood", Count: 1}, {Name: "Mind Rot", Count: 1},
		{Name: "Island", Count: 7}, {Name: "Swamp", Count: 7},
	},
	// G/W: Earthbender Ascension and enchantments for Shredder's Technique,
	// creatures, and lands for landfall (River Boa is not here: gorge's
	// Landwalk:Island keyword has no kernel projection bit)
	{
		{Name: "Earthbender Ascension", Count: 4}, {Name: "Pacifism", Count: 3},
		{Name: "Glorious Anthem", Count: 2}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Llanowar Elves", Count: 3}, {Name: "Savannah Lions", Count: 5}, {Name: "Giant Growth", Count: 2},
		{Name: "Evolving Wilds", Count: 3}, {Name: "Forest", Count: 9}, {Name: "Plains", Count: 5},
	},
}

// The branch names krem2Scan counts and the minimum each must reach.
const (
	krem2ShredEnch   = "shredder: enchantment destroyed, life lost"
	krem2ShredCreat  = "shredder: creature destroyed, no life lost"
	krem2ShredSpared = "shredder: target not destroyed"
	krem2ShredExiled = "shredder: destroy replaced by exile"
	krem2Well        = "wishing well: reflexive trigger on the stack"
	krem2Earth       = "earthbender: reflexive trigger at >= 4 quest"
)

var krem2Min = []struct {
	branch string
	min    int
}{
	{krem2ShredEnch, 15}, {krem2ShredCreat, 15}, {krem2Well, 20}, {krem2Earth, 10},
}

func krem2Setup(t *testing.T) (*cards.Registry, cardset.Set, *kop.Table) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("rememberdestroyed")
	if err != nil {
		t.Fatal(err)
	}
	tbl, _, _, err := set.Matchup(krem2Decks[0], krem2Decks[1])
	if err != nil {
		t.Fatal(err)
	}
	return reg, set, tbl
}

// krem2Scan adds one game's branch counts from gorge's event log.
func krem2Scan(g *GorgeSide, n map[string]int) {
	if g == nil {
		return
	}
	gm := g.E.G
	evs := g.E.L.Events
	for i, ev := range evs {
		switch ev.Kind {
		case events.DelayedPush:
			switch {
			case ev.Counter == "TrigPlay" && kremFace(gm, ev.Obj) == "Wishing Well":
				n[krem2Well]++
			case ev.Counter == "TrigPutCounterP1P1" && kremFace(gm, ev.Obj) == "Earthbender Ascension":
				n[krem2Earth]++
			}
			continue
		case events.Resolve:
		default:
			continue
		}
		so := gm.Obj(ev.Obj)
		if so == nil || kremName(gm, ev.Obj) != "Shredder's Technique" {
			continue
		}
		ctrl := so.Controller
		lost, toGY, toEx := false, false, false
		for j := i + 1; j < len(evs) && evs[j].Kind != events.Priority && evs[j].Kind != events.Resolve; j++ {
			e := evs[j]
			switch {
			case e.Kind == events.LifeChange && e.Player == ctrl && e.Amount == -2:
				lost = true
			case e.Kind == events.MoveZone && e.From == state.ZBattlefield && e.To == state.ZGraveyard:
				toGY = true
			case e.Kind == events.MoveZone && e.From == state.ZBattlefield && e.To == state.ZExile:
				toEx = true
			}
		}
		switch {
		case lost:
			n[krem2ShredEnch]++
		case toGY:
			n[krem2ShredCreat]++
		case toEx:
			n[krem2ShredExiled]++
		default:
			n[krem2ShredSpared]++
		}
	}
}

// krem2Run plays seeds 1..seeds through tbl, counting branches into n. It
// returns the games played, whether one diverged (logged when logDiv) and
// the kernel stalls.
func krem2Run(t *testing.T, reg *cards.Registry, set cardset.Set, tbl *kop.Table, seeds uint64, n map[string]int, logDiv bool) (games int, div bool, stalls int) {
	t.Helper()
	decks := kcastDeck(t, reg, krem2Decks)
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"krem2-a", "krem2-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		var gs *GorgeSide
		inner := RandomPolicy(seed)
		pol := PolicyFunc(func(c *PickCtx) Canon {
			if c.G != nil {
				gs = c.G
			}
			return inner.Pick(c)
		})
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 25})
		CloseSide(k)
		games++
		krem2Scan(gs, n)
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			t.Errorf("seed %d: kernel stall %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			div = true
			if !logDiv {
				t.Logf("seed %d: diverged at decision %d: %s %s", seed, d.At, d.What, d.Field)
				return
			}
			lo := max(d.At-12, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			kcastDump(t, gs)
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			return
		}
	}
	return
}

func TestRememberDestroyedAmountLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	reg, set, tbl := krem2Setup(t)
	n := map[string]int{}
	games, div, stalls := krem2Run(t, reg, set, tbl, 256, n, true)
	t.Logf("%d games, diverged %v, kernel stalls %d", games, div, stalls)
	for _, b := range []string{krem2ShredEnch, krem2ShredCreat, krem2ShredSpared, krem2ShredExiled, krem2Well, krem2Earth} {
		t.Logf("branch %-45s %d", b, n[b])
	}
	if div {
		return
	}
	for _, m := range krem2Min {
		if n[m.branch] < m.min {
			t.Errorf("branch %q: %d, want at least %d", m.branch, n[m.branch], m.min)
		}
	}
}

// TestRememberDestroyedAmountMutation: with dirac.DebugRem2Always (RememberDestroyed$
// remembers a target that was not destroyed; RememberAmount$ counts
// CounterNum$, not the landed count) the lockstep must diverge within 128
// seeds: a Shredder's Technique whose enchantment target Leyline of the
// Void exiles loses the caster 2 life in the kernel only.
func TestRememberDestroyedAmountMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	reg, set, tbl := krem2Setup(t)
	dirac.DebugRem2Always = true
	defer func() { dirac.DebugRem2Always = false }()
	n := map[string]int{}
	_, div, _ := krem2Run(t, reg, set, tbl, 128, n, false)
	if !div {
		t.Fatalf("DebugRem2Always: no divergence in 128 seeds")
	}
}
