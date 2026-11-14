package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The persistent restricted mana lockstep (CR 106.6, 500.4): Karn, Legacy
// Reforged adds {C} per artifact at its controller's upkeep, spendable on
// artifact spells and on no nonartifact spell, and the units are not lost as
// steps and phases end until the turn does. The kernel keeps the restricted
// batch through the draw and main steps (rbatch.pers); the counters below read
// gorge's log: a Karn ManaAdd waits for the spells pushed after the upkeep,
// funds an artifact spell cast in a later step, is refused a nonartifact one,
// and is lost at the turn change.

var kprDecks = [2][]cardset.Entry{
	{
		{Name: "Karn, Legacy Reforged", Count: 4}, {Name: "Ornithopter", Count: 3}, {Name: "Bonesplitter", Count: 3},
		{Name: "Swiftfoot Boots", Count: 3}, {Name: "Short Sword", Count: 2}, {Name: "Sol Ring", Count: 2},
		{Name: "Mind Stone", Count: 2}, {Name: "Hill Giant", Count: 3}, {Name: "Shock", Count: 2}, {Name: "Mountain", Count: 16},
	},
	{
		{Name: "Hill Giant", Count: 8}, {Name: "Shock", Count: 4}, {Name: "Bonesplitter", Count: 3}, {Name: "Ornithopter", Count: 4},
		{Name: "Mind Stone", Count: 1}, {Name: "Short Sword", Count: 3}, {Name: "Mountain", Count: 17},
	},
}

const (
	kprAdded   = iota // Karn's persistent restricted units added at an upkeep
	kprKept           // a spell pushed in a later step than the add while units float
	kprArtPaid        // a Karn unit spent in a later step than the add
	kprRefused        // a nonartifact spell cast while the units floated
	kprEmptied        // units lost at the turn change unspent
	kprN
)

var kprNames = [kprN]string{"units added", "kept to a later step", "kept unit spent", "nonartifact spell cast beside them", "emptied at the turn change"}

// kprKarn: ev is a ManaAdd (amount > 0) or spend (amount < 0) of mana
// restricted by Karn, Legacy Reforged's CantCastNonArtifactSpells.
func kprKarn(g *GorgeSide, ev *events.Event) bool {
	if ev.Amount == 0 || !strings.Contains(ev.Text, "CantCastNonArtifactSpells") {
		return false
	}
	_, src, _, _, ok := events.ManaRestrictionFromText(restrSourceCutPersistent(ev.Text))
	if !ok || src == 0 {
		return false
	}
	ob := g.E.G.Obj(src)
	return ob != nil && ob.Face() != nil && ob.Face().Name == "Karn, Legacy Reforged"
}

// kprScan walks gorge's log for Karn's persistent restricted mana: what is
// added, what floats into a later step, what is spent there, what is lost.
func kprScan(g *GorgeSide, n *[kprN]int) {
	if g == nil {
		return
	}
	left, born, step := 0, state.Step(0), state.Step(0)
	for i := range g.E.L.Events {
		ev := &g.E.L.Events[i]
		switch ev.Kind {
		case events.StepChange:
			step = ev.Step
		case events.ManaAdd:
			if !kprKarn(g, ev) {
				continue
			}
			if ev.Amount > 0 {
				if !strings.HasSuffix(ev.Text, " pm") {
					continue
				}
				n[kprAdded]++
				left, born = left+int(ev.Amount), step
				continue
			}
			if step != born {
				n[kprArtPaid]++
			}
			left = max(left+int(ev.Amount), 0)
		case events.TurnChange:
			if left > 0 {
				n[kprEmptied]++
			}
			left = 0
		case events.PutOnStack:
			if left == 0 || step == born {
				continue
			}
			n[kprKept]++
			ob := g.E.G.Obj(ev.Obj)
			if ob == nil || ob.Face() == nil {
				continue
			}
			isArt := false
			for _, t := range ob.Face().Types {
				isArt = isArt || t == "Artifact"
			}
			if !isArt {
				n[kprRefused]++
			}
		}
	}
}

func TestPersistentRestrictedManaLockstep(t *testing.T) {
	t.Parallel()
	reg, _ := kcastSetup(t)
	set, err := cardset.RawByName("persistrestr")
	if err != nil {
		t.Fatal(err)
	}
	decks := kcastDeck(t, reg, kprDecks)
	tbl, _, _, err := set.Matchup(kprDecks[0], kprDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	var fires [kprN]int
	games, divs := 0, 0
	for seed := uint64(1); seed <= 192; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kpr-a", "kpr-b"}, decks, 1)
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
		kprScan(gs, &fires)
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Errorf("seed %d: kernel stall %s", seed, res.Stall)
		}
		if res.Div != nil {
			divs++
			if divs <= 6 {
				t.Errorf("seed %d: diverged at %d: %s %s ref=%v ker=%v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.RefOptions, res.Div.KerOptions)
			}
		}
	}
	t.Logf("%d games, %d divergent; %v %v", games, divs, kprNames, fires)
	for i, c := range fires {
		if c < 5 {
			t.Errorf("%s: %d times, want at least 5", kprNames[i], c)
		}
	}
}
