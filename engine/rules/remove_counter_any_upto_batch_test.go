package rules

// TestRemoveCounterAnyUpToBatchesCounterRemovedOnce pins the event contract
// Glissa Sunslayer's `CounterType$ Any` election must keep: the whole
// multi-counter removal is logged as ONE negative-Amount CounterChange per
// kind, so a CounterRemovedOnce trigger (Regenerations Restored) fires once
// with the batch magnitude instead of once per counter. The per-counter ask
// sequence is unchanged; only the event granularity is.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

func TestRemoveCounterAnyUpToBatchesCounterRemovedOnce(t *testing.T) {
	t.Parallel()
	e := corpusTokenEngine(t)
	reg := onBoardCard(t, e, 0, mshCorpusCard(t, "Regenerations Restored"))
	e.emit(events.Event{Kind: events.CounterChange, Obj: reg, Counter: "TIME", Amount: 4})
	if o := e.G.Obj(reg); o == nil || o.Zone != state.ZBattlefield || o.Counter("TIME") != 4 {
		t.Fatalf("precondition: Regenerations Restored = %+v, want battlefield with four TIME", o)
	}
	if len(e.pendingTriggers) != 0 {
		t.Fatalf("precondition: the TIME put queued %d triggers, want 0", len(e.pendingTriggers))
	}

	glissa := mshCorpusCard(t, "Glissa Sunslayer")
	mode := cards.ResolveSVar(glissa.Faces[0].SVars, "DBRemove")
	if mode == nil || mode.API != "RemoveCounter" {
		t.Fatalf("Glissa DBRRemove = %+v, want RemoveCounter", mode)
	}
	lifeBefore := e.G.Players[0].Life

	kr4Resolve(e, func() *effects.Ctx {
		return &effects.Ctx{Controller: 0, Targets: []state.Target{{Obj: reg}}, TargetsOffered: true}
	}, mode)
	// Remove two TIME counters through the per-counter election, then Done.
	for i := 0; i < 2; i++ {
		d := e.Pending()
		if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "counter_kind" {
			t.Fatalf("ask %d = %+v, want a counter_kind KChoose", i, d)
		}
		idx := optionIndexByLabel(d, "TIME")
		if idx < 0 {
			t.Fatalf("ask %d options = %+v, want TIME", i, d.Options)
		}
		submitChoices(t, e, idx)
	}
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "counter_kind" {
		t.Fatalf("third ask = %+v, want the Done election", d)
	}
	done := -1
	for _, o := range d.Options {
		if o.Kind == "remove_counter_done" {
			done = o.Index
		}
	}
	if done < 0 {
		t.Fatalf("third ask options = %+v, want a Done option", d.Options)
	}
	submitChoices(t, e, done)

	if got := e.G.Obj(reg).Counter("TIME"); got != 2 {
		t.Fatalf("TIME after the two removals = %d, want 2", got)
	}
	removals := 0
	for _, ev := range e.L.Events {
		if ev.Kind == events.CounterChange && ev.Obj == reg && ev.Amount < 0 {
			removals++
			if ev.Counter != "TIME" || ev.Amount != -2 {
				t.Fatalf("removal event = %+v, want one TIME -2 batch", ev)
			}
		}
	}
	if removals != 1 {
		t.Fatalf("removal events = %d, want 1 (one event carries the whole batch)", removals)
	}
	if len(e.pendingTriggers) != 1 {
		t.Fatalf("pendingTriggers after the -2 TIME batch = %d, want 1 (CounterRemovedOnce)", len(e.pendingTriggers))
	}
	e.putTriggersOnStack()
	e.resolveTop()
	if d := e.Pending(); d == nil || d.Kind != decision.KArrange {
		t.Fatalf("expected Regenerations Restored's scry KArrange ask, got %+v", d)
	}
	submitChoices(t, e)
	if got := e.G.Players[0].Life; got != lifeBefore+1 {
		t.Fatalf("life after the CounterRemovedOnce body = %d, want %d (one scry, one life)", got, lifeBefore+1)
	}
}
