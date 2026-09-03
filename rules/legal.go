package rules

import (
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// sorcerySpeed reports whether p may take a sorcery-speed action right now.
func (e *Engine) sorcerySpeed(p state.PlayerID) bool {
	return e.G.Active == p && e.G.Step.IsMain() && len(e.G.Stack) == 0
}

// legalActions enumerates everything p may legally do with priority. The
// result is the complete rules surface a client ever sees.
func (e *Engine) legalActions(p state.PlayerID) []decision.Option {
	var out []decision.Option
	add := func(kind, label string, obj state.ObjID) {
		out = append(out, decision.Option{Index: len(out), Kind: kind, Label: label, Obj: obj})
	}
	sorcery := e.sorcerySpeed(p)

	for _, id := range e.G.Zone(state.ZHand, p) {
		o := e.G.Obj(id)
		f := o.Face()
		if f == nil {
			continue
		}
		if f.IsLand() {
			if sorcery && e.G.Players[p].LandsPlayed < 1 {
				add("play_land", "Play "+f.Name, id)
			}
			continue
		}
		if e.castRestricted(p, id) {
			continue
		}
		instantSpeed := f.IsInstant() || e.HasKeyword(id, "Flash")
		if !instantSpeed && !sorcery {
			continue
		}
		if e.adjustedCost(p, id).CanPay(e.G.Players[p].Pool) {
			add("cast", "Cast "+f.Name, id)
		}
		for i, alt := range e.alternativeCosts(p, id) {
			if alt.CanPay(e.G.Players[p].Pool) {
				// AltCostIndex is i+1, not i: the zero value must mean "the
				// card's own cost" so every other Option literal in the tree
				// (play_land, activate, pass, and the base "cast" option
				// added just above via the shared add closure) needs no
				// change to keep meaning that.
				out = append(out, decision.Option{Index: len(out), Kind: "cast",
					Label: altCostLabel(f.Name, i), Obj: id, AltCostIndex: i + 1})
			}
		}
	}

	for _, id := range e.G.Zone(state.ZBattlefield, p) {
		o := e.G.Obj(id)
		f := o.Face()
		if f == nil || o.Tapped {
			continue
		}
		if e.abilityRestricted(id) {
			continue
		}
		// M1 activates only mana abilities, whose cost is always {T}. Task 18
		// widens this once activated abilities with real costs land.
		if len(f.ManaAbilities()) > 0 {
			add("activate", "Tap "+f.Name+" for mana", id)
		}
	}

	// Pass is last so a client can safely default to the final option.
	add("pass", "Pass priority", 0)
	return out
}

func (e *Engine) handlePriority(d *decision.Decision, in decision.Intent) {
	opt := d.Chosen(in)[0]
	switch opt.Kind {
	case "pass":
		passes := e.G.Passes + 1
		if passes >= int32(e.G.AliveCount()) {
			if len(e.G.Stack) > 0 {
				e.resolveTop()
				// The pass count resets: priority returns to the active
				// player after a resolution, same as at the start of any
				// other step.
				e.emit(events.Event{Kind: events.Priority, Player: e.G.Active})
				return
			}
			// advanceStep's own emit carries the reset pass count; the count
			// this round reached is never itself a value anything observes.
			e.advanceStep()
			return
		}
		e.emit(events.Event{Kind: events.Priority, Player: e.G.NextAlive(e.G.Priority), Amount: passes})

	case "play_land":
		e.emit(events.Event{Kind: events.Priority, Player: e.G.Priority, Amount: 0})
		e.emit(events.Event{Kind: events.MoveZone, Obj: opt.Obj,
			From: state.ZHand, To: state.ZBattlefield})
		e.emit(events.Event{Kind: events.LandPlayed, Player: in.Player})

	case "activate":
		e.emit(events.Event{Kind: events.Priority, Player: e.G.Priority, Amount: 0})
		o := e.G.Obj(opt.Obj)
		e.emit(events.Event{Kind: events.Tap, Obj: opt.Obj})
		for _, ma := range o.Face().ManaAbilities() {
			e.resolveAbility(opt.Obj, in.Player, nil, ma, o.Face().SVars)
		}

	case "cast":
		e.emit(events.Event{Kind: events.Priority, Player: e.G.Priority, Amount: 0})
		e.castSpell(in.Player, opt)
	}
}
