package rules

import (
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// weightOfConscienceBoard puts Weight of Conscience on the battlefield
// attached to a Grizzly Bears controlled by seat 1, with the named creatures
// untapped on seat 0's battlefield. The enchanted Bear is deliberately the
// opponent's: the Aura's cost taps "creatures you control", so the only
// candidates are the named seat-0 creatures.
func weightOfConscienceBoard(t *testing.T, creatures ...string) (*Engine, state.ObjID, []state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	extras0 := []*cards.Card{mustCorpusCard(t, reg, "Weight of Conscience")}
	for _, name := range creatures {
		extras0 = append(extras0, mustCorpusCard(t, reg, name))
	}
	e, _ := corpusEngineCfg(t, reg, extras0, []*cards.Card{mustCorpusCard(t, reg, "Grizzly Bears")})
	bear := moveCorpusCard(t, e, "Grizzly Bears", 1, state.ZBattlefield)
	var ids []state.ObjID
	for _, name := range creatures {
		ids = append(ids, moveCorpusCard(t, e, name, 0, state.ZBattlefield))
	}
	if b := e.G.Obj(bear); b == nil || b.Zone != state.ZBattlefield || b.Controller != 1 || b.Tapped {
		t.Fatalf("precondition: enchanted Bear zone=%v controller=%v tapped=%v, want an untapped seat-1 battlefield creature",
			b.Zone, b.Controller, b.Tapped)
	}
	for _, id := range ids {
		o := e.G.Obj(id)
		if o == nil || o.Zone != state.ZBattlefield || o.Controller != 0 || o.Tapped {
			t.Fatalf("precondition: cost creature %d zone=%v controller=%v tapped=%v, want an untapped seat-0 battlefield creature",
				id, o.Zone, o.Controller, o.Tapped)
		}
	}
	aura := lossAttach(t, e, "Weight of Conscience", 0, bear)
	if a := e.G.Obj(aura); a == nil || a.Zone != state.ZBattlefield || a.AttachedTo != bear {
		t.Fatalf("precondition: Aura zone=%v attachedTo=%d, want the battlefield on the Bear %d", a.Zone, a.AttachedTo, bear)
	}
	return e, aura, ids
}

// weightOfConscienceActivation submits the Aura's activated ability off the
// pending priority decision and returns the decision that follows it.
func weightOfConscienceActivation(t *testing.T, e *Engine, aura state.ObjID) *decision.Decision {
	t.Helper()
	opt, ok := findAbilityOption(e, aura, 0)
	if !ok {
		d := e.Pending()
		t.Fatalf("Weight of Conscience's ability is not offered: %+v", d.Options)
	}
	if err := e.Submit(decision.Intent{Seq: e.Pending().Seq, Player: 0, Choices: []int{opt.Index}}); err != nil {
		t.Fatalf("submit the Aura activation: %v", err)
	}
	return e.Pending()
}

// weightOption finds the decision option naming obj.
func weightOption(t *testing.T, d *decision.Decision, obj state.ObjID) decision.Option {
	t.Helper()
	for _, o := range d.Options {
		if o.Obj == obj {
			return o
		}
	}
	t.Fatalf("no option for object %d: %+v", obj, d.Options)
	return decision.Option{}
}

// TestWeightOfConscienceCostPairSharesType pins Forge's CostTapType reading
// of "Tap two untapped creatures you control that share a creature type":
// the two tapped creatures must share a type with EACH OTHER, not with the
// Aura source (which has no creature type at all). The offer gate, the
// election's one legal-answer home (Decision.Validate/SetPropShared), the
// bot repair and the auto-pay arm must all agree.
func TestWeightOfConscienceCostPairSharesType(t *testing.T) {
	t.Parallel()

	t.Run("election enforces the pair", func(t *testing.T) {
		e, aura, ids := weightOfConscienceBoard(t, "Goblin Piker", "Goblin Piker", "Llanowar Elves")
		goblinA, goblinB, elf := ids[0], ids[1], ids[2]
		// Precondition: the Elf shares no creature type with either Goblin,
		// while the Goblins share Goblin. The compared token sets differ.
		gobTypes, elfTypes := slices.Clone(e.Derived(goblinA).Types), slices.Clone(e.Derived(elf).Types)
		if typeSetsShare(gobTypes, elfTypes) {
			t.Fatalf("precondition: Goblin and Elf types %v / %v share a type", gobTypes, elfTypes)
		}
		if !typeSetsShare(slices.Clone(e.Derived(goblinA).Types), slices.Clone(e.Derived(goblinB).Types)) {
			t.Fatalf("precondition: the two Goblins do not share a type")
		}

		d := weightOfConscienceActivation(t, e, aura)
		if d == nil || d.Kind != decision.KChoose || d.Min != 2 || d.Max != 2 {
			t.Fatalf("expected the Min == Max == 2 tap election, got %+v", d)
		}
		if d.SetPropMode != decision.SetPropShared {
			t.Fatalf("tap election SetPropMode = %q, want %q", d.SetPropMode, decision.SetPropShared)
		}
		if len(d.Options) != 3 {
			t.Fatalf("tap election has %d options, want 3: %+v", len(d.Options), d.Options)
		}
		gobOpt, elOpt := weightOption(t, d, goblinA), weightOption(t, d, elf)
		if len(gobOpt.SetProps) == 0 || len(elOpt.SetProps) == 0 {
			t.Fatalf("candidate SetProps missing: Goblin %v, Elf %v", gobOpt.SetProps, elOpt.SetProps)
		}

		wrong := decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{gobOpt.Index, elOpt.Index}}
		if err := d.Validate(wrong); err == nil {
			t.Fatalf("Validate accepted a Goblin+Elf pair, which shares no creature type: %+v", wrong)
		}
		right := decision.Intent{Seq: d.Seq, Player: d.Player,
			Choices: []int{gobOpt.Index, weightOption(t, d, goblinB).Index}}
		if err := d.Validate(right); err != nil {
			t.Fatalf("Validate rejected the two Goblins: %v", err)
		}
		bot := botpolicy.Clamp(d, decision.Intent{Seq: d.Seq, Player: d.Player})
		if err := d.Validate(bot); err != nil {
			t.Fatalf("botpolicy.Clamp returned an answer Validate rejects: %v: %+v", err, bot)
		}
	})

	t.Run("two Goblins auto-pay", func(t *testing.T) {
		e, aura, ids := weightOfConscienceBoard(t, "Goblin Piker", "Goblin Piker")
		goblinA, goblinB := ids[0], ids[1]
		if !typeSetsShare(slices.Clone(e.Derived(goblinA).Types), slices.Clone(e.Derived(goblinB).Types)) {
			t.Fatalf("precondition: the two Goblins do not share a type")
		}
		d := weightOfConscienceActivation(t, e, aura)
		if d != nil && d.Kind == decision.KChoose && d.Prompt == "Choose permanents to tap" {
			t.Fatalf("exactly two payable Goblins must auto-pay, not ask: %+v", d)
		}
		for _, id := range ids {
			if !e.G.Obj(id).Tapped {
				t.Errorf("cost creature %d is untapped after the auto-paid activation", id)
			}
		}
		if len(e.G.Stack) == 0 {
			t.Fatalf("the activated ability did not reach the stack")
		}
	})

	t.Run("Goblin and Elf are not offered", func(t *testing.T) {
		e, aura, ids := weightOfConscienceBoard(t, "Goblin Piker", "Llanowar Elves")
		if typeSetsShare(slices.Clone(e.Derived(ids[0]).Types), slices.Clone(e.Derived(ids[1]).Types)) {
			t.Fatalf("precondition: Goblin and Elf share a type")
		}
		if opt, ok := findAbilityOption(e, aura, 0); ok {
			t.Fatalf("the ability is offered with only a Goblin and an Elf, which share no type: %+v", opt)
		}
		for _, id := range ids {
			if e.G.Obj(id).Tapped {
				t.Fatalf("precondition: cost creature %d is tapped", id)
			}
		}
	})
}

// typeSetsShare reports whether two type lists share at least one word,
// case-insensitively -- the precondition reader for the fixtures above.
func typeSetsShare(a, b []string) bool {
	for _, x := range a {
		if !effects.CreatureTypeWords(x) {
			continue
		}
		for _, y := range b {
			if effects.CreatureTypeWords(y) && strings.EqualFold(x, y) {
				return true
			}
		}
	}
	return false
}
