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
		base := e.adjustedCost(p, id)
		if e.castable(p, id, base) {
			add("cast", "Cast "+f.Name, id)
		}
		for i, alt := range e.alternativeCosts(p, id) {
			// Ruling (Task 9 fix round 1, Important 1): this used to gate on
			// mana-only alt.CanPay, but ParseCost now produces Sac/SubCounter/
			// Tap parts that the cast flow enforces -- an AlternativeCost whose
			// Cost$ carries Sac<N/...> was offered without checking that N
			// matching permanents exist, and beginCast then asked a sacrifice
			// decision with zero options that no answer could escape. castable
			// is the same gate every other "cast" option uses.
			if e.castable(p, id, alt) {
				// AltCostIndex is i+1, not i: the zero value must mean "the
				// card's own cost" so every other Option literal in the tree
				// (play_land, activate, pass, and the base "cast" option
				// added just above via the shared add closure) needs no
				// change to keep meaning that.
				out = append(out, decision.Option{Index: len(out), Kind: "cast",
					Label: altCostLabel(f.Name, i), Obj: id, AltCostIndex: i + 1})
			}
		}
		if kc, ok := kickerCost(f); ok && e.castable(p, id, base.Plus(kc)) {
			out = append(out, decision.Option{Index: len(out), Kind: "cast",
				Label: "Cast " + f.Name + " (kicked)", Obj: id, Mode: "kicked"})
		}
		if sc, ok := surgeCost(f); ok && e.spellsCastThisTurn(p) > 0 && e.castable(p, id, sc) {
			out = append(out, decision.Option{Index: len(out), Kind: "cast",
				Label: "Cast " + f.Name + " (surged)", Obj: id, Mode: "surged"})
		}
	}

	// Flashback: a graveyard walk, same instant-speed timing as hand cards,
	// gated on the derived keyword (so a continuous-effect grant, e.g.
	// Snapcaster Mage, counts) rather than the printed one.
	for _, id := range e.G.Zone(state.ZGraveyard, p) {
		o := e.G.Obj(id)
		f := o.Face()
		if f == nil || !e.HasKeyword(id, "Flashback") {
			continue
		}
		if e.castRestricted(p, id) {
			continue
		}
		instantSpeed := f.IsInstant() || e.HasKeyword(id, "Flash")
		if !instantSpeed && !sorcery {
			continue
		}
		if fc := e.flashbackCost(id); e.castable(p, id, fc) {
			out = append(out, decision.Option{Index: len(out), Kind: "cast",
				Label: "Cast " + f.Name + " (flashback)", Obj: id, Mode: "flashback"})
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
		// landed real activated-ability costs, but this enumeration was
		// never widened to non-mana activated abilities (Equip, and every
		// other AB$ ability with a real cost) -- a parked limitation
		// (T19c-b), not a pending task.
		if len(f.ManaAbilities()) > 0 {
			add("activate", "Tap "+f.Name+" for mana", id)
		}
		// Task 14 (T19c-b's parked limitation, now in scope): Equip is an
		// ordinary activated ability -- sorcery-speed, mana cost, one
		// target -- offered here like any other legal action. It is the
		// ONLY non-mana activated ability wired up (the card-script
		// expansion tags it Params["Keyword"] == "Equip"), because a fully
		// general non-mana activation loop (Mother of Runes' protection
		// ability and so on) is exactly the bot-exhaustible livelock the
		// acceptance gate would catch: the bot chooses the first
		// "activate" option forever instead of passing, and a free or
		// low-cost general ability then never lets the stack resolve.
		// Restricting to Equip (whose {N} cost is a hard per-turn cap the
		// bot cannot dodge) keeps the loop bounded. A sorcery-speed-only
		// ability (Equip's SorcerySpeed$ True) is offered only in a main
		// phase with an empty stack; its cost must be payable (checked
		// the same way castable checks a cast option's cost); and a
		// target-hungry ability needs at least one legal target on the
		// board for the option to be worth offering (askTarget would
		// otherwise fizzle the instant it is taken). Mana abilities were
		// already handled above, so they are skipped here.
		for ai, ab := range f.Abilities {
			if ab.Kind != "AB" || ab.Params["Keyword"] != "Equip" {
				continue
			}
			if ab.Params["SorcerySpeed"] == "True" && !sorcery {
				continue
			}
			cost := ParseCost(ab.Params["Cost"])
			if !e.castable(p, id, cost) {
				continue
			}
			if spec := ab.Params["ValidTgts"]; spec != "" && !e.hasLegalTarget(p, id, spec) {
				continue
			}
			out = append(out, decision.Option{Index: len(out), Kind: "activate",
				Label: "Activate " + f.Name + " " + ab.Params["SpellDescription"],
				Obj:   id, Mode: "ability", AbilityIndex: ai})
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
		if opt.Mode == "ability" {
			e.activateAbility(in.Player, opt.Obj, opt.AbilityIndex)
			return
		}
		e.emit(events.Event{Kind: events.Tap, Obj: opt.Obj})
		for _, ma := range o.Face().ManaAbilities() {
			e.resolveAbility(opt.Obj, in.Player, nil, ma, o.Face().SVars)
		}

	case "cast":
		e.emit(events.Event{Kind: events.Priority, Player: e.G.Priority, Amount: 0})
		e.beginCast(in.Player, opt)
	}
}
