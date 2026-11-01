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

// The lockstep (gorge pay/payment.go:606-607, CR 106.6):
// Guidelight Optimizer and Purple Dragon Punks add mana restricted by
// `RestrictValid$ Spell.Artifact,Activated`: an artifact spell or ANY
// activated ability, nothing else. The mechanism is counted from gorge's log
// (a restricted ManaAdd whose batch the same payment spends), classified by
// what the payment pushed.

var kmrDecks = [][2][]cardset.Entry{{
	{
		{Name: "Guidelight Optimizer", Count: 4}, {Name: "Ornithopter", Count: 3}, {Name: "Mind Stone", Count: 3},
		{Name: "Wayfarer's Bauble", Count: 3}, {Name: "Guardian Idol", Count: 3}, {Name: "Walking Ballista", Count: 3},
		{Name: "Wind Drake", Count: 3}, {Name: "Divination", Count: 3}, {Name: "Prodigal Sorcerer", Count: 3},
		{Name: "Island", Count: 12},
	},
	{
		{Name: "Purple Dragon Punks", Count: 4}, {Name: "Furnace Whelp", Count: 4}, {Name: "Goblin Chieftain", Count: 3},
		{Name: "Mogg Fanatic", Count: 3}, {Name: "Lightning Bolt", Count: 2}, {Name: "Shock", Count: 2},
		{Name: "Ornithopter", Count: 2}, {Name: "Mind Stone", Count: 3}, {Name: "Walking Ballista", Count: 3},
		{Name: "Guardian Idol", Count: 3}, {Name: "Wayfarer's Bauble", Count: 2}, {Name: "Mountain", Count: 9},
	},
}, {
	{
		{Name: "Guidelight Optimizer", Count: 3}, {Name: "Purple Dragon Punks", Count: 3}, {Name: "Ornithopter", Count: 3},
		{Name: "Mind Stone", Count: 2}, {Name: "Wayfarer's Bauble", Count: 2}, {Name: "Walking Ballista", Count: 3},
		{Name: "Pestilent Cauldron", Count: 2}, {Name: "Furnace Whelp", Count: 3}, {Name: "Prodigal Sorcerer", Count: 3},
		{Name: "Lightning Bolt", Count: 2}, {Name: "Pacifism", Count: 2}, {Name: "Divination", Count: 2},
		{Name: "Island", Count: 6}, {Name: "Mountain", Count: 4},
	},
	{
		{Name: "Guidelight Optimizer", Count: 4}, {Name: "Purple Dragon Punks", Count: 4}, {Name: "Guardian Idol", Count: 3},
		{Name: "Goblin Chieftain", Count: 2}, {Name: "Mogg Fanatic", Count: 3}, {Name: "Shock", Count: 3},
		{Name: "Wind Drake", Count: 3}, {Name: "Grizzly Bears", Count: 2}, {Name: "Mind Stone", Count: 2},
		{Name: "Island", Count: 7}, {Name: "Mountain", Count: 7},
	},
}}

const (
	kmrAct       = iota // restricted mana funds an activated ability
	kmrArtCast          // ...an artifact spell
	kmrOtherCast        // ...a non-artifact spell (must stay 0)
	kmrN
)

var kmrNames = [kmrN]string{"funds an activated ability", "funds an artifact spell", "funds a non-artifact spell"}

// kmrScan walks gorge's log like restrSourceScan: a restricted positive ManaAdd
// names its source (the " @id" suffix); the mana funds the push made since the
// last decision, or the next one when floated before any push.
func kmrScan(g *GorgeSide, n *[kmrN]int) {
	if g == nil {
		return
	}
	var pending []state.ObjID
	var cur *events.Event
	credit := func(push *events.Event, src state.ObjID) {
		name := ""
		if ob := g.E.G.Obj(src); ob != nil && ob.Face() != nil {
			name = ob.Face().Name
		}
		if name != "Guidelight Optimizer" && name != "Purple Dragon Punks" {
			return
		}
		spell := push.Kind == events.PutOnStack
		art := false
		if ob := g.E.G.Obj(push.Obj); ob != nil && ob.Face() != nil {
			for _, t := range ob.Face().Types {
				art = art || t == "Artifact"
			}
		}
		switch {
		case !spell:
			n[kmrAct]++
		case art:
			n[kmrArtCast]++
		default:
			n[kmrOtherCast]++
		}
	}
	for i := range g.E.L.Events {
		ev := &g.E.L.Events[i]
		switch ev.Kind {
		case events.DecisionMade:
			cur = nil
		case events.ManaAdd:
			if ev.Amount <= 0 {
				continue
			}
			if _, src, _, _, ok := events.ManaRestrictionFromText(restrSourceCutPersistent(ev.Text)); ok && src != 0 {
				if cur != nil {
					credit(cur, src)
				} else {
					pending = append(pending, src)
				}
			}
		case events.ManaClear, events.TurnChange:
			pending = pending[:0]
		case events.PutOnStack, events.AbilityPush:
			cur = ev
			for _, src := range pending {
				credit(ev, src)
			}
			pending = pending[:0]
		}
	}
}

func kmrSetup(t *testing.T) (*cards.Registry, cardset.Set) {
	t.Helper()
	reg, _ := kcastSetup(t)
	set, err := cardset.RawByName("manarestrict")
	if err != nil {
		t.Fatal(err)
	}
	return reg, set
}

func TestManarestrictLockstep(t *testing.T) {
	t.Parallel()
	reg, set := kmrSetup(t)
	var fires [kmrN]int
	games, divs := 0, 0
	for mi, md := range kmrDecks {
		decks := kcastDeck(t, reg, md)
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= 256; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"kmr-a", "kmr-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			var gs *GorgeSide
			inner := eagerPolicy(seed, map[string]int{})
			pol := PolicyFunc(func(c *PickCtx) Canon {
				if c.G != nil {
					gs = c.G
				}
				return inner.Pick(c)
			})
			res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
			CloseSide(k)
			games++
			kmrScan(gs, &fires)
			if strings.HasPrefix(res.Stall, KernelStallPrefix) {
				t.Errorf("matchup %d seed %d: kernel stall %s", mi, seed, res.Stall)
			}
			if res.Div != nil {
				divs++
				if divs <= 6 {
					t.Errorf("matchup %d seed %d: diverged at %d: %s %s %v", mi, seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
				}
			}
		}
	}
	t.Logf("%d games, %d divergent; restricted spends %v %v", games, divs, kmrNames, fires)
	if fires[kmrAct] < 30 {
		t.Errorf("%s: %d times, want at least 30", kmrNames[kmrAct], fires[kmrAct])
	}
	if fires[kmrArtCast] < 20 {
		t.Errorf("%s: %d times, want at least 20", kmrNames[kmrArtCast], fires[kmrArtCast])
	}
	if fires[kmrOtherCast] != 0 {
		t.Errorf("%s: %d times, want 0", kmrNames[kmrOtherCast], fires[kmrOtherCast])
	}
}

// TestManarestrictMutation: with dirac.DebugManaRestrictActNone the
// any-card activated restriction admits nothing, which must diverge from gorge.
func TestManarestrictMutation(t *testing.T) {
	reg, set := kmrSetup(t)
	old := dirac.DebugManaRestrictActNone
	dirac.DebugManaRestrictActNone = true
	defer func() { dirac.DebugManaRestrictActNone = old }()
	for mi, md := range kmrDecks {
		decks := kcastDeck(t, reg, md)
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= 64; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"kmr-a", "kmr-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			res := Lockstep(m, seed, k, eagerPolicy(seed, map[string]int{}), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
			CloseSide(k)
			if res.Div != nil {
				t.Logf("mutation diverged: matchup %d seed %d at %d: %s %s", mi, seed, res.Div.At, res.Div.What, res.Div.Field)
				return
			}
		}
	}
	t.Fatal("mutation did not diverge")
}
