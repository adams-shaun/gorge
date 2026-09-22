package effects

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func attackingFixture(t *testing.T) (*fakeHost, *Ctx, state.ObjID) {
	h, c := fixtureHost(t)
	id := state.ObjID(1)
	o := h.g.Obj(id)
	h.Emit(events.Event{Kind: events.MoveZone, Obj: id, From: o.Zone, To: state.ZBattlefield})
	return h, c, id
}

func TestAttackingEntryMarksDefender(t *testing.T) {
	h, c, id := attackingFixture(t)
	c.DefendingPlayer = state.Target{IsPlayer: true, Player: 1}
	applyAttackingEntry(h, c, &cards.SA{Params: map[string]string{"Attacking": "True"}}, id, 0, state.ZBattlefield)
	o := h.g.Obj(id)
	if o == nil || !o.IsAttacking || o.Attacking != 1 {
		t.Fatalf("object attacking=%v defender=%d, want defender 1", o.IsAttacking, o.Attacking)
	}
	if len(h.log) != 2 || h.log[1].Kind != events.TokenAttacks {
		t.Fatalf("log = %+v, want one TokenAttacks after setup", h.log)
	}
}

func TestAttackingEntryWithoutDefenderDegradesOnce(t *testing.T) {
	h, c, id := attackingFixture(t)
	applyAttackingEntry(h, c, &cards.SA{Params: map[string]string{"Attacking": "True"}}, id, 0, state.ZBattlefield)
	o := h.g.Obj(id)
	if o == nil || o.IsAttacking {
		t.Fatalf("object = %+v, want it not attacking", o)
	}
	notes := 0
	for _, ev := range h.log {
		if ev.Kind == events.Note && strings.Contains(ev.Text, "Attacking$") {
			notes++
		}
	}
	if notes != 1 {
		t.Fatalf("Attacking notes = %d, want 1; log=%+v", notes, h.log)
	}
}

func TestAttackingEntryNonTrueSelectorDegradesOnce(t *testing.T) {
	h, c, id := attackingFixture(t)
	applyAttackingEntry(h, c, &cards.SA{Params: map[string]string{"Attacking": "Remembered"}}, id, 0, state.ZBattlefield)
	if h.g.Obj(id).IsAttacking {
		t.Fatal("unsupported selector made the object attack")
	}
	if len(h.log) != 2 || h.log[1].Kind != events.Note {
		t.Fatalf("log = %+v, want one Note after setup", h.log)
	}
}
