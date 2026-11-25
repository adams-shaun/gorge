package kgen

import "strings"

// A spend-restricted mana ability's cost beyond a plain {T}.
//
// The kernel's restricted gap search does not yet prove identical to gorge for
// a restricted source whose cost pays mana, sacrifices its source, pays life
// or taps another permanent, so every such ability is refused by name with the
// part that goes beyond the plain {T} instead of the old two-class reason. A
// counter cost, a sacrifice of another permanent and a tap of another
// permanent are outside the kernel's unrestricted walk too. The lowering and
// the lockstep evidence are the follow-up ticket
// (restrictedmana-cost-lockstep): no card is tolerated while that evidence is
// missing (AGENTS.md's lockstep rule).

// costPart is one named part of an activation cost; has reads it from c.
type costPart struct {
	name string
	has  func(lCost) bool
}

// restrictedCostParts names the cost parts beyond the plain {T}, in the order
// the refusal reports them: a mana fee first, then the irreversible parts.
// Each predicate is a small function of its own so the table's reader stays
// under the codeshape ceiling.
var restrictedCostParts = [...]costPart{
	{"an announced X cost", func(c lCost) bool { return c.XMin != 0 }},
	{"a self-sacrifice", func(c lCost) bool { return c.SacSelf }},
	{"a life payment", func(c lCost) bool { return c.PayLife > 0 || c.LifeX || c.LifeBy != nil || c.LifeIdent }},
	{"a counter cost", func(c lCost) bool { return c.SubCtr != 0 || c.SubCtrX || c.SubAmongN != 0 }},
	{"a sacrifice of another permanent", func(c lCost) bool { return c.Sac != nil || c.SacN != 0 || c.SacX }},
	{"a tap of another permanent", func(c lCost) bool { return c.TapN != 0 || c.TapXVar || c.TapHost }},
	{"an untap cost", func(c lCost) bool { return c.Untap }},
	{"a discard cost", func(c lCost) bool {
		return c.DiscardN != 0 || c.DiscardSpec != "" || c.DiscardHand || c.DiscardSelf || c.DiscardF != nil ||
			c.Discard2N != 0 || c.Discard2F != nil || c.DiscardX
	}},
	{"an exile cost", func(c lCost) bool {
		return c.ExileG != nil || c.ExileGSelf || c.ExileHandN != 0 || c.ExileHand || c.ExileSelf ||
			c.ExileStackSelf || c.ExileTrigNew || c.ExileGBf
	}},
	{"a cost of the granting host", func(c lCost) bool { return c.SacHost || c.HostExile }},
	{"a forage cost", func(c lCost) bool { return c.Forage != nil }},
	{"an energy cost", func(c lCost) bool { return c.Energy != 0 || c.EnergyBy != nil }},
	{"a waterbend cost", func(c lCost) bool { return c.Waterbend != 0 || c.WaterbendX }},
	{"a blight cost", func(c lCost) bool { return c.BlightN != 0 }},
	{"a return-to-hand cost", func(c lCost) bool { return c.ReturnSelf || c.Return != nil }},
	{"a draw cost", func(c lCost) bool { return c.DrawN != 0 || c.DrawWho != "" }},
	{"a damage cost", func(c lCost) bool { return c.DamageYouN != 0 }},
	{"a collect-evidence cost", func(c lCost) bool { return c.EvidenceN != 0 || c.EvidenceX }},
	{"a behold cost", func(c lCost) bool { return c.BeholdN != 0 || c.BeholdF != nil }},
	{"a mill cost", func(c lCost) bool { return c.MillN != 0 }},
	{"a reveal cost", func(c lCost) bool { return c.RevealSelf || c.RevealN != 0 || c.RevealF != nil || c.RevealChosenT }},
	{"a put-into-library cost", func(c lCost) bool { return c.PutLibSelf }},
	{"an exert cost", func(c lCost) bool { return c.Exert }},
	{"an add-counter cost", func(c lCost) bool { return c.AddCtr != 0 || c.AddCtrKnd != "" }},
	{"a tap-power cost", func(c lCost) bool { return c.TapPowN != 0 || c.TapPowCrew || c.TapPowSaddle }},
}

// restrictedManaCostPart names what keeps c, a spend-restricted ability's cost,
// out of the kernel's restricted units: an unsupported mana fee (a hybrid,
// two-brid, Phyrexian, snow or {X} part), another cost part, or — when the cost
// is a plain {T} or a supported fee plus {T} — the production m the model does
// not carry. "" when nothing does (the ability lowers).
func restrictedManaCostPart(c lCost, m *lMana) string {
	if !manaFeeSupported(c.Mana) {
		return "a mana fee " + c.Mana.String()
	}
	for _, p := range restrictedCostParts {
		if p.has(c) {
			return p.name
		}
	}
	if !c.Tap {
		return "a cost without {T}"
	}
	return restrictedProductionPart(m)
}

// restrictedProductionPart names the production a restricted unit does not
// model, when the cost is a plain {T} or a supported fee plus {T}.
func restrictedProductionPart(m *lMana) string {
	if m == nil {
		return "a production the kernel does not model"
	}
	switch {
	case m.Colors == "AnyDifferent":
		return "an AnyDifferent production (two mana of different colours)"
	case m.Each && len(m.Colors) > 1:
		return "an all-at-once production"
	case m.Choice && m.Colors != "WUBRG":
		return "a colour-choice production"
	case m.Colors == "WUBRG":
		return "an any-colour production (one mana of any colour)"
	case m.Colors == "Chosen":
		return "a chosen-colour production"
	}
	return "a production the kernel's restricted fee unit does not model"
}

// onePipColour reports whether p's pips name at most one colour (any count of
// it): the fee a kernel fee unit carries (pcard.manaFeeN) is one pip mask and
// one count.
func onePipColour(p [5]int) bool {
	n := 0
	for _, v := range p {
		if v != 0 {
			n++
		}
	}
	return n <= 1
}

// manaFeeEmpty reports whether m has no mana part at all.
func manaFeeEmpty(m manaCost) bool {
	return m.None || m.Generic == 0 && m.Pips == [5]int{} && m.C == 0 && len(m.Hybrid) == 0 &&
		len(m.Twobrid) == 0 && len(m.Phyrexian) == 0 && !m.X && !m.XX && !m.XXX && m.Snow == 0 && m.XColor == 0
}

// manaFeeSupported reports whether the mana part is a plain generic/coloured
// fee, not an announced {X}, hybrid, two-brid, Phyrexian or snow part.
func manaFeeSupported(m manaCost) bool {
	if m.None {
		return true
	}
	return !m.X && !m.XX && !m.XXX && len(m.Hybrid) == 0 && len(m.Twobrid) == 0 &&
		len(m.Phyrexian) == 0 && m.Snow == 0 && m.XColor == 0
}

// restrictedManaUnit reports whether a restricted mana ability whose cost is
// literal mana plus {T} (Castle Garenbrig's "{2}{G}{G}, {T}: Add six {G}") is one
// the kernel plans as a restricted unit of its own: gorge's cast-window probe
// prices the fee like an unrestricted one, paid from the floating mana before
// the production joins (engine/rules cast_window.go
// castWindowGenericCostShape, CR 605.3b), so the kernel charges the fee from
// the mana the payment already holds (dirac mana_restrictedsource.go). One
// fixed colour or {C}, a const amount of one to seven, no choice, each-colour
// production or rider flag; the spell and the activated-ability restriction
// are both allowed (kop.RestrictedManaCostShape).
func restrictedManaUnit(c lCost, m *lMana) bool {
	if !c.Tap || c.XMin != 0 || manaFeeEmpty(c.Mana) || !manaFeeSupported(c.Mana) || c.Mana.C != 0 || c.Mana.Snow != 0 ||
		!onePipColour(c.Mana.Pips) {
		return false
	}
	c.Mana = manaCost{}
	if !plainTapCost(c) {
		return false
	}
	return restrictedFeeProduction(m) && m.Special == "" && !m.NonSpell && !m.Deferred &&
		!m.NoCounter && m.Split == nil && m.HasteValid == nil
}

// restrictedFeeProduction reports whether a fee unit's production m is a
// shape the kernel's restricted unit carries: one fixed colour or {C} of a
// literal amount, or N mana in any combination of colours (Combo Any: Great
// Hall of the Citadel, Orb of Dragonkind), or one mana of each of a fixed set
// of colours (Cormela, Glamour Thief's {U}{B}{R}), or one mana of any colour
// (Hydro-Channeler's Produced$ Any, a fee alternative beside a free restricted
// one: refuseMultiMana refuses it anywhere else, refuseFeeAnyAlone), or two
// mana of different colours (Combo AnyDifferent, Interplanar Beacon) as two
// any-colour units the availability model couples (avail.nDist); a larger
// amount of a distinct pair has no such model and keeps the refusal.
func restrictedFeeProduction(m *lMana) bool {
	if !m.Amount.isConst() || m.Amount.N < 1 || m.Amount.N > maxRestrictedFeeAmount {
		return false
	}
	switch {
	case m.Colors == "AnyDifferent":
		return m.Choice && !m.Each && m.Amount.N == 2
	case m.Each:
		// one mana of each of a fixed set of colours (Cormela's {U}{B}{R}),
		// two to five colours, each a literal amount
		return !m.Choice && len(m.Colors) >= 2 && strings.Trim(m.Colors, "WUBRG") == ""
	case m.Choice:
		return m.Colors == "WUBRG"
	case m.Colors == "WUBRG":
		return m.Amount.N == 1
	default:
		return len(m.Colors) == 1 && strings.Contains("WUBRGC", m.Colors)
	}
}

// restrictedLifeTap reports whether c is exactly "{T}, Pay N life" (N a
// fixed count): the cost of a restricted last-resort unit (kernel
// mana_restrictedcost.go). A source with another mana ability stays refused
// by refuseMultiMana, which models a life ability only unrestricted.
func restrictedLifeTap(c lCost) bool {
	if !c.Tap || c.PayLife < 1 {
		return false
	}
	c.PayLife = 0
	return plainTapCost(c)
}

// restrictedLifeUnit reports whether a restricted "{T}, Pay N life" ability
// producing m is one the kernel plans as a last-resort unit: one fixed colour
// or {C}, or two mana of any one colour (Thieving Varmint), no choice,
// each-colour production, rider or activated-ability restriction
// (kop.RestrictedLifeShape).
func restrictedLifeUnit(c lCost, m *lMana) bool {
	col := len(m.Colors) == 1 && strings.Contains("WUBRGC", m.Colors) ||
		m.Colors == "WUBRG" && m.Amount.isConst() && m.Amount.N >= 1 && m.Amount.N <= 2
	return restrictedLifeTap(c) && col && !m.Choice && !m.Each &&
		m.Special == "" && m.RestrictAb == nil && !m.NonSpell && !m.Deferred && !m.NoCounter && m.Split == nil && m.HasteValid == nil
}

// restrictedLifeBackFace reports whether the lowered face is a later face of a
// multi-face card (its name carries "#<index>"): the restricted life lockstep
// plays front faces only, and a back face's cast mode (Life of Toshiro
// Umezawa // Memory of Toshiro) diverges in the pod, so it stays refused.
func (lc *lowerCtx) restrictedLifeBackFace() bool { return strings.Contains(lc.name, "#") }

// restrictedSacSelfUnit reports whether a restricted "{T}, Sacrifice this"
// ability producing m is one the kernel plans as a sacrificing last-resort
// unit: one fixed colour or {C}, or one to three mana of any one colour
// (Jeweled Lotus), no choice, each-colour production, rider or
// activated-ability restriction (kop.RestrictedSacSelfShape).
func restrictedSacSelfUnit(c lCost, m *lMana) bool {
	if !c.Tap || !c.SacSelf {
		return false
	}
	c.SacSelf = false
	col := len(m.Colors) == 1 && strings.Contains("WUBRGC", m.Colors) ||
		m.Colors == "WUBRG" && m.Amount.isConst() && m.Amount.N >= 1 && m.Amount.N <= 3
	return plainTapCost(c) && col && !m.Choice && !m.Each &&
		m.Special == "" && m.RestrictAb == nil && !m.NonSpell && !m.Deferred && !m.NoCounter && m.Split == nil && m.HasteValid == nil
}

// restrictedSacXUnit reports whether a restricted "{T}, Sacrifice another
// <filter>: Add N of one fixed colour" ability (Master of Dark Rites' three
// {B}, Vampire, Cleric and Demon spells only) is one the kernel's tap-another
// search funds as a census gap (kop.SacXRestrManaAb, mana_tapanother.go):
// exactly one other permanent sacrificed, a const amount of one to three mana
// of one fixed colour or {C}, no choice, each-colour production, rider or
// activated-ability restriction.
func restrictedSacXUnit(c lCost, m *lMana) bool {
	if !c.Tap || c.Sac == nil || c.SacN != 1 || c.SacX {
		return false
	}
	c.Sac, c.SacN = nil, 0
	col := len(m.Colors) == 1 && strings.Contains("WUBRGC", m.Colors)
	return plainTapCost(c) && col && m.Amount.isConst() && m.Amount.N >= 1 && m.Amount.N <= 3 &&
		!m.Choice && !m.Each && m.Special == "" && m.RestrictAb == nil && !m.NonSpell && !m.Deferred &&
		!m.NoCounter && m.Split == nil && m.HasteValid == nil
}

// restrictedTapXUnit reports whether a restricted "Tap an untapped <spec> you
// control" ability (no {T} of its own) producing m is one the kernel prices:
// one or two mana of one fixed colour or {C}, no choice, each-colour
// production, rider or flag; the spell and the activated-ability restriction
// are both allowed (kop.RestrictedTapXOneShape, Grand Architect).
func restrictedTapXUnit(c lCost, m *lMana) bool {
	if c.Tap || c.TapN != 1 || c.TapPowN != 0 {
		return false
	}
	c.TapN, c.TapF, c.Tap = 0, nil, true
	return plainTapCost(c) && len(m.Colors) == 1 && strings.Contains("WUBRGC", m.Colors) &&
		m.Amount.isConst() && m.Amount.N >= 1 && m.Amount.N <= 2 && !m.Choice && !m.Each &&
		m.Special == "" && !m.NonSpell && !m.Deferred && !m.NoCounter && m.Split == nil && m.HasteValid == nil
}
