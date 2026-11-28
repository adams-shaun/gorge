package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/state"
)

// The restricted-equip lockstep: Blackblade Reforged and Brotherhood Regalia
// carry "Equip legendary creature {N}" beside a plain "Equip {M}" (CR
// 702.6a; kgen build_equipshapes.go). Both sides hold a legendary creature
// (Isamaru) and non-legendary ones, so the restricted equip has a target only
// some of the time and the plain one is priced at its own cost.
var equipShapesDecks = [2][]cardset.Entry{
	{{Name: "Blackblade Reforged", Count: 4}, {Name: "Brotherhood Regalia", Count: 4}, {Name: "Isamaru, Hound of Konda", Count: 6}, {Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 4}, {Name: "Mountain", Count: 9}, {Name: "Plains", Count: 9}},
	{{Name: "Brotherhood Regalia", Count: 4}, {Name: "Lightning Bolt", Count: 4}, {Name: "Isamaru, Hound of Konda", Count: 4}, {Name: "Hill Giant", Count: 6}, {Name: "Grizzly Bears", Count: 4}, {Name: "Mountain", Count: 9}, {Name: "Plains", Count: 9}},
}

// equipShapesSeen counts the equip activations gorge put on the stack: the
// restricted (legendary) one and the plain one.
type equipShapesSeen struct{ restricted, plain int }

// watch returns a hit function counting each equip ability once per stack
// object; the game counts as a hit when either was activated.
func (s *equipShapesSeen) watch() func(*GorgeSide) bool {
	var last state.ObjID
	return func(g *GorgeSide) bool {
		st := g.E.G.Stack
		if len(st) == 0 {
			return false
		}
		o := g.E.G.Obj(st[len(st)-1])
		if o == nil || o.ID == last || o.Ability == nil || o.Ability.API != "Attach" {
			return false
		}
		src := g.E.G.Obj(o.Source)
		if src == nil || src.Face() == nil || (src.Face().Name != "Blackblade Reforged" && src.Face().Name != "Brotherhood Regalia") {
			return false
		}
		last = o.ID
		if strings.Contains(o.Ability.Params["ValidTgts"], "Legendary") {
			s.restricted++
		} else {
			s.plain++
		}
		return true
	}
}

// TestEquipShapesLockstep plays 64 seeds: no divergence, no kernel stall, and
// both the restricted and the plain equip were activated across the games.
func TestEquipShapesLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	var s equipShapesSeen
	_, a := sp3Run(t, "equipshapes", equipShapesDecks, pkPolicy, 32, false, nil, s.watch())
	_, b := sp3Run(t, "equipshapes", equipShapesDecks, ExplorePolicy, 32, false, nil, s.watch())
	t.Logf("an equip was activated in %d random and %d explore games: restricted %d times, plain %d", a, b, s.restricted, s.plain)
	if s.restricted == 0 || s.plain == 0 {
		t.Errorf("restricted equip activated %d times, plain %d: want both", s.restricted, s.plain)
	}
}

// TestEquipShapesMutation: with the restricted equip's target filter cleared
// to the plain one's, the kernel offers "Equip legendary creature" on any
// creature and some game diverges, so TestEquipShapesLockstep does not pass
// vacuously.
func TestEquipShapesMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	mutate := func(tb *kop.Table) {
		for i := range tb.Cards {
			c := &tb.Cards[i]
			if c.Name != "Blackblade Reforged" && c.Name != "Brotherhood Regalia" {
				continue
			}
			var eq []*kop.Ability
			for j := range c.Abilities {
				if a := &c.Abilities[j]; a.Kind == kop.AbActivated && a.AFlags&kop.AfEquip != 0 {
					eq = append(eq, a)
				}
			}
			if len(eq) != 2 || eq[0].Targets[0].Filter == eq[1].Targets[0].Filter {
				panic("equipshapes: " + c.Name + " lost its two distinct equip targets")
			}
			eq[0].Targets[0].Filter = eq[1].Targets[0].Filter
		}
	}
	if d, _ := sp3Run(t, "equipshapes", equipShapesDecks, ExplorePolicy, 64, true, mutate, func(*GorgeSide) bool { return false }); d == nil {
		t.Error("no divergence with the restricted equip's filter cleared")
	}
}
