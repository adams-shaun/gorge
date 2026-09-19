package effects

// Defined$ ValidStack unit coverage: the arm's kind/controller half is the
// SAME matcher target legality's TargetType$ census uses (state.StackKindOf
// / state.StackKindTokenOf / state.StackKindAdmits -- the grammar lives in
// state because effects cannot import rules), so these tests pin the arm's
// own additions on top of it: the Other qualifier, the sharesNameWith
// qualifier (resolvable inner spec admits; an inner spec this build cannot
// resolve -- Grimoire Thief's ExiledWithSource, which needs exile
// provenance -- fails CLOSED to an empty name set), and the
// RememberCountered$/Remembered$Amount count a SubAbility$ reads.

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// abilityCasterSrc carries one activated ability and one trigger, so the
// fixture can mint one ability object of each kind from its face.
const abilityCasterSrc = "Name:Caster\nManaCost:1 U\nTypes:Creature Wizard\nPT:1/3\n" +
	"A:AB$ Draw | Cost$ T | Defined$ You | SpellDescription$ Tap: draw a card.\n" +
	"T:Mode$ Phase | Phase$ Upkeep | ValidPlayer$ You | Execute$ TrigDraw | TriggerDescription$ At your upkeep, draw a card.\n" +
	"SVar:TrigDraw:DB$ Draw | Defined$ You\nOracle:x\n"

// validStackIds names the fixture objects: seat 0's own spell (the Ctx's
// source), seat 1's spell, and seat 1's activated/triggered ability objects.
type validStackIds struct {
	own, opp, activated, triggered state.ObjID
}

func validStackBoard(t *testing.T) (*fakeHost, *Ctx, validStackIds) {
	t.Helper()
	h := newHost(t, 2)
	caster := h.g.AddObject(mkCard(t, abilityCasterSrc), 1)
	caster.Zone = state.ZBattlefield
	h.g.SetZone(state.ZBattlefield, 1, []state.ObjID{caster.ID})
	face := h.g.Obj(caster.ID).Face()

	own := h.g.AddObject(mkCard(t, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"), 0)
	own.Zone = state.ZStack
	opp := h.g.AddObject(mkCard(t, "Name:Ogre\nManaCost:3 R\nTypes:Creature Ogre\nPT:3/3\nOracle:x\n"), 1)
	opp.Zone = state.ZStack
	activated := h.g.AddObject(nil, 1)
	activated.Ability = face.Abilities[0]
	activated.Source = caster.ID
	activated.Zone = state.ZStack
	triggered := h.g.AddObject(nil, 1)
	triggered.Ability = face.Triggers[0].Effect
	triggered.Source = caster.ID
	triggered.Zone = state.ZStack
	h.g.SetZone(state.ZStack, 0, []state.ObjID{own.ID, opp.ID, activated.ID, triggered.ID})
	ctx := &Ctx{Source: own.ID, Controller: 0}
	return h, ctx, validStackIds{own.ID, opp.ID, activated.ID, triggered.ID}
}

// TestValidStackDefinedMatchesKindAndController pins the Glen Elendra's
// Answer spec: from seat 0, every seat-1 stack object of the named kinds, in
// stack order, and never a seat-0 object.
func TestValidStackDefinedMatchesKindAndController(t *testing.T) {
	h, ctx, ids := validStackBoard(t)
	got := Defined(h, ctx, sa(t, "SP$ Counter | Defined$ ValidStack Spell.OppCtrl,Activated.OppCtrl,Triggered.OppCtrl"))
	var out []state.ObjID
	for _, x := range got {
		out = append(out, x.Obj)
	}
	want := []state.ObjID{ids.opp, ids.activated, ids.triggered}
	if len(out) != len(want) {
		t.Fatalf("Defined() = %v, want %v", out, want)
	}
	for i := range want {
		if out[i] != want[i] {
			t.Fatalf("Defined() = %v, want %v (stack order)", out, want)
		}
	}
}

// TestValidStackDefinedOtherExcludesTheSource pins Swift Silence's
// `Spell.Other`: every OTHER spell on the stack regardless of controller,
// never the resolving source itself.
func TestValidStackDefinedOtherExcludesTheSource(t *testing.T) {
	h, ctx, ids := validStackBoard(t)
	got := Defined(h, ctx, sa(t, "SP$ Counter | Defined$ ValidStack Spell.Other"))
	if len(got) != 1 || got[0].Obj != ids.opp {
		t.Fatalf("Defined() = %v, want only the opponent spell %d (the source %d is excluded)", got, ids.opp, ids.own)
	}
}

// TestValidStackSharesNameWithResolvableSpec pins the sharesNameWith
// qualifier on an inner spec this build CAN resolve: a seat-1 spell whose
// name matches a card the inner spec names is admitted; one whose name does
// not is not.
func TestValidStackSharesNameWithResolvableSpec(t *testing.T) {
	h, ctx, ids := validStackBoard(t)
	// A third spell on the stack, seat 1's, named "Twin". The inner spec
	// `Card.YouCtrl` names every seat-0-controlled card, so the name set is
	// {Bear} and the token admits exactly the stack spell named Bear -- seat
	// 0's own spell (the Other-excluded source), not the Ogre or the Twin.
	twin := h.g.AddObject(mkCard(t, "Name:Twin\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"), 1)
	twin.Zone = state.ZStack
	h.g.SetZone(state.ZStack, 0, []state.ObjID{ids.own, ids.opp, ids.activated, ids.triggered, twin.ID})

	got := Defined(h, ctx, sa(t, "SP$ Counter | Defined$ ValidStack Spell.sharesNameWith Card.YouCtrl"))
	if len(got) != 1 || got[0].Obj != ids.own {
		t.Fatalf("Defined() = %v, want only the spell sharing seat 0's spell's name (%d)", got, ids.own)
	}
}

// TestValidStackSharesNameWithExiledWithSourceFailsClosed pins Grimoire
// Thief's real spec: `Spell.sharesNameWith ExiledWithSource`. This build
// tracks no exile provenance, so `ExiledWithSource` is an unknown predicate
// that matches no card, the name set is empty, and the token admits nothing
// -- the counter counts zero rather than widening to every spell.
func TestValidStackSharesNameWithExiledWithSourceFailsClosed(t *testing.T) {
	h, ctx, _ := validStackBoard(t)
	got := Defined(h, ctx, sa(t, "SP$ Counter | Defined$ ValidStack Spell.sharesNameWith ExiledWithSource"))
	if len(got) != 0 {
		t.Fatalf("Defined() = %v, want nothing (ExiledWithSource resolves to no cards, so the name set is empty)", got)
	}
}

// TestValidStackUnknownTokenDegradesToSpellOnly pins the narrow default: a
// ValidStack spec whose tokens name no stack kind degrades to Spell-only --
// every card-object spell on the stack, either controller, no ability object
// -- the same default StackKindTokens gives a TargetType$ that names no
// stack kind.
func TestValidStackUnknownTokenDegradesToSpellOnly(t *testing.T) {
	h, ctx, ids := validStackBoard(t)
	got := Defined(h, ctx, sa(t, "SP$ Counter | Defined$ ValidStack Creature.OppCtrl"))
	if len(got) != 2 || got[0].Obj != ids.own || got[1].Obj != ids.opp {
		t.Fatalf("Defined() = %v, want the spell-only default (%d, %d)", got, ids.own, ids.opp)
	}
}

// TestCounterRememberCounteredFeedsRememberedAmount pins the counting chain
// Swift Silence's SubAbility reads: effCounter with RememberCountered$ True
// appends each countered object to Ctx.Remembered, and SVar:X:Remembered$Amount
// evaluates to that count, so the chained DB$ Draw draws exactly one card
// per countered object.
func TestCounterRememberCounteredFeedsRememberedAmount(t *testing.T) {
	h, ctx, ids := validStackBoard(t)
	src := "Name:T\nTypes:Sorcery\n" +
		"A:SP$ Counter | Defined$ ValidStack Spell.Other | RememberCountered$ True | SubAbility$ DBDraw\n" +
		"SVar:DBDraw:DB$ Draw | NumCards$ X\n" +
		"SVar:X:Remembered$Amount\nOracle:x\n"
	c, d := cards.ParseBytes("t.txt", []byte(src))
	if len(d) != 0 {
		t.Fatalf("diags: %v", d)
	}
	c.Link()
	SetSVars(ctx, c.Faces[0].SVars)
	// One card in the drawing seat's library, so the chained Draw draws it
	// rather than hitting the empty-library loss (state.NewGame deals no
	// libraries).
	lib := h.g.AddObject(mkCard(t, "Name:Filler\nTypes:Sorcery\nOracle:x\n"), 0)
	lib.Zone = state.ZLibrary
	h.g.SetZone(state.ZLibrary, 0, []state.ObjID{lib.ID})
	Resolve(h, ctx, c.Faces[0].Abilities[0])

	n := 0
	for _, ev := range h.log {
		if ev.Kind == events.Draw && ev.Player == ctx.Controller {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("drew %d cards, want 1 (one spell countered by Spell.Other)", n)
	}
	if z := h.g.Obj(ids.opp).Zone; z != state.ZGraveyard {
		t.Fatalf("countered spell went to %s, want graveyard", z)
	}
}

// abilityStackBoard mints one activated and one triggered ability wrapper
// for EACH seat plus one spell per seat, so the `Ability` base's two-kind
// reach and its controller qualifier can be pinned from seat 0's ctx.
// Objects are minted directly (the same licence validStackBoard uses).
func abilityStackBoard(t *testing.T) (*fakeHost, *Ctx, validStackIds, validStackIds) {
	t.Helper()
	h := newHost(t, 2)
	caster0 := h.g.AddObject(mkCard(t, abilityCasterSrc), 0)
	caster0.Zone = state.ZBattlefield
	caster1 := h.g.AddObject(mkCard(t, abilityCasterSrc), 1)
	caster1.Zone = state.ZBattlefield
	face0 := h.g.Obj(caster0.ID).Face()
	face1 := h.g.Obj(caster1.ID).Face()

	own := h.g.AddObject(mkCard(t, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"), 0)
	own.Zone = state.ZStack
	opp := h.g.AddObject(mkCard(t, "Name:Ogre\nManaCost:3 R\nTypes:Creature Ogre\nPT:3/3\nOracle:x\n"), 1)
	opp.Zone = state.ZStack
	act0 := h.g.AddObject(nil, 0)
	act0.Ability = face0.Abilities[0]
	act0.Source = caster0.ID
	act0.Zone = state.ZStack
	trg0 := h.g.AddObject(nil, 0)
	trg0.Ability = face0.Triggers[0].Effect
	trg0.Source = caster0.ID
	trg0.Zone = state.ZStack
	act1 := h.g.AddObject(nil, 1)
	act1.Ability = face1.Abilities[0]
	act1.Source = caster1.ID
	act1.Zone = state.ZStack
	trg1 := h.g.AddObject(nil, 1)
	trg1.Ability = face1.Triggers[0].Effect
	trg1.Source = caster1.ID
	trg1.Zone = state.ZStack
	h.g.SetZone(state.ZStack, 0, []state.ObjID{own.ID, opp.ID, act0.ID, trg0.ID, act1.ID, trg1.ID})
	ctx := &Ctx{Source: own.ID, Controller: 0}
	return h, ctx, validStackIds{own.ID, opp.ID, act0.ID, trg0.ID}, validStackIds{own.ID, opp.ID, act1.ID, trg1.ID}
}

// idsOf extracts the stack-order object ids of a Defined() result.
func idsOf(ts []state.Target) []state.ObjID {
	out := make([]state.ObjID, 0, len(ts))
	for _, x := range ts {
		out = append(out, x.Obj)
	}
	return out
}

// TestValidStackAbilityBaseAdmitsBothAbilityKindsYouCtrl pins the `Ability`
// base (task abcopy3): an alias for Activated+Triggered -- never a spell
// kind -- and its YouCtrl qualifier reads through the shared
// state.StackKindAdmits matcher.
func TestValidStackAbilityBaseAdmitsBothAbilityKindsYouCtrl(t *testing.T) {
	h, ctx, s0, s1 := abilityStackBoard(t)
	got := Defined(h, ctx, sa(t, "SP$ Counter | Defined$ ValidStack Ability.YouCtrl"))
	if want := []state.ObjID{s0.activated, s0.triggered}; len(idsOf(got)) != len(want) || idsOf(got)[0] != want[0] || idsOf(got)[1] != want[1] {
		t.Fatalf("Ability.YouCtrl = %v, want %v (both ability kinds you control, stack order; never a spell, never seat 1's %v/%v)",
			idsOf(got), want, s1.activated, s1.triggered)
	}
	// No qualifier: every ability wrapper either seat controls, still never
	// a spell.
	got = Defined(h, ctx, sa(t, "SP$ Counter | Defined$ ValidStack Ability"))
	if len(idsOf(got)) != 4 {
		t.Fatalf("Ability = %v, want all four ability wrappers %v", idsOf(got), []state.ObjID{s0.activated, s0.triggered, s1.activated, s1.triggered})
	}
}

// TestValidStackOtherAbilityExcludesTheResolvingWrapper pins the
// otherAbility qualifier's anchor: Ctx.ResolvingObj -- the resolving
// stack-object WRAPPER -- not Ctx.Source, which for an ability resolution is
// the source permanent (Ruling T20-b) and excludes nothing on the stack.
// Without this, Ulalek's sub-copy would copy its own still-resolving wrapper
// (each copy asking its pay question again).
func TestValidStackOtherAbilityExcludesTheResolvingWrapper(t *testing.T) {
	h, ctx, s0, _ := abilityStackBoard(t)
	// Plain YouCtrl first: both seat-0 wrappers are admitted before the
	// exclusion narrows anything.
	got := Defined(h, ctx, sa(t, "SP$ Counter | Defined$ ValidStack Ability.YouCtrl+otherAbility"))
	if len(idsOf(got)) != 2 || idsOf(got)[0] != s0.activated || idsOf(got)[1] != s0.triggered {
		t.Fatalf("Ability.YouCtrl+otherAbility with no anchor = %v, want both seat-0 wrappers", idsOf(got))
	}
	// Anchor on the resolving wrapper: the triggered wrapper is excluded.
	ctx.ResolvingObj = s0.triggered
	got = Defined(h, ctx, sa(t, "SP$ Counter | Defined$ ValidStack Ability.YouCtrl+otherAbility"))
	if len(idsOf(got)) != 1 || idsOf(got)[0] != s0.activated {
		t.Fatalf("with ResolvingObj = %d: %v, want only the activated wrapper (the resolving one excluded)", s0.triggered, idsOf(got))
	}
	// A context with ResolvingObj zero (a hand-built one) falls back to the
	// Source anchor -- never widened.
	ctx2 := &Ctx{Source: s0.activated, Controller: 0}
	got = Defined(h, ctx2, sa(t, "SP$ Counter | Defined$ ValidStack Ability.YouCtrl+otherAbility"))
	if len(idsOf(got)) != 1 || idsOf(got)[0] != s0.triggered {
		t.Fatalf("Source-anchored fallback = %v, want only the triggered wrapper (the source excluded)", idsOf(got))
	}
}

// TestValidStackUnknownAbilityTokenDegradesToSpellOnly pins the guard
// against a near-miss base: `Ability2` names no stack kind, so the token is
// dropped and the spec degrades to the Spell-only default (no controller
// qualifier) -- the same narrow default every unparsed token gets, never a
// widened one, and copy.go's known-token guard keeps it fail-closed.
func TestValidStackUnknownAbilityTokenDegradesToSpellOnly(t *testing.T) {
	h, ctx, ids, _ := abilityStackBoard(t)
	got := Defined(h, ctx, sa(t, "SP$ Counter | Defined$ ValidStack Ability2.OppCtrl"))
	if want := []state.ObjID{ids.own, ids.opp}; len(idsOf(got)) != 2 || idsOf(got)[0] != want[0] || idsOf(got)[1] != want[1] {
		t.Fatalf("Ability2.OppCtrl = %v, want the spell-only default %v", idsOf(got), want)
	}
}

// TestValidStackOtherAbilityExcludesTheSameAbilityFamily pins the loop
// guard's family half: the exclusion covers not only the resolving wrapper
// but every other instance and copy of the SAME printed ability (same source
// permanent AND same Ability pointer -- StackCopy preserves both and every
// mint of one printed trigger shares the parsed slice's pointer). Without it
// a paid Ulalek trigger whose sub-copy copies an earlier still-on-the-stack
// instance of its own trigger regresses: the copy asks the same pay question,
// a deterministic host answers it the same way, and the walk never ends.
func TestValidStackOtherAbilityExcludesTheSameAbilityFamily(t *testing.T) {
	h := newHost(t, 2)
	caster0 := h.g.AddObject(mkCard(t, abilityCasterSrc), 0)
	caster0.Zone = state.ZBattlefield
	face0 := h.g.Obj(caster0.ID).Face()
	own := h.g.AddObject(mkCard(t, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"), 0)
	own.Zone = state.ZStack
	// Two instances of the SAME printed activated ability (the same SA
	// pointer), plus one instance of the trigger.
	inst1 := h.g.AddObject(nil, 0)
	inst1.Ability = face0.Abilities[0]
	inst1.Source = caster0.ID
	inst1.Zone = state.ZStack
	inst2 := h.g.AddObject(nil, 0)
	inst2.Ability = face0.Abilities[0]
	inst2.Source = caster0.ID
	inst2.Zone = state.ZStack
	trg := h.g.AddObject(nil, 0)
	trg.Ability = face0.Triggers[0].Effect
	trg.Source = caster0.ID
	trg.Zone = state.ZStack
	h.g.SetZone(state.ZStack, 0, []state.ObjID{own.ID, inst1.ID, inst2.ID, trg.ID})
	ctx := &Ctx{Source: own.ID, Controller: 0, ResolvingObj: inst1.ID}

	got := Defined(h, ctx, sa(t, "SP$ Counter | Defined$ ValidStack Ability.YouCtrl+otherAbility"))
	if len(got) != 1 || got[0].Obj != trg.ID {
		t.Fatalf("with the resolving inst1 anchor: %v, want only the DIFFERENT ability %d (inst1 %d and its same-ability sibling inst2 %d excluded)",
			idsOf(got), trg.ID, inst1.ID, inst2.ID)
	}
}
