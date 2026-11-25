package effects

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestMoveCounterAnyFromSeveralOrigins pins Aetherborn Marauder's ETB:
// `ValidSource$ Permanent.YouCtrl+Other+counters_GE1_P1P1 | Defined$ Self |
// CounterType$ P1P1 | CounterNum$ Any`. With TWO counter-carrying origins the
// primitive used to loud-degrade ("CounterNum$ Any over multiple origins")
// and move nothing; the controller now picks a permanent and how many
// counters move from it, repeatedly, until the election declines or every
// origin is drained.

// moveCounterAnyAnswer is one scripted answer: an origin chosen by object id,
// or an amount chosen from the current ask.
type moveCounterAnyAnswer struct {
	objID  state.ObjID
	amount int32
}

// moveCounterAnyHost serves the origin/amount election through the ask seam,
// so the test exercises the real decisions instead of the no-host stand-in.
type moveCounterAnyHost struct {
	*fakeHost
	answers []moveCounterAnyAnswer
	asks    []*decision.Decision
}

func (h *moveCounterAnyHost) AskCount() uint64 { return 0 }

func (h *moveCounterAnyHost) TapeAnswer(d *decision.Decision) (decision.Intent, bool) {
	if d.ResumeKind != "move_counter_origin" && d.ResumeKind != "move_counter" {
		return decision.Intent{}, false
	}
	h.asks = append(h.asks, d)
	if len(h.answers) == 0 {
		return decision.Intent{}, false
	}
	a := h.answers[0]
	h.answers = h.answers[1:]
	for _, o := range d.Options {
		if d.ResumeKind == "move_counter_origin" && o.Obj == a.objID {
			return decision.Intent{Choices: []int{o.Index}}, true
		}
		if d.ResumeKind == "move_counter" && o.Amount == int(a.amount) {
			return decision.Intent{Choices: []int{o.Index}}, true
		}
	}
	return decision.Intent{}, false
}

func TestMoveCounterAnyFromSeveralOrigins(t *testing.T) {
	card, sa := corpusSA(t, "Aetherborn Marauder", "TrigMoveCounters")
	if sa.API != "MoveCounter" {
		t.Fatalf("Aetherborn Marauder's SVar = %s, want MoveCounter", sa.API)
	}
	if got := sa.ParamStr(cards.PKCounterType); !strings.EqualFold(got, "P1P1") {
		t.Fatalf("CounterType$ = %q, want P1P1", got)
	}
	if got := sa.ParamStr(cards.PKCounterNum); !strings.EqualFold(got, "Any") {
		t.Fatalf("CounterNum$ = %q, want Any", got)
	}
	if got := sa.ParamStr(cards.PKValidSource); !strings.EqualFold(got, "Permanent.YouCtrl+Other+counters_GE1_P1P1") {
		t.Fatalf("ValidSource$ = %q, want the counters_GE1_P1P1 sweep", got)
	}

	h := &moveCounterAnyHost{fakeHost: newHost(t, 2)}
	src := h.g.AddObject(card, 0)
	h.Emit(events.Event{Kind: events.MoveZone, Obj: src.ID, From: state.ZLibrary, To: state.ZBattlefield})
	alpha := mcCreature(t, h.g, "Alpha", 0)
	beta := mcCreature(t, h.g, "Beta", 0)
	h.g.Obj(alpha).AddCounter("P1P1", 2)
	h.g.Obj(beta).AddCounter("P1P1", 1)

	// Precondition: both origins are on the battlefield carrying counters and
	// the destination carries none, so the filter sees two real origins.
	for _, id := range []state.ObjID{alpha, beta} {
		o := h.g.Obj(id)
		if o.Zone != state.ZBattlefield || o.Counter("P1P1") == 0 {
			t.Fatalf("origin %d = zone %s counters %+v, want a battlefield carrier", id, o.Zone, o.Counters)
		}
	}
	if got := h.g.Obj(src.ID).Counter("P1P1"); got != 0 {
		t.Fatalf("destination P1P1 = %d, want 0 before the move", got)
	}

	// Move 1 off Beta (the second origin), then the remaining 2 off Alpha.
	h.answers = []moveCounterAnyAnswer{
		{objID: beta},
		{amount: 1},
		{amount: 2},
	}
	Resolve(h, &Ctx{Controller: 0, Source: src.ID}, sa)

	if len(h.asks) != 3 {
		t.Fatalf("asks = %d, want 3 (origin, amount, then the sole remaining origin's amount): %+v", len(h.asks), h.asks)
	}
	originAsk := h.asks[0]
	if originAsk.Kind != decision.KChoose || originAsk.ResumeKind != "move_counter_origin" ||
		originAsk.Min != 0 || originAsk.Max != 1 || len(originAsk.Options) != 2 {
		t.Fatalf("origin ask = %+v, want a 1-of KChoose over the two origins", originAsk)
	}
	seen := map[state.ObjID]bool{}
	for _, o := range originAsk.Options {
		seen[o.Obj] = true
	}
	if !seen[alpha] || !seen[beta] {
		t.Fatalf("origin options = %+v, want Alpha and Beta", originAsk.Options)
	}
	if asks := h.asks[1]; asks.ResumeKind != "move_counter" || asks.Max != 1 ||
		len(asks.Options) != 2 || asks.Options[1].Amount != 1 {
		t.Fatalf("first amount ask = %+v, want 0..1 for Beta", asks)
	}
	if asks := h.asks[2]; asks.ResumeKind != "move_counter" || asks.Max != 2 || len(asks.Options) != 3 {
		t.Fatalf("second amount ask = %+v, want 0..2 for Alpha", asks)
	}

	if got := h.g.Obj(alpha).Counter("P1P1"); got != 0 {
		t.Fatalf("Alpha P1P1 = %d, want 0 (both moved)", got)
	}
	if got := h.g.Obj(beta).Counter("P1P1"); got != 0 {
		t.Fatalf("Beta P1P1 = %d, want 0 (its only counter moved)", got)
	}
	if got := h.g.Obj(src.ID).Counter("P1P1"); got != 3 {
		t.Fatalf("Aetherborn Marauder P1P1 = %d, want 3 (2 + 1 moved in)", got)
	}
	// CR 122.5: a move conserves the total -- the events must sum to zero.
	var net int32
	pairs := map[state.ObjID]int32{}
	for _, ev := range h.log {
		if ev.Kind != events.CounterChange || ev.Counter != "P1P1" {
			continue
		}
		net += ev.Amount
		pairs[ev.Obj] += ev.Amount
	}
	if net != 0 {
		t.Fatalf("P1P1 CounterChange events sum to %d, want 0 (a move never mints)", net)
	}
	if pairs[alpha] != -2 || pairs[beta] != -1 || pairs[src.ID] != 3 {
		t.Fatalf("per-object P1P1 deltas = %+v, want Alpha -2, Beta -1, destination +3", pairs)
	}
}
