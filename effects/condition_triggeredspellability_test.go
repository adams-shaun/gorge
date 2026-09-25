package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// corpusCardSA returns the card and the named SVar on its front face: the
// real compiled Forge script, never a re-spelled fixture, so the tests pin
// the corpus shape Shiko and Narset, Unified and Orvar, the All-Form carry.
func corpusCardSA(t *testing.T, name, svar string) (*cards.Card, *cards.SA) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	c, ok := reg.Lookup(name)
	if !ok {
		t.Fatalf("corpus has no %q", name)
	}
	sa := cards.ResolveSVar(c.Faces[0].SVars, svar)
	if sa == nil {
		t.Fatalf("%s has no SVar %q", name, svar)
	}
	return c, sa
}

// stackSpell places a fresh sorcery on the stack, controlled by ctlr, and
// returns its id. Targets are set separately by the caller.
func stackSpell(t *testing.T, h *fakeHost, ctlr state.PlayerID) state.ObjID {
	t.Helper()
	o := h.g.AddObject(mkCard(t, "Name:Probe\nManaCost:R\nTypes:Sorcery\nOracle:x\n"), ctlr)
	events.Move(h.g, o.ID, state.ZLibrary, state.ZStack)
	return o.ID
}

// TestConditionDefinedTriggeredSpellAbilityTargetFilter pins the group's
// present-filter semantics against Shiko and Narset, Unified's REAL compiled
// TrigCopy SA: `ConditionDefined$ TriggeredSpellAbility | ConditionPresent$
// Spell.IsTargeting Valid Permanent,Spell.IsTargeting Player` is met only
// when the triggering spell actually targets a permanent or a player. A
// non-targeting spell is a RESOLVED non-match (never the fail-open an absent
// binding gets), and an absent trigger referent stays fail-open.
func TestConditionDefinedTriggeredSpellAbilityTargetFilter(t *testing.T) {
	_, sa := corpusCardSA(t, "Shiko and Narset, Unified", "TrigCopy")
	// PRECONDITION: this is the real gate shape the fix targets; a bare
	// `Card` present-filter would pass for the wrong reason.
	if sa.API != "CopySpellAbility" || sa.Params["ConditionDefined"] != "TriggeredSpellAbility" {
		t.Fatalf("Shiko TrigCopy = %+v, want CopySpellAbility with ConditionDefined$ TriggeredSpellAbility", sa.Params)
	}
	if !isSpellTargetingPresent(sa.Params["ConditionPresent"]) {
		t.Fatalf("Shiko ConditionPresent = %q, want the Spell.IsTargeting form", sa.Params["ConditionPresent"])
	}

	h := newHost(t, 2)
	bear := h.g.AddObject(mkCard(t, "Name:Bear\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"), 0)
	events.Move(h.g, bear.ID, state.ZLibrary, state.ZBattlefield)
	spell := stackSpell(t, h, 0)

	// PRECONDITION: the spell is on the stack and the Bear on the
	// battlefield -- the zones the copy and the target filter read.
	if h.g.Obj(spell).Zone != state.ZStack || h.g.Obj(bear.ID).Zone != state.ZBattlefield {
		t.Fatalf("precondition zones: spell=%s bear=%s", h.g.Obj(spell).Zone, h.g.Obj(bear.ID).Zone)
	}

	ctx := &Ctx{Controller: 0, Remembered: []state.Target{{Obj: spell}}}
	// A spell targeting the Bear satisfies the first alternative.
	h.g.Obj(spell).Targets = []state.Target{{Obj: bear.ID}}
	if met, resolved := conditionMet(h, ctx, sa); !met || !resolved {
		t.Fatalf("spell targeting a permanent: met=%v resolved=%v, want true true", met, resolved)
	}
	// A spell targeting a player satisfies the second alternative.
	h.g.Obj(spell).Targets = []state.Target{{Player: 1, IsPlayer: true}}
	if met, resolved := conditionMet(h, ctx, sa); !met || !resolved {
		t.Fatalf("spell targeting a player: met=%v resolved=%v, want true true", met, resolved)
	}
	// PRECONDITION: the target list is genuinely empty before the
	// non-targeting assertion, so a stale target cannot make it pass.
	h.g.Obj(spell).Targets = nil
	if len(h.g.Obj(spell).Targets) != 0 {
		t.Fatal("precondition: probe spell still carries targets")
	}
	if met, resolved := conditionMet(h, ctx, sa); met || !resolved {
		t.Fatalf("non-targeting spell: met=%v resolved=%v, want false true (a resolved non-match)", met, resolved)
	}
	// An absent binding is fail-open: no referent, so the gate cannot answer.
	if _, resolved := conditionMet(h, &Ctx{Controller: 0}, sa); resolved {
		t.Fatal("absent trigger referent resolved -- must stay fail-open")
	}
}

// TestConditionDefinedTriggeredSpellAbilityAbilityReferent pins the
// AbilityCast arm of the referent selection: the group is Ctx.TriggerAbility
// (the minted stack wrapper) when the trigger fired on an activated ability,
// exactly the binding Defined$ TriggeredSpellAbility reads, so a target
// filter over that wrapper is enforced too.
func TestConditionDefinedTriggeredSpellAbilityAbilityReferent(t *testing.T) {
	_, sa := corpusCardSA(t, "Shiko and Narset, Unified", "TrigCopy")
	h := newHost(t, 2)
	bear := h.g.AddObject(mkCard(t, "Name:Bear\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"), 0)
	events.Move(h.g, bear.ID, state.ZLibrary, state.ZBattlefield)
	ability := stackSpell(t, h, 1)
	if h.g.Obj(ability).Zone != state.ZStack {
		t.Fatalf("precondition: ability wrapper zone = %s, want stack", h.g.Obj(ability).Zone)
	}
	ctx := &Ctx{Controller: 0, TriggerContext: TriggerContext{TriggerAbility: ability}}
	h.g.Obj(ability).Targets = []state.Target{{Obj: bear.ID}}
	if met, resolved := conditionMet(h, ctx, sa); !met || !resolved {
		t.Fatalf("trigger ability targeting a permanent: met=%v resolved=%v, want true true", met, resolved)
	}
	h.g.Obj(ability).Targets = nil
	if met, resolved := conditionMet(h, ctx, sa); met || !resolved {
		t.Fatalf("non-targeting trigger ability: met=%v resolved=%v, want false true", met, resolved)
	}
}

// libraryCard places a fresh card in p's library ZONE INDEX: AddObject only
// appends to Game.Objs, and the draw primitive reads the index, so a card
// that is only AddObject'ed would not actually be drawable.
func libraryCard(t *testing.T, h *fakeHost, p state.PlayerID) {
	t.Helper()
	o := h.g.AddObject(mkCard(t, "Name:Fodder\nTypes:Land\nOracle:x\n"), p)
	h.g.SetZone(state.ZLibrary, p, append(h.g.Zone(state.ZLibrary, p), o.ID))
}

// TestConditionDefinedTriggeredSpellAbilityNoCopyContinues drives the REAL
// Shiko copy chain end to end: a non-targeting triggering spell makes no
// copy, and the "If you don't copy a spell this way, draw a card" rider's
// Remembered EQ0 gate then draws. A targeting spell copies and draws nothing.
func TestConditionDefinedTriggeredSpellAbilityNoCopyContinues(t *testing.T) {
	card, sa := corpusCardSA(t, "Shiko and Narset, Unified", "TrigCopy")
	if sa.Sub == nil || sa.Sub.API != "Draw" {
		t.Fatalf("precondition: Shiko TrigCopy sub = %+v, want the Draw rider", sa.Sub)
	}
	if sa.Params["RememberCopies"] != "True" {
		t.Fatalf("precondition: Shiko TrigCopy lacks RememberCopies$ True: %+v", sa.Params)
	}

	h := newHost(t, 2)
	src := h.g.AddObject(card, 0)
	events.Move(h.g, src.ID, state.ZLibrary, state.ZBattlefield)
	bear := h.g.AddObject(mkCard(t, "Name:Bear\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"), 1)
	events.Move(h.g, bear.ID, state.ZLibrary, state.ZBattlefield)
	// A library card for the rider's draw, and the spell the trigger
	// remembered.
	libraryCard(t, h, 0)
	spell := stackSpell(t, h, 1)

	// PRECONDITION: the non-targeting spell really is on the stack with an
	// empty target list, and Shiko is on the battlefield as the source.
	if h.g.Obj(spell).Zone != state.ZStack || len(h.g.Obj(spell).Targets) != 0 ||
		h.g.Obj(src.ID).Zone != state.ZBattlefield {
		t.Fatalf("precondition: spell=%s targets=%v src=%s",
			h.g.Obj(spell).Zone, h.g.Obj(spell).Targets, h.g.Obj(src.ID).Zone)
	}

	// The trigger's own capture excludes the triggering spell from the
	// Remembered group (rememberedExcludingCapture), so only the COPIES the
	// copy effect remembered remain -- exactly the engine's binding.
	Resolve(h, &Ctx{Source: src.ID, Controller: 0, SVars: card.Faces[0].SVars,
		Remembered: []state.Target{{Obj: spell}}, Captured: []state.Target{{Obj: spell}}}, sa)
	if n := copyEvents(h); n != 0 {
		t.Fatalf("%d StackCopy events for a non-targeting spell, want 0", n)
	}
	if n := countKind(h, events.Draw); n == 0 {
		t.Fatal("the \"didn't copy\" Draw rider did not run for a non-targeting spell")
	}

	// Now a targeting spell: the copy is made, and the EQ0 rider is denied.
	h2 := newHost(t, 2)
	src2 := h2.g.AddObject(card, 0)
	events.Move(h2.g, src2.ID, state.ZLibrary, state.ZBattlefield)
	tgt := h2.g.AddObject(mkCard(t, "Name:Bear\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"), 1)
	events.Move(h2.g, tgt.ID, state.ZLibrary, state.ZBattlefield)
	libraryCard(t, h2, 0)
	spell2 := stackSpell(t, h2, 1)
	h2.g.Obj(spell2).Targets = []state.Target{{Obj: tgt.ID}}
	// PRECONDITION: the target list is non-empty and names a live permanent.
	if len(h2.g.Obj(spell2).Targets) != 1 || h2.g.Obj(tgt.ID).Zone != state.ZBattlefield {
		t.Fatalf("precondition: targeting setup is empty or the target left the battlefield")
	}
	Resolve(h2, &Ctx{Source: src2.ID, Controller: 0, SVars: card.Faces[0].SVars,
		Remembered: []state.Target{{Obj: spell2}}, Captured: []state.Target{{Obj: spell2}}}, sa)
	if n := copyEvents(h2); n != 1 {
		t.Fatalf("%d StackCopy events for a targeting spell, want 1", n)
	}
	if n := countKind(h2, events.Draw); n != 0 {
		t.Fatalf("%d Draw events after a copy, want 0 (the EQ0 rider is denied)", n)
	}
}

// TestConditionDefinedTriggeredSpellAbilityOrvarSharesEvaluator pins that
// Orvar, the All-Form's corpus spelling -- the same condition group over a
// `~Other` source-excluding target spec -- goes through the same evaluator:
// a spell targeting another permanent its controller controls admits, one
// targeting Orvar himself or nothing does not.
func TestConditionDefinedTriggeredSpellAbilityOrvarSharesEvaluator(t *testing.T) {
	card, sa := corpusCardSA(t, "Orvar, the All-Form", "TrigCopyTarget")
	if sa.Params["ConditionDefined"] != "TriggeredSpellAbility" {
		t.Fatalf("Orvar TrigCopyTarget = %+v, want ConditionDefined$ TriggeredSpellAbility", sa.Params)
	}
	if !isSpellTargetingPresent(sa.Params["ConditionPresent"]) {
		t.Fatalf("Orvar ConditionPresent = %q, want the Spell.IsTargeting form", sa.Params["ConditionPresent"])
	}
	h := newHost(t, 2)
	orvar := h.g.AddObject(card, 0)
	events.Move(h.g, orvar.ID, state.ZLibrary, state.ZBattlefield)
	other := h.g.AddObject(mkCard(t, "Name:Rock\nTypes:Artifact\nOracle:x\n"), 0)
	events.Move(h.g, other.ID, state.ZLibrary, state.ZBattlefield)
	spell := stackSpell(t, h, 0)
	ctx := &Ctx{Controller: 0, Source: orvar.ID, Remembered: []state.Target{{Obj: spell}}}
	if h.g.Obj(spell).Zone != state.ZStack || h.g.Obj(other.ID).Zone != state.ZBattlefield {
		t.Fatalf("precondition zones: spell=%s other=%s", h.g.Obj(spell).Zone, h.g.Obj(other.ID).Zone)
	}
	// Another permanent its controller controls: met.
	h.g.Obj(spell).Targets = []state.Target{{Obj: other.ID}}
	if met, resolved := conditionMet(h, ctx, sa); !met || !resolved {
		t.Fatalf("spell targeting another permanent you control: met=%v resolved=%v, want true true", met, resolved)
	}
	// Orvar himself is the source, so `~Other` excludes it: resolved false.
	h.g.Obj(spell).Targets = []state.Target{{Obj: orvar.ID}}
	if met, resolved := conditionMet(h, ctx, sa); met || !resolved {
		t.Fatalf("spell targeting the source: met=%v resolved=%v, want false true", met, resolved)
	}
	// Nothing targeted: resolved false, not fail-open.
	h.g.Obj(spell).Targets = nil
	if met, resolved := conditionMet(h, ctx, sa); met || !resolved {
		t.Fatalf("non-targeting spell: met=%v resolved=%v, want false true", met, resolved)
	}
}

// countKind is declared in token_test.go and shared here for the same
// reason: an events-package double's log is the observable behaviour.
