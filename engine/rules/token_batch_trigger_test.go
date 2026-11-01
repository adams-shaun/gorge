package rules

// One api:Token resolution that creates several tokens is ONE simultaneous
// batch (CR 603.2c) for Mode$ ChangesZoneAll: Woodland Champion triggers once
// and TriggerCount$Amount is the number of tokens.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

func TestTokenBatchTriggersChangesZoneAllOnceWithCount(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	champ := searchCorpusCard(t, reg, "Woodland Champion")
	e, _ := millTriggerEngine(t, champ)

	id := seatOnBattlefield(t, e, champ, 0)
	o := e.G.Obj(id)
	if o == nil || o.Zone != state.ZBattlefield || o.Counter("P1P1") != 0 {
		t.Fatalf("Woodland Champion %d = %+v, want a counterless battlefield card (vacuous setup)", id, o)
	}

	effects.Resolve(e, &effects.Ctx{Controller: 0}, &cards.SA{Kind: "DB", API: "Token",
		Params: map[string]string{"TokenAmount": "2", "TokenScript": "g_1_1_squirrel", "TokenOwner": "You"}})

	if n := len(battlefieldTokens(e, 0)); n != 2 {
		t.Fatalf("battlefield holds %d tokens, want 2 (token script did not mint)", n)
	}
	drainMillTrigger(t, e, 40)

	pushes := 0
	for _, ev := range e.L.Events {
		if ev.Kind == events.TriggerPush && ev.Obj == id {
			pushes++
		}
	}
	if pushes != 1 {
		t.Fatalf("Woodland Champion TriggerPush count = %d, want 1 (one batch)", pushes)
	}
	if got := e.G.Obj(id).Counter("P1P1"); got != 2 {
		t.Fatalf("Woodland Champion P1P1 counters = %d, want 2", got)
	}
}
