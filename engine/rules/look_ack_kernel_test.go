package rules

// A bare private look
// (PeekAndReveal NoReveal$, RevealHand Look$) gates on a look_ack posed to
// the LOOKER; the answer emits exactly one Secret look Note scoped to the
// looker, poses no second ask, and the chained sub runs after it. A
// Look$+Optional$ shape keeps the reveal_optional ask alone: the "yes" is
// the consent, with no second look_ack gate. (The multi-target cursor leaf
// is TestCaseTheJointLookAcksEachTargetOnce.)

import (
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

func TestBareNoRevealLookAckAnsweredEmitsExactlyOnce(t *testing.T) {
	t.Parallel()
	e := kernelEngine(t, 2)
	top := putCardIn(t, e, 0, cardOfSrc(t, "Name:Peeked\nTypes:Sorcery\nOracle:x\n"), state.ZLibrary, true)
	spell := putCardIn(t, e, 0, sorceryCard(t, "Bare Peek",
		"A:SP$ PeekAndReveal | Defined$ You | NoReveal$ True | SubAbility$ DBPing",
		"SVar:DBPing:DB$ LoseLife | Defined$ You | LifeAmount$ 1"), state.ZHand, false)
	from := len(e.L.Events)
	d := requireResumeAsk(t, castWithMana(t, e, 0, spell), "look_ack")
	if d.Player != 0 || d.Kind != decision.KChoose {
		t.Fatalf("ack = %+v, want a look_ack KChoose for seat 0", d)
	}
	if d = answerDecision(t, e, d, 0); d != nil {
		t.Fatalf("the answered ack posed a second ask: %+v", d)
	}
	notes := secretLooksFrom(e, from)
	if len(notes) != 1 {
		t.Fatalf("got %d Secret look Notes (%+v), want exactly one", len(notes), notes)
	}
	if n := notes[0]; n.Player != 0 || n.From != state.ZLibrary || !slices.Equal(n.IDs, []state.ObjID{top}) {
		t.Fatalf("look Note = %+v, want a Secret library look for seat 0 carrying %d", n, top)
	}
	losses := 0
	for _, ev := range eventsOfKindFrom(e, from, events.LifeChange) {
		if ev.Player == 0 && ev.Amount < 0 {
			losses++
		}
	}
	if losses != 1 {
		t.Fatalf("%d chained-sub life losses, want exactly 1", losses)
	}
}

func TestMandatoryLookAcksTheLooker(t *testing.T) {
	t.Parallel()
	e := kernelEngine(t, 2)
	var hand []state.ObjID
	for _, n := range []string{"Bolt", "Bear", "Wrenn", "Snares"} {
		hand = append(hand, putCardIn(t, e, 1, cardOfSrc(t, "Name:"+n+"\nTypes:Sorcery\nOracle:x\n"), state.ZHand, false))
	}
	spell := putCardIn(t, e, 0, sorceryCard(t, "Probe", "A:SP$ RevealHand | ValidTgts$ Player | Look$ True"), state.ZHand, false)
	from := len(e.L.Events)
	d := castWithMana(t, e, 0, spell)
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("cast = %+v, want the player target ask", d)
	}
	d = requireResumeAsk(t, answerDecision(t, e, d, playerOptionIndex(t, d, 1)), "look_ack")
	if d.Player != 0 || d.Kind != decision.KChoose {
		t.Fatalf("ack = %+v, want a look_ack KChoose for the looker (seat 0), never the looked-at seat", d)
	}
	if !strings.Contains(d.Prompt, "hand") || !strings.Contains(d.Prompt, "Bear") {
		t.Fatalf("prompt = %q, want it to name the hand and its cards", d.Prompt)
	}
	if n := secretLooksFrom(e, from); len(n) != 0 {
		t.Fatalf("a look Note landed before the ack: %+v", n)
	}
	if d = answerDecision(t, e, d, 0); d != nil {
		t.Fatalf("the answered ack posed a second ask: %+v", d)
	}
	notes := secretLooksFrom(e, from)
	if len(notes) != 1 {
		t.Fatalf("got %d Secret look Notes (%+v), want exactly one", len(notes), notes)
	}
	if n := notes[0]; n.Player != 0 || n.From != state.ZHand || !slices.Equal(n.IDs, hand) {
		t.Fatalf("look Note = %+v, want a Secret whole-hand look for seat 0 over %v", n, hand)
	}
}

func TestLookOptionalConsentCoversTheFollowThrough(t *testing.T) {
	t.Parallel()
	e := kernelEngine(t, 2)
	putCardIn(t, e, 0, cardOfSrc(t, "Name:Bear\nManaCost:9\nTypes:Creature\nPT:2/2\nOracle:x\n"), state.ZHand, false)
	spell := putCardIn(t, e, 0, sorceryCard(t, "May Look", "A:SP$ RevealHand | Defined$ You | Look$ True | Optional$ True"), state.ZHand, false)
	from := len(e.L.Events)
	d := requireResumeAsk(t, castWithMana(t, e, 0, spell), "reveal_optional")
	if d = answerDecision(t, e, d, optionKindIndex(t, d, "yes")); d != nil {
		t.Fatalf("the consented look posed a second gate: %+v", d)
	}
	if n := secretLooksFrom(e, from); len(n) != 1 {
		t.Fatalf("the consented look emitted %+v, want exactly one Secret look note", n)
	}
}
