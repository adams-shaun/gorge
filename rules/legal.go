package rules

import (
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// sorcerySpeed reports whether p may take a sorcery-speed action right now.
func (e *Engine) sorcerySpeed(p state.PlayerID) bool {
	return e.G.Active == p && e.G.Step.IsMain() && len(e.G.Stack) == 0
}

// abilityZoneOK reports whether ability ab may be activated while the
// source cardinal is in zone z (CR 602.1b): the printed ActivationZone$
// when present, the battlefield by default. Values other than Battlefield /
// Graveyard (Hand, Command, Exile, Stack) are not enumerated by this
// build's zone walk, so they simply never offer an option.
func abilityZoneOK(ab *cards.SA, z state.Zone) bool {
	az, ok := ab.Params["ActivationZone"]
	if !ok {
		return z == state.ZBattlefield
	}
	switch strings.TrimSpace(az) {
	case "Battlefield":
		return z == state.ZBattlefield
	case "Graveyard":
		return z == state.ZGraveyard
	}
	return false
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
		// The "activate" (tap for mana) option is offered only while at least
		// one mana ability on this permanent is itself unrestrictable: a
		// CantBeActivated whose ValidSA$ expressly spares mana abilities
		// (Activated.!ManaAbility) must not shut the land's tap-for-mana off
		// (Task 10). The option is still the permanent's whole mana ability
		// set, tapped together, exactly as before.
		if len(f.ManaAbilities()) > 0 {
			restricted := true
			for _, ma := range f.ManaAbilities() {
				if !e.abilityRestricted(p, id, ma) {
					restricted = false
					break
				}
			}
			if !restricted {
				add("activate", "Tap "+f.Name+" for mana", id)
			}
		}
	}

	// Activated abilities (Task 10): every non-mana AB$ ability on a
	// permanent p controls, and every one on a card in p's graveyard whose
	// ActivationZone$ names the graveyard, offered as an "ability" option the
	// way a cast is (rules/activate.go resolves one). Each is gated on the
	// same rules the cast options are: its own zone matches ActivationZone$,
	// SorcerySpeed$ True needs a full sorcery window, no CantBeActivated
	// restriction scopes down to it, a {T} cost needs an untapped source that
	// is neither tapped nor (for a creature, CR 302.6) summoning-sick without
	// Haste, and -- the totality gate -- the whole cost must be castable
	// (mana payable, every Sac/SubCounter part satisfiable) before the option
	// is ever offered.
	for _, z := range []state.Zone{state.ZBattlefield, state.ZGraveyard} {
		for _, id := range e.G.Zone(z, p) {
			o := e.G.Obj(id)
			f := o.Face()
			if f == nil {
				continue
			}
			for i, ab := range f.Abilities {
				if ab.Kind != "AB" || ab.API == "Mana" {
					continue
				}
				if !abilityZoneOK(ab, z) {
					continue
				}
				if ab.Params["SorcerySpeed"] == "True" && !sorcery {
					continue
				}
				if e.abilityRestricted(p, id, ab) {
					continue
				}
				cost := ParseCost(ab.Params["Cost"])
				if cost.Tap && (o.Tapped || (z == state.ZBattlefield && o.SummonSick && !e.HasKeyword(id, "Haste"))) {
					continue
				}
				if !e.castable(p, id, cost) {
					continue
				}
				out = append(out, decision.Option{Index: len(out), Kind: "ability",
					Label: f.Name + ": " + ab.Params["SpellDescription"], Obj: id, Ability: i})
			}
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
		// Task 12: a land with an "as this enters" choice (an
		// ETBReplacement whose ReplaceWith$ is NameCard/ChooseType/
		// ChooseNumber, e.g. Cavern of Souls) goes through the same
		// one-stage cast flow a spell does -- collect the choice, ask it via
		// chooseETB, record it with a Choose event, then commitCast moves the
		// land and logs the play. A land with none keeps the original direct
		// path (no pendingCast, no flow), so ordinary lands are untouched.
		// Both paths share the same continuation machinery: etbAnswer/
		// continueCast/commitCast below, never a parallel one.
		pc := &pendingCast{player: in.Player, card: opt.Obj, from: state.ZHand, mode: "land", ability: -1}
		e.cast = pc
		e.collectETBChoices(in.Player)
		if len(pc.etbs) == 0 {
			e.cast = nil
			e.emit(events.Event{Kind: events.MoveZone, Obj: opt.Obj,
				From: state.ZHand, To: state.ZBattlefield})
			e.emit(events.Event{Kind: events.LandPlayed, Player: in.Player})
			return
		}
		e.continueCast()

	case "activate":
		e.emit(events.Event{Kind: events.Priority, Player: e.G.Priority, Amount: 0})
		o := e.G.Obj(opt.Obj)
		e.emit(events.Event{Kind: events.Tap, Obj: opt.Obj})
		// Fix r1 (reviewer Important 2): gate and activation must agree on
		// WHICH mana ability is activated. legalActions offers the tap-for-
		// mana option iff at least one of the permanent's mana abilities is
		// unrestricted; resolving every one here would also resolve a
		// restricted member (e.g. CantBeActivated scoping down to a single
		// mana ability), activating an ability whose restriction was never
		// checked. Skip each mana ability abilityRestricted flags so the
		// exact set resolved is the exact set the offer gate found legal.
		for _, ma := range o.Face().ManaAbilities() {
			if e.abilityRestricted(in.Player, opt.Obj, ma) {
				continue
			}
			e.resolveAbility(opt.Obj, in.Player, nil, ma, o.Face().SVars)
		}

	case "ability":
		// Task 10: an activated ability (non-mana AB$) was chosen. Reset the
		// pass count the same way every other non-pass action does, then drive
		// the same cost flow a cast drives (rules/activate.go's
		// beginActivation -> pendingCast -> continueCast -> commitCast).
		e.emit(events.Event{Kind: events.Priority, Player: e.G.Priority, Amount: 0})
		e.beginActivation(in.Player, opt)

	case "cast":
		e.emit(events.Event{Kind: events.Priority, Player: e.G.Priority, Amount: 0})
		e.beginCast(in.Player, opt)
	}
}
