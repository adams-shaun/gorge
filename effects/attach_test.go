package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// attachBoard builds a 2-seat game with an Equipment (source) and a creature
// (bear) both on seat 0's battlefield, plus a second copy of the bear card
// sitting in seat 0's library (off the battlefield, so it can stand in for a
// non-permanent "target"). The Ctx sources the equipment.
func attachBoard(t *testing.T) (*fakeHost, *Ctx, map[string]state.ObjID) {
	t.Helper()
	h := newHost(t, 2)
	eqCard := mkCard(t, "Name:Sword\nManaCost:3\nTypes:Artifact Equipment\nOracle:x\n")
	bearCard := mkCard(t, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	eq := h.g.AddObject(eqCard, 0)
	bear := h.g.AddObject(bearCard, 0)
	inLib := h.g.AddObject(bearCard, 0)
	// Set zones by ID lookup, never by the stored pointers: AddObject's
	// returned pointer aliases g.Objs's backing array, so a later append
	// (inLib below) can invalidate an earlier one the moment g.Objs grows --
	// setting Zone through a stale pointer would silently write to dead
	// memory. board() in filter_test.go dodges this by placing each object
	// immediately after its own AddObject, before the next one.
	for _, id := range []state.ObjID{eq.ID, bear.ID} {
		h.g.Obj(id).Zone = state.ZBattlefield
		h.g.SetZone(state.ZBattlefield, 0, append(h.g.Zone(state.ZBattlefield, 0), id))
	}
	h.g.SetZone(state.ZLibrary, 0, []state.ObjID{inLib.ID})
	return h, &Ctx{Source: eq.ID, Controller: 0}, map[string]state.ObjID{
		"eq": eq.ID, "bear": bear.ID, "inLib": inLib.ID,
	}
}

func TestAttachOntoARememberedObjectEmitsAttach(t *testing.T) {
	h, c, ids := attachBoard(t)
	c.Remembered = []state.Target{{Obj: ids["bear"]}}
	Resolve(h, c, sa(t, "SP$ Attach | Object$ Self | Defined$ Remembered"))

	var attachs []events.Event
	for _, ev := range h.log {
		if ev.Kind == events.Attach {
			attachs = append(attachs, ev)
		}
	}
	if len(attachs) != 1 || attachs[0].Obj != ids["eq"] ||
		len(attachs[0].IDs) != 1 || attachs[0].IDs[0] != ids["bear"] {
		t.Fatalf("attachs = %+v, want one Attach{eq->bear}", attachs)
	}
	// Apply ran, so the live object is actually attached.
	if h.g.Obj(ids["eq"]).AttachedTo != ids["bear"] {
		t.Fatalf("eq.AttachedTo = %d, want %d", h.g.Obj(ids["eq"]).AttachedTo, ids["bear"])
	}
}

func TestAttachOntoAPlayerRefusesWithANote(t *testing.T) {
	h, c, _ := attachBoard(t)
	c.Remembered = []state.Target{{Player: 1, IsPlayer: true}}
	Resolve(h, c, sa(t, "SP$ Attach | Object$ Self | Defined$ Remembered"))

	for _, ev := range h.log {
		if ev.Kind == events.Attach {
			t.Fatalf("unexpected Attach %+v onto a player", ev)
		}
	}
	if !hasNoteLike(h.log, "no legal target") {
		t.Fatalf("expected a Note refusing the attach, got %+v", h.log)
	}
}

func TestAttachOntoANonPermanentRefusesWithANote(t *testing.T) {
	h, c, ids := attachBoard(t)
	c.Remembered = []state.Target{{Obj: ids["inLib"]}} // not on the battlefield
	Resolve(h, c, sa(t, "SP$ Attach | Object$ Self | Defined$ Remembered"))

	for _, ev := range h.log {
		if ev.Kind == events.Attach {
			t.Fatalf("unexpected Attach %+v onto a non-permanent", ev)
		}
	}
	if !hasNoteLike(h.log, "no legal target") {
		t.Fatalf("expected a Note refusing the attach, got %+v", h.log)
	}
}

func TestAttachRefusesToAttachToItself(t *testing.T) {
	h, c, ids := attachBoard(t)
	c.Remembered = []state.Target{{Obj: ids["eq"]}} // the attachment object itself
	Resolve(h, c, sa(t, "SP$ Attach | Object$ Self | Defined$ Remembered"))

	for _, ev := range h.log {
		if ev.Kind == events.Attach {
			t.Fatalf("unexpected Attach %+v onto itself", ev)
		}
	}
}

func TestEquipedByMatchesOnlyTheAttachedPermanent(t *testing.T) {
	h, c, ids := attachBoard(t)
	// Attach the equipment to the bear first.
	c.Remembered = []state.Target{{Obj: ids["bear"]}}
	Resolve(h, c, sa(t, "SP$ Attach | Object$ Self | Defined$ Remembered"))
	if h.g.Obj(ids["eq"]).AttachedTo != ids["bear"] {
		t.Fatalf("setup: eq not attached to bear")
	}

	if got := MatchesSpecFrom(h.g, "Creature.EquippedBy", ids["bear"], 0, ids["eq"]); !got {
		t.Errorf("EquippedBy should match the attached bear")
	}
	if got := MatchesSpecFrom(h.g, "Creature.EnchantedBy", ids["bear"], 0, ids["eq"]); !got {
		t.Errorf("EnchantedBy should match the attached bear")
	}
	// The in-library copy is the same card name but is NOT the attached one.
	if got := MatchesSpecFrom(h.g, "Creature.EquippedBy", ids["inLib"], 0, ids["eq"]); got {
		t.Errorf("EquippedBy must not match an unattached/non-battlefield same-named object")
	}
}

func TestCompiledPredicateAttachmentTermsAreDefinite(t *testing.T) {
	h, c, ids := attachBoard(t)
	c.Remembered = []state.Target{{Obj: ids["bear"]}}
	Resolve(h, c, sa(t, "SP$ Attach | Object$ Self | Defined$ Remembered"))
	ps := CompilePredicatePrograms([]string{
		"Creature.EquippedBy", "Creature.EnchantedBy", "Creature.AttachedBy",
	})
	for _, tc := range []struct {
		spec string
		id   state.ObjID
		want PredicateResult
	}{
		{"Creature.EquippedBy", ids["bear"], PredicateYes},
		{"Creature.EnchantedBy", ids["bear"], PredicateYes},
		{"Creature.AttachedBy", ids["bear"], PredicateYes},
		{"Creature.EquippedBy", ids["inLib"], PredicateNo},
	} {
		if got := ps.Evaluate(tc.spec, h.g, h.g.Obj(tc.id), SpecContext{You: 0, Source: ids["eq"]}); got != tc.want {
			t.Errorf("Evaluate(%q) = %v, want %v", tc.spec, got, tc.want)
		}
	}
}

func hasNoteLike(log []events.Event, substr string) bool {
	for _, ev := range log {
		if ev.Kind == events.Note && len(ev.Text) > 0 && contains(ev.Text, substr) {
			return true
		}
	}
	return false
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// RememberAttached$ True (all seven corpus carriers) puts the permanent just
// attached into the ability's Remembered, both halves -- the ctx list a
// chained SubAbility / RepeatEach fold reads and the source's event-backed
// persistent list eventRemember writes -- the same two-half discipline
// RememberTokens$ and RememberTargets$ apply.
func TestAttachRememberAttachedJoinsBothHalves(t *testing.T) {
	h, c, ids := attachBoard(t)
	c.Remembered = []state.Target{{Obj: ids["bear"]}}
	Resolve(h, c, sa(t, "SP$ Attach | Object$ Self | Defined$ Remembered | RememberAttached$ True"))

	if h.g.Obj(ids["eq"]).AttachedTo != ids["bear"] {
		t.Fatalf("setup: eq not attached to bear")
	}
	found := false
	for _, t2 := range c.Remembered {
		if !t2.IsPlayer && t2.Obj == ids["eq"] {
			found = true
		}
	}
	if !found {
		t.Fatalf("ctx Remembered = %+v, want the attached equipment", c.Remembered)
	}
	if src := h.g.Obj(ids["eq"]); src == nil || !objIDIn(src.Remembered, ids["eq"]) {
		t.Fatalf("source persistent Remembered = %+v, want the attached equipment", src.Remembered)
	}
	// The negative control: without the param neither half gains the entry.
	h2, c2, ids2 := attachBoard(t)
	c2.Remembered = []state.Target{{Obj: ids2["bear"]}}
	Resolve(h2, c2, sa(t, "SP$ Attach | Object$ Self | Defined$ Remembered"))
	for _, t2 := range c2.Remembered {
		if !t2.IsPlayer && t2.Obj == ids2["eq"] {
			t.Fatalf("unrequested remember: ctx Remembered = %+v", c2.Remembered)
		}
	}
}

func objIDIn(ts []state.Target, id state.ObjID) bool {
	for _, t := range ts {
		if !t.IsPlayer && t.Obj == id {
			return true
		}
	}
	return false
}

// Choices$ with no Object$ names the OBJECT to attach (Goldwardens' Gambit,
// unexpected_request). The fake host cannot answer an ask (AskNoHost, the
// R-9 decline stand-in), so a two-candidate pool attaches nothing; the
// answered re-entry consumes Ctx.AttachChoice/AttachDests (fx42 scoping) and
// attaches the chosen object, remembering it under RememberAttached$.
func TestAttachChoicesObjectPoolAsksAndAnsweredReentryAttaches(t *testing.T) {
	h, c, ids := attachBoard(t)
	eq2 := h.g.AddObject(mkCard(t, "Name:Sword2\nManaCost:3\nTypes:Artifact Equipment\nOracle:x\n"), 0)
	h.g.Obj(eq2.ID).Zone = state.ZBattlefield
	h.g.SetZone(state.ZBattlefield, 0, append(h.g.Zone(state.ZBattlefield, 0), eq2.ID))

	Resolve(h, c, sa(t, "DB$ Attach | Choices$ Equipment.YouCtrl+!IsRemembered | Defined$ Remembered | RememberAttached$ True"))
	for _, ev := range h.log {
		if ev.Kind == events.Attach {
			t.Fatalf("unanswered ask must not attach: %+v", ev)
		}
	}

	// The answered re-entry: the choice named eq2, the destination the bear.
	c.AttachChoice = []state.ObjID{eq2.ID}
	c.AttachChoiceDone = true
	c.AttachDests = []state.ObjID{ids["bear"]}
	Resolve(h, c, sa(t, "DB$ Attach | Choices$ Equipment.YouCtrl+!IsRemembered | Defined$ Remembered | RememberAttached$ True"))
	var attachs []events.Event
	for _, ev := range h.log {
		if ev.Kind == events.Attach {
			attachs = append(attachs, ev)
		}
	}
	if len(attachs) != 1 || attachs[0].Obj != eq2.ID || attachs[0].IDs[0] != ids["bear"] {
		t.Fatalf("attachs = %+v, want eq2->bear", attachs)
	}
	if !objIDIn(c.Remembered, eq2.ID) || h.g.Obj(eq2.ID) == nil {
		t.Fatalf("ctx Remembered = %+v, want the chosen equipment", c.Remembered)
	}
	// fx42 scoping: the answer was consumed.
	if c.AttachChoice != nil || c.AttachChoiceDone || c.AttachDests != nil {
		t.Fatalf("answer fields not cleared: %+v done=%v dests=%v", c.AttachChoice, c.AttachChoiceDone, c.AttachDests)
	}
}

// A mandatory Min-1 pool with exactly one candidate takes it without an ask
// (the strict-supersets convention); a Min-0 Optional pool answered with
// nothing chosen is the decline -- no Attach, no Note.
func TestAttachChoicesSingleCandidateTakesItAndEmptyAnswerDeclines(t *testing.T) {
	h, c, ids := attachBoard(t)
	c.Remembered = []state.Target{{Obj: ids["bear"]}}
	Resolve(h, c, sa(t, "DB$ Attach | Choices$ Equipment.YouCtrl | Defined$ Remembered | RememberAttached$ True"))
	if h.g.Obj(ids["eq"]).AttachedTo != ids["bear"] {
		t.Fatalf("single-candidate mandatory pool did not attach")
	}

	h2, c2, _ := attachBoard(t)
	c2.AttachChoiceDone = true // answered, nothing chosen
	Resolve(h2, c2, sa(t, "DB$ Attach | Optional$ True | Choices$ Equipment.YouCtrl | Defined$ Remembered"))
	for _, ev := range h2.log {
		if ev.Kind == events.Attach {
			t.Fatalf("decline must emit no Attach: %+v", ev)
		}
	}
	// Count the notes the decline emitted; don't scan the whole log with
	// hasNoteLike, which matches any note anywhere and would miss a spurious
	// one outside this Resolve. The decline must be entirely silent.
	notes := 0
	for _, ev := range h2.log {
		if ev.Kind == events.Note {
			notes++
		}
	}
	if notes != 0 {
		t.Fatalf("decline must be silent, got %d note(s): %+v", notes, h2.log)
	}
}

// Choices$ WITH Object$ present names the DESTINATION pool (Breath of
// Fury's "attach CARDNAME to a creature you control"): the aura attaches to
// the single matching creature and RememberAttached$ remembers the AURA.
func TestAttachChoicesDestinationPoolAttachesAndRemembersTheObject(t *testing.T) {
	h, c, ids := attachBoard(t)
	Resolve(h, c, sa(t, "DB$ Attach | Object$ Self | Choices$ Creature.YouCtrl | RememberAttached$ True"))
	if h.g.Obj(ids["eq"]).AttachedTo != ids["bear"] {
		t.Fatalf("destination pool did not attach to the bear")
	}
	if !objIDIn(c.Remembered, ids["eq"]) {
		t.Fatalf("ctx Remembered = %+v, want the attached aura", c.Remembered)
	}
}

// The destination ask's options must be the LEGAL destination list, never
// the raw pool sweep: aura_graft's `Object$ Self | Choices$ Permanent`
// admits the attaching Aura itself (it IS a battlefield Permanent), and
// offering the source as its own destination self-attaches on a bot's
// option-0 answer (AttachChoice carries Option.Obj, so the re-entry's
// attachTo(answered[0]) emits Attach{IDs:[obj]} and events.Apply sets
// obj.AttachedTo == obj). Pinned both ways: the source is not offered, and
// a stale/malformed answer naming the source is refused.
func TestAttachDestinationAskNeverOffersTheSourceAndRefusesAChosenSource(t *testing.T) {
	h, c, ids := attachBoard(t)
	// A second bear on the battlefield: with only one legal destination the
	// auto-take fires and no ask is ever posed.
	bearCard := mkCard(t, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	bear2 := h.g.AddObject(bearCard, 0)
	h.g.Obj(bear2.ID).Zone = state.ZBattlefield
	h.g.SetZone(state.ZBattlefield, 0, append(h.g.Zone(state.ZBattlefield, 0), bear2.ID))

	ah := &askHost{fakeHost: *h}
	Resolve(ah, c, sa(t, "DB$ Attach | Object$ Self | Choices$ Permanent"))
	if ah.asked == nil {
		t.Fatalf("expected a destination ask, none posed")
	}
	if len(ah.asked.Options) != 2 {
		t.Fatalf("options = %+v, want the two bears", ah.asked.Options)
	}
	for _, o := range ah.asked.Options {
		if o.Obj == ids["eq"] {
			t.Fatalf("the attaching object was offered as its own destination: %+v", ah.asked.Options)
		}
		if o.Obj != ids["bear"] && o.Obj != bear2.ID {
			t.Fatalf("unexpected option %+v in %+v", o, ah.asked.Options)
		}
	}

	// An answer naming the source (a stale or malformed host-side answer)
	// is refused: no Attach event, and no live self-attachment.
	h2, c2, ids2 := attachBoard(t)
	c2.AttachChoiceDone = true
	c2.AttachChoice = []state.ObjID{ids2["eq"]}
	Resolve(h2, c2, sa(t, "DB$ Attach | Object$ Self | Choices$ Permanent"))
	for _, ev := range h2.log {
		if ev.Kind == events.Attach {
			t.Fatalf("a chosen source must be refused, got %+v", ev)
		}
	}
	if h2.g.Obj(ids2["eq"]).AttachedTo != 0 {
		t.Fatalf("source self-attached: AttachedTo = %d", h2.g.Obj(ids2["eq"]).AttachedTo)
	}

	// A well-formed answer naming a legal destination still attaches (the
	// load-bearing direction).
	c2.AttachChoice = []state.ObjID{ids2["bear"]}
	Resolve(h2, c2, sa(t, "DB$ Attach | Object$ Self | Choices$ Permanent"))
	if h2.g.Obj(ids2["eq"]).AttachedTo != ids2["bear"] {
		t.Fatalf("legal destination answer did not attach: AttachedTo = %d", h2.g.Obj(ids2["eq"]).AttachedTo)
	}
}
