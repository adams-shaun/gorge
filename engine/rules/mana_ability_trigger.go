package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// A mana ability is an activated ability (CR 605.1a) that never uses the stack
// (CR 605.3b), so it mints none of the events.IsActivationMint kinds the
// Mode$ AbilityCast / SpellAbilityCast matchers observe. The activation is
// announced by one marker, events.ManaAbilityActivation, emitted from
// resolveManaAbilityRefOriginal -- but only while a trigger that could watch
// it is in play, so a game without such a carrier logs exactly what it logged
// before. (ManaActivate is NOT reused: it is the activation-limit scan
// marker, TriggerNone on purpose, and widening its emission would move the
// limit scans.)

// manaActivationWatchZones are the zones a watching trigger is looked for in:
// the battlefield (the ordinary home), the command zone (emblems) and the two
// public zones a TriggerZones$ line can name.
var manaActivationWatchZones = [...]state.Zone{state.ZBattlefield, state.ZCommand, state.ZGraveyard, state.ZExile}

// isActivationWatcher reports whether t observes an ability's activation.
func isActivationWatcher(t *cards.Trigger) bool {
	switch t.ModeKind() {
	case cards.TriggerAbilityCast, cards.TriggerSpellAbilityCast:
		return true
	}
	return false
}

// manaActivationWatched reports whether some trigger that could observe a
// mana ability's activation exists: a printed Mode$ AbilityCast /
// SpellAbilityCast line on any face the trigger scan walks (the shared
// roomTriggerFaces / triggerFacesWithMerged enumeration, so the gate cannot
// drift from the scan), or a static-granted one. Deliberately conservative: a
// carrier whose ValidSA$ excludes mana abilities still counts -- the matcher
// rejects the marker, and no legacy deck carries either mode. A pure read over
// zone slices in seat order; nothing here can reach an event but the answer.
func (e *Engine) manaActivationWatched() bool {
	g := e.G
	for p := range g.Players {
		for _, z := range manaActivationWatchZones {
			for _, id := range g.Zone(z, state.PlayerID(p)) {
				if o := g.Obj(id); o != nil && objectWatchesActivations(o) {
					return true
				}
			}
		}
	}
	if !e.trigGrantsPossible() {
		return false
	}
	active := e.active()
	for i := range active {
		ce := &active[i]
		if ce.AddTrigger != nil && isActivationWatcher(ce.AddTrigger) {
			return true
		}
		for _, gf := range ce.GainedTriggerFaces {
			if gf.Face == nil {
				continue
			}
			for ti := range gf.Face.Triggers {
				if isActivationWatcher(&gf.Face.Triggers[ti]) {
					return true
				}
			}
		}
	}
	return false
}

// objectWatchesActivations reports whether any face the trigger scan walks for
// o carries an activation-watching trigger line.
func objectWatchesActivations(o *state.Object) bool {
	f := o.Face()
	if f == nil {
		return false
	}
	faces, n := roomTriggerFaces(o, f)
	walk := faces[:n]
	if len(o.MergedCards) > 0 {
		walk = triggerFacesWithMerged(o, walk)
	}
	for _, fc := range walk {
		if fc.face == nil {
			continue
		}
		for ti := range fc.face.Triggers {
			if isActivationWatcher(&fc.face.Triggers[ti]) {
				return true
			}
		}
	}
	return false
}

// emitManaAbilityActivation announces one mana-ability activation to the
// AbilityCast / SpellAbilityCast scan when a watcher is in play. A gained
// ability (gained.Face != nil) takes the GainedAbilityPush shape -- IDs[0] the
// foreign card, Amount its face-local index; a printed one carries the flat
// pile index of original (-1 when original is no pile ability of source, e.g. a
// keyword-synthesised one, which the matcher then reads as a body-less
// activation).
func (e *Engine) emitManaAbilityActivation(p state.PlayerID, source state.ObjID, original *cards.SA, gainedFrom state.ObjID, gainedIdx int, gained bool) {
	if !e.manaActivationWatched() {
		return
	}
	ev := events.Event{Kind: events.ManaAbilityActivation, Player: p, Obj: source}
	if gained {
		ev.IDs = []state.ObjID{gainedFrom}
		ev.Amount = int32(gainedIdx)
	} else {
		ev.Amount = -1
		if idx, _, found := pileAbilityRefOf(e.G.Obj(source), original); found {
			ev.Amount = int32(idx)
		}
	}
	e.emit(ev)
}
