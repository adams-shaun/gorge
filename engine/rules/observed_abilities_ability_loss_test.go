package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestObservedAbilitiesAfterAbilityLoss: a keyword grant newer than a "loses
// all abilities" effect survives it (CR 613.1f, layer 6 in timestamp order),
// so the inventory lists the granted ability exactly as the offer walk offers
// it, whether or not the permanent also printed the (now dead) line.
func TestObservedAbilitiesAfterAbilityLoss(t *testing.T) {
	t.Parallel()
	const kw = "Saddle:1"
	const types = "Creature Horse Mount"
	cases := []struct {
		name  string
		setup func(t *testing.T, e *Engine) state.ObjID
	}{
		{"printed/lost/granted", func(t *testing.T, e *Engine) state.ObjID {
			return regrantedKeywordPermanent(t, e, types, "Creature", kw, true)
		}},
		{"unprinted/lost/granted", func(t *testing.T, e *Engine) state.ObjID {
			return regrantedKeywordPermanent(t, e, types, "Creature", kw, false)
		}},
		{"unprinted/granted/control", func(t *testing.T, e *Engine) state.ObjID {
			id := onBoard(t, e, 0, "Name:Keyword Bearer\nManaCost:1 G\nTypes:"+types+"\nPT:2/2\nOracle:x\n")
			onBoard(t, e, 0, "Name:Keyword Lord\nManaCost:2\nTypes:Enchantment\n"+
				"S:Mode$ Continuous | Affected$ Creature.YouCtrl | AddKeyword$ "+kw+" | Description$ x\nOracle:x\n")
			if e.printedAbilitiesGone(e.G.Obj(id)) {
				t.Fatalf("precondition: the control permanent lost its abilities")
			}
			return id
		}},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e, _ := tokenReplGame(t, 5101+uint64(i))
			toMain1(t, e)
			id := tc.setup(t, e)
			onBoard(t, e, 0, "Name:Keyword Rider\nManaCost:G\nTypes:Creature Elf\nPT:2/2\nOracle:x\n")
			if lines := e.grantedKeywordLines(id); len(lines) != 1 || lines[0] != kw {
				t.Fatalf("precondition: grantedKeywordLines = %q, want [%s]", lines, kw)
			}
			e.pending = nil
			e.priorityRound()
			d := e.Pending()
			if d == nil || d.Kind != decision.KPriority || d.Player != 0 {
				t.Fatalf("precondition: want seat 0's priority, got %+v", d)
			}
			offered := 0
			for _, op := range d.Options {
				if op.Kind == "ability" && op.Obj == id {
					offered++
				}
			}
			if offered != 1 {
				t.Fatalf("precondition: the offer walk offers %d abilities of the bearer, want 1", offered)
			}
			got := ObservedAbilities(e, 0, id)
			if len(got) != 1 || got[0].Keyword != kw || got[0].Ability != -1 || got[0].Index != 0 {
				t.Fatalf("ObservedAbilities = %+v, want the one granted %s line", got, kw)
			}
		})
	}
}
