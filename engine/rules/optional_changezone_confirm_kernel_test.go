package rules

// An explicit Optional$ hand fetch (hand_move_confirm) and an Optional$ Hidden$
// public-origin pick (hidden_pick_confirm) ask "proceed?" BEFORE any card is
// picked; a decline poses no pick and moves nothing, an accept enters the
// Min-0 pick (which may still take nothing), an empty eligible pool still
// confirms, and ChoiceOptional$ / Mandatory$ / a markerless text-may stay
// confirmation-free. Yuna's Decision is the real corpus carrier. (The no-host
// leaves stay in effects/optional_changezone_confirm_kernel_test.go.)

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

const (
	handOptionalLine   = "A:SP$ ChangeZone | Origin$ Hand | Destination$ Battlefield | ChangeType$ Land | ChangeNum$ 1 | Optional$ True"
	hiddenOptionalLine = "A:SP$ ChangeZone | Hidden$ True | Origin$ Graveyard | Destination$ Battlefield | ChangeType$ Creature | ChangeNum$ 1 | Optional$ True"
	isleKernelSrc      = "Name:Isle\nTypes:Basic Land Island\nOracle:x\n"
	bearKernelSrc      = "Name:Grizzly Bears\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
)

// handFetchBoard seats Bear, Isle, Isle in seat 0's hand and casts line.
func handFetchBoard(t *testing.T, line string) (*Engine, []state.ObjID, *decision.Decision) {
	t.Helper()
	e := kernelEngine(t, 2)
	ids := []state.ObjID{
		putCardIn(t, e, 0, cardOfSrc(t, bearKernelSrc), state.ZHand, false),
		putCardIn(t, e, 0, cardOfSrc(t, isleKernelSrc), state.ZHand, false),
		putCardIn(t, e, 0, cardOfSrc(t, isleKernelSrc), state.ZHand, false),
	}
	spell := putCardIn(t, e, 0, sorceryCard(t, "Fetcher", line), state.ZHand, false)
	return e, ids, castWithMana(t, e, 0, spell)
}

func requireConfirmAsk(t *testing.T, d *decision.Decision, resume string) {
	t.Helper()
	requireResumeAsk(t, d, resume)
	if d.Player != 0 || d.Kind != decision.KChoose || d.Min != 1 || d.Max != 1 ||
		len(d.Options) != 2 || d.Options[0].Kind != "yes" || d.Options[1].Kind != "no" {
		t.Fatalf("confirm = %+v, want a 1-of yes/no %s gate for seat 0", d, resume)
	}
}

func requireZones(t *testing.T, e *Engine, ids []state.ObjID, z state.Zone, what string) {
	t.Helper()
	for i, id := range ids {
		if got := e.G.Obj(id).Zone; got != z {
			t.Fatalf("%s: ids[%d] is on %s, want %s", what, i, got, z)
		}
	}
}

func TestOptionalChangeZoneHandConfirm(t *testing.T) {
	t.Parallel()
	t.Run("gate then decline", func(t *testing.T) {
		t.Parallel()
		e, ids, d := handFetchBoard(t, handOptionalLine)
		requireConfirmAsk(t, d, "hand_move_confirm")
		requireZones(t, e, ids, state.ZHand, "the confirmation itself")
		if d = answerDecision(t, e, d, optionKindIndex(t, d, "no")); d != nil {
			t.Fatalf("a declined confirmation posed %+v", d)
		}
		requireZones(t, e, ids, state.ZHand, "decline")
	})
	t.Run("accept then pick", func(t *testing.T) {
		t.Parallel()
		e, ids, d := handFetchBoard(t, handOptionalLine)
		d = requireResumeAsk(t, answerDecision(t, e, d, optionKindIndex(t, d, "yes")), "hand_move")
		if d.Min != 0 || d.Max != 1 || len(d.Options) != 2 || d.Options[0].Obj != ids[1] || d.Options[1].Obj != ids[2] {
			t.Fatalf("pick = %+v, want a Min 0 hand_move over the two Isles", d)
		}
		if d = answerDecision(t, e, d, objOptionIndex(t, d, ids[2])); d != nil {
			t.Fatalf("unexpected ask: %+v", d)
		}
		requireZones(t, e, ids[2:], state.ZBattlefield, "answered isle")
		requireZones(t, e, ids[:2], state.ZHand, "unchosen cards")
	})
	t.Run("accept then pick none", func(t *testing.T) {
		t.Parallel()
		e, ids, d := handFetchBoard(t, handOptionalLine)
		d = requireResumeAsk(t, answerDecision(t, e, d, optionKindIndex(t, d, "yes")), "hand_move")
		if d = answerDecision(t, e, d); d != nil {
			t.Fatalf("unexpected ask: %+v", d)
		}
		requireZones(t, e, ids, state.ZHand, "accept-then-none")
	})
	t.Run("markerless may stays confirmation-free", func(t *testing.T) {
		t.Parallel()
		_, _, d := handFetchBoard(t, "A:SP$ ChangeZone | Origin$ Hand | Destination$ Battlefield | ChangeType$ Land | ChangeNum$ 1 | SpellDescription$ You may put a land card from your hand onto the battlefield.")
		if requireResumeAsk(t, d, "hand_move"); d.Min != 0 || d.Max != 1 {
			t.Fatalf("markerless first ask = %+v, want the Min 0 hand_move pick", d)
		}
	})
}

func TestYunasDecisionFetchConfirmsThenOffersEachMember(t *testing.T) {
	t.Parallel()
	e := kernelEngine(t, 2)
	fodder := putCardIn(t, e, 0, cardOfSrc(t, "Name:Fodder\n"+creatureKernelSrc), state.ZBattlefield, false)
	bear := putCardIn(t, e, 0, cardOfSrc(t, bearKernelSrc), state.ZHand, false)
	isle := putCardIn(t, e, 0, cardOfSrc(t, isleKernelSrc), state.ZHand, false)
	yuna := putCardIn(t, e, 0, corpusFixtureCard(t, "Yuna's Decision"), state.ZHand, false)
	d := castWithMana(t, e, 0, yuna)
	for i := 0; d != nil && d.ResumeKind != "hand_move_confirm" && i < 6; i++ {
		switch {
		case d.Kind == decision.KModes:
			d = answerDecision(t, e, d, d.Options[0].Index) // Continue the Pilgrimage
		case d.ResumeKind == "sacrifice" || d.Kind == decision.KChoose:
			d = answerDecision(t, e, d, objOptionIndex(t, d, fodder))
		default:
			t.Fatalf("unexpected decision %+v", d)
		}
	}
	requireConfirmAsk(t, d, "hand_move_confirm")
	if z := e.G.Obj(fodder).Zone; z != state.ZGraveyard {
		t.Fatalf("the sacrificed creature is on %s, want graveyard", z)
	}
	d = requireResumeAsk(t, answerDecision(t, e, d, optionKindIndex(t, d, "yes")), "hand_move")
	objOptionIndex(t, d, bear)
	objOptionIndex(t, d, isle)
}

// hiddenFetchBoard seats Bear and Isle in seat 0's graveyard (just Isle when
// noCreature) and casts line.
func hiddenFetchBoard(t *testing.T, line string, noCreature bool) (*Engine, state.ObjID, state.ObjID, *decision.Decision) {
	t.Helper()
	e := kernelEngine(t, 2)
	var bear state.ObjID
	if !noCreature {
		bear = putCardIn(t, e, 0, cardOfSrc(t, bearKernelSrc), state.ZGraveyard, false)
	}
	isle := putCardIn(t, e, 0, cardOfSrc(t, isleKernelSrc), state.ZGraveyard, false)
	spell := putCardIn(t, e, 0, sorceryCard(t, "Reanimator", line), state.ZHand, false)
	return e, bear, isle, castWithMana(t, e, 0, spell)
}

func TestOptionalChangeZoneHiddenPickConfirm(t *testing.T) {
	t.Parallel()
	t.Run("gate then decline", func(t *testing.T) {
		t.Parallel()
		e, bear, _, d := hiddenFetchBoard(t, hiddenOptionalLine, false)
		requireConfirmAsk(t, d, "hidden_pick_confirm")
		requireZones(t, e, []state.ObjID{bear}, state.ZGraveyard, "the confirmation itself")
		if d = answerDecision(t, e, d, optionKindIndex(t, d, "no")); d != nil {
			t.Fatalf("a declined confirmation posed %+v", d)
		}
		requireZones(t, e, []state.ObjID{bear}, state.ZGraveyard, "decline")
	})
	t.Run("accept picks the eligible card", func(t *testing.T) {
		t.Parallel()
		e, bear, isle, d := hiddenFetchBoard(t, hiddenOptionalLine, false)
		d = requireResumeAsk(t, answerDecision(t, e, d, optionKindIndex(t, d, "yes")), "hidden_pick")
		if d.Min != 0 || d.Max != 1 || len(d.Options) != 1 || d.Options[0].Obj != bear {
			t.Fatalf("pick = %+v, want a Min 0 hidden_pick whose only option is the bear", d)
		}
		if d = answerDecision(t, e, d, 0); d != nil {
			t.Fatalf("unexpected ask: %+v", d)
		}
		requireZones(t, e, []state.ObjID{bear}, state.ZBattlefield, "answered creature")
		requireZones(t, e, []state.ObjID{isle}, state.ZGraveyard, "the land")
	})
	t.Run("accept then pick none", func(t *testing.T) {
		t.Parallel()
		e, bear, _, d := hiddenFetchBoard(t, hiddenOptionalLine, false)
		d = requireResumeAsk(t, answerDecision(t, e, d, optionKindIndex(t, d, "yes")), "hidden_pick")
		if d = answerDecision(t, e, d); d != nil {
			t.Fatalf("unexpected ask: %+v", d)
		}
		requireZones(t, e, []state.ObjID{bear}, state.ZGraveyard, "accept-then-none")
	})
	for _, accept := range []bool{false, true} {
		name := "empty pool still confirms, decline"
		if accept {
			name = "empty pool still confirms, accept"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			e, _, isle, d := hiddenFetchBoard(t, hiddenOptionalLine, true)
			requireConfirmAsk(t, d, "hidden_pick_confirm")
			choice := optionKindIndex(t, d, "no")
			if accept {
				choice = optionKindIndex(t, d, "yes")
			}
			if d = answerDecision(t, e, d, choice); d != nil {
				t.Fatalf("an empty-pool fetch posed %+v after the confirmation", d)
			}
			requireZones(t, e, []state.ObjID{isle}, state.ZGraveyard, "the land")
		})
	}
	t.Run("ChoiceOptional$ does not confirm", func(t *testing.T) {
		t.Parallel()
		_, _, _, d := hiddenFetchBoard(t, "A:SP$ ChangeZone | Hidden$ True | Origin$ Graveyard | Destination$ Battlefield | ChangeType$ Creature | ChangeNum$ 1 | ChoiceOptional$ True", false)
		if requireResumeAsk(t, d, "hidden_pick"); d.Min != 0 || d.Max != 1 {
			t.Fatalf("ChoiceOptional$ first ask = %+v, want the Min 0 hidden_pick", d)
		}
	})
	t.Run("Mandatory$ does not confirm", func(t *testing.T) {
		t.Parallel()
		_, _, _, d := hiddenFetchBoard(t, "A:SP$ ChangeZone | Hidden$ True | Origin$ Graveyard | Destination$ Battlefield | ChangeType$ Creature | ChangeNum$ 1 | Mandatory$ True", false)
		if requireResumeAsk(t, d, "hidden_pick"); d.Min != 1 || d.Max != 1 {
			t.Fatalf("Mandatory$ first ask = %+v, want the Min 1 hidden_pick", d)
		}
	})
}
