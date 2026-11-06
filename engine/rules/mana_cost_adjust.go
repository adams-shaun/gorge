package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

// A mana ability is an activated ability, so its total cost is composed like
// any other's (CR 605.1a, 602.2b -> 601.2f): the RaiseCost and ReduceCost
// statics that reach it add to and take from its mana cost before it is
// paid. Forge prices it the same way (CostPayment's constructor runs
// CostAdjustment.adjust on every non-trigger ability). Blossoming Tortoise's
// "activated abilities of lands you control cost {1} less" turns Crystal
// Grotto's "{1}, {T}: add one mana of any color" into "{T}: ..."; a static
// that exempts mana abilities (ValidSpell$ Activated.!ManaAbility) leaves them
// alone, which the shared static gate decides.

// costStaticMayReachManaAbility is a cheap, conservative pre-test over one
// collected cost static: false only when the static cannot apply to an
// activated ability that is a mana ability, so a board holding none of the
// others prices a mana ability as it always did. The exact answer is the
// static gate's (costStaticGate); this only keeps the common spell-cost
// statics off the mana-tap path.
func costStaticMayReachManaAbility(sv staticView) bool {
	switch sv.ParamStr(cards.PKType) {
	case "Spell", "Foretell":
		return false
	}
	raw := sv.ParamStr(cards.PKValidSpell)
	if raw == "" {
		return true
	}
	for start := 0; start <= len(raw); {
		end := start
		for end < len(raw) && raw[end] != ',' {
			end++
		}
		if validSpellTokenMayBeManaAbility(raw[start:end]) {
			return true
		}
		start = end + 1
	}
	return false
}

// validSpellTokenMayBeManaAbility: one ValidSpell$ alternative can match a
// mana ability only if it names activated abilities and neither excludes mana
// abilities nor names a keyword ability no mana ability is.
func validSpellTokenMayBeManaAbility(tok string) bool {
	for len(tok) > 0 && tok[0] == ' ' {
		tok = tok[1:]
	}
	for len(tok) > 0 && tok[len(tok)-1] == ' ' {
		tok = tok[:len(tok)-1]
	}
	const head, notMana = "Activated", "!ManaAbility"
	if len(tok) < len(head) || tok[:len(head)] != head {
		return false
	}
	rest := tok[len(head):]
	switch rest {
	case ".Equip", ".Cycling", ".Ninjutsu", ".Boast", ".Exhaust", ".PowerUp", ".Loyalty", ".Channel":
		return false
	}
	for i := 0; i+len(notMana) <= len(rest); i++ {
		if rest[i:i+len(notMana)] == notMana {
			return false
		}
	}
	return true
}

// markCostManaReaching sets views.manaReaching from the raise/reduce/set
// lists. Every finalisation site of those lists calls it once
// (scanCostStatics and the fused board-statics build, walkcache.go), so the
// per-ability gate pricedManaCC reads is a field read, not a rescan of the
// collected statics.
func markCostManaReaching(views *costStaticViews) {
	for _, list := range [...][]staticView{views.raise, views.reduce, views.set} {
		for _, sv := range list {
			if costStaticMayReachManaAbility(sv) {
				views.manaReaching = true
				return
			}
		}
	}
}

// anyManaReaching reports whether some collected raise, reduce or set static
// may reach a mana ability: the mark the lists' finalisation left
// (markCostManaReaching).
func (s costStaticViews) anyManaReaching() bool {
	return s.manaReaching
}

// anyManaReachingCostStatic is costStaticViews.anyManaReaching for the fused
// board-statics build, memoised there: pricedManaCC asks once per mana
// ability per walk, and the scan walks every collected raise/reduce/set
// static. Outside a walk (no fused build to key) it scans as before.
func (e *Engine) anyManaReachingCostStatic() bool {
	if c, ok := e.boardStaticsRef(); ok {
		if c.manaReachingVer == c.ver {
			return c.manaReaching
		}
		c.manaReaching = c.v.cost.anyManaReaching()
		c.manaReachingVer = c.ver
		return c.manaReaching
	}
	if e.manaReachingValid && e.manaReachingLog == e.L && e.manaReachingTape == e.tapeEpoch &&
		e.manaReachingLogLen == len(e.L.Events) && e.manaReachingVersion == e.continuousVersion &&
		e.manaReachingObjs == len(e.G.Objs) {
		if derivedMemoVerify {
			if fresh := e.scanCostStatics().anyManaReaching(); fresh != e.manaReachingCost {
				panic("rules: mana-reaching cost-static memo is stale")
			}
		}
		return e.manaReachingCost
	}
	e.manaReachingCost = e.scanCostStatics().anyManaReaching()
	e.manaReachingValid = true
	e.manaReachingLog = e.L
	e.manaReachingTape = e.tapeEpoch
	e.manaReachingLogLen = len(e.L.Events)
	e.manaReachingVersion = e.continuousVersion
	e.manaReachingObjs = len(e.G.Objs)
	return e.manaReachingCost
}

// pricedManaCC is cc, the printed cost of p's mana ability ma on source,
// after the cost statics that reach it; cc itself (shared, read-only) when
// none does. The result is read-only like cc.
func (e *Engine) pricedManaCC(p state.PlayerID, source state.ObjID, ma *cards.SA, cc *compiledCost) *compiledCost {
	// The common board has no cost static that reaches a mana ability, and
	// the answer is memoised per state change, so ask it before collecting
	// the statics the composition would need: the scan is the whole price of
	// the common path.
	if !e.anyManaReachingCostStatic() {
		return cc
	}
	statics := e.collectCostStatics()
	mods := e.costModifiersWithTargetsUsing(statics, p, source, abilityScope(ma), nil, false)
	if mods.Empty() {
		return cc
	}
	c := cc.Cost
	mods.ApplyTo(&c)
	return pay.CompiledCostFromCost(c)
}

// manaAbilityCC is the compiled cost p pays to activate ma on source: the
// printed Cost$ composed with the cost statics (pricedManaCC).
func (e *Engine) manaAbilityCC(p state.PlayerID, source state.ObjID, ma *cards.SA) *compiledCost {
	return e.pricedManaCC(p, source, ma, e.abilityCompiledCost(ma))
}

// manaStaticFor is manaStaticOf with the cost-derived facts (free, tap-only,
// tapping, untapping) read from the priced cost, so a census that sorts p's
// mana abilities by what activating them takes sees an ability a cost static
// made free as free, and a bare tap a surcharge made paid as paid.
func (e *Engine) manaStaticFor(p state.PlayerID, source state.ObjID, ma *cards.SA) pay.ManaStatic {
	f := e.manaStaticOf(ma)
	cc := e.manaAbilityCC(p, source, ma)
	if cc == e.abilityCompiledCost(ma) {
		return f
	}
	f.FreeCost, f.TapOnly = pay.ManaFreeCost(cc.Cost), pay.PaymentPlanTapOnlyCost(cc.Cost)
	f.Tap, f.Untap = cc.Tap, cc.Untap
	return f
}
