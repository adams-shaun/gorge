package rules

// Reap's graveyard slot (the DB$ ChangeZone sub with TargetMax$ over
// Count$LastStateBattlefieldWithFallback Permanent.Black+TargetedPlayerCtrl)
// is part of the CR 601.2c announcement: the bound reads the opponent chosen
// one slot earlier, so the slot is asked at cast with that player's black
// permanent count as its max, not at resolution.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

func TestReapAnnouncesItsGraveyardTargetsAtCast(t *testing.T) {
	t.Parallel()
	e, _, spell, caster := snapshotConfig(t, 307, "Reap", "Walking Corpse", "Typhoid Rats")
	opp := 1 - caster
	toBattlefield(t, e, opp, "Walking Corpse")
	toBattlefield(t, e, opp, "Typhoid Rats")
	// Three cards in the caster's graveyard (their library Mountains).
	moved := 0
	for i := range e.G.Objs {
		o := &e.G.Objs[i]
		if moved < 3 && o.Owner == caster && o.Zone == state.ZLibrary && o.Face() != nil && o.Face().Name == "Mountain" {
			e.emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: o.Zone, To: state.ZGraveyard})
			moved++
		}
	}
	e.pending = nil
	e.priorityRound()
	if moved != 3 {
		t.Fatalf("precondition: moved %d graveyard cards, want 3", moved)
	}
	addMana(t, e, caster, "1G")

	submitChoices(t, e, castOptionFor(t, e, spell).Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("precondition: opponent target ask not pending: %+v", d)
	}
	picked := -1
	for _, o := range d.Options {
		if o.Kind == "player" && o.Player == opp {
			picked = o.Index
		}
	}
	if picked < 0 {
		t.Fatalf("precondition: opponent not offered: %+v", d.Options)
	}
	submitChoices(t, e, picked)

	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("graveyard slot not announced at cast; pending %+v", d)
	}
	if d.Min != 0 || d.Max != 2 || len(d.Options) != 3 {
		t.Fatalf("graveyard slot min/max/options = %d/%d/%d, want 0/2/3", d.Min, d.Max, len(d.Options))
	}
}
