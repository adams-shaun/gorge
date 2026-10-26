package rules

import (
	"slices"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

// defiler_reducecolor.go is the Defiler cycle's optional cost-for-reduction
// (`Mode$ OptionalCost | ValidCard$ Permanent.Green | ValidSA$ Spell |
// Activator$ You | Cost$ PayLife<2> | ReduceColor$ G`): "As an additional
// cost to cast green permanent spells, you may pay 2 life. Those spells cost
// {G} less to cast if you paid life this way." Each static is an independent
// optional additional cost (CR 601.2b) paired with a one-pip reduction of its
// colour that never spills to generic (CR 601.2f).

// reduceColorView is one applicable ReduceColor$ OptionalCost static: its
// additional cost and the ManaIndex of the colour whose pip it removes.
type reduceColorView struct {
	raw   string
	cost  Cost
	color int
}

// reduceColorOptionalViews returns the OptionalCost statics carrying a valid
// ReduceColor$ (W/U/B/R/G) that apply to the spell id p casts, in collector
// order. With no OptionalCost static it is a length check.
func (e *Engine) reduceColorOptionalViews(statics costStaticViews, p state.PlayerID, id state.ObjID) []reduceColorView {
	if len(statics.optional) == 0 {
		return nil
	}
	var out []reduceColorView
	for _, sv := range statics.optional {
		// ReduceColor$ is one of W/U/B/R/G; ManaIndex maps anything else to
		// colourless.
		col := sv.ParamStr(cards.PKReduceColor)
		if len(col) != 1 || state.ManaIndex(col[0]) == state.MC ||
			!e.costStaticApplies(sv, "OptionalCost", p, id, spellScope(""), nil, false) {
			continue
		}
		raw := sv.ParamStr(cards.PKCost)
		if c := ParseCost(raw); len(c.Unknown) == 0 {
			out = append(out, reduceColorView{raw: raw, cost: c, color: state.ManaIndex(col[0])})
		}
	}
	return out
}

// onePip is the CostMod a paid ReduceColor$ optional cost adds: one pip of
// colour ci, never generic.
func onePip(ci int) costMod {
	m := costMod{HasColor: true, IgnoreGeneric: true}
	m.Colored[ci] = 1
	return m
}

// reducedOptionalCastable is the offer gate's second chance for a plain cast
// that is unaffordable at full price: CR 601.2b lets the caster pay every
// applicable Defiler's life, and CR 119.4 lets them do so only while their
// life covers it. The spell is offered when it is payable at that
// most-reduced composition; the per-Defiler choices are asked mid-cast
// (defilerAsk). With no ReduceColor$ static it does nothing.
func (w *legalWalk) reducedOptionalCastable(p state.PlayerID, id state.ObjID, base Cost) bool {
	statics := w.costStatics.get()
	views := w.e.reduceColorOptionalViews(statics, p, id)
	if len(views) == 0 {
		return false
	}
	budget := w.e.G.Players[p].Life - max(base.Life, 0)
	if w.e.playerLifeChangeForbidden(p) || w.e.playerLifeLossForbidden(p) {
		budget = 0
	}
	left := base.Colored
	var extra []costMod
	for _, v := range views {
		if left[v.color] <= 0 || v.cost.Life > budget {
			continue
		}
		left[v.color]--
		budget -= v.cost.Life
		base = base.Plus(v.cost)
		extra = append(extra, onePip(v.color))
	}
	if len(extra) == 0 {
		return false
	}
	return w.e.offerCastableUsingExtra(statics, p, id, &base, spellScope(""), false, w.pricing(), extra)
}

// positiveDecimal renders n > 0 in decimal for a prompt (the string-machinery
// ratchet keeps strconv out of this package's new code).
func positiveDecimal(n int32) string {
	var b [10]byte
	i := len(b)
	for ; n > 0; n /= 10 {
		i--
		b[i] = byte('0' + n%10)
	}
	return string(b[i:])
}

// lifePayable reports whether p can pay n life in total: CR 119.4 (a player
// can pay life only while their life total is at least that much) and
// CR 119.8 (a player who can't lose life can't pay it).
func (e *Engine) lifePayable(p state.PlayerID, n int32) bool {
	if n <= 0 {
		return true
	}
	return e.G.Players[p].Life >= n && !e.playerLifeChangeForbidden(p) && !e.playerLifeLossForbidden(p)
}

// foldDefilerMods re-adds the answered reductions after a modifier refresh
// (repriceForTargets recomputes pc.mods from the statics, which do not know
// the cast-time answers).
func (pc *pendingCast) foldDefilerMods() {
	if pc.defilerRed.HasColor {
		pc.mods.Reduces = append(slices.Clone(pc.mods.Reduces), pc.defilerRed)
	}
}

// defilerAsk poses, one yes/no per applicable Defiler in collector order,
// "pay [the Cost$]: this spell costs {C} less?" (CR 601.2b). An ask is posed
// only when the life is payable (CR 119.4) and the spell's cost still has a
// pip of that colour after the earlier reductions. The answer folds into the
// pending cost (castAnswer, castAnswerDefiler).
func (e *Engine) defilerAsk() bool {
	pc := e.cast
	if pc == nil || pc.defilerDone || pc.isAbility() {
		return false
	}
	views := e.reduceColorOptionalViews(e.collectCostStatics(), pc.player, pc.card)
	for int(pc.defilerNext) < len(views) {
		v := views[pc.defilerNext]
		pc.defilerNext++
		after := pc.mods.Apply(pc.cost)
		if after.Colored[v.color] <= 0 || !e.lifePayable(pc.player, pc.cost.Life+v.cost.Life) ||
			!pay.NonManaCastable(asPayer(e), pc.player, pc.card, pc.cost.Plus(v.cost), false, tapCostSAKind(e.pcAbility(pc))) {
			continue
		}
		pc.defilerAsked = true
		pc.defilerParam = v.raw
		pc.defilerColor = int32(v.color)
		name := "this spell"
		if o := e.G.Obj(pc.card); o != nil && o.Face() != nil {
			name = o.Face().Name
		}
		pip := "{" + string("WUBRGC"[v.color]) + "}"
		pays := "Pay the optional cost"
		if v.cost.Life > 0 {
			pays = "Pay " + positiveDecimal(v.cost.Life) + " life"
		}
		d := &decision.Decision{Player: pc.player, Kind: decision.KChoose, Min: 1, Max: 1,
			Prompt: pays + ": " + name + " costs " + pip + " less?", Source: pc.card}
		d.Options = append(d.Options,
			decision.Option{Index: 0, Kind: "yes", Label: pays},
			decision.Option{Index: 1, Kind: "no", Label: "Don't pay"})
		e.choosing = chooseCast
		e.ask(d)
		return true
	}
	pc.defilerDone = true
	return false
}

// answerDefiler records the yes/no for the Defiler defilerAsk just posed: yes
// adds its Cost$ to the pending cost and one pip of its colour to the
// reductions pc.mods (and every later refresh) applies.
func (e *Engine) answerDefiler(pc *pendingCast, chosen []decision.Option) {
	if !pc.defilerAsked {
		return
	}
	pc.defilerAsked = false
	if len(chosen) == 0 || chosen[0].Kind != "yes" {
		return
	}
	pc.cost = pc.cost.Plus(ParseCost(pc.defilerParam))
	pc.defilerTimes++
	if !pc.defilerRed.HasColor {
		pc.defilerRed = costMod{HasColor: true, IgnoreGeneric: true}
	}
	pc.defilerRed.Colored[pc.defilerColor]++
	pc.mods.Reduces = append(slices.Clone(pc.mods.Reduces), onePip(int(pc.defilerColor)))
}
