package kop

import (
	"strings"
)

// AbilityKind is what kind of ability an Ability is.
type AbilityKind uint8

const (
	AbSpell       AbilityKind = iota // the spell ability of an instant/sorcery (or the permanent spell's own resolution)
	AbActivated                      // activated (non-mana) ability
	AbMana                           // mana ability (resolves immediately; autopay source)
	AbTriggered                      // triggered ability
	AbStatic                         // static ability
	AbReplacement                    // replacement-lite effect
	AbAltCost                        // alternative / additional cast mode (flashback, kicker branch carrier)
)

// Ability is one ability of a card.
type Ability struct {
	Kind     AbilityKind
	Cost     Cost
	Trig     Trigger
	Static   Static
	Repl     Repl
	Targets  [MaxTargets]TargetSpec
	NTargets uint8
	// Modes: a charm offers NModes modes and the caster picks ModesPick
	// (OpCharm branches on the chosen bits).
	NModes    uint8
	ModesPick uint8
	// ModesFew: picks the controller may decline; the minimum is
	// ModesPick-ModesFew (CR 700.2; gorge CharmModeBounds). 0 = exactly ModesPick.
	ModesFew uint8
	// ModeRestrict: a charm's ChoiceRestriction$ scope, "choose one that hasn't
	// been chosen [this turn]" (gorge CharmEligibleModes): 0 none, 1 ThisTurn,
	// 2 ThisGame. The kernel reads it only behind prep.anyCharmPick.
	ModeRestrict uint8
	// TgtModes[i] is the set of modes (bit per mode) that use target slot i;
	// 0 = every mode. A slot none of whose modes was chosen is not chosen.
	TgtModes [MaxTargets]uint8
	// Mana abilities: produces ManaN mana of colours ManaColors (ColAny: any one colour).
	ManaColors ColorMask
	ManaN      uint8
	// ManaColOrd records a Produced$ Combo's printed colour order when it is
	// not WUBRG (Jeskai Devotee's "Combo U R W"): three bits per printed
	// position from bit 0, each the WUBRG index (W=0 .. G=4) of that
	// position's colour. gorge's askManaColor offers ComboColours in printed
	// order and a mask cannot say it, so ManaColors supplies membership and
	// this the order. 0 = WUBRG order (single colours and Combo Any too).
	ManaColOrd uint16
	Prog       []Instr

	// Appended by dirac/kgen.
	AFlags       AbilityFlags
	ManaV        ValueRef // the mana amount when AbManaDynamic
	ManaRestrict FilterID // spend only on spells matching (0 = any)
	// ManaRestrictAb: ... or on activated abilities of objects matching
	// (Secluded Courtyard's "Activated.Creature+ChosenType"; 0 = none);
	// AfManaNonSpell adds every payment that is not a spell.
	ManaRestrictAb FilterID
	ActZone        Zone     // the zone the source must be in to activate (ZoneNone = battlefield)
	ManaAmong      FilterID // AfManaEachColor: the permanents whose colours are produced
	Gate           Gate     // a threshold gate: the ability exists only while it holds
	// OwnReduce: an activated hand ability's own ReduceCost$ (Channel): its
	// mana cost's generic part is reduced by this value, floored at zero,
	// at the offer and the charge alike (gorge ownReduceCost; CR 601.2f).
	// The zero value (Const 0) reduces nothing.
	OwnReduce ValueRef
	// Exhaust: an exhaust ability (CR 702.177a; Exhaust$ True): the class
	// test of Mode$ AbilityCast ValidSA$ Activated.Exhaust. AbilityFlags
	// has no free bit; this is a table byte, not State.
	Exhaust bool
	// Once: the ability's ordinal among its card's activate-only-once
	// abilities (CostOncePerGame), 1..3 in ability order; 0 = none. The
	// kernel keeps the first such ability's bit in pfOnceGame and the
	// second and third in the arena masks (dirac/legal.go onceUsed). A
	// table byte, not State.
	Once uint8
	// CantCopy: "this ability can't be copied" (CantCopy$ True; gorge
	// SpellCopyAllowed): an ExtCopyAbility naming its stack entry copies
	// nothing. A table byte, not State (AbilityFlags has no free bit).
	CantCopy bool
	// Evidence: CollectEvidence<N> in the activation cost (CR 701.59a; gorge
	// cast_targets.go evidenceAsk); 0 = none. A table byte: CostFlags has no
	// free bit.
	// A spell's AbSpell Evidence of EvidenceTargetsMV is a mandatory
	// CollectEvidence<X> additional cost whose X is the total mana value of
	// the spell's targets (Urgent Necropsy).
	Evidence uint8
	// GrantedKw: the trigger a granted keyword gives its holder (Firebending),
	// a layer-6 grant listed by timestamp among the grants, where a trigger a
	// static or an effect grants by AddTrigger$ is listed with its own kind
	// (grantedAbilitiesN). A table byte: AbilityFlags has no free bit.
	GrantedKw bool
	// MeleeGrant: the trigger a granted Melee gives its holder (CR
	// 702.121a; a static's AddKeyword$ Melee). gorge queues it in its own
	// keyword walk (rules/melee.go checkGrantedAttackPumpTriggers), not among the
	// AddTrigger$ grants: CardDef.AtkFace places it (kw_melee.go). A table
	// byte: AbilityFlags has no free bit.
	MeleeGrant bool
	// ManaEach: the mana ability adds ManaN mana of EACH colour in ManaColors
	// at once (Produced$ "W U", Timeless Lotus's {W}{U}{B}{R}{G}; CR 106.1),
	// not one of them. A table byte: AbilityFlags has no free bit.
	ManaEach bool
	// ManaDeferred: Pyromancer's Goggles' "{T}: Add {R}. When that mana is
	// spent ..." (TriggersWhenSpent$): gorge's payment planner defers the
	// source, so autopay never taps it and only a potential play's prefix
	// script or the manual mana window does (kernel mana_deferred.go). A
	// table byte: AbilityFlags has no free bit.
	ManaDeferred bool
	// ManaHasteValid: AddsKeywords$ Haste | AddsKeywordsValid$ ... |
	// AddsKeywordsUntil$ UntilEOT (Carnelian Orb of Dragonkind; CR 106.6):
	// a spell matching this filter, paid with this ability's mana, gains
	// haste until end of turn as it enters (0 = no rider). A table field.
	ManaHasteValid FilterID
	// RepMax: a charm that may choose a mode more than once (CanRepeatModes$)
	// has RepMax[m] > 0, the most times mode m can be chosen. stackEntry.mode
	// then holds a COUNT per mode instead of a bitmask: mode m's count sits
	// in bits.Len8(RepMax[m]) bits above the earlier modes' (RepeatLayout).
	// A table field, not State; repeat.go has the readers.
	RepMax [4]uint8
	// RepCost / RepBudget: a pawprint charm's per-mode cost and its budget
	// (CR 700.2i, 107.18). RepBudget 0 = no budget.
	RepCost   [4]uint8
	RepBudget uint8
}

// Timing is when a card may be cast.
type Timing uint8

const (
	TimingSorcery Timing = iota // main phase, own turn, empty stack
	TimingInstant               // any time with priority
	TimingLand                  // a land play
)

// AbilityFlags (Ability.AFlags).
type AbilityFlags uint16

const (
	AfLatent         AbilityFlags = 1 << iota // not active on the card: reached through an op, a grant or an emblem
	AfManaDynamic                             // the mana amount is ManaV
	AfManaChosen                              // produces one mana of the source's chosen colour
	AfOptionalCost                            // the ability's cost is offered on resolution ("you may pay")
	AfCheckOnTrigger                          // the intervening-if is checked only when the event happens
	AfEquip                                   // the Equip ability
	AfCharmTrigger                            // reserved
	AfManaEachColor                           // adds one mana of each colour among the controller's permanents matching ManaAmong (Bloom Tender); not an autopay source
	AfManaChoice                              // the mana ability's colour is a choice among ManaColors (Produced$ Combo ...), not simultaneous production
	AfGift                                    // a permanent's gift trigger (CR 702.174c): the gift is given (TrigGiveGift) as it is put on the stack
	AfResolveCount                            // a triggered ability whose resolutions this turn are counted (ExResolvedThisTurn)
	AfCycling                                 // a cycling ability: its discard-this cost is a cycle (Tf2Cycled triggers; gorge cyclingKeyword)
)

// AfStation: the Station ability (CR 702.184a, kcounters): an activated
// ability canonicalised as gorge's priority special action "station", its
// one-creature tap pick (CostTapX N=1) posed before the push and always
// asked (ChStation, gorge's KChoose "station"); the tapped creature is the
// entry's trigger object, whose power its program reads at resolution.
const AfStation AbilityFlags = AfCycling << 1

// AfNinjutsu: the K:Ninjutsu ability (CR 702.49): an
// ActZone hand ability offered only in the declare-blockers, combat-damage
// and end-of-combat steps; its Return pick captures the returned attacker's
// defender on the entry (trigObj) and its program is OpNinjutsu.
const AfNinjutsu AbilityFlags = AfStation << 1

// AfManaReflect: the mana ability adds one mana of any colour
// among the colours of the controller's permanents matching ManaAmong
// (AB$ ManaReflected, ReflectProperty$ Is; Plaza of Heroes). The colours
// are read at each use; none, and the ability adds nothing (gorge lists no
// such ability). Kernel mana_reflected.go. A printed one with no ManaAmong
// filter reads the colours of the cards its source imprinted (Chrome Mox;
// kernel mana_imprint.go); a granted one with none, the grantee's own
// colours (Katilda; kernel mana_owncolours.go).
const AfManaReflect AbilityFlags = AfNinjutsu << 1

// AfManaNonSpell the restricted mana ability
// may also pay every payment that is not a spell (an activated ability,
// echo, cumulative upkeep, a triggered cost): RestrictValid$
// CantCastNonArtifactSpells (the Powerstone), CR 106.6. ManaRestrict then
// holds the spell half (Card.Artifact).
const AfManaNonSpell AbilityFlags = AfManaReflect << 1

// Which mana-ability costs the kernel can activate.
//
// The kernel's mana model reads a permanent's FIRST mana ability (kernel
// prep.go manaAb) and honours it only for the cost shapes below; a card whose
// first mana ability costs anything else was silently dropped as a mana
// source (Saruli Caretaker's "{T}, tap an untapped creature you control" was
// never used: 0.00 of kernel board states against 0.31 in XMage, a priority
// comparison). kgen refuses every other shape (dirac/kgen buildCard); this file is the
// one statement of which shapes those are, shared by that refusal, the
// kernel's prep (which reads the same predicates) and the census test
// (dirac/kgen TestManaAbilityCostsAreHonoured).

// CtrManaAb reports whether mana ability a costs only counters put on its
// source, once each turn, with a fixed production of one colour (Wall of
// Roots; kernel mana_countercost.go), or is the all-at-once removal shape
// of CtrEachManaAb.
func CtrManaAb(a *Ability) bool {
	return a.Cost.Flags == CostAddCtr|CostOncePerTurn && a.Cost.Mana.MV() == 0 && a.Cost.Mana.X == 0 &&
		a.AFlags == 0 && a.ManaRestrict == 0 && a.ManaColors != 0 && a.ManaColors&(a.ManaColors-1) == 0 || CtrEachManaAb(a)
}

// CtrEachManaAb reports whether mana ability a is the all-at-once shape of
// CtrManaAb: "Remove N counters from this: Add {W}{W}{U}{U}...", once each
// turn (Ramos, Dragon Engine): every colour of ManaColors, ManaN of each.
func CtrEachManaAb(a *Ability) bool {
	return a.ManaEach && a.Cost.Flags == CostRemoveCtr|CostOncePerTurn && a.Cost.N > 0 && a.ManaColors&^ColAny == 0 && a.ManaN > 0 &&
		a.Cost.Mana.MV() == 0 && a.Cost.Mana.X == 0 && a.AFlags == 0 && a.ManaRestrict == 0 && len(a.Prog) == 0
}

// TapXManaAb reports whether mana ability a is "{T}, tap an untapped <spec>
// you control: Add one mana of any color" (Saruli Caretaker, Springleaf
// Drum): exactly one tapped permanent, no mana, a plain unrestricted
// any-colour unit (kernel mana_tapanother.go).
func TapXManaAb(a *Ability) bool {
	return a.Cost.Flags == CostTap|CostTapX && a.Cost.N == 1 && a.Cost.Mana.MV() == 0 && a.Cost.Mana.X == 0 &&
		a.AFlags == 0 && a.ManaRestrict == 0 && a.ManaColors == ColAny && a.ManaN <= 1 && len(a.Prog) == 0 &&
		a.Gate.Ctr == 0 && a.Trig.Cond == 0 && (a.ActZone == ZoneNone || a.ActZone == ZoneBattlefield)
}

// TapXManaTwo reports whether mana ability a is "Tap two untapped <spec> you
// control: Add one mana of any color" with no {T} (Baylen, the Haymaker's
// tokens): exactly two tapped permanents, no mana, a plain unrestricted any-
// colour unit. The source itself neither taps nor needs to be unsick.
func TapXManaTwo(a *Ability) bool {
	return a.Cost.Flags == CostTapX && a.Cost.N == 2 && a.Cost.Mana.MV() == 0 && a.Cost.Mana.X == 0 &&
		a.AFlags == 0 && a.ManaRestrict == 0 && a.ManaColors == ColAny && a.ManaN <= 1 && len(a.Prog) == 0 &&
		a.Gate.Ctr == 0 && a.Trig.Cond == 0 && (a.ActZone == ZoneNone || a.ActZone == ZoneBattlefield)
}

// TapXMana is either tap-other-permanents mana shape.
func TapXMana(a *Ability) bool { return TapXManaAb(a) || TapXManaTwo(a) }

// PipFeeManaAb reports whether a is an activate-only-once mana ability whose
// fee is exactly one coloured pip (with a {T}) and no generic part, and the
// pip's colour. Loot, the Pathfinder's "{G}, {T}: Add three mana of any one
// colour" is the shape: the kernel models it as a filter
// (dirac/mana_filterfirst.go), paying the pip from the pool. A mana ability
// that also sacrifices, pays life, or has no once limit is not one.
func PipFeeManaAb(a *Ability) (col int, ok bool) {
	m := &a.Cost.Mana
	if a.Kind != AbMana || a.Cost.Flags != CostTap|CostOncePerGame ||
		m.Generic != 0 || m.X != 0 || m.Hyb != ([4]ColorMask{}) || m.Tb != 0 ||
		a.ManaRestrict != 0 || a.ManaN == 0 || len(a.Prog) != 0 ||
		a.Gate.Ctr != 0 || a.Trig.Cond != 0 || a.ActZone != ZoneNone && a.ActZone != ZoneBattlefield {
		return 0, false
	}
	col = -1
	for c, n := range m.Pips {
		if n == 0 {
			continue
		}
		if n != 1 || col >= 0 || c > 4 {
			return 0, false
		}
		col = c
	}
	if col < 0 {
		return 0, false
	}
	switch a.AFlags {
	case 0:
		return col, a.ManaColors != 0
	case AfManaChoice:
		c := a.ManaColors
		return col, c&^ColAny == 0 && c&(c-1) != 0
	}
	return 0, false
}

// FilterManaAb reports whether mana ability a is a filter the kernel models
// as a card's first mana ability: a generic-only fee,
// {1} once each turn (ActivationLimit$ 1) or {1}, {T}, one unrestricted
// mana of any colour (Produced$ Any) or of one of at least two colours
// (Produced$ Combo, AfManaChoice). gorge's planner census misses it (a paid
// census gap); the kernel runs gorge's prefix search over it (kernel
// mana_filter.go, mana_filterfirst.go). {1}, {T}, sacrifice this
// (Omni-Cheese Pizza, Chromatic Star, Golden Egg) is one too: the kernel
// sacrifices the source as the activation is applied (sacFilterSource).
// {N}, {T}: one of each of two or more colours at once (Skycloud Expanse's
// {1},{T}: Add {W}{U}; FilterEachManaAb) is one: its relaxed census
// alternative is one fixed production (kernel mana_filtereach.go).
func FilterManaAb(a *Ability) bool {
	if _, ok := PipFeeManaAb(a); ok {
		return true
	}
	m := &a.Cost.Mana
	if a.Kind != AbMana || m.Generic == 0 || m.X != 0 || m.Hyb != ([4]ColorMask{}) || m.Tb != 0 || m.Pips != ([NumManaKinds]uint8{}) ||
		a.Cost.Flags != CostOncePerTurn && a.Cost.Flags != CostTap && a.Cost.Flags != CostTap|CostSacSelf || a.ManaRestrict != 0 || a.ManaN > 1 || len(a.Prog) != 0 ||
		a.Gate.Ctr != 0 || a.Trig.Cond != 0 || a.ActZone != ZoneNone && a.ActZone != ZoneBattlefield {
		return false
	}
	if a.ManaEach {
		c := a.ManaColors
		return a.AFlags == 0 && a.Cost.Flags == CostTap && c&^ColAny == 0 && c&(c-1) != 0
	}
	switch a.AFlags {
	case 0:
		return a.ManaColors == ColAny
	case AfManaChoice:
		c := a.ManaColors
		return c&^ColAny == 0 && c&(c-1) != 0
	}
	return false
}

// FilterEachManaAb reports whether mana ability a is the all-at-once filter
// FilterManaAb admits: "{N}, {T}: Add {c1}{c2}..." of two or more distinct
// WUBRG colours, one of each, no choice (Skycloud Expanse). A plain {T} one
// (Azorius Chancery) is not: it has no fee.
func FilterEachManaAb(a *Ability) bool { return a.ManaEach && FilterManaAb(a) }

// SacXEachManaAb reports whether mana ability a is "{T}, Sacrifice another
// <filter>: Add <c1><c2>..." (Bolg's Company's "{T}, Sacrifice another Goblin:
// Add {B}{R}"): a mana-free {T} plus a sacrifice of exactly one permanent
// matching Cost.Filter (its Other clause leaves the source out), two or more
// distinct WUBRG colours once each, unrestricted, no choice. gorge's census
// defers it (a census gap) and relaxes it to one free fixed production; the
// kernel activates it in the tap-another search (kernel mana_tapanother.go).
func SacXEachManaAb(a *Ability) bool {
	c := a.ManaColors
	return a.Kind == AbMana && a.ManaEach && a.Cost.Flags == CostTap|CostSac && a.Cost.N == 1 && a.Cost.Filter != 0 && !a.Cost.ExileBf &&
		a.Cost.Mana == (ManaCost{}) && a.ManaRestrict == 0 && a.ManaN <= 1 && a.AFlags == 0 && len(a.Prog) == 0 &&
		a.Gate.Ctr == 0 && a.Trig.Cond == 0 && (a.ActZone == ZoneNone || a.ActZone == ZoneBattlefield) &&
		c&^ColAny == 0 && c&(c-1) != 0
}

// ManaAbilityHonoured reports whether the kernel can activate ability j of
// cd, an AbMana. It is true for any other ability kind.
//
//   - the first mana ability: a mana-free {T}, {T} + sacrifice this,
//     sacrifice this alone (pcard.manaTap / manaSac / manaSacOnly), a counter
//     placed on or removed from the source once a turn (CtrManaAb), the tap-another-
//     permanent shape of TapXManaAb, the sacrifice-another all-at-once shape
//     of SacXEachManaAb, or a filter (FilterManaAb);
//   - a granted (latent) one: a mana-free {T}, {T} + pay N life, or
//     sacrifice this alone (genPrep.grantPC; dirac mana_grantcost.go);
//   - any later own mana ability: a {T} (a fixed-colour set, a restricted,
//     filter or each-colour gap), {T} + pay life (a pain alternative), or the
//     TapXManaAb shape beside a plain fixed-colour {T} first one
//     (SecondTapXManaAb), or the CtrTapSecondManaAb shape beside a plain {T}
//     first one (SecondCtrTapManaAb); the shape guards in dirac/kgen refuseMultiMana
//     choose among those.
func ManaAbilityHonoured(cd *CardDef, j int) bool {
	a := &cd.Abilities[j]
	if a.Kind != AbMana {
		return true
	}
	f := a.Cost.Flags
	plain := a.Cost.Mana.MV() == 0 && a.Cost.Mana.X == 0
	if a.AFlags&AfLatent != 0 {
		// {T}, {T} + pay N life (Forgotten Monument), or sacrifice this alone (Lotus Ring)
		return plain && (f == CostTap || f == CostTap|CostPayLife && a.Cost.N >= 1 || f == CostSacSelf)
	}
	first := -1
	for i := range cd.Abilities {
		if cd.Abilities[i].Kind == AbMana {
			first = i
			break
		}
	}
	if j != first {
		return f == CostTap || f == CostTap|CostPayLife || SecondTapXManaAb(cd, first, j) || SecondCtrTapManaAb(cd, first, j)
	}
	if _, ok := PipFeeManaAb(a); ok {
		return true
	}
	switch {
	case plain && (f == CostTap || f == CostTap|CostOncePerGame || f == CostTap|CostSacSelf || f == CostSacSelf):
		return true
	case CtrManaAb(a), TapXManaAb(a), TapXManaTwo(a), FilterManaAb(a), SacXEachManaAb(a), SoleLifeManaAb(cd, j):
		return true
	}
	return false
}

// SoleLifeManaAb reports whether ability j of cd is the card's only own mana
// ability and is "{T}, Pay N life: Add one mana" (Mana Confluence), with no
// rider, restriction, flag or program: a pain-only source whose cost is life,
// gorge's last-resort Consequence{Life: N} with no normal alternative
// (dirac mana_pain.go prepPainOnlyLife).
func SoleLifeManaAb(cd *CardDef, j int) bool {
	a := &cd.Abilities[j]
	if a.Kind != AbMana || a.AFlags != 0 || a.ManaRestrict != 0 || a.ManaRestrictAb != 0 || len(a.Prog) != 0 || a.ManaEach ||
		a.Cost.Flags != CostTap|CostPayLife || a.Cost.N < 1 || a.Cost.Mana.MV() != 0 || a.Cost.Mana.X != 0 || a.ManaN > 1 || a.ManaColors == 0 {
		return false
	}
	for i := range cd.Abilities {
		if i != j && cd.Abilities[i].Kind == AbMana && cd.Abilities[i].AFlags&AfLatent == 0 {
			return false
		}
	}
	return true
}

// SecondTapXManaAb reports whether ability j of cd, a later mana ability than
// its first (index first), is the TapXManaAb shape beside a plain first one
// (Scene of the Crime: "{T}: Add {C}", then "{T}, tap an untapped creature you
// control: Add one mana of any color"). The kernel reads it as the card's
// tap-another-permanent gap (kernel mana_tapanother.go prep.tapXAb), its first
// ability a plain planner source.
func SecondTapXManaAb(cd *CardDef, first, j int) bool {
	if first < 0 || j <= first || j >= len(cd.Abilities) || cd.Abilities[j].Kind != AbMana {
		return false
	}
	return TapXManaAb(&cd.Abilities[j]) && plainFixedTapAb(&cd.Abilities[first])
}

// CtrTapSecondManaAb reports whether mana ability a is "{T}, Remove N counters
// from this: Add <fixed colour>" with no once-each-turn limit (Weather Maker's
// "{T}, Remove two charge counters: Add {C}{C}"): a mana-free cost of the
// source's own {T} and counters, a fixed unrestricted production of one colour
// ({C} counts), ManaN of it. Its {T} is the source's, so the limit is the tap
// alone. Kernel mana_countercost.go reads it, through prep.ctrAb, as a
// counter-cost gap beside a plain first ability (SecondCtrTapManaAb).
func CtrTapSecondManaAb(a *Ability) bool {
	c := a.ManaColors
	return a.Kind == AbMana && a.Cost.Flags == CostTap|CostRemoveCtr && a.Cost.N > 0 && a.Cost.Mana.MV() == 0 && a.Cost.Mana.X == 0 &&
		a.AFlags == 0 && a.ManaRestrict == 0 && !a.ManaEach && c != 0 && c&(c-1) == 0 && a.ManaN >= 1 && len(a.Prog) == 0 &&
		a.Gate.Ctr == 0 && a.Trig.Cond == 0 && (a.ActZone == ZoneNone || a.ActZone == ZoneBattlefield)
}

// SecondCtrTapManaAb reports whether ability j of cd, a later mana ability
// than its first (index first), is the CtrTapSecondManaAb shape beside a plain
// {T} first one (plainTapAb): the first ability is a planner source as usual,
// the second a counter-cost gap of the same permanent.
func SecondCtrTapManaAb(cd *CardDef, first, j int) bool {
	if first < 0 || j <= first || j >= len(cd.Abilities) || cd.Abilities[j].Kind != AbMana {
		return false
	}
	return CtrTapSecondManaAb(&cd.Abilities[j]) && plainTapAb(&cd.Abilities[first])
}

// plainTapAb: a is a mana-free {T} ability of one fixed colour or any one
// colour, amount one, unrestricted (the plain first ability a second
// CtrTapSecondManaAb one is modelled beside: Weather Maker's any colour).
func plainTapAb(a *Ability) bool {
	c := a.ManaColors
	return a.Cost.Flags == CostTap && a.Cost.Mana.MV() == 0 && a.Cost.Mana.X == 0 && a.AFlags == 0 && a.ManaRestrict == 0 &&
		(c != 0 && c&(c-1) == 0 || c == ColAny) && a.ManaN <= 1 && len(a.Prog) == 0
}

// plainFixedTapAb: a is a mana-free {T} ability of one fixed colour, amount
// one, unrestricted (the plain first ability a second TapXManaAb one is
// modelled beside: Scene of the Crime's {C}).
func plainFixedTapAb(a *Ability) bool {
	c := a.ManaColors
	return a.Cost.Flags == CostTap && a.Cost.Mana.MV() == 0 && a.Cost.Mana.X == 0 && a.AFlags == 0 && a.ManaRestrict == 0 &&
		c != 0 && c&(c-1) == 0 && a.ManaN <= 1 && len(a.Prog) == 0
}

// ManaCostShape names the cost shape of a mana ability, for the census and
// kgen's refusal reason ("T+TapX", "Sac", "T+PayLife+Mana").
func ManaCostShape(a *Ability) string {
	var parts []string
	names := [...]struct {
		f CostFlags
		n string
	}{
		{CostTap, "T"}, {CostUntap, "Untap"}, {CostSacSelf, "SacSelf"}, {CostSac, "Sac"},
		{CostRemoveCtr, "RemoveCtr"}, {CostAddCtr, "AddCtr"}, {CostDiscard, "Discard"},
		{CostPayLife, "PayLife"}, {CostTapX, "TapX"}, {CostExileGrave, "ExileGrave"},
		{CostSorcery, "Sorcery"}, {CostOncePerTurn, "OncePerTurn"}, {CostLoyalty, "Loyalty"},
		{CostOncePerGame, "OncePerGame"},
	}
	var seen CostFlags
	for _, x := range names {
		seen |= x.f
		if a.Cost.Flags&x.f != 0 {
			parts = append(parts, x.n)
		}
	}
	if a.Cost.Flags&^seen != 0 {
		parts = append(parts, "other")
	}
	if a.Cost.Mana.MV() != 0 || a.Cost.Mana.X != 0 {
		parts = append(parts, "Mana")
	}
	return strings.Join(parts, "+")
}
