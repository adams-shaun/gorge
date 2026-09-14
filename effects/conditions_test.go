package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// conditionBoard builds a 2-seat game with an instant, a sorcery and a land
// on it, and returns the host plus their ids (bolt, ritual, mountain, then
// the source creature).
func conditionBoard(t *testing.T) (*fakeHost, []state.ObjID) {
	t.Helper()
	h := newHost(t, 2)
	bolt := mkCard(t, "Name:Bolt\nTypes:Instant\nOracle:3 damage\n")
	ritual := mkCard(t, "Name:Ritual\nTypes:Sorcery\nOracle:add RRR\n")
	mtn := mkCard(t, "Name:Mountain\nTypes:Land\nOracle:x\n")
	fix := mkCard(t, "Name:Fixture\nTypes:Creature\nPT:1/1\nOracle:x\n")
	var ids []state.ObjID
	ids = append(ids, h.g.AddObject(bolt, 0).ID)
	ids = append(ids, h.g.AddObject(ritual, 0).ID)
	ids = append(ids, h.g.AddObject(mtn, 0).ID)
	ids = append(ids, h.g.AddObject(fix, 0).ID)
	return h, ids
}

// TestConditionGateDelverShape pins the exact shape Delver of Secrets
// transforms on: ConditionDefined$ Remembered + ConditionPresent$
// Card.Instant,Card.Sorcery + ConditionCompare$ EQ1 is met exactly when the
// remembered set holds exactly one instant/sorcery (task
// fb-20260914T033246Z-3f1cc033, defect 2).
func TestConditionGateDelverShape(t *testing.T) {
	h, ids := conditionBoard(t)
	sa := sa(t, "DB$ SetState | Defined$ Self | Mode$ Transform | ConditionDefined$ Remembered | ConditionPresent$ Card.Instant,Card.Sorcery | ConditionCompare$ EQ1")
	ctx := &Ctx{Controller: 0, Source: ids[3], Remembered: []state.Target{{Obj: ids[0]}}}
	if met, resolved := conditionMet(h, ctx, sa); !met || !resolved {
		t.Fatalf("one remembered instant: met=%v resolved=%v, want true true", met, resolved)
	}
	ctx.Remembered = []state.Target{{Obj: ids[2]}} // a land
	if met, resolved := conditionMet(h, ctx, sa); met || !resolved {
		t.Fatalf("one remembered land: met=%v resolved=%v, want false true", met, resolved)
	}
	ctx.Remembered = []state.Target{{Obj: ids[0]}, {Obj: ids[1]}} // two spells
	if met, resolved := conditionMet(h, ctx, sa); met || !resolved {
		t.Fatalf("two remembered spells vs EQ1: met=%v resolved=%v, want false true", met, resolved)
	}
	ctx.Remembered = nil
	if met, resolved := conditionMet(h, ctx, sa); met || !resolved {
		t.Fatalf("nothing remembered: met=%v resolved=%v, want false true", met, resolved)
	}
	// A player entry in Remembered never matches a Card spec.
	ctx.Remembered = []state.Target{{Player: 1, IsPlayer: true}}
	if met, resolved := conditionMet(h, ctx, sa); met || !resolved {
		t.Fatalf("only a player remembered: met=%v resolved=%v, want false true", met, resolved)
	}
}

// TestConditionGateCompareOperators pins the literal operator set and the
// no-Compare default (presence: count >= 1).
func TestConditionGateCompareOperators(t *testing.T) {
	h, ids := conditionBoard(t)
	base := "DB$ Pump | ConditionDefined$ Remembered | ConditionPresent$ Card | ConditionCompare$ "
	ctx := &Ctx{Controller: 0, Source: ids[3], Remembered: []state.Target{{Obj: ids[0]}, {Obj: ids[1]}}}
	for _, tc := range []struct {
		cmp  string
		want bool
	}{
		{"EQ2", true}, {"EQ1", false}, {"NE2", false}, {"NE1", true},
		{"GE2", true}, {"GE3", false}, {"GT1", true}, {"GT2", false},
		{"LT3", true}, {"LT2", false}, {"LE2", true}, {"LE1", false},
	} {
		gate := sa(t, base+tc.cmp)
		if met, resolved := conditionMet(h, ctx, gate); !resolved || met != tc.want {
			t.Errorf("Compare %s over 2 remembered: met=%v resolved=%v, want met=%v", tc.cmp, met, resolved, tc.want)
		}
	}
	// Forge spells the compare without a space; the spaced spelling parses too.
	gate := sa(t, "DB$ Pump | ConditionDefined$ Remembered | ConditionPresent$ Card | ConditionCompare$ EQ 2")
	if met, resolved := conditionMet(h, ctx, gate); !met || !resolved {
		t.Fatalf("spaced EQ 2: met=%v resolved=%v, want true true", met, resolved)
	}
	// No Compare key: presence (>= 1 of the Present spec).
	gate = sa(t, "DB$ Pump | ConditionDefined$ Remembered | ConditionPresent$ Card.Instant")
	if met, resolved := conditionMet(h, ctx, gate); !met || !resolved {
		t.Fatalf("Present with no Compare: met=%v resolved=%v, want true true", met, resolved)
	}
	// Neither Present nor Compare: the defined group is non-empty.
	gate = sa(t, "DB$ Pump | ConditionDefined$ Remembered")
	if met, resolved := conditionMet(h, ctx, gate); !met || !resolved {
		t.Fatalf("Defined only: met=%v resolved=%v, want true true", met, resolved)
	}
}

// TestConditionGateUnresolvedShapesRunUnconditionally pins the documented
// boundary: a gate this build cannot evaluate leaves the sub ungated —
// conditionMet reports resolved=false and Resolve runs the sub, exactly the
// pre-gate behaviour (see conditions.go's scope comment).
func TestConditionGateUnresolvedShapesRunUnconditionally(t *testing.T) {
	h, _ := conditionBoard(t)
	other := "DB$ Pump | ConditionDefined$ Targeted | ConditionPresent$ Creature | ConditionCompare$ EQ1"
	if _, resolved := conditionMet(h, &Ctx{Controller: 0}, sa(t, other)); resolved {
		t.Fatal("ConditionDefined$ Targeted resolved — out of the scoped shape")
	}
	checkSVar := "DB$ Pump | ConditionDefined$ Remembered | ConditionCheckSVar$ X | ConditionSVarCompare$ EQ1"
	if _, resolved := conditionMet(h, &Ctx{Controller: 0}, sa(t, checkSVar)); resolved {
		t.Fatal("ConditionCheckSVar$ resolved — out of the scoped shape")
	}
	notPresent := "DB$ Pump | ConditionDefined$ Remembered | ConditionNotPresent$ Card"
	if _, resolved := conditionMet(h, &Ctx{Controller: 0}, sa(t, notPresent)); resolved {
		t.Fatal("ConditionNotPresent$ resolved — out of the scoped shape")
	}
	unknownPred := "DB$ Pump | ConditionDefined$ Remembered | ConditionPresent$ Card.ExiledWithSource"
	if _, resolved := conditionMet(h, &Ctx{Controller: 0}, sa(t, unknownPred)); resolved {
		t.Fatal("an unknown predicate in Present resolved — would count a false zero")
	}
	nonLiteral := "DB$ Pump | ConditionDefined$ Remembered | ConditionPresent$ Card | ConditionCompare$ GTX"
	if _, resolved := conditionMet(h, &Ctx{Controller: 0}, sa(t, nonLiteral)); resolved {
		t.Fatal("a non-literal Compare resolved — out of the scoped shape")
	}
	// No Condition* key at all: not gated, run.
	if met, resolved := conditionMet(h, &Ctx{Controller: 0}, sa(t, "DB$ Pump")); resolved || !met {
		t.Fatalf("ungated sub: met=%v resolved=%v, want true false", met, resolved)
	}
}

// TestConditionGateBattlefieldDefault pins the no-ConditionDefined group:
// ConditionPresent$ without a Defined group counts over the BATTLEFIELD and
// the spec's own controller qualifier scopes it — Dominaria's Judgment's
// five-condition chain (Land.YouCtrl per colour), which used to grant every
// protection unconditionally and now grants each only when that land is
// actually in play under the caster's control.
func TestConditionGateBattlefieldDefault(t *testing.T) {
	h, ids := conditionBoard(t)
	plains := mkCard(t, "Name:Plains\nTypes:Land Plains Basic\nOracle:x\n")
	pid := h.g.AddObject(plains, 0).ID
	h.g.Obj(pid).Zone = state.ZBattlefield
	h.g.SetZone(state.ZBattlefield, 0, append(h.g.Zone(state.ZBattlefield, 0), pid))
	sa := sa(t, "DB$ PumpAll | ValidCards$ Creature.YouCtrl | KW$ Protection from white | ConditionPresent$ Plains.YouCtrl | ConditionCompare$ GE1")
	if met, resolved := conditionMet(h, &Ctx{Controller: 0, Source: ids[3]}, sa); !met || !resolved {
		t.Fatalf("a Plains in play: met=%v resolved=%v, want true true", met, resolved)
	}
	// Seat 1's Plains does not satisfy seat 0's Plains.YouCtrl.
	p1 := h.g.AddObject(plains, 1).ID
	h.g.Obj(p1).Zone = state.ZBattlefield
	h.g.SetZone(state.ZBattlefield, 1, append(h.g.Zone(state.ZBattlefield, 1), p1))
	h.g.SetZone(state.ZBattlefield, 0, h.g.Zone(state.ZBattlefield, 0)[:0])
	if met, resolved := conditionMet(h, &Ctx{Controller: 0, Source: ids[3]}, sa); met || !resolved {
		t.Fatalf("only the opponent's Plains: met=%v resolved=%v, want false true", met, resolved)
	}
}

// TestResolveSkipsAnUnmetConditionGate pins the chain-walk: a gated sub
// whose condition is not met is SKIPPED and the chain continues past it —
// Delver's DBTransform must not flip when the gate fails — while a met gate
// runs the sub.
func TestResolveSkipsAnUnmetConditionGate(t *testing.T) {
	h, ids := conditionBoard(t)
	// A two-faced source so SetState has something to flip.
	src := mkCard(t, "Name:Front\nTypes:Creature\nPT:1/1\nOracle:x\n\nALTERNATE\n\nName:Back\nTypes:Creature\nPT:3/3\nOracle:x\n")
	srcID := h.g.AddObject(src, 0).ID
	chain := "DB$ SetState | Defined$ Self | Mode$ Transform | ConditionDefined$ Remembered | ConditionPresent$ Card.Instant,Card.Sorcery | ConditionCompare$ EQ1 | SubAbility$ DBCleanup\nSVar:DBCleanup:DB$ Cleanup | ClearRemembered$ True"
	sa := sa(t, chain)

	// Land remembered: no FlipFace, chain continues (the Cleanup note runs).
	ctx := &Ctx{Controller: 0, Source: srcID, Remembered: []state.Target{{Obj: ids[2]}}}
	Resolve(h, ctx, sa)
	flips := 0
	for _, e := range h.log {
		if e.Kind == events.FlipFace {
			flips++
		}
	}
	if flips != 0 {
		t.Fatalf("the unmet gate still flipped: %d FlipFace events", flips)
	}

	// Instant remembered: exactly one flip.
	h2, ids2 := conditionBoard(t)
	srcID2 := h2.g.AddObject(src, 0).ID
	ctx2 := &Ctx{Controller: 0, Source: srcID2, Remembered: []state.Target{{Obj: ids2[0]}}}
	Resolve(h2, ctx2, sa)
	flips = 0
	for _, e := range h2.log {
		if e.Kind == events.FlipFace {
			flips++
		}
	}
	if flips != 1 {
		t.Fatalf("the met gate did not flip exactly once: %d FlipFace events", flips)
	}
}
