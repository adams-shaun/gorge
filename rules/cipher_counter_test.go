package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// Cipher's encode is a reflexive triggered ability of the spell's RESOLUTION
// (CR 702.99a: "Then you may exile this spell card encoded ..."). CR 702.99c
// settles the countering case: "If the spell is countered, the Cipher ability
// ... does not apply." The expansion (cards/kw_cipher.go) therefore fires only
// on the engine's own resolution move off the stack -- the one stack exit with
// an EMPTY Text. Every counter/fizzle/reversal path tags its MoveZone
// ("countered", "countered: no legal targets", "fizzled: no legal targets
// remain", "reversed"), so ResolvedOnly$ True (rules/trigmatch_zone.go) gates
// the trigger against all of them. These tests pin that gate: the SAME board
// that poses the ask on a resolution poses nothing on a countered move.

// drainCipherEncodeAsk drives the stack and reports whether a Cipher encode
// ask (KModes with ResumeKind "cipher") was posed at all. It answers every ask
// with option 0 / pass so the stack drains. Unlike drainCipher it does not
// require an encode ask, so it can prove the absent case.
func drainCipherEncodeAsk(t *testing.T, e *Engine, limit int) bool {
	t.Helper()
	sawEncode := false
	for i := 0; i < limit && !e.G.Over && len(e.G.Stack) > 0; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatalf("no decision while draining the cipher stack (depth %d)", len(e.G.Stack))
		}
		if d.Kind == decision.KModes && d.ResumeKind == "cipher" {
			sawEncode = true
		}
		idx := 0
		if d.Kind == decision.KPriority {
			if p, ok := cipherFindOption(d, "pass"); ok {
				idx = p
			}
		}
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{idx}}); err != nil {
			t.Fatalf("submit %v: %v", d.Kind, err)
		}
	}
	if !e.G.Over && len(e.G.Stack) > 0 {
		t.Fatalf("cipher stack never emptied (depth %d)", len(e.G.Stack))
	}
	return sawEncode
}

// TestCipherCounteredSpellDoesNotEncode: a countered Cipher spell moves
// stack->graveyard with the engine's "countered" marker (the exact event
// effCounter emits, effects/misc.go). The encode must NOT be offered, the card
// must stay in the graveyard, and no creature may carry the association.
//
// The test proves its own setup reaches the rule: a companion probe first
// shows the SAME board with a resolution move DOES pose the encode ask, so the
// negative assertion cannot pass because the feature was never wired.
func TestCipherCounteredSpellDoesNotEncode(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	probe := card(t, cipherProbeSrc)
	if !cipherHasExpanderTrigger(probe) {
		t.Fatal("precondition: Cipher keyword did not expand")
	}

	// Control: the resolution move (empty Text) poses the encode ask.
	{
		e, _, cipherID, creatures := cipherBoard(t, reg, probe, cipherBeanSrc)
		if o := e.G.Obj(creatures[0]); o == nil || o.Zone != state.ZBattlefield {
			t.Fatal("precondition: eligible creature missing")
		}
		e.emit(events.Event{Kind: events.PutOnStack, Obj: cipherID, Player: 0,
			From: state.ZLibrary, To: state.ZStack, Text: probe.Faces[0].Name})
		e.emit(events.Event{Kind: events.MoveZone, Obj: cipherID,
			From: state.ZStack, To: state.ZGraveyard})
		e.priorityRound()
		if !drainCipherEncodeAsk(t, e, 40) {
			t.Fatal("control: a RESOLVED Cipher spell posed no encode ask, so the counter case below cannot prove anything")
		}
	}

	// Countered: the engine's countered move carries Text "countered".
	e, cfg, cipherID, creatures := cipherBoard(t, reg, probe, cipherBeanSrc)
	creature := creatures[0]
	if o := e.G.Obj(creature); o == nil || o.Zone != state.ZBattlefield {
		t.Fatal("precondition: eligible creature missing")
	}
	e.emit(events.Event{Kind: events.PutOnStack, Obj: cipherID, Player: 0,
		From: state.ZLibrary, To: state.ZStack, Text: probe.Faces[0].Name})
	e.emit(events.Event{Kind: events.MoveZone, Obj: cipherID,
		From: state.ZStack, To: state.ZGraveyard, Text: "countered"})
	e.priorityRound()
	if drainCipherEncodeAsk(t, e, 40) {
		t.Fatal("a COUNTERED Cipher spell posed the encode ask (CR 702.99c forbids it)")
	}
	if co := e.G.Obj(cipherID); co == nil || co.Zone != state.ZGraveyard {
		t.Fatalf("countered Cipher card zone = %v, want graveyard", zoneOf(co))
	}
	if got := cipherEncodedOn(e, creature); len(got) != 0 {
		t.Fatalf("countered Cipher spell still encoded %v", got)
	}
	replayCheck(t, e, cfg)
}

// TestCipherFizzledSpellDoesNotEncode: a spell that fizzles for lack of a
// legal target never resolves (CR 608.2b), so its encode must not apply
// either. The engine tags that exit "fizzled: no legal targets remain". This
// is the fizzle sibling of the countered case above.
func TestCipherFizzledSpellDoesNotEncode(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	probe := card(t, cipherProbeSrc)
	if !cipherHasExpanderTrigger(probe) {
		t.Fatal("precondition: Cipher keyword did not expand")
	}
	e, cfg, cipherID, creatures := cipherBoard(t, reg, probe, cipherBeanSrc)
	creature := creatures[0]
	if o := e.G.Obj(creature); o == nil || o.Zone != state.ZBattlefield {
		t.Fatal("precondition: eligible creature missing")
	}
	e.emit(events.Event{Kind: events.PutOnStack, Obj: cipherID, Player: 0,
		From: state.ZLibrary, To: state.ZStack, Text: probe.Faces[0].Name})
	e.emit(events.Event{Kind: events.MoveZone, Obj: cipherID,
		From: state.ZStack, To: state.ZGraveyard, Text: "fizzled: no legal targets remain"})
	e.priorityRound()
	if drainCipherEncodeAsk(t, e, 40) {
		t.Fatal("a FIZZLED Cipher spell posed the encode ask (it never resolved)")
	}
	if co := e.G.Obj(cipherID); co == nil || co.Zone != state.ZGraveyard {
		t.Fatalf("fizzled Cipher card zone = %v, want graveyard", zoneOf(co))
	}
	if got := cipherEncodedOn(e, creature); len(got) != 0 {
		t.Fatalf("fizzled Cipher spell still encoded %v", got)
	}
	replayCheck(t, e, cfg)
}
