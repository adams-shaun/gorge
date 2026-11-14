package kgen

import "github.com/adams-shaun/gorge/engine/cards"

// Start your engines! and speed (CR 702.179), read
// from gorge (engine/rules/speed.go, engine/effects/change_speed.go):
//
//   - K:Start your engines is the CardDef.Engines flag (lower.go); the
//     keyword's trigger and the speed start are the kernel's own.
//   - A max-speed static `S:Mode$ Continuous | AffectedDefined$ Self (Affected$ Card.Self) |
//     Condition$ MaxSpeed | AddAbility$|AddTrigger$ <svar>` (CR 702.179e,
//     gorge maxSpeedAbilities / the static walk's MaxSpeed condition)
//     lowers as the card's own activated or triggered ability carrying a
//     max-speed gate (Gate{CtrMaxSpeed}): it exists only while the
//     controller's speed is 4. Any other grant (AddStaticAbility,
//     AddReplacementEffect), another affected set or zone, a granted mana
//     ability, or any other unread parameter refuses.
//   - Count$YourSpeed is vYourSpeed (value.go).
//   - api ChangeSpeed lowers one shape: Mode$ Decrease, Defined$
//     TargetedController, the highest-speed condition (Spikeshell Harrier);
//     every other Mode$ / Defined$ / condition refuses.

// maxSpeedZone reads a max-speed static's zone pair: EffectZone$ and
// AffectedZone$ both absent (the battlefield) or both Graveyard (the card in
// its owner's graveyard, gorge offBattlefieldGrantedWalk). It consumes
// CharacteristicDefining$ True, which gorge's maxSpeedAbilities never reads.
func maxSpeedZone(sp *params) (bool, error) {
	ez, hasE := sp.get("EffectZone")
	az, hasA := sp.get("AffectedZone")
	sp.consume("EffectZone", "AffectedZone")
	if v, ok := sp.get("CharacteristicDefining"); ok {
		if v != "True" {
			return false, refuse("MaxSpeed static CharacteristicDefining$ %q", v)
		}
		sp.consume("CharacteristicDefining")
	}
	if !hasE && !hasA {
		return false, nil
	}
	if ez != "Graveyard" || az != "Graveyard" {
		return false, refuse("MaxSpeed static zone %q/%q", ez, az)
	}
	return true, nil
}

// maxSpeedStatic lowers a Condition$ MaxSpeed static.
func (lc *lowerCtx) maxSpeedStatic(s cards.Static) error {
	lc.granted++
	defer func() { lc.granted-- }()
	sp := newParams(s.Params)
	sp.consume("Mode", "Condition", "Description")
	if s.Mode != "Continuous" {
		return refuse("MaxSpeed condition on static mode %s", s.Mode)
	}
	gy, err := maxSpeedZone(sp)
	if err != nil {
		return err
	}
	// gorge rewrites AffectedDefined$ Self to Affected$ Card.Self
	if d := sp.str("Affected"); d != "Card.Self" {
		return refuse("MaxSpeed static Affected$ %q", d)
	}
	if inner, ok := sp.get("AddStaticAbility"); ok {
		return lc.maxSpeedGrantedStatic(sp, inner)
	}
	if rn, ok := sp.get("AddReplacementEffect"); ok {
		return lc.maxSpeedGrantedRepl(sp, rn, gy) // lower_maxspeedrepl.go
	}
	ab, hasAb := sp.get("AddAbility")
	tr, hasTr := sp.get("AddTrigger")
	if hasAb == hasTr {
		return refuse("MaxSpeed static grants no single AddAbility$ / AddTrigger$ (AddStaticAbility, AddReplacementEffect)")
	}
	if left := sp.unread(); len(left) > 0 {
		return refuse("MaxSpeed static unread params %v", left)
	}
	if gy && hasTr {
		return refuse("MaxSpeed granted trigger from the graveyard")
	}
	idx := len(lc.out.Abilities)
	if hasAb {
		sa := cards.ResolveSVar(lc.face.SVars, ab)
		if sa == nil || sa.Kind != "AB" {
			return refuse("MaxSpeed granted ability %q", ab)
		}
		if err := lc.withAbility(lc.ab, func() error { return lc.lowerAbility(sa) }); err != nil {
			return err
		}
		if len(lc.out.Abilities) != idx+1 || lc.out.Abilities[idx].Kind != akActivated {
			return refuse("MaxSpeed granted ability %q is not a plain activated ability", ab)
		}
		// gorge's granted offer (legal_walk_battlefield.go,
		// legal_walk_offbattlefield_grants.go) prices a mana cost from the
		// floating pool alone, not the planner's lands, so the kernel offers
		// a mana-costed grant only when the pool pays it
		// (kw_maxspeed.go); gorge's planner declines {X}
		if lc.out.Abilities[idx].Cost.Mana.X {
			return refuse("MaxSpeed granted ability %q with an {X} cost", ab)
		}
		// grantedAnchorZoneOK: the ability's ActivationZone$ must match the
		// carrier's zone
		if inGy := lc.out.Abilities[idx].Zone == zGraveyard; inGy != gy {
			return refuse("MaxSpeed granted ability %q activation zone against the carrier's zone", ab)
		}
	} else {
		t, ok := cards.ParseTriggerLine(lc.face.SVars[tr])
		if !ok {
			return refuse("MaxSpeed granted trigger %q", tr)
		}
		if t.Effect == nil {
			t.Effect = cards.ResolveSVar(lc.face.SVars, t.Params["Execute"])
		}
		if err := lc.withAbility(lc.ab, func() error { return lc.lowerTriggered(t, "") }); err != nil {
			return err
		}
		if len(lc.out.Abilities) != idx+1 || lc.out.Abilities[idx].Kind != akTriggered {
			return refuse("MaxSpeed granted trigger %q lowering", tr)
		}
		if z := lc.out.Abilities[idx].Trigger.Zone; z != 0 && z != zBattlefield {
			return refuse("MaxSpeed granted trigger %q outside the battlefield", tr)
		}
	}
	if lc.out.Abilities[idx].Gate != nil {
		return refuse("MaxSpeed grant beside another gate")
	}
	lc.out.Abilities[idx].Gate = &lGate{Speed: true}
	lc.out.Abilities[idx].Name = "MaxSpeed"
	return nil
}

// changeSpeed lowers api:ChangeSpeed (gorge effChangeSpeed).
func (lc *lowerCtx) changeSpeed(ps *params) error {
	mode, def := ps.str("Mode"), ps.str("Defined")
	chk, cmp := ps.str("ConditionCheckSVar"), ps.str("ConditionSVarCompare")
	if mode != "Decrease" || def != "TargetedController" ||
		chk != "TargetedController$Speed" || cmp != "GTPlayerCountDefinedNonTargetedController$HighestSpeed" {
		return refuse("ChangeSpeed Mode$ %q Defined$ %q condition %q %q", mode, def, chk, cmp)
	}
	obj, err := lc.objOrTarget(ps, "TargetedController", zNone)
	if err != nil {
		return err
	}
	// the condition is the op's own (B.N bit 1: speed greater than every
	// other player's): nodeCondition must not lower it again
	lc.condDone = true
	lc.emit(lInstr{Op: "ChangeSpeed", Obj: obj, C: 2, B: cst(2)})
	return nil
}

// maxSpeedGrantedStatic lowers a max-speed static's AddStaticAbility$ grant
// to the source itself (Burnout Bashtronaut's "Max speed — This creature
// has double strike"). gorge's layer walk (static_layerwalk.go
// AddStaticAbility) queues the inner Continuous static while the outer one
// is live (Condition$ MaxSpeed: static_condition.go condMaxSpeed, the
// controller's speed is 4), so it lowers as the inner static gated on the
// controller's speed being 4. The inner static must itself affect only the
// source; any other outer parameter refuses. Lightwheel Enhancements'
// graveyard "you may cast this card" (a MayPlay inner static) lowers the same
// way.
func (lc *lowerCtx) maxSpeedGrantedStatic(sp *params, value string) error {
	if left := sp.unread(); len(left) > 0 {
		return refuse("MaxSpeed static unread params %v", left)
	}
	names := cards.SplitGrantNames(value)
	if len(names) == 0 {
		return refuse("MaxSpeed granted static %q", value)
	}
	// gorge grants each named body in the value's order (Tsagan, Raider
	// Warlord's "SelfDT & WideFS": cards.ParseGrantedStatics); the lowered
	// statics keep that order, so their timestamps follow it
	for _, name := range names {
		if err := lc.maxSpeedGrantedOne(name, len(names) > 1); err != nil {
			return err
		}
	}
	return nil
}

// maxSpeedGrantedOne lowers one granted static body of a max-speed grant
// (maxSpeedGrantedStatic). Only a list member (listed) may affect the
// controller's other creatures (Creature.YouCtrl+Other, the host's
// controller's side), which gorge resolves against the host.
func (lc *lowerCtx) maxSpeedGrantedOne(name string, listed bool) error {
	inners, ok := cards.ParseStaticLines(lc.face.SVars[name])
	if !ok || len(inners) != 1 || (inners[0].Mode != "Continuous" && inners[0].Mode != "ReduceCost") {
		return refuse("MaxSpeed granted static %q", name)
	}
	in := inners[0]
	ps := make(map[string]string, len(in.Params))
	for k, v := range in.Params {
		switch k {
		case "Secondary", "Description":
		case "AffectedDefined":
			if in.Mode != "Continuous" {
				return refuse("MaxSpeed granted %s static AffectedDefined$ %q", in.Mode, v)
			}
			if v != "Self" {
				return refuse("MaxSpeed granted static AffectedDefined$ %q", v)
			}
			ps["Affected"] = "Card.Self"
		default:
			ps[k] = v
		}
	}
	// a ReduceCost reads its own ValidCard$ / Activator$ (staticBody);
	// gorge's cost walk applies it to the controller's spells while the
	// outer static is live (Racers' Scoreboard)
	if in.Mode == "Continuous" && ps["Affected"] != "Card.Self" && !(listed && ps["Affected"] == "Creature.YouCtrl+Other") {
		return refuse("MaxSpeed granted static Affected$ %q", ps["Affected"])
	}
	for _, k := range []string{"Condition", "CheckSVar", "IsPresent", "ClassBand"} {
		if _, ok := ps[k]; ok {
			return refuse("MaxSpeed granted static beside its own %s$", k)
		}
	}
	in.Params = ps
	sts, err := lc.staticBody(in)
	if err != nil {
		return err
	}
	speed := lCond{Kind: cValue, A: lValue{Kind: vYourSpeed}, Op: opGE, B: cst(4)}
	for _, st := range sts {
		if st.Kind == "" {
			continue
		}
		st.Cond = andCond(st.Cond, speed)
		lc.out.Statics = append(lc.out.Statics, st)
	}
	return nil
}
