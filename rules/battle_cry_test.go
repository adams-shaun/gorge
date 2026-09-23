package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// TestBattleCryPumpsOtherAttackingCreatures pins the real Hero of Bladehold
// keyword expansion and CR 702.33's source exclusion: the second attacker
// receives +1/+0, while the Battle cry source does not pump itself.
func TestBattleCryPumpsOtherAttackingCreatures(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	hero := mustCorpusCard(t, reg, "Hero of Bladehold")
	wurm := mustCorpusCard(t, reg, "Craw Wurm")
	if d := hero.Link(); len(d) != 0 {
		t.Fatalf("link Hero of Bladehold: %v", d)
	}
	if d := wurm.Link(); len(d) != 0 {
		t.Fatalf("link Craw Wurm: %v", d)
	}
	if !hero.Faces[0].HasKeyword("Battle cry") {
		t.Fatal("Hero of Bladehold does not print Battle cry in the corpus")
	}
	if got := len(hero.Faces[0].Triggers); got == 0 {
		t.Fatal("Hero of Bladehold has no linked triggers")
	}
	found := false
	for _, tr := range hero.Faces[0].Triggers {
		if tr.Params["Keyword"] == "Battle cry" {
			found = true
		}
	}
	if !found {
		t.Fatal("Hero of Bladehold's Battle cry did not expand to a real trigger")
	}

	e, cfg := trainingDeck(t, 70233, hero, wurm)
	heroID := battlefieldID(t, e, "Hero of Bladehold")
	wurmID := battlefieldID(t, e, "Craw Wurm")
	e.emit(events.Event{Kind: events.MoveZone, Obj: heroID, From: state.ZHand, To: state.ZBattlefield})
	e.emit(events.Event{Kind: events.MoveZone, Obj: wurmID, From: state.ZHand, To: state.ZBattlefield})
	baseHeroPower := e.Derived(heroID).Power
	baseWurmPower := e.Derived(wurmID).Power
	if baseHeroPower <= 0 || baseWurmPower <= baseHeroPower {
		t.Fatalf("test setup powers are not distinct and positive: Hero=%d Wurm=%d", baseHeroPower, baseWurmPower)
	}

	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{heroID, wurmID}})
	if !e.G.Obj(heroID).IsAttacking || !e.G.Obj(wurmID).IsAttacking {
		t.Fatal("test setup failed: Hero and Wurm are not both attacking")
	}
	answerTriggerOrders(t, e)
	e.priorityRound()
	answerTriggerOrders(t, e)
	passUntilStackEmpty(t, e, 20)
	if got := e.Derived(wurmID).Power; got != baseWurmPower+1 {
		t.Fatalf("other attacker power = %d, want %d", got, baseWurmPower+1)
	}
	if got := e.Derived(heroID).Power; got != baseHeroPower {
		t.Fatalf("Battle cry source power = %d, want unchanged %d", got, baseHeroPower)
	}
	replayCheck(t, e, cfg)
}
