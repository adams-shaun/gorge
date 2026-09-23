// The remaining Attached replacement elections: the ChooseCard body
// (Paleontologist's Pick-Axe / Dinosaur Headdress, "choose an exiled creature
// card used to craft CARDNAME") and the ChooseColor body (Sanctuary Blade,
// "choose a color"). Both are driven end to end on the real corpus cards:
// equip the permanent, answer the replacement's ask, and assert the answer
// governs the event-backed chosen-card / chosen-color state.
package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// TestAttachedReplacementChooseCardRecordsCraftCard drives Dinosaur Headdress
// (the transformed face of Paleontologist's Pick-Axe): as it becomes attached
// it must offer the exiled creature cards associated with it and record the
// answered one on the source as the event-backed chosen card the equipped
// creature's Clone static reads.
func TestAttachedReplacementChooseCardRecordsCraftCard(t *testing.T) {
	t.Parallel()
	pickaxe := corpusAlternativeCard(t, "Paleontologist's Pick-Axe")
	bearer := card(t, "Name:Grizzly Bears\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	alpha := card(t, "Name:Craft Fodder Alpha\nManaCost:G\nTypes:Creature Beast\nPT:1/1\nOracle:x\n")
	beta := card(t, "Name:Craft Fodder Beta\nManaCost:G\nTypes:Creature Beast\nPT:1/1\nOracle:x\n")
	e, cfg, _ := corpusDeckEngine(t, nil, []*cards.Card{pickaxe, bearer, alpha, beta})

	var pickaxeID, bearerID, alphaID, betaID state.ObjID
	for i := range e.G.Objs {
		o := &e.G.Objs[i]
		switch o.Card {
		case pickaxe:
			pickaxeID = o.ID
		case bearer:
			bearerID = o.ID
		case alpha:
			alphaID = o.ID
		case beta:
			betaID = o.ID
		}
	}
	// PRECONDITION: the Equipment, its intended bearer and both craft-fodder
	// creatures are really on the battlefield. A vacuous setup must fail
	// loudly, not drive an ask over an empty pool.
	if pickaxeID == 0 || bearerID == 0 || alphaID == 0 || betaID == 0 {
		t.Fatalf("setup: pickaxe=%d bearer=%d alpha=%d beta=%d (all must be on the battlefield)",
			pickaxeID, bearerID, alphaID, betaID)
	}

	// Flip to Dinosaur Headdress -- the face whose Attached replacement is the
	// exiled-craft-card pick. The active face is the precondition the matcher
	// reads; a wrong face would pose no ask at all.
	e.emit(events.Event{Kind: events.FlipFace, Obj: pickaxeID, Amount: 1})
	if got := e.Name(pickaxeID); got != "Dinosaur Headdress" {
		t.Fatalf("precondition: active face name = %q, want Dinosaur Headdress", got)
	}

	// Exile both craft creatures and record the source's own exiled-with
	// association (Forge's hostCard.getExiledCards), which is the pool
	// `DefinedCards$ ExiledWith` names. Insertion order is deterministic, so
	// the offered list is Alpha then Beta.
	for _, id := range []state.ObjID{alphaID, betaID} {
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZBattlefield, To: state.ZExile})
		e.emit(events.Event{Kind: events.Imprint, Obj: pickaxeID, IDs: []state.ObjID{id}, Text: "exiled-with"})
	}
	src := e.G.Obj(pickaxeID)
	if len(src.ExiledCards) != 2 || src.ExiledCards[0] != alphaID || src.ExiledCards[1] != betaID {
		t.Fatalf("precondition: ExiledCards = %v, want [%d %d]", src.ExiledCards, alphaID, betaID)
	}
	for _, id := range []state.ObjID{alphaID, betaID} {
		if o := e.G.Obj(id); o == nil || o.Zone != state.ZExile {
			t.Fatalf("precondition: craft fodder %d is not in exile: %+v", id, o)
		}
	}

	addMana(t, e, 0, "GG")
	opt := abilityOptionByLabel(t, e, pickaxeID, "Equip")
	submitChoices(t, e, opt.Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("equip target decision: %+v", d)
	}
	idx := indexOfObjOption(d, bearerID)
	if idx < 0 {
		t.Fatalf("equip target ask does not offer the bearer: %+v", d.Options)
	}
	submitChoices(t, e, idx)

	// The Attached replacement must now pose the exiled-card pick.
	d = passUntilAskKind(t, e, decision.KChoose, 30)
	if d.Prompt != "Choose an exiled card" {
		t.Fatalf("expected the attached card ask, got %+v", d)
	}
	alphaIdx := indexOfObjOption(d, alphaID)
	betaIdx := indexOfObjOption(d, betaID)
	if alphaIdx < 0 || betaIdx < 0 {
		t.Fatalf("card ask must offer BOTH exiled craft cards (alpha=%d beta=%d): %+v", alphaID, betaID, d.Options)
	}
	if alphaIdx == betaIdx {
		t.Fatalf("card ask offered the same slot for Alpha and Beta: %+v", d.Options)
	}
	submitChoices(t, e, betaIdx)
	passUntilStackEmpty(t, e, 20)

	src = e.G.Obj(pickaxeID)
	if src.AttachedTo != bearerID {
		t.Fatalf("pickaxe attached to %d, want bearer %d (the parked Attach must still complete)", src.AttachedTo, bearerID)
	}
	// The ANSWER must govern: the deterministic first offered option is Alpha,
	// so recording Beta proves the election drove the state rather than a
	// silent pick.
	if len(src.Chosen) != 1 || src.Chosen[0].IsPlayer || src.Chosen[0].Obj != betaID {
		t.Fatalf("recorded chosen card = %+v, want exactly the answered Beta %d", src.Chosen, betaID)
	}
	replayCheck(t, e, cfg)
}

// TestAttachedReplacementChooseColorRecordsColor drives Sanctuary Blade: as it
// becomes attached it must pose the real colour ask and record the answered
// colour's WUBRG letter on the source, which the equipped creature's
// protection-from-chosen-colour static reads.
func TestAttachedReplacementChooseColorRecordsColor(t *testing.T) {
	t.Parallel()
	blade := corpusAlternativeCard(t, "Sanctuary Blade")
	bearer := card(t, "Name:Grizzly Bears\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	e, cfg, _ := corpusDeckEngine(t, nil, []*cards.Card{blade, bearer})

	var bladeID, bearerID state.ObjID
	for i := range e.G.Objs {
		o := &e.G.Objs[i]
		switch o.Card {
		case blade:
			bladeID = o.ID
		case bearer:
			bearerID = o.ID
		}
	}
	// PRECONDITION: the Equipment and its bearer are on the battlefield, the
	// blade is unattached, and no colour has been chosen yet.
	if bladeID == 0 || bearerID == 0 {
		t.Fatalf("setup: blade=%d bearer=%d (both must be on the battlefield)", bladeID, bearerID)
	}
	if o := e.G.Obj(bladeID); o.AttachedTo != 0 || o.ChosenColor != "" {
		t.Fatalf("precondition: blade attached=%d chosenColor=%q, want unattached and no colour", o.AttachedTo, o.ChosenColor)
	}

	addMana(t, e, 0, "GGG")
	opt := abilityOptionByLabel(t, e, bladeID, "Equip")
	submitChoices(t, e, opt.Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("equip target decision: %+v", d)
	}
	idx := indexOfObjOption(d, bearerID)
	if idx < 0 {
		t.Fatalf("equip target ask does not offer the bearer: %+v", d.Options)
	}
	submitChoices(t, e, idx)

	// The Attached replacement must pose the real colour ask.
	d = passUntilAskKind(t, e, decision.KChoose, 30)
	if d.Prompt != "Choose a color" {
		t.Fatalf("expected the attached colour ask, got %+v", d)
	}
	greenIdx := optionByLabel(d.Options, "Green")
	whiteIdx := optionByLabel(d.Options, "White")
	if greenIdx < 0 || whiteIdx < 0 {
		t.Fatalf("colour ask must offer Green and White: %+v", d.Options)
	}
	if greenIdx == whiteIdx {
		t.Fatalf("colour ask offered the same slot for Green and White: %+v", d.Options)
	}
	submitChoices(t, e, greenIdx)
	passUntilStackEmpty(t, e, 20)

	o := e.G.Obj(bladeID)
	if o.AttachedTo != bearerID {
		t.Fatalf("blade attached to %d, want bearer %d (the parked Attach must still complete)", o.AttachedTo, bearerID)
	}
	// The ANSWER must govern: the deterministic first-WUBRG fallback is "W",
	// so recording "G" proves the election drove the state.
	if o.ChosenColor != "G" {
		t.Fatalf("recorded chosen colour = %q, want the answered Green G", o.ChosenColor)
	}
	replayCheck(t, e, cfg)
}
