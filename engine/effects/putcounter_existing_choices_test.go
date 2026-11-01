package effects

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// CounterType$ ExistingCounter + Choices$ names the
// SOURCE of the counter kind, never the recipient, and never a literal
// counter called ExistingCounter.

const (
	// Aven Courier's TrigPutCounter, as the corpus writes it.
	avenCourierLine = "DB$ PutCounter | ValidTgts$ Permanent.YouCtrl | TgtPrompt$ Select target permanent you control | CounterType$ ExistingCounter | Choices$ Permanent.YouCtrl+HasCounters | PutOnDefined$ Targeted | OnlyNewKind$ True"
	// Contractual Safeguard's DBPutCounter, as the corpus writes it.
	safeguardLine = "DB$ PutCounter | CounterType$ ExistingCounter | Choices$ Creature.YouCtrl+HasCounters | PutOnEachOther$ Creature.YouCtrl"
)

// kindTapeHost serves one fixed option index for every ask (-1: unserved).
type kindTapeHost struct {
	*fakeHost
	answer int
	asked  *decision.Decision
}

func (h *kindTapeHost) AskCount() uint64 { return 0 }

func (h *kindTapeHost) TapeAnswer(d *decision.Decision) (decision.Intent, bool) {
	h.asked = d
	if h.answer < 0 || h.answer >= len(d.Options) {
		return decision.Intent{}, false
	}
	return decision.Intent{Choices: []int{h.answer}}, true
}

// existingOnBattlefield adds a creature for seat p and moves it onto the
// battlefield zone list (the enumeration a Choices$ walk reads).
func existingOnBattlefield(t *testing.T, h *fakeHost, p state.PlayerID) state.ObjID {
	t.Helper()
	o := h.g.AddObject(mkCard(t, "Name:PutTee\nTypes:Creature\nPT:1/1\nOracle:x\n"), p)
	h.Emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: state.ZLibrary, To: state.ZBattlefield})
	return o.ID
}

func existingBoard(t *testing.T, n int) (*fakeHost, []state.ObjID) {
	t.Helper()
	h := newHost(t, 2)
	ids := make([]state.ObjID, n)
	for i := range ids {
		ids[i] = existingOnBattlefield(t, h, 0)
		if o := h.g.Obj(ids[i]); o.Zone != state.ZBattlefield || o.Controller != 0 {
			t.Fatalf("precondition: object %d zone %s controller %d", ids[i], o.Zone, o.Controller)
		}
	}
	return h, ids
}

func assertNoExistingLiteral(t *testing.T, h *fakeHost, ids []state.ObjID) {
	t.Helper()
	for _, id := range ids {
		for _, c := range h.g.Obj(id).Counters {
			if strings.EqualFold(c.Kind, "ExistingCounter") {
				t.Fatalf("object %d carries a literal ExistingCounter: %+v", id, h.g.Obj(id).Counters)
			}
		}
	}
	for _, ev := range h.log {
		if strings.Contains(ev.Text, "unimplemented") || strings.Contains(ev.Text, "OnlyNewKind") ||
			strings.Contains(ev.Text, "PutOn") {
			t.Fatalf("ability noted: %q", ev.Text)
		}
	}
}

func TestPutCounterExistingChoicesAvenCourierCopiesTheKind(t *testing.T) {
	h, ids := existingBoard(t, 2)
	src, tgt := ids[0], ids[1]
	h.g.Obj(src).Counters = []state.Counter{{Kind: "P1P1", N: 1}}
	if h.g.Obj(tgt).Counter("P1P1") != 0 {
		t.Fatal("precondition: target already has P1P1")
	}
	s := sa(t, avenCourierLine)
	if u := PutCounterOf(s).Unread; len(u) != 0 {
		t.Fatalf("Unread = %v, want none", u)
	}

	Resolve(h, existingCtx(src, state.Target{Obj: tgt}), s)

	if got := h.g.Obj(tgt).Counter("P1P1"); got != 1 {
		t.Fatalf("target P1P1 = %d, want 1; counters %+v", got, h.g.Obj(tgt).Counters)
	}
	if got := h.g.Obj(src).Counter("P1P1"); got != 1 {
		t.Fatalf("source P1P1 = %d, want unchanged 1", got)
	}
	assertNoExistingLiteral(t, h, ids)
}

func TestPutCounterExistingChoicesOnlyNewKindSkipsACarrier(t *testing.T) {
	h, ids := existingBoard(t, 2)
	src, tgt := ids[0], ids[1]
	h.g.Obj(src).Counters = []state.Counter{{Kind: "P1P1", N: 1}}
	h.g.Obj(tgt).Counters = []state.Counter{{Kind: "P1P1", N: 2}}
	if h.g.Obj(tgt).Counter("P1P1") != 2 {
		t.Fatal("precondition: target lacks P1P1")
	}

	Resolve(h, existingCtx(src, state.Target{Obj: tgt}), sa(t, avenCourierLine))

	if got := h.g.Obj(tgt).Counter("P1P1"); got != 2 {
		t.Fatalf("target P1P1 = %d, want 2 (OnlyNewKind$ skips a carrier)", got)
	}
	if n := counterChangeCount(h, 0); n != 0 {
		t.Fatalf("CounterChange events = %d, want 0", n)
	}
	assertNoExistingLiteral(t, h, ids)
}

func TestPutCounterExistingChoicesAsksTheKindAcrossTheSourceSet(t *testing.T) {
	h, ids := existingBoard(t, 3)
	src1, src2, tgt := ids[0], ids[1], ids[2]
	h.g.Obj(src1).Counters = []state.Counter{{Kind: "P1P1", N: 1}}
	h.g.Obj(src2).Counters = []state.Counter{{Kind: "CHARGE", N: 1}}
	th := &kindTapeHost{fakeHost: h, answer: 1}

	Resolve(th, existingCtx(src1, state.Target{Obj: tgt}), sa(t, avenCourierLine))

	d := th.asked
	if d == nil || d.ResumeKind != "counter_kind" || d.Min != 1 || d.Max != 1 {
		t.Fatalf("ask = %+v, want a counter_kind 1..1", d)
	}
	var labels []string
	for _, o := range d.Options {
		labels = append(labels, o.Label)
	}
	if strings.Join(labels, ",") != "P1P1,CHARGE" {
		t.Fatalf("offered kinds = %v, want P1P1,CHARGE", labels)
	}
	if got := h.g.Obj(tgt).Counter("CHARGE"); got != 1 {
		t.Fatalf("target CHARGE = %d, want 1 (the answered kind)", got)
	}
	if got := h.g.Obj(tgt).Counter("P1P1"); got != 0 {
		t.Fatalf("target P1P1 = %d, want 0", got)
	}
	assertNoExistingLiteral(t, h, ids)

	// The bot's answer (first Max options, botpolicy's counter_kind arm)
	// must be a legal answer to the same decision.
	in := decision.Intent{Choices: []int{d.Options[0].Index}}
	if err := d.Validate(in); err != nil {
		t.Fatalf("bot answer rejected by Validate: %v", err)
	}
}

func TestPutCounterExistingChoicesSafeguardSpreadsToTheOthers(t *testing.T) {
	h, ids := existingBoard(t, 3)
	h.g.Obj(ids[1]).Counters = []state.Counter{{Kind: "CHARGE", N: 1}}
	opp := h.g.Obj(existingOnBattlefield(t, h, 1))
	if opp.Zone != state.ZBattlefield || opp.Controller != 1 {
		t.Fatalf("precondition: opponent creature zone %s controller %d", opp.Zone, opp.Controller)
	}

	Resolve(h, &Ctx{Source: ids[0], Controller: 0}, sa(t, safeguardLine))

	for _, i := range []int{0, 2} {
		if got := h.g.Obj(ids[i]).Counter("CHARGE"); got != 1 {
			t.Fatalf("creature %d CHARGE = %d, want 1", i, got)
		}
	}
	if got := h.g.Obj(ids[1]).Counter("CHARGE"); got != 1 {
		t.Fatalf("carrier CHARGE = %d, want unchanged 1", got)
	}
	if got := opp.Counter("CHARGE"); got != 0 {
		t.Fatalf("opponent's creature took CHARGE = %d", got)
	}
	assertNoExistingLiteral(t, h, append(ids, opp.ID))
}

func TestPutCounterExistingChoicesNoCountersPlacesNothing(t *testing.T) {
	h, ids := existingBoard(t, 3)
	for _, id := range ids {
		if len(h.g.Obj(id).Counters) != 0 {
			t.Fatal("precondition: counters present")
		}
	}
	Resolve(h, &Ctx{Source: ids[0], Controller: 0}, sa(t, safeguardLine))
	for _, id := range ids {
		if len(h.g.Obj(id).Counters) != 0 {
			t.Fatalf("object %d took counters: %+v", id, h.g.Obj(id).Counters)
		}
	}
	if n := counterChangeCount(h, 0); n != 0 {
		t.Fatalf("CounterChange events = %d, want 0", n)
	}
	assertNoExistingLiteral(t, h, ids)
}
