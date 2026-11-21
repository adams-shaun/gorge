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
	{"a mana fee", func(c lCost) bool { return !manaFeeEmpty(c.Mana) }},
	{"an announced X cost", func(c lCost) bool { return c.XMin != 0 }},
	{"a self-sacrifice", func(c lCost) bool { return c.SacSelf }},
	{"a life payment", func(c lCost) bool { return c.PayLife > 0 || c.LifeX || c.LifeBy != nil }},
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

// restrictedManaCostPart names the part of c's cost beyond a plain {T}: a mana
// fee, a self-sacrifice, a life payment, a counter cost, a sacrifice of
// another permanent, a tap of another permanent or another cost part. "" when
// the cost is a plain {T} (or empty).
func restrictedManaCostPart(c lCost) string {
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
	return ""
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
