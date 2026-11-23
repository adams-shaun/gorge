package rules

import "github.com/adams-shaun/gorge/engine/state"

// departedSourceLKI is the last-known object of source for a protection read
// (CR 702.16 with CR 113.7a): an ability on the stack whose source left the
// battlefield is a source with the characteristics that source had as it left,
// not the card now in the graveyard. Artisan of Forms that copied a black
// Festering Goblin and died puts its "-1/-1" trigger on the stack as a black
// source, so a permanent with protection from black is no legal target, while
// the graveyard card reads blue.
//
// Only a trigger of the source itself carries the snapshot (triggerLKI holds
// the object that changed zones, and it is the source exactly when the source
// died or was exiled). It is nil when no ability of source on the stack holds
// one, when the source is still on the battlefield as that same incarnation,
// and for a source on the stack. The returned object is a copy marked as no
// longer on the battlefield, so the read takes its copied face's colours and
// types without a layer walk of the live card.
func departedSourceLKI(g *state.Game, lkis map[state.ObjID]triggerObjectLKI, source state.ObjID) *state.Object {
	live := g.Obj(source)
	if live == nil || live.Zone == state.ZBattlefield || live.Zone == state.ZStack {
		return nil
	}
	for _, id := range g.Stack {
		so := g.Obj(id)
		if so == nil || so.Ability == nil || so.Source != source {
			continue
		}
		lki, ok := lkis[id]
		if !ok || lki.object == nil || lki.object.ID != source || lki.object.Zone != state.ZBattlefield {
			continue
		}
		cp := *lki.object
		cp.Zone = live.Zone
		return &cp
	}
	return nil
}
