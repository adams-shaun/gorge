package effects

import (
	"slices"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// enteringDefender is one CR 508.4 defender a creature put onto the
// battlefield attacking can attack: Player is the defending seat (the
// planeswalker's controller or the battle's protector when Obj is set), Obj
// the planeswalker or battle, 0 for the player itself.
type enteringDefender struct {
	Player state.PlayerID
	Obj    state.ObjID
}

// enteringDefenders lists the CR 508.4 candidates for a creature entering
// attacking under controller, in a fixed order: every living seat other than
// controller (AliveFrom(0) order); then each such seat's face-up
// planeswalkers (battlefield zone order); then every battle whose recorded
// protector is a living seat other than controller (AliveFrom(0) controller
// walk, battlefield zone order) -- the same eligibility battleDefenders
// (engine/rules/attack_cost.go) applies to declared attacks. objects false
// lists the players only.
func enteringDefenders(h Host, controller state.PlayerID, objects bool) []enteringDefender {
	g := h.Game()
	var out []enteringDefender
	for _, q := range g.AliveFrom(0) {
		if q != controller {
			out = append(out, enteringDefender{Player: q})
		}
	}
	if !objects {
		return out
	}
	for _, q := range g.AliveFrom(0) {
		if q == controller {
			continue
		}
		for _, id := range g.Zone(state.ZBattlefield, q) {
			o := g.Obj(id)
			if o != nil && o.Controller == q && !o.FaceDown && slices.Contains(h.Chars(id).Types, "Planeswalker") {
				out = append(out, enteringDefender{Player: q, Obj: id})
			}
		}
	}
	for _, q := range g.AliveFrom(0) {
		for _, id := range g.Zone(state.ZBattlefield, q) {
			o := g.Obj(id)
			if o == nil || o.FaceDown || !o.ProtectorValid || o.Protector == controller ||
				int(o.Protector) >= len(g.Players) || g.Players[o.Protector].Lost {
				continue
			}
			if slices.Contains(h.Chars(id).Types, "Battle") {
				out = append(out, enteringDefender{Player: o.Protector, Obj: id})
			}
		}
	}
	return out
}

// resolveEnteringDefender is CR 508.4's defender for one "enters attacking"
// effect call whose text does not name the defender (Attacking$ True,
// TokenAttacking$ True). ok false means the creature enters but does not
// attack (no combat defender exists for it). See the cr-g11b spec.
func resolveEnteringDefender(h Host, c *Ctx, controller state.PlayerID, objects bool, noun string) (enteringDefender, bool) {
	g := h.Game()
	if c.DefendingPlayer.IsPlayer {
		fb := enteringDefender{Player: c.DefendingPlayer.Player, Obj: c.DefendingBattle}
		if c.DefenderFixed {
			return fb, true
		}
		return chooseEnteringDefender(h, c, controller, fb, objects, noun), true
	}
	if g.Step < state.StepDeclareAttackers || g.Step > state.StepEndCombat || controller != g.Active {
		return enteringDefender{}, false
	}
	cands := enteringDefenders(h, controller, objects)
	if len(cands) == 0 {
		return enteringDefender{}, false
	}
	return chooseEnteringDefender(h, c, controller, cands[0], objects, noun), true
}

// chooseEnteringDefender asks controller which candidate the entering
// creature attacks, fallback first; with fewer than two candidates, or with
// no answer served (the R-9 stand-in), it is fallback.
func chooseEnteringDefender(h Host, c *Ctx, controller state.PlayerID, fallback enteringDefender, objects bool, noun string) enteringDefender {
	cands := enteringDefenders(h, controller, objects)
	if i := slices.Index(cands, fallback); i >= 0 {
		cands = slices.Delete(cands, i, i+1)
	}
	cands = slices.Insert(cands, 0, fallback)
	if len(cands) < 2 {
		return fallback
	}
	g := h.Game()
	d := &decision.Decision{Player: controller, Kind: decision.KChoose, Source: c.Source, Min: 1, Max: 1,
		ResumeKind: "entering_defender",
		Prompt:     "Choose which player, planeswalker or battle the entering " + noun + " attacks"}
	for _, cd := range cands {
		opt := decision.Option{Index: len(d.Options), Kind: "player", Player: cd.Player}
		if cd.Obj != 0 {
			opt.Kind, opt.Obj, opt.Label = "card", cd.Obj, objName(g, cd.Obj)
		} else if int(cd.Player) < len(g.Players) {
			opt.Label = g.Players[cd.Player].Name
		}
		d.Options = append(d.Options, opt)
	}
	if ans, ok := AskTape(h, d); ok && len(ans) == 1 {
		return enteringDefender{Player: ans[0].Player, Obj: ans[0].Obj}
	}
	return fallback
}
