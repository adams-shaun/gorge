package effects

// ChoiceZone$ / ChoiceOptional$ / ExcludeChosen$ on the DB$ Clone Choices$
// pick (ticket agent-20260923T090459Z-65bbfecf).
//
// Before this change the standalone/trigger Clone route honoured only the
// battlefield pool and a mandatory pick: ChoiceZone$ was ignored (the pool was
// always the battlefield), ChoiceOptional$ declined nothing (the pick was
// mandatory), and ExcludeChosen$ included the chosen source in the become pool
// (a replay-visible self-copy). These tests drive the REAL corpus carriers
// through the primitive, so a regression in any of the three clauses fails.

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// clonePermanentTargets returns the Obj ids that received a ClonePermanent
// event in h's log. It is the copy basis the become loop emits once per
// (source, become) pair.
func clonePermanentTargets(h *fakeHost) []state.ObjID {
	var out []state.ObjID
	for _, e := range h.log {
		if e.Kind == events.ClonePermanent {
			out = append(out, e.Obj)
		}
	}
	return out
}

func clonePermanentCount(h *fakeHost, id state.ObjID) int {
	n := 0
	for _, e := range h.log {
		if e.Kind == events.ClonePermanent && e.Obj == id {
			n++
		}
	}
	return n
}

// TestCloneExcludeChosenSkipsThePickedSource is the Sakashima's Will pin:
// "Choose a creature you control. Each OTHER creature you control becomes a
// copy of that creature until end of turn." ExcludeChosen$ True must drop the
// chosen creature from the become pool, or it takes a replay-visible
// self-copy ClonePermanent (and the bottom loop would register a copy marker
// on it).
func TestCloneExcludeChosenSkipsThePickedSource(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	will, ok := reg.Lookup("Sakashima's Will")
	if !ok || will == nil {
		t.Fatal("missing corpus card Sakashima's Will")
	}
	h := &fakeHost{g: state.NewGame(names(2))}
	bear := mkCard(t, "Name:Fixture Exclusion Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	// A second creature you control, the copy's become operand.
	other := mkCard(t, "Name:Fixture Exclusion Elk\nManaCost:1 G\nTypes:Creature Elk\nPT:3/3\nOracle:x\n")
	chosen := h.g.AddObject(bear, 0).ID
	otherID := h.g.AddObject(other, 0).ID
	h.g.SetZone(state.ZBattlefield, 0, []state.ObjID{chosen, otherID})
	h.g.Obj(chosen).Zone = state.ZBattlefield
	h.g.Obj(otherID).Zone = state.ZBattlefield

	body := cards.ResolveSVar(will.Faces[0].SVars, "DBClone")
	if body == nil || body.Params["ExcludeChosen"] != "True" {
		t.Fatal("precondition: Sakashima's Will DBClone must carry ExcludeChosen$ True")
	}
	if body.Params["Choices"] != "Creature.YouCtrl" || body.Params["CloneTarget"] != "Valid Creature.YouCtrl" {
		t.Fatalf("precondition: unexpected body %+v", body.Params)
	}
	// Precondition: both creatures are in the become pool named by CloneTarget$.
	if !bodyBecomeContains(h, body, chosen) || !bodyBecomeContains(h, body, otherID) {
		t.Fatal("precondition: CloneTarget$ Valid Creature.YouCtrl must cover both creatures")
	}
	if h.g.Obj(chosen).Face().Name == h.g.Obj(otherID).Face().Name {
		t.Fatal("precondition: the two creatures must be distinct")
	}

	// The answered Choices$ pick (bypassing the ask) names the chosen bear.
	Resolve(h, &Ctx{Source: chosen, Controller: 0,
		ClonePick: chosen, ClonePickDone: true, SVars: will.Faces[0].SVars}, body)

	if n := clonePermanentCount(h, chosen); n != 0 {
		t.Fatalf("the CHOSEN creature received %d self-copy ClonePermanent events; ExcludeChosen$ must drop it", n)
	}
	if n := clonePermanentCount(h, otherID); n != 1 {
		t.Fatalf("the OTHER creature received %d ClonePermanent events, want exactly 1 (got targets %v)",
			n, clonePermanentTargets(h))
	}
	if h.g.Obj(chosen).CopyFace != nil {
		t.Fatal("the chosen creature has a copy basis: it copied itself")
	}
}

// bodyBecomeContains reports whether the body's CloneTarget$ sweep covers id.
func bodyBecomeContains(h *fakeHost, body *cards.SA, id state.ObjID) bool {
	for _, tgt := range battlefieldValidTargets(h, &Ctx{Source: id, Controller: 0}, "Creature.YouCtrl") {
		if tgt.Obj == id {
			return true
		}
	}
	return false
}

// TestCloneChoiceOptionalDeclineMakesNoCopy is the Brudiclad pin:
// ChoiceOptional$ True offers a real decline, and the answered decline means
// no copy at all -- recorded as a real answer, with NO "named no object" Note
// (that Note is reserved for a malformed/empty answer on a mandatory pick).
func TestCloneChoiceOptionalDeclineMakesNoCopy(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	brud, ok := reg.Lookup("Brudiclad, Telchor Engineer")
	if !ok || brud == nil {
		t.Fatal("missing corpus card Brudiclad, Telchor Engineer")
	}
	h := &fakeHost{g: state.NewGame(names(2))}
	token := mkCard(t, "Name:Fixture Myr Token\nManaCost:0\nTypes:Creature Myr\nPT:1/1\nOracle:x\n")
	brudID := h.g.AddObject(brud, 0).ID
	tokID := h.g.AddObject(token, 0).ID
	h.g.Obj(tokID).IsToken = true
	h.g.SetZone(state.ZBattlefield, 0, []state.ObjID{brudID, tokID})
	h.g.Obj(brudID).Zone = state.ZBattlefield
	h.g.Obj(tokID).Zone = state.ZBattlefield

	body := cards.ResolveSVar(brud.Faces[0].SVars, "DBClone")
	if body == nil || body.Params["ChoiceOptional"] != "True" || body.Params["ExcludeChosen"] != "True" {
		t.Fatalf("precondition: Brudiclad DBClone must carry ChoiceOptional$/ExcludeChosen$ True: %+v", body.Params)
	}

	// First pass: the ask must be Min 0 and offer the token plus an explicit
	// decline option.
	h.askResult = true
	Resolve(h, &Ctx{Source: brudID, Controller: 0, SVars: brud.Faces[0].SVars}, body)
	d := h.lastAsk
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "clone_choice" {
		t.Fatalf("expected the Choices$ clone_choice ask, got %+v", d)
	}
	if d.Min != 0 {
		t.Fatalf("ChoiceOptional$ ask Min = %d, want 0 (a legal decline)", d.Min)
	}
	sawToken, sawDecline := false, false
	for _, o := range d.Options {
		switch o.Kind {
		case "decline":
			sawDecline = true
		case "permanent", "card":
			if o.Obj == tokID {
				sawToken = true
			}
		}
	}
	if !sawToken {
		t.Fatalf("the token is not offered in %+v", d.Options)
	}
	if !sawDecline {
		t.Fatalf("ChoiceOptional$ asks with no explicit decline option: %+v", d.Options)
	}

	// The answered decline re-enters with a zero pick.
	h2 := &fakeHost{g: state.NewGame(names(2))}
	brud2 := h2.g.AddObject(brud, 0).ID
	tok2 := h2.g.AddObject(token, 0).ID
	h2.g.Obj(tok2).IsToken = true
	h2.g.SetZone(state.ZBattlefield, 0, []state.ObjID{brud2, tok2})
	h2.g.Obj(brud2).Zone = state.ZBattlefield
	h2.g.Obj(tok2).Zone = state.ZBattlefield
	Resolve(h2, &Ctx{Source: brud2, Controller: 0, ClonePick: 0, ClonePickDone: true,
		SVars: brud.Faces[0].SVars}, body)

	if len(clonePermanentTargets(h2)) != 0 {
		t.Fatalf("an answered decline still copied: targets %v", clonePermanentTargets(h2))
	}
	for _, note := range cloneNoteTexts(h2) {
		if note == "Clone Choices$ answer named no object; no copy" {
			t.Fatal("a choice-optional DECLINE emitted the malformed-answer Note")
		}
	}
}

// TestCloneChoiceOptionalExcludesChosenFromBecome pins the second half of
// Brudiclad: when the pick IS made, ExcludeChosen$ True drops the chosen token
// from the become pool, so the chosen token does not copy itself while the
// other token copies it.
func TestCloneChoiceOptionalExcludesChosenFromBecome(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	brud, ok := reg.Lookup("Brudiclad, Telchor Engineer")
	if !ok || brud == nil {
		t.Fatal("missing corpus card Brudiclad, Telchor Engineer")
	}
	h := &fakeHost{g: state.NewGame(names(2))}
	tokenA := mkCard(t, "Name:Fixture Token Alpha\nManaCost:0\nTypes:Creature Myr\nPT:1/1\nOracle:x\n")
	tokenB := mkCard(t, "Name:Fixture Token Beta\nManaCost:0\nTypes:Creature Myr\nPT:1/1\nOracle:x\n")
	brudID := h.g.AddObject(brud, 0).ID
	a := h.g.AddObject(tokenA, 0).ID
	b := h.g.AddObject(tokenB, 0).ID
	h.g.Obj(a).IsToken, h.g.Obj(b).IsToken = true, true
	h.g.SetZone(state.ZBattlefield, 0, []state.ObjID{brudID, a, b})
	h.g.Obj(brudID).Zone = state.ZBattlefield
	h.g.Obj(a).Zone = state.ZBattlefield
	h.g.Obj(b).Zone = state.ZBattlefield

	body := cards.ResolveSVar(brud.Faces[0].SVars, "DBClone")
	if body == nil || body.Params["ExcludeChosen"] != "True" {
		t.Fatal("precondition: Brudiclad DBClone must carry ExcludeChosen$ True")
	}
	// Precondition: the become pool covers the chosen token itself, so without
	// ExcludeChosen$ a self-copy would occur.
	if !bodyBecomeContains(h, body, a) {
		t.Fatal("precondition: CloneTarget$ Valid Card.token+YouCtrl must include the chosen token")
	}

	Resolve(h, &Ctx{Source: brudID, Controller: 0, ClonePick: a, ClonePickDone: true,
		SVars: brud.Faces[0].SVars}, body)

	if n := clonePermanentCount(h, a); n != 0 {
		t.Fatalf("the chosen token received %d self-copy events; ExcludeChosen$ must drop it", n)
	}
	if n := clonePermanentCount(h, b); n != 1 {
		t.Fatalf("the other token received %d ClonePermanent events, want 1 (targets %v)",
			n, clonePermanentTargets(h))
	}
	if h.g.Obj(a).CopyFace != nil {
		t.Fatal("the chosen token copied itself")
	}
}

// TestCloneChoiceZoneExilePoolsTheExiledCards is the Lazav-style pin:
// Choices$ Creature.ExiledWithSource | ChoiceZone$ Exile must offer the cards
// exiled with the source, and must NOT offer a battlefield creature. A
// battlefield-only pool must not silently win.
func TestCloneChoiceZoneExilePoolsTheExiledCards(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	lazav, ok := reg.Lookup("Lazav, Wearer of Faces")
	if !ok || lazav == nil {
		t.Fatal("missing corpus card Lazav, Wearer of Faces")
	}
	h := &fakeHost{g: state.NewGame(names(2))}
	battle := mkCard(t, "Name:Fixture Bystander Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	exiled := mkCard(t, "Name:Fixture Exiled Ogre\nManaCost:2 R\nTypes:Creature Ogre\nPT:4/4\nOracle:x\n")
	lazID := h.g.AddObject(lazav, 0).ID
	battleID := h.g.AddObject(battle, 0).ID
	exileID := h.g.AddObject(exiled, 0).ID
	h.g.SetZone(state.ZBattlefield, 0, []state.ObjID{lazID, battleID})
	h.g.Obj(lazID).Zone = state.ZBattlefield
	h.g.Obj(battleID).Zone = state.ZBattlefield
	h.g.SetZone(state.ZExile, 0, []state.ObjID{exileID})
	h.g.Obj(exileID).Zone = state.ZExile
	// The exile association ExiledWithSource reads: the card was exiled by the
	// source (Lazav's TrigExile ChangeZone).
	h.g.Obj(exileID).ExiledWith = lazID

	body := cards.ResolveSVar(lazav.Faces[0].SVars, "TrigClone")
	if body == nil || body.Params["ChoiceZone"] != "Exile" || body.Params["Choices"] != "Creature.ExiledWithSource" {
		t.Fatalf("precondition: Lazav TrigClone shape %+v", body.Params)
	}
	if h.g.Obj(exileID).ExiledWith != lazID || h.g.Obj(battleID).ExiledWith != 0 {
		t.Fatal("precondition: exile association not set as intended")
	}

	h.askResult = true
	Resolve(h, &Ctx{Source: lazID, Controller: 0, SVars: lazav.Faces[0].SVars}, body)
	d := h.lastAsk
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "clone_choice" {
		t.Fatalf("expected the Choices$ clone_choice ask, got %+v", d)
	}
	sawExiled, sawBattlefield := false, false
	for _, o := range d.Options {
		switch o.Obj {
		case exileID:
			sawExiled = true
			if o.Kind != "card" {
				t.Fatalf("off-battlefield option kind = %q, want card", o.Kind)
			}
		case battleID:
			sawBattlefield = true
		}
	}
	if !sawExiled {
		t.Fatalf("the exiled card is not offered: %+v", d.Options)
	}
	if sawBattlefield {
		t.Fatalf("a battlefield creature was offered by the ChoiceZone$ Exile pool: %+v", d.Options)
	}
}

// TestCloneChoiceZoneUnsupportedFailsClosed pins the fail-closed landing for a
// ChoiceZone$ value this build does not implement: one loud Note, no copy,
// never a battlefield fall-through.
func TestCloneChoiceZoneUnsupportedFailsClosed(t *testing.T) {
	h := &fakeHost{g: state.NewGame(names(2))}
	bear := mkCard(t, "Name:Fixture Zone Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	id := h.g.AddObject(bear, 0).ID
	h.g.SetZone(state.ZBattlefield, 0, []state.ObjID{id})
	h.g.Obj(id).Zone = state.ZBattlefield
	body := sa(t, "DB$ Clone | Choices$ Creature | ChoiceZone$ Library")
	if body.Params["ChoiceZone"] != "Library" {
		t.Fatal("precondition: body carries the unsupported zone")
	}
	h.askResult = true
	Resolve(h, &Ctx{Source: id, Controller: 0}, body)
	if h.askCount != 0 {
		t.Fatal("an unsupported ChoiceZone$ still posed an ask")
	}
	if len(clonePermanentTargets(h)) != 0 {
		t.Fatal("an unsupported ChoiceZone$ still made a copy")
	}
	found := false
	for _, note := range cloneNoteTexts(h) {
		if note == "Clone ChoiceZone$ Library is not a zone this build can choose from; no copy" {
			found = true
		}
	}
	if !found {
		t.Fatalf("no fail-closed Note; notes: %v", cloneNoteTexts(h))
	}
}

// TestCloneKayaTriggeredCardsPoolFailsClosed pins the deliberate scoping: the
// `Card.TriggeredCards` filter head rides a trigger-Remembered referent this
// grammar cannot bind, so Kaya's exiled-card pool matches nothing and the walk
// records one loud Note and makes no copy -- never a battlefield fall-through.
func TestCloneKayaTriggeredCardsPoolFailsClosed(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	kaya, ok := reg.Lookup("Kaya, Spirits' Justice")
	if !ok || kaya == nil {
		t.Fatal("missing corpus card Kaya, Spirits' Justice")
	}
	h := &fakeHost{g: state.NewGame(names(2))}
	exiled := mkCard(t, "Name:Fixture Kaya Exile\nManaCost:1 B\nTypes:Creature Spirit\nPT:1/1\nOracle:x\n")
	tok := mkCard(t, "Name:Fixture Kaya Token\nManaCost:0\nTypes:Creature Spirit\nPT:1/1\nOracle:x\n")
	kayaID := h.g.AddObject(kaya, 0).ID
	tokID := h.g.AddObject(tok, 0).ID
	exileID := h.g.AddObject(exiled, 0).ID
	h.g.Obj(tokID).IsToken = true
	h.g.SetZone(state.ZBattlefield, 0, []state.ObjID{kayaID, tokID})
	h.g.Obj(kayaID).Zone = state.ZBattlefield
	h.g.Obj(tokID).Zone = state.ZBattlefield
	h.g.SetZone(state.ZExile, 0, []state.ObjID{exileID})
	h.g.Obj(exileID).Zone = state.ZExile
	h.g.Obj(exileID).ExiledWith = kayaID

	body := cards.ResolveSVar(kaya.Faces[0].SVars, "TrigCopy")
	if body == nil || body.Params["ChoiceZone"] != "Exile" || body.Params["Choices"] != "Card.TriggeredCards+Creature" {
		t.Fatalf("precondition: Kaya TrigCopy shape %+v", body.Params)
	}
	// Kaya's TrigCopy carries a mandatory ValidTgts$ (the target token), so the
	// test supplies that target: the ask this test counts must be the Choices$
	// pick, not the target offer that would otherwise come first.
	h.askResult = true
	Resolve(h, &Ctx{Source: kayaID, Controller: 0, Targets: []state.Target{{Obj: tokID}}, TargetsOffered: true, SVars: kaya.Faces[0].SVars}, body)
	if h.askCount != 0 {
		t.Fatal("the unresolvable TriggeredCards pool still posed an ask")
	}
	if len(clonePermanentTargets(h)) != 0 {
		t.Fatal("the unresolvable TriggeredCards pool still made a copy")
	}
	found := false
	for _, note := range cloneNoteTexts(h) {
		if note == "Clone Choices$ Card.TriggeredCards+Creature has no eligible object; no copy" {
			found = true
		}
	}
	if !found {
		t.Fatalf("no fail-closed Note; notes: %v", cloneNoteTexts(h))
	}
}
