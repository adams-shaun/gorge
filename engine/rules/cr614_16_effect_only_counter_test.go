package rules

// Reference: Magic: The Gathering Comprehensive Rules, 2026-08-07.
// CR 614.16: a replacement effect that applies only to an effect (Doubling
// Season) applies to a cost's counter placement once another replacement
// (Pir) has applied to it. CR 616.1f: the process repeats taking into account
// replacement effects that are now applicable.

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// effectOnlyCounterRun casts Chandra, Fire Artisan under Doubling Season and
// Pir, answers the entry order with the named source first, then activates
// the +1 loyalty ability. It returns the loyalty on entry and after the cost
// and ability resolve.
func effectOnlyCounterRun(t *testing.T, first string) (entry, final int32) {
	t.Helper()
	e := crResolutionEngine(t, []string{"Chandra, Fire Artisan", "Doubling Season", "Pir, Imaginative Rascal"}, nil)
	crAbortMove(t, e, 0, "Doubling Season", state.ZBattlefield)
	crAbortMove(t, e, 0, "Pir, Imaginative Rascal", state.ZBattlefield)
	chandra := crAbortMove(t, e, 0, "Chandra, Fire Artisan", state.ZHand)
	e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "R", Amount: 2})
	e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "C", Amount: 2})
	e.askPriority(0)
	crAbortAnswer(t, e, "cast", crAbortOption(t, e, "cast", "cast", chandra))

	for i := 0; i < 40; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatalf("no decision before Chandra entered (zone %v)", e.G.Obj(chandra).Zone)
		}
		if d.Kind == decision.KReplacement {
			pick := -1
			for _, o := range d.Options {
				if strings.Contains(o.Label, first) {
					pick = o.Index
				}
			}
			if pick < 0 {
				t.Fatalf("no replacement option naming %q: %+v", first, d.Options)
			}
			crAbortAnswer(t, e, "entry order", pick)
			continue
		}
		if o := e.G.Obj(chandra); o != nil && o.Zone == state.ZBattlefield &&
			d.Kind == decision.KPriority && len(e.G.Stack) == 0 {
			break
		}
		if d.Kind == decision.KTarget {
			// Pir's own entry trigger asks for a player target.
			crAbortAnswer(t, e, "target", 0)
			continue
		}
		if d.Kind == decision.KChoose {
			crAbortAnswer(t, e, "choose", d.Options[0].Index)
			continue
		}
		if d.Kind != decision.KPriority {
			t.Fatalf("unexpected decision kind %v: %s", d.Kind, d.Prompt)
		}
		crAbortAnswer(t, e, "pass", crAbortOption(t, e, "pass", "pass", 0))
	}
	o := e.G.Obj(chandra)
	if o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Chandra not on the battlefield")
	}
	entry = o.Counter("LOYALTY")
	// The first "ability" option is the +1 (Chandra lists her loyalty abilities in card order).
	crAbortAnswer(t, e, "activate", crAbortOption(t, e, "activate", "ability", chandra))
	passUntilStackEmpty(t, e, 20)
	return entry, e.G.Obj(chandra).Counter("LOYALTY")
}

func TestCR61416EffectOnlyCounterSeasonFirst(t *testing.T) {
	t.Parallel()
	entry, final := effectOnlyCounterRun(t, "Doubling Season")
	if entry != 9 {
		t.Fatalf("entry loyalty = %d, want 9", entry)
	}
	if final == entry+2 {
		t.Fatalf("the +1 cost was only Pir-modified (%d -> %d): Doubling Season never reached it", entry, final)
	}
	if final != 13 {
		t.Fatalf("final loyalty = %d, want 13 (9 + (1+1)*2)", final)
	}
}

func TestCR61416EffectOnlyCounterPirFirst(t *testing.T) {
	t.Parallel()
	entry, final := effectOnlyCounterRun(t, "Pir")
	if entry != 10 {
		t.Fatalf("entry loyalty = %d, want 10", entry)
	}
	if final == entry+2 {
		t.Fatalf("the +1 cost was only Pir-modified (%d -> %d): Doubling Season never reached it", entry, final)
	}
	if final != 14 {
		t.Fatalf("final loyalty = %d, want 14 (10 + (1+1)*2)", final)
	}
}
