package effects

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// CounterType$ ExistingCounter names "a kind of counter the
// recipient already has", never a literal counter called ExistingCounter.
// These pins resolve the real effPutCounter on the corpus's own lines.

const (
	// Powerful Broker's ability, as the corpus writes it.
	existingEachLine = "AB$ PutCounter | Cost$ T | ValidTgts$ Permanent,Player | CounterType$ ExistingCounter | EachExistingCounter$ True | CounterNum$ 1 | SorcerySpeed$ True"
	// Animation Module's ability, as the corpus writes it ("choose a counter").
	existingChooseLine = "AB$ PutCounter | Cost$ 3 T | ValidTgts$ Permanent,Player | CounterType$ ExistingCounter | CounterNum$ 1"
)

func existingCtx(src state.ObjID, t state.Target) *Ctx {
	return &Ctx{Source: src, Controller: 0, Targets: []state.Target{t}, TargetsOffered: true}
}

// assertNoLiteralExisting fails if the literal pseudo-kind landed anywhere or
// the ability's EachExistingCounter$ was reported unread.
func assertNoLiteralExisting(t *testing.T, h *fakeHost, o *state.Object) {
	t.Helper()
	for _, c := range o.Counters {
		if strings.EqualFold(c.Kind, "ExistingCounter") {
			t.Fatalf("literal ExistingCounter counter placed: %+v", o.Counters)
		}
	}
	for _, ev := range h.log {
		if strings.Contains(ev.Text, "EachExistingCounter") {
			t.Fatalf("EachExistingCounter$ noted: %q", ev.Text)
		}
	}
}

func TestPutCounterEachExistingAddsOneOfEveryKind(t *testing.T) {
	h := newHost(t, 2)
	id := putCounterObject(t, h)
	o := h.g.Obj(id)
	o.Counters = []state.Counter{{Kind: "P1P1", N: 2}, {Kind: "CHARGE", N: 3}, {Kind: "TIME", N: 0}}
	if o.Zone != state.ZBattlefield || o.Counter("P1P1") != 2 || o.Counter("CHARGE") != 3 {
		t.Fatalf("precondition: zone %s counters %+v", o.Zone, o.Counters)
	}

	Resolve(h, existingCtx(id, state.Target{Obj: id}), sa(t, existingEachLine))

	if got := o.Counter("P1P1"); got != 3 {
		t.Fatalf("P1P1 = %d, want 3", got)
	}
	if got := o.Counter("CHARGE"); got != 4 {
		t.Fatalf("CHARGE = %d, want 4", got)
	}
	// A drained slot (N == 0) is not a kind of counter on it (CR 122.1).
	if got := o.Counter("TIME"); got != 0 {
		t.Fatalf("TIME = %d, want 0 (no TIME counter was on it)", got)
	}
	if n := counterChangeCount(h, id); n != 2 {
		t.Fatalf("CounterChange events = %d, want 2 (one per existing kind)", n)
	}
	if h.lastAsk != nil {
		t.Fatalf("EachExistingCounter$ asked %q; it places every kind", h.lastAsk.Prompt)
	}
	assertNoLiteralExisting(t, h, o)
}

func TestPutCounterEachExistingCounterlessTakesNothing(t *testing.T) {
	h := newHost(t, 2)
	id := putCounterObject(t, h)
	o := h.g.Obj(id)
	if o.Zone != state.ZBattlefield || len(o.Counters) != 0 {
		t.Fatalf("precondition: zone %s counters %+v", o.Zone, o.Counters)
	}

	Resolve(h, existingCtx(id, state.Target{Obj: id}), sa(t, existingEachLine))

	if len(o.Counters) != 0 {
		t.Fatalf("counterless target took counters: %+v", o.Counters)
	}
	if n := counterChangeCount(h, 0); n != 0 {
		t.Fatalf("CounterChange events = %d, want 0", n)
	}
	assertNoLiteralExisting(t, h, o)
}

func TestPutCounterEachExistingOnPlayer(t *testing.T) {
	h := newHost(t, 2)
	id := putCounterObject(t, h)
	p := &h.g.Players[1]
	p.Counters = []state.Counter{{Kind: "POISON", N: 1}, {Kind: "EXPERIENCE", N: 2}}
	if p.Counter("POISON") != 1 || p.Counter("EXPERIENCE") != 2 {
		t.Fatalf("precondition: player counters %+v", p.Counters)
	}

	Resolve(h, existingCtx(id, state.Target{Player: 1, IsPlayer: true}), sa(t, existingEachLine))

	if got := p.Counter("POISON"); got != 2 {
		t.Fatalf("POISON = %d, want 2", got)
	}
	if got := p.Counter("EXPERIENCE"); got != 3 {
		t.Fatalf("EXPERIENCE = %d, want 3", got)
	}
	if got := p.Counter("ExistingCounter"); got != 0 {
		t.Fatalf("literal ExistingCounter player counter = %d", got)
	}
	if got := h.g.Players[0].Counter("POISON"); got != 0 {
		t.Fatalf("non-target player took POISON = %d", got)
	}
}

// Animation Module's "choose a counter": one kind among those present, asked
// as the shared counter_kind KChoose; the unserved stand-in is the first kind.
func TestPutCounterExistingChoosesOneKind(t *testing.T) {
	h := newHost(t, 2)
	id := putCounterObject(t, h)
	o := h.g.Obj(id)
	o.Counters = []state.Counter{{Kind: "LOYALTY", N: 4}, {Kind: "P1P1", N: 1}}
	if o.Counter("LOYALTY") != 4 || o.Counter("P1P1") != 1 {
		t.Fatalf("precondition: counters %+v", o.Counters)
	}

	Resolve(h, existingCtx(id, state.Target{Obj: id}), sa(t, existingChooseLine))

	d := h.lastAsk
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "counter_kind" || d.Min != 1 || d.Max != 1 {
		t.Fatalf("ask = %+v, want a counter_kind KChoose 1..1", d)
	}
	var labels []string
	for _, opt := range d.Options {
		labels = append(labels, opt.Label)
	}
	if strings.Join(labels, ",") != "LOYALTY,P1P1" {
		t.Fatalf("offered kinds = %v, want the two kinds present", labels)
	}
	if o.Counter("LOYALTY") != 5 || o.Counter("P1P1") != 1 {
		t.Fatalf("counters = %+v, want LOYALTY 5 (stand-in pick), P1P1 1", o.Counters)
	}
	assertNoLiteralExisting(t, h, o)

	// A single kind present places without asking.
	h2 := newHost(t, 2)
	id2 := putCounterObject(t, h2)
	o2 := h2.g.Obj(id2)
	o2.Counters = []state.Counter{{Kind: "LORE", N: 1}}
	Resolve(h2, existingCtx(id2, state.Target{Obj: id2}), sa(t, existingChooseLine))
	if h2.lastAsk != nil || o2.Counter("LORE") != 2 {
		t.Fatalf("single kind: ask %v counters %+v, want no ask and LORE 2", h2.lastAsk, o2.Counters)
	}
	assertNoLiteralExisting(t, h2, o2)
}
