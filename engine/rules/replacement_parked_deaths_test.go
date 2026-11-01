package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestParkedDeathsAskOncePerCreature pins CR 616.1 on a batch of deaths: each
// dying creature's controller chooses once. Answering one leaves the others
// parked while state-based actions run again, and those must not propose the
// same deaths a second time, nor ask about a creature that has already left.
func TestParkedDeathsAskOncePerCreature(t *testing.T) {
	t.Parallel()
	const bear = "Name:Doomed\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
	const sliver = "Name:Doomed Sliver\nTypes:Creature Sliver\nPT:2/2\nOracle:x\n"
	for _, tc := range []struct {
		name, creature string
		statics        []string
	}{
		{"competing replacements", bear, []string{"Rest in Peace", "Rest in Peace"}},
		{"a lone optional replacement", sliver, []string{"Pulmonic Sliver"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e := handEngine(t)
			for i, name := range tc.statics {
				onBoardCard(t, e, state.PlayerID(i), corpusAlternativeCard(t, name))
			}
			var doomed []state.ObjID
			for _, seat := range []state.PlayerID{0, 0, 1} {
				doomed = append(doomed, onBoard(t, e, seat, tc.creature))
			}
			for _, id := range doomed {
				e.emit(events.Event{Kind: events.Damage, Obj: id, Amount: 2})
			}
			e.pending = nil
			e.checkStateBased()
			var asked []state.ObjID
			for d := e.Pending(); d != nil && d.Kind == decision.KReplacement; d = e.Pending() {
				if o := e.G.Obj(d.Source); o.Zone != state.ZBattlefield {
					t.Fatalf("asked about %d, already in zone %v", d.Source, o.Zone)
				}
				asked = append(asked, d.Source)
				if len(asked) > len(doomed) {
					t.Fatalf("asked %v for %d dying creatures", asked, len(doomed))
				}
				submitChoices(t, e, 0)
			}
			if !slices.Equal(asked, doomed) {
				t.Fatalf("asked about %v, want each of %v once", asked, doomed)
			}
			for _, id := range doomed {
				if z := e.G.Obj(id).Zone; z == state.ZBattlefield {
					t.Fatalf("creature %d is still on the battlefield", id)
				}
			}
		})
	}
}

// TestParkedDeathKeepsItsAskWhenASourceLeaves: a creature and one of the two
// replacements competing for its death die together. The source's departure does
// not settle the choice, and a state-based pass under the ask leaves the creature
// where it is.
func TestParkedDeathKeepsItsAskWhenASourceLeaves(t *testing.T) {
	t.Parallel()
	e := handEngine(t)
	shadow := onBoardCard(t, e, 0, corpusAlternativeCard(t, "Misery's Shadow"))
	onBoardCard(t, e, 0, corpusAlternativeCard(t, "Stone of Erech"))
	bear := onBoard(t, e, 1, "Name:Doomed\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	e.emit(events.Event{Kind: events.Damage, Obj: bear, Amount: 2})
	e.emit(events.Event{Kind: events.Damage, Obj: shadow, Amount: 2})
	e.pending = nil
	e.checkStateBased()
	e.checkStateBased()
	d := e.Pending()
	if d == nil || d.Kind != decision.KReplacement || d.Source != bear {
		t.Fatalf("pending %+v, want the replacement choice about %d", d, bear)
	}
	if z := e.G.Obj(bear).Zone; z != state.ZBattlefield {
		t.Fatalf("creature moved to zone %v under its ask", z)
	}
	if z := e.G.Obj(shadow).Zone; z != state.ZGraveyard {
		t.Fatalf("source in zone %v, want the graveyard", z)
	}
	submitChoices(t, e, 0)
	if d := e.Pending(); d != nil && d.Kind == decision.KReplacement {
		t.Fatalf("asked again: %+v", d)
	}
	if z := e.G.Obj(bear).Zone; z != state.ZExile {
		t.Fatalf("creature in zone %v, want exile", z)
	}
}
