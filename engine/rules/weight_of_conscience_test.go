package rules

// Weight of Conscience's cost -- "Tap two untapped creatures you control that
// share a creature type: Exile enchanted creature" -- is a SET constraint on
// the tapped pair (CR 602.1): the two tapped creatures must share a type with
// EACH OTHER, not with the Aura. Forge writes the cost as
// tapXType<2/Creature.sharesCreatureTypeWith/...>; read as a per-object filter
// against the source, the bare term binds the referent "Self" (the Aura),
// which has no creature types, so no candidate ever matches and the ability is
// withheld forever. The fix moves the relation onto decision.SetPropShared
// over each candidate's derived creature-type tokens -- one definition for
// offer, Decision.Validate and botpolicy.Clamp.
//
// No Forge script text is committed here; the carrier is the compiled .cards
// corpus card Weight of Conscience, found through testkit/testutil.CorpusRegistry
// (paidCostEngine's searchTestRegistry).

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// weightTapOptions maps the pending tap ask's object ids to their option
// indexes, failing unless every option is a tapcost option.
func weightTapOptions(t *testing.T, d *decision.Decision) map[state.ObjID]int {
	t.Helper()
	byObj := make(map[state.ObjID]int, len(d.Options))
	for _, o := range d.Options {
		if o.Kind != "tapcost" {
			t.Fatalf("tap ask option kind = %q, want tapcost: %+v", o.Kind, d.Options)
		}
		byObj[o.Obj] = o.Index
	}
	return byObj
}

// weightAbilityOption reports whether the pending priority decision offers the
// Aura's activated ability and returns its option index.
func weightAbilityOption(t *testing.T, e *Engine, aura state.ObjID) (int, bool) {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("not at priority: %+v", d)
	}
	for _, o := range d.Options {
		if o.Obj == aura && (o.Kind == "activate" || o.Kind == "ability") && o.Ability == 0 {
			return o.Index, true
		}
	}
	return -1, false
}

// weightBoard builds the real card attached to an opposing creature. The
// creature fixtures ride the seat-0 deck; each test moves the ones it needs, so
// a test that must NOT offer the ability never accidentally drafts a second
// sharing creature onto the battlefield.
func weightBoard(t *testing.T) (e *Engine, cfg Config, aura, bearer state.ObjID) {
	t.Helper()
	e, cfg = paidCostEngine(t,
		[]string{"Weight of Conscience", "Llanowar Elves", "Llanowar Elves", "Goblin Piker", "Universal Automaton"},
		[]string{"Grizzly Bears"})
	bearer = paidCostMoveTo(t, e, 1, "Grizzly Bears", state.ZBattlefield)
	aura = lossAttach(t, e, "Weight of Conscience", 0, bearer)
	toMain1(t, e)
	e.priorityRound()
	if o := e.G.Obj(aura); o == nil || o.Zone != state.ZBattlefield || o.AttachedTo != bearer {
		t.Fatalf("precondition: Weight of Conscience = %+v, want an Aura on the battlefield attached to %d", o, bearer)
	}
	if o := e.G.Obj(bearer); o == nil || o.Zone != state.ZBattlefield || o.Controller != 1 || o.Tapped {
		t.Fatalf("precondition: enchanted creature = %+v, want an untapped battlefield permanent controlled by seat 1", o)
	}
	return e, cfg, aura, bearer
}

// weightMovedToBoard is paidCostMoveTo for a duplicate-name fixture: it moves
// one more copy after avoiding the ids already moved.
func weightMovedToBoard(t *testing.T, e *Engine, name string, avoid ...state.ObjID) state.ObjID {
	t.Helper()
	id := paidCostMoveTo(t, e, 0, name, state.ZBattlefield, avoid...)
	if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield || o.Tapped {
		t.Fatalf("precondition: %s = %+v, want an untapped battlefield permanent", name, o)
	}
	return id
}

// TestWeightOfConscienceTapCostSharesTypeAmongTapped is the end-to-end pin:
// with two Elves and a Goblin the ability is offered, the ask is a
// SetPropShared 2..2 Choose whose SetProps are the candidates' creature types,
// Decision.Validate and botpolicy.Clamp read that one rule, and paying taps
// exactly the shared pair and exiles the enchanted creature. With only an Elf
// and a Goblin it is not offered. A changeling shares with a typed creature.
func TestWeightOfConscienceTapCostSharesTypeAmongTapped(t *testing.T) {
	t.Parallel()

	t.Run("shared pair is offered and pays", func(t *testing.T) {
		t.Parallel()
		e, cfg, aura, bearer := weightBoard(t)
		elf1 := paidCostMoveTo(t, e, 0, "Llanowar Elves", state.ZBattlefield)
		elf2 := weightMovedToBoard(t, e, "Llanowar Elves", elf1)
		goblin := paidCostMoveTo(t, e, 0, "Goblin Piker", state.ZBattlefield)
		if o := e.G.Obj(elf1); o == nil || o.Zone != state.ZBattlefield || o.Tapped {
			t.Fatalf("precondition: first Elf = %+v, want an untapped battlefield permanent", o)
		}
		if o := e.G.Obj(goblin); o == nil || o.Zone != state.ZBattlefield || o.Tapped {
			t.Fatalf("precondition: Goblin = %+v, want an untapped battlefield permanent", o)
		}
		if !slices.Contains(e.Derived(elf1).Types, "Elf") || !slices.Contains(e.Derived(elf2).Types, "Elf") {
			t.Fatalf("precondition: Elf types = %v / %v, want Elf", e.Derived(elf1).Types, e.Derived(elf2).Types)
		}
		if !slices.Contains(e.Derived(goblin).Types, "Goblin") || slices.Contains(e.Derived(goblin).Types, "Elf") {
			t.Fatalf("precondition: Goblin types = %v, want Goblin and not Elf", e.Derived(goblin).Types)
		}
		if slices.Contains(e.Derived(elf1).Types, "Goblin") {
			t.Fatalf("precondition: Elf types = %v, must not share the Goblin's type", e.Derived(elf1).Types)
		}

		idx, offered := weightAbilityOption(t, e, aura)
		if !offered {
			t.Fatal("the ability was not offered although two untapped Elves share a creature type")
		}
		submitChoices(t, e, idx)

		d := e.Pending()
		if d == nil || d.Kind != decision.KChoose || d.Min != 2 || d.Max != 2 {
			t.Fatalf("no 2..2 tap ask pending: %+v", d)
		}
		if d.SetPropMode != decision.SetPropShared {
			t.Fatalf("tap ask SetPropMode = %q, want shared", d.SetPropMode)
		}
		byObj := weightTapOptions(t, d)
		if len(d.Options) != 3 {
			t.Fatalf("tap ask offers %d options, want the three untapped creatures", len(d.Options))
		}
		e1, e2, g := byObj[elf1], byObj[elf2], byObj[goblin]
		if e1 < 0 || e2 < 0 || g < 0 {
			t.Fatalf("tap ask does not offer all three creatures: %+v", d.Options)
		}
		for _, o := range d.Options {
			if len(o.SetProps) == 0 {
				t.Fatalf("tap option %d carries no creature-type SetProps: %+v", o.Index, o)
			}
		}

		if err := d.Validate(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{e1, e2}}); err != nil {
			t.Fatalf("Validate rejected the shared Elf pair {Elf,Elf}: %v", err)
		}
		if err := d.Validate(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{e1, g}}); err == nil {
			t.Fatal("Validate accepted the non-sharing pair {Elf,Goblin}")
		}

		// The bot's repair reads the same SetPropShared rule the validator
		// does: its answer must pass Validate (the one-home rule).
		clamped := botpolicy.Clamp(d, decision.Intent{Seq: d.Seq, Player: d.Player})
		if err := d.Validate(clamped); err != nil {
			t.Fatalf("botpolicy.Clamp answer %v fails Validate: %v", clamped.Choices, err)
		}

		submitChoices(t, e, e1, e2)
		passUntilStackEmpty(t, e, 30)

		if o := e.G.Obj(bearer); o == nil || o.Zone != state.ZExile {
			t.Fatalf("enchanted creature = %+v, want exiled", o)
		}
		if o := e.G.Obj(elf1); o == nil || !o.Tapped {
			t.Fatalf("first Elf = %+v, want tapped (it paid)", o)
		}
		if o := e.G.Obj(elf2); o == nil || !o.Tapped {
			t.Fatalf("second Elf = %+v, want tapped (it paid)", o)
		}
		if o := e.G.Obj(goblin); o == nil || o.Tapped {
			t.Fatalf("Goblin = %+v, want untapped (it did not pay)", o)
		}
		replayCheck(t, e, cfg)
	})

	t.Run("no shared pair is not offered", func(t *testing.T) {
		t.Parallel()
		e, _, aura, _ := weightBoard(t)
		elf := paidCostMoveTo(t, e, 0, "Llanowar Elves", state.ZBattlefield)
		goblin := paidCostMoveTo(t, e, 0, "Goblin Piker", state.ZBattlefield)
		if o := e.G.Obj(elf); o == nil || o.Zone != state.ZBattlefield || o.Tapped {
			t.Fatalf("precondition: Elf = %+v, want an untapped battlefield permanent", o)
		}
		if o := e.G.Obj(goblin); o == nil || o.Zone != state.ZBattlefield || o.Tapped {
			t.Fatalf("precondition: Goblin = %+v, want an untapped battlefield permanent", o)
		}
		if !slices.Contains(e.Derived(elf).Types, "Elf") || !slices.Contains(e.Derived(goblin).Types, "Goblin") {
			t.Fatalf("precondition: types = %v / %v, want Elf / Goblin", e.Derived(elf).Types, e.Derived(goblin).Types)
		}
		if slices.Contains(e.Derived(elf).Types, "Goblin") || slices.Contains(e.Derived(goblin).Types, "Elf") {
			t.Fatalf("precondition: Elf/Goblin must not share a type: %v / %v", e.Derived(elf).Types, e.Derived(goblin).Types)
		}
		if _, offered := weightAbilityOption(t, e, aura); offered {
			t.Fatal("the ability was offered although no two untapped creatures share a creature type")
		}
	})

	t.Run("changeling shares with a typed creature", func(t *testing.T) {
		t.Parallel()
		e, cfg, aura, bearer := weightBoard(t)
		elf := paidCostMoveTo(t, e, 0, "Llanowar Elves", state.ZBattlefield)
		auto := paidCostMoveTo(t, e, 0, "Universal Automaton", state.ZBattlefield)
		// A Goblin keeps the ask posed (two candidates would auto-settle), so
		// the changeling relation is exercised at the Validate/SetProps stage.
		goblin := paidCostMoveTo(t, e, 0, "Goblin Piker", state.ZBattlefield)
		for _, id := range []state.ObjID{elf, auto, goblin} {
			if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield || o.Tapped {
				t.Fatalf("precondition: creature %d = %+v, want an untapped battlefield permanent", id, o)
			}
		}
		if !slices.Contains(e.Derived(elf).Types, "Elf") {
			t.Fatalf("precondition: Elf types = %v, want Elf", e.Derived(elf).Types)
		}
		if !e.Derived(auto).AllCreatureTypes {
			t.Fatalf("precondition: Universal Automaton types = %v, all-creature-types = %v, want a changeling",
				e.Derived(auto).Types, e.Derived(auto).AllCreatureTypes)
		}

		idx, offered := weightAbilityOption(t, e, aura)
		if !offered {
			t.Fatal("the ability was not offered although the changeling shares the Elf's type")
		}
		submitChoices(t, e, idx)
		d := e.Pending()
		if d == nil || d.Kind != decision.KChoose || d.SetPropMode != decision.SetPropShared {
			t.Fatalf("no SetPropShared tap ask pending: %+v", d)
		}
		byObj := weightTapOptions(t, d)
		if len(d.Options) != 3 {
			t.Fatalf("tap ask offers %d options, want the Elf, the changeling and the Goblin", len(d.Options))
		}
		ei, ai, gi := byObj[elf], byObj[auto], byObj[goblin]
		if ei < 0 || ai < 0 || gi < 0 {
			t.Fatalf("tap ask does not offer the Elf, the changeling and the Goblin: %+v", d.Options)
		}
		if err := d.Validate(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{ei, gi}}); err == nil {
			t.Fatal("Validate accepted the non-sharing pair {Elf,Goblin} in the changeling board")
		}
		if err := d.Validate(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{ei, ai}}); err != nil {
			t.Fatalf("Validate rejected the {Elf,Changeling} pair: %v", err)
		}
		submitChoices(t, e, ei, ai)
		passUntilStackEmpty(t, e, 30)
		if o := e.G.Obj(bearer); o == nil || o.Zone != state.ZExile {
			t.Fatalf("enchanted creature = %+v, want exiled", o)
		}
		if o := e.G.Obj(elf); o == nil || !o.Tapped {
			t.Fatalf("Elf = %+v, want tapped", o)
		}
		if o := e.G.Obj(auto); o == nil || !o.Tapped {
			t.Fatalf("changeling = %+v, want tapped", o)
		}
		replayCheck(t, e, cfg)
	})
}
