package trigmatch

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Fight and FightOnce (edh-trigD): "Whenever a creature you control fights"
// (Foe-Razer Regent, per fighter) and "whenever one or more creatures you
// control fight" (Neyith of the Dire Hunt, once per fight). A fight (CR 701.14)
// is emitted as ordinary non-combat Damage events -- one per direction, the
// FIGHTER as the damage source -- and events.Event carries no fight flag, so
// the provenance is Facts.FightDamaging, which effects' emitFightHit publishes
// around each hit. Both modes therefore fire off the fight's Damage events:
// a fighter whose own hit deals no damage (zero power, fully prevented)
// emits no event of its own and fires only Mode$ FightOnce through its
// opponent's hit.
//
// Fight matches ValidCard$ against the hit's SOURCE (the fighter that dealt
// it), so each fighter fires once on its own hit. FightOnce matches ValidCard$
// against either fighter (the hit's source or its recipient: both fought) and
// is latched to one instance per fight by rules' damage-batch gate, since
// effFight brackets the whole exchange in one batch.
func fightMatches(e Board, t cards.Trigger, source state.ObjID, ev events.Event, eitherFighter bool) bool {
	if ev.Kind != events.Damage || ev.Amount <= 0 || ev.Obj == 0 || !e.Facts().FightDamaging {
		return false
	}
	v, ok := t.Param(cards.PKValidCard)
	if !ok {
		return true
	}
	ctrl := e.ControllerOf(source)
	if src := DamageEventSource(e); src != 0 && e.MatchesSpec(v, src, source, ctrl, SpecOpts{}) {
		return true
	}
	return eitherFighter && e.MatchesSpec(v, ev.Obj, source, ctrl, SpecOpts{})
}

func init() {
	registerTrigMatcher(func(e Board, t cards.Trigger, source state.ObjID, ev events.Event, _ *state.Object) bool {
		return fightMatches(e, t, source, ev, false)
	}, "Fight")
	registerTrigMatcher(func(e Board, t cards.Trigger, source state.ObjID, ev events.Event, _ *state.Object) bool {
		return fightMatches(e, t, source, ev, true)
	}, "FightOnce")
	effects.RegisterNonAPI("trig:Fight", "trig:FightOnce")
}
