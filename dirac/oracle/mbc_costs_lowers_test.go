package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Mystery Booster Commander costs-10 family's lockstep, over the
// mbccosts10 pool. Nils, Discipline Enforcer's end-step trigger reads
// TargetsForEachPlayer$ True, TargetMax$ OneEach over Creature of any
// controller: gorge asks one choice of up to one creature per controller
// (the options grouped by controller). The kernel's slot is the
// different-controllers "up to" (dirac/kgen/lower_targeteachopponent.go,
// livingEach): zero, one, or one creature each side. Battle at the Helvault
// is the same shape over a nonland, non-Saga permanent (chapters I and II
// exile until it leaves). Foil's second discard
// part stays refused by kgen (TestMBCCosts10FoilRefused).

var kmbc10Decks = [2][]cardset.Entry{
	{
		{Name: "Nils, Discipline Enforcer", Count: 5}, {Name: "Battle at the Helvault", Count: 8},
		{Name: "Savannah Lions", Count: 2}, {Name: "Memnite", Count: 2}, {Name: "Ornithopter", Count: 1},
		{Name: "Plains", Count: 22},
	},
	{
		{Name: "Grizzly Bears", Count: 8}, {Name: "Memnite", Count: 4},
		{Name: "Ornithopter", Count: 4}, {Name: "Forest", Count: 24},
	},
}

func TestMBCCosts10Lowers(t *testing.T) {
	t.Parallel()

	// Nils triggers by how many creatures got a counter (and whether they
	// were both players'), and Helvault chapters by the permanents exiled,
	// counted from gorge's event log.
	n := map[string]int{}
	games, stalls := runMyriadPool(t, "mbccosts10", kmbc10Decks, 250, func(g *GorgeSide) {
		if g == nil {
			return
		}
		name := func(id state.ObjID) string {
			if ob := g.E.G.Obj(id); ob != nil && ob.Face() != nil {
				return ob.Face().Name
			}
			return ""
		}
		var inTrig bool
		var trig string
		var owners map[state.PlayerID]bool
		flush := func() {
			if inTrig {
				n[trig+":trigger"]++
				if len(owners) > 0 {
					n[trig+":"+strings.Repeat("+", len(owners))]++
				}
			}
			inTrig, owners = false, nil
		}
		for _, ev := range g.E.L.Events {
			switch ev.Kind {
			case events.TriggerPush, events.DelayedPush:
				flush()
				if nm := name(ev.Obj); nm == "Nils, Discipline Enforcer" || nm == "Battle at the Helvault" {
					inTrig, trig, owners = true, strings.SplitN(nm, ",", 2)[0], map[state.PlayerID]bool{}
				}
			case events.PutOnStack, events.StepChange, events.TurnChange:
				flush()
			case events.CounterChange:
				if inTrig && trig == "Nils" && ev.Counter == "P1P1" && ev.Amount > 0 {
					if o := g.E.G.Obj(ev.Obj); o != nil {
						owners[o.Controller] = true
					}
				}
			case events.MoveZone:
				if inTrig && trig == "Battle at the Helvault" && ev.From == state.ZBattlefield && ev.To == state.ZExile {
					if o := g.E.G.Obj(ev.Obj); o != nil {
						owners[o.Controller] = true
					}
				}
			}
		}
		flush()
	})
	if t.Failed() {
		return
	}
	t.Logf("%d games, %d stalls; Nils triggers %v", games, stalls, n)
	if stalls != 0 {
		t.Fatalf("%d stalls", stalls)
	}
	for _, w := range []struct {
		key string
		min int
	}{{"Nils:+", 10}, {"Nils:++", 10}, {"Battle at the Helvault:+", 2}, {"Battle at the Helvault:++", 2}} {
		if n[w.key] < w.min {
			t.Errorf("%s = %d, want at least %d (one target / one each side)", w.key, n[w.key], w.min)
		}
	}
}
