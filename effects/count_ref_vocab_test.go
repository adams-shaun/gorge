package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// These tests pin the count-reference vocabulary fix (task
// agent-20260920T120233Z-357a4f24, absorbing agent-20260923T161220Z-c9e65f56's
// SpellTargeted ref gap and agent-20260919T195359Z-b6f18d60's vocabulary
// report).
//
// Two things changed in effects/count.go's refTargets:
//
//  1. A structural fallback: a ref the explicit cases do not name now resolves
//     through effects/context.go's knownDefinedTargets/definedSpec -- the SAME
//     resolver a body's own Defined$ spelling goes through -- so the count
//     vocabulary can no longer lag the defined-targets vocabulary and the next
//     sibling spelling is covered without a new hand-written case. The
//     explicit cases still precede the fallback, preserving the deliberate
//     count-only distinctions (notably the RAW Imprinted associations).
//  2. Explicit count refs that definedSpec does not model but Forge's calcX
//     does: TargetedObjects/TargetedObjectsDistinct (the object-only union of
//     the chain's target choices) and SpellTargeted (the targeted spell).
//
// Every test loads the RECORDED SVar body off a real corpus card (corpus
// Lookup) and evaluates exactly that body, the cast_ref_head_test.go pattern.
// Forge card scripts stay out of the repository: only the body string is
// asserted and driven through the engine, never committed as a .txt.

// definedTargetsFor resolves a bare Defined$ spec through the public Defined
// entry point, so a test can compare the count ref's referent set with the
// body's own Defined$ spelling (the "cannot disagree" contract).
func definedTargetsFor(t *testing.T, h *fakeHost, c *Ctx, spec string) []state.Target {
	t.Helper()
	return Defined(h, c, sa(t, "SP$ Pump | Defined$ "+spec))
}

// objIDs reduces a target list to its object ids, sorted-free (the resolver
// preserves script order, so the caller compares the slices directly).
func objIDs(ts []state.Target) []state.ObjID {
	out := make([]state.ObjID, 0, len(ts))
	for _, t := range ts {
		if !t.IsPlayer && t.Obj != 0 {
			out = append(out, t.Obj)
		}
	}
	return out
}

// TestCountRefEnchantedEquippedMatchDefinedSpec pins the structural fallback on
// two real Aura/Equipment carriers whose SVar is exactly Enchanted$CardPower /
// Equipped$CardManaCost (Bind the Monster, Hedron Matrix). Before the fallback
// refTargets returned (nil, false) for both refs, so the count body read zero
// while the body's own Defined$ Enchanted/Equipped resolved the bearer; the
// test asserts the two readers now name the SAME referent set.
func TestCountRefEnchantedEquippedMatchDefinedSpec(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	auraCard, ok := reg.Lookup("Bind the Monster")
	if !ok {
		t.Fatal("corpus missing Bind the Monster")
	}
	equipCard, ok := reg.Lookup("Hedron Matrix")
	if !ok {
		t.Fatal("corpus missing Hedron Matrix")
	}
	auraBody := auraCard.Faces[0].SVars["X"]
	equipBody := equipCard.Faces[0].SVars["X"]
	// The recorded bodies are the point of the pin: if the corpus spelling
	// ever changes, this test must move with it, not silently pass.
	if auraBody != "Enchanted$CardPower" {
		t.Fatalf("Bind the Monster SVar X = %q, want Enchanted$CardPower", auraBody)
	}
	if equipBody != "Equipped$CardManaCost" {
		t.Fatalf("Hedron Matrix SVar X = %q, want Equipped$CardManaCost", equipBody)
	}

	h, c := fixtureHost(t)
	// The Aura source (c.Source) is attached to a 3-power creature; the
	// Equipment source is a DIFFERENT object attached to a {4} artifact. Two
	// different bearers so a swapped source cannot pass.
	aura := mkCard(t, "Name:Enchanted Source\nTypes:Enchantment Aura\nOracle:x\n")
	equip := mkCard(t, "Name:Equipped Source\nTypes:Artifact Equipment\nOracle:x\n")
	bearer := mkCard(t, "Name:Enchanted Bearer\nTypes:Creature Beast\nPT:3/3\nOracle:x\n")
	artifact := mkCard(t, "Name:Equipped Bearer\nManaCost:4\nTypes:Artifact\nOracle:x\n")
	auraObj := h.g.AddObject(aura, 0)
	equipObj := h.g.AddObject(equip, 0)
	bearerObj := h.g.AddObject(bearer, 0)
	artifactObj := h.g.AddObject(artifact, 0)
	// Resolve through Game.Obj, never the AddObject return value: a later
	// AddObject may have reallocated g.Objs and invalidated the earlier
	// pointer.
	h.g.Obj(auraObj.ID).AttachedTo = bearerObj.ID
	h.g.Obj(equipObj.ID).AttachedTo = artifactObj.ID

	// Preconditions: both links exist and the two bearers DIFFER in the value
	// each property reads (power 3 vs mana value 4), so a wrong-bearer read
	// cannot coincide with the expected value.
	if h.g.Obj(h.g.Obj(auraObj.ID).AttachedTo) == nil || h.g.Obj(h.g.Obj(equipObj.ID).AttachedTo) == nil {
		t.Fatal("precondition: an attached bearer is missing")
	}
	if bearerObj.Face() == nil || bearerObj.Face().Power() != 3 {
		t.Fatalf("precondition: aura bearer power = %d, want 3", bearerObj.Face().Power())
	}
	if artifactObj.Face() == nil || artifactObj.Face().Cmc() != 4 {
		t.Fatalf("precondition: equipment bearer mana value = %d, want 4", artifactObj.Face().Cmc())
	}

	// The aura source's Enchanted read.
	auraCtx := *c
	auraCtx.Source = auraObj.ID
	if got := EvalCount(h, &auraCtx, auraBody); got != 3 {
		t.Errorf("Enchanted$CardPower = %d, want 3 (the aura's bearer)", got)
	}
	if ids := objIDs(definedTargetsFor(t, h, &auraCtx, "Enchanted")); !sameIDs(ids, []state.ObjID{bearerObj.ID}) {
		t.Errorf("Defined$ Enchanted = %v, want [%d]", ids, bearerObj.ID)
	}

	// The equipment source's Equipped read.
	equipCtx := *c
	equipCtx.Source = equipObj.ID
	if got := EvalCount(h, &equipCtx, equipBody); got != 4 {
		t.Errorf("Equipped$CardManaCost = %d, want 4 (the equipment's bearer)", got)
	}
	if ids := objIDs(definedTargetsFor(t, h, &equipCtx, "Equipped")); !sameIDs(ids, []state.ObjID{artifactObj.ID}) {
		t.Errorf("Defined$ Equipped = %v, want [%d]", ids, artifactObj.ID)
	}

	// The exotic verdict: the ref is admitted (not fail-closed), so a caller
	// that distinguishes a modelled zero from an unmodelled head sees true.
	if _, ok := EvalCountOK(h, &auraCtx, auraBody); !ok {
		t.Error("EvalCountOK(Enchanted$CardPower) reported not-evaluated; the ref is not admitted")
	}
}

// TestCountRefTargetedObjects pins Forge's target union count ref
// (AbilityUtils.calcX): all chosen targets including players, and the Distinct
// spelling de-duplicated. Carrier: Builder's Bane's
// SVar:X:TargetedObjects$Amount ("Destroy X target artifacts").
func TestCountRefTargetedObjects(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	card, ok := reg.Lookup("Builder's Bane")
	if !ok {
		t.Fatal("corpus missing Builder's Bane")
	}
	body := card.Faces[0].SVars["X"]
	if body != "TargetedObjects$Amount" {
		t.Fatalf("Builder's Bane SVar X = %q, want TargetedObjects$Amount", body)
	}

	h, c := fixtureHost(t)
	artA := h.g.AddObject(mkCard(t, "Name:Artifact A\nTypes:Artifact\nOracle:x\n"), 1)
	artB := h.g.AddObject(mkCard(t, "Name:Artifact B\nTypes:Artifact\nOracle:x\n"), 1)
	c.Targets = []state.Target{
		{Obj: artA.ID},
		{Player: 1, IsPlayer: true},
		{Obj: artB.ID},
	}
	// Precondition: the two object targets exist and are distinct, and a
	// player target is interleaved so the player-inclusive Amount is exercised.
	if h.g.Obj(artA.ID) == nil || h.g.Obj(artB.ID) == nil || artA.ID == artB.ID {
		t.Fatal("precondition: object targets missing or not distinct")
	}
	if !c.Targets[1].IsPlayer {
		t.Fatal("precondition: no player target seeded")
	}

	// Two objects plus one player target.
	if got := EvalCount(h, c, body); got != 3 {
		t.Errorf("%s = %d, want 3 (two objects and one player target)", body, got)
	}
	if got := EvalCount(h, c, "TargetedObjects$Amount"); got != 3 {
		t.Errorf("TargetedObjects$Amount = %d, want 3", got)
	}

	// The chain union (Ctx.AllTargets) is what Forge enumerates: it carries a
	// DUPLICATE of artA, so the plain count sees 4 target entries while
	// Distinct de-duplicates to 3 (two objects and the player). This proves
	// Distinct reads the union and dedups by full target identity.
	c.AllTargets = []state.Target{
		{Obj: artA.ID},
		{Player: 1, IsPlayer: true},
		{Obj: artB.ID},
		{Obj: artA.ID},
	}
	if got := EvalCount(h, c, "TargetedObjects$Amount"); got != 4 {
		t.Errorf("union TargetedObjects$Amount = %d, want 4 (duplicate counted)", got)
	}
	if got := EvalCount(h, c, "TargetedObjectsDistinct$Amount"); got != 3 {
		t.Errorf("TargetedObjectsDistinct$Amount = %d, want 3 (de-duplicated)", got)
	}

	// A player-only target list counts that player; it is not lost as a
	// non-object sentinel.
	onlyPlayers := *c
	onlyPlayers.AllTargets = nil
	onlyPlayers.Targets = []state.Target{{Player: 1, IsPlayer: true}}
	if n, ok := EvalCountOK(h, &onlyPlayers, body); !ok || n != 1 {
		t.Errorf("player-only %s = (%d,%v), want (1,true)", body, n, ok)
	}
}

// TestCountRefSpellTargetedCardManaCostLKI pins the consolidated ticket's named
// acceptance: SpellTargeted$CardManaCostLKI reads the targeted SPELL's mana
// value on the real carrier Reject Imperfection ("Counter target spell. If
// that spell's mana value was 3 or less, proliferate." -- SVar:X is exactly
// this spelling). Before the ref existed, refTargets failed closed and the
// intervening-if gate's X was permanently 0, so the proliferate never ran.
func TestCountRefSpellTargetedCardManaCostLKI(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	card, ok := reg.Lookup("Reject Imperfection")
	if !ok {
		t.Fatal("corpus missing Reject Imperfection")
	}
	body := card.Faces[0].SVars["X"]
	if body != "SpellTargeted$CardManaCostLKI" {
		t.Fatalf("Reject Imperfection SVar X = %q, want SpellTargeted$CardManaCostLKI", body)
	}

	h, c := fixtureHost(t)
	// A {2}{U} targeted spell: mana value 3. Put it on the stack so the
	// target is where a real counter's target sits.
	spell := mkCard(t, "Name:Targeted Spell\nManaCost:2 U\nTypes:Instant\nOracle:x\n")
	spellObj := h.g.AddObject(spell, 1)
	stackSpell := h.g.Obj(spellObj.ID)
	stackSpell.Zone = state.ZStack
	h.g.SetZone(state.ZStack, 1, []state.ObjID{spellObj.ID})
	c.Targets = []state.Target{{Obj: spellObj.ID}}
	// Precondition: the target is a real stack object with mana value 3, and
	// the SOURCE (the counterspell) has a DIFFERENT mana value, so a read of
	// the wrong object cannot coincide with the expected 3.
	if o := h.g.Obj(spellObj.ID); o == nil || o.Zone != state.ZStack || o.Face() == nil || o.Face().Cmc() != 3 {
		t.Fatalf("precondition: target spell not a cmc-3 stack object: %+v", h.g.Obj(spellObj.ID))
	}
	if src := h.g.Obj(c.Source); src != nil && src.Face() != nil && src.Face().Cmc() == 3 {
		t.Fatal("precondition: source mana value equals the target's; proves nothing")
	}

	if got := EvalCount(h, c, body); got != 3 {
		t.Errorf("%s = %d, want 3 (the targeted spell's mana value)", body, got)
	}
	// The same body against a different targeted spell answers that spell's
	// value, so the read tracks the target rather than a constant.
	big := h.g.AddObject(mkCard(t, "Name:Big Spell\nManaCost:3 R R\nTypes:Sorcery\nOracle:x\n"), 1)
	h.g.Obj(big.ID).Zone = state.ZStack
	h.g.SetZone(state.ZStack, 1, []state.ObjID{big.ID})
	bigCtx := *c
	bigCtx.Targets = []state.Target{{Obj: big.ID}}
	if big.Face() == nil || big.Face().Cmc() != 5 {
		t.Fatalf("precondition: big spell mana value = %d, want 5", big.Face().Cmc())
	}
	if got := EvalCount(h, &bigCtx, body); got != 5 {
		t.Errorf("%s (bigger target) = %d, want 5", body, got)
	}
	if _, ok := EvalCountOK(h, &bigCtx, body); !ok {
		t.Error("EvalCountOK(SpellTargeted$CardManaCostLKI) reported not-evaluated; the ref is not admitted")
	}
}

// TestCountRefImprintedKeepsRawAssociations pins the deliberate count-only
// distinction the structural fallback must NOT erase: the Imprinted REF reads
// the source's RAW imprint association (Forge's getImprintedCards, no zone
// gate), while definedSpec's Defined$ Imprinted applies the CR 607.2a exile
// gate. A battlefield-imprinted object is therefore counted by the ref and
// NOT resolved by Defined$, and the explicit `case "Imprinted"` preceding the
// fallback is what preserves that.
func TestCountRefImprintedKeepsRawAssociations(t *testing.T) {
	h, c := fixtureHost(t)
	imprinted := h.g.AddObject(mkCard(t, "Name:Imprinted Card\nManaCost:2\nTypes:Artifact\nOracle:x\n"), 0)
	// ON THE BATTLEFIELD: a real getImprintedCards read still sees it, the
	// exile gate refuses it.
	h.g.Obj(imprinted.ID).Zone = state.ZBattlefield
	h.g.SetZone(state.ZBattlefield, 0, []state.ObjID{imprinted.ID})
	h.g.Obj(c.Source).Imprinted = []state.ObjID{imprinted.ID}

	// Precondition: the association is set and the object is NOT in exile, so
	// the two readers genuinely have different answers.
	if h.g.Obj(imprinted.ID).Zone != state.ZBattlefield {
		t.Fatalf("precondition: imprinted object zone = %v, want battlefield", h.g.Obj(imprinted.ID).Zone)
	}

	// The raw count ref sees it.
	if got := EvalCount(h, c, "Imprinted$Amount"); got != 1 {
		t.Errorf("Imprinted$Amount = %d, want 1 (raw association, no zone gate)", got)
	}
	// Defined$ Imprinted applies the exile gate and resolves nothing -- the
	// distinction the fallback must keep.
	if ids := objIDs(definedTargetsFor(t, h, c, "Imprinted")); len(ids) != 0 {
		t.Errorf("Defined$ Imprinted = %v, want [] (CR 607.2a exile gate)", ids)
	}
}

// TestCountRefUnknownFailsClosed pins the fail-closed direction the fallback
// must preserve: a ref neither the explicit cases nor knownDefinedTargets
// models is not evaluated, so a gate on it stays silent exactly as before.
func TestCountRefUnknownFailsClosed(t *testing.T) {
	h, c := fixtureHost(t)
	if n, ok := EvalCountOK(h, c, "NoSuchRefAnywhere$Amount"); ok {
		t.Errorf("EvalCountOK(NoSuchRefAnywhere$Amount) = (%d, true), want not-evaluated", n)
	}
	if got := EvalCount(h, c, "NoSuchRefAnywhere$Amount"); got != 0 {
		t.Errorf("EvalCount(NoSuchRefAnywhere$Amount) = %d, want 0", got)
	}
}
