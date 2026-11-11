package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// damagePreventedPermanents lists the battlefield permanents a damage
// target ask's damage would be spent on for nothing: each carries its own
// unconditional "prevent all damage dealt to this" replacement (CR 615.1a)
// whose rider leaves it unchanged, and source's damage can be prevented
// (CR 615.12). A rider that removes or adds counters (the Phantom
// creatures) changes the permanent, so that permanent is not listed. The
// list is the target payload's hint for the bot; it never changes legality.
func (e *Engine) damagePreventedPermanents(source state.ObjID) []state.ObjID {
	var out []state.ObjID
	for i := range e.G.Players {
		for _, id := range e.G.Zone(state.ZBattlefield, e.G.Players[i].ID) {
			f := e.replacementFace(id, events.Event{Kind: events.Damage, Obj: id})
			if f == nil || !facePreventsAllOwnDamage(f) || e.cantPreventDamage(source, id) {
				continue
			}
			out = append(out, id)
		}
	}
	return out
}

// facePreventsAllOwnDamage reports a face's DamageDone replacement that
// prevents every damage to the face's own permanent: no source, combat or
// condition filter, and a body (if any) that touches no counters.
func facePreventsAllOwnDamage(f *cards.Face) bool {
	for i := range f.Repls {
		r := f.Repls[i]
		if r.EventKind() != cards.ReplDamageDone || r.ParamStr(cards.PKValidTarget) != "Card.Self" {
			continue
		}
		if !damageReplacementPrevents(r) && r.ParamStr(cards.PKPreventionEffect) != "True" {
			continue
		}
		if r.HasParam(cards.PKValidSource) || r.HasParam(cards.PKIsCombat) || r.HasParam(cards.PKCheckSVar) || r.HasParam(cards.PKIsPresent) {
			continue
		}
		if r.With != nil && (r.With.API == "RemoveCounter" || r.With.API == "PutCounter") {
			continue
		}
		return true
	}
	return false
}
