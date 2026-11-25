package kgen

import (
	"github.com/adams-shaun/gorge/dirac/kop"
	"strings"
)

func (lc *lowerCtx) dealDamage(ps *params) error {
	var (
		n      lValue
		dmgSrc lRef
		obj    lRef
		err    error
	)
	// a node with its own ValidTgts$ declares the slot first: its amount's
	// Targeted$ and its DamageSource$ name that slot (gorge c.Targets)
	own := ownTargetFirst(ps)
	if own {
		if obj, err = lc.objOrTarget(ps, "", zBattlefield); err != nil {
			return err
		}
	}
	if n, err = lc.amount(ps, "NumDmg", 0); err != nil {
		return err
	}
	if s, ok := ps.get("DamageSource"); ok {
		if dmgSrc, err = lc.defined(s); err != nil {
			return err
		}
	}
	if !own {
		if obj, err = lc.objOrTarget(ps, "", zBattlefield); err != nil {
			return err
		}
	}
	in := lInstr{Op: "DealDamage", Obj: obj, A: n}
	if dmgSrc.Kind != rNone {
		if ps.is("DamageMap", "True") {
			// the kernel flushes no damage map (Self-Destruct's source = target hit
			// plus a second node's "any other target" that may be a player)
			return refuse("DamageMap$ True with DamageSource$: the kernel has no damage-map flush")
		}
		if obj.Kind == rEachOpp || obj.Kind == rDefending {
			return lc.dmgSrcPlayers(ps, dmgSrc, obj, n) // lower_damagesourceplayers.go
		}
		if obj.Kind == rTarget {
			// Bite / DealDamageFrom hit permanent slots only (hitBatch); a
			// player the slot may name is offered by the kernel, then dropped
			if f := lc.ab.Targets[obj.N].F; f != nil && f.Players != 0 {
				if dmgSrc.Kind == rTriggered {
					return lc.dmgSrcAnyTarget(ps, dmgSrc, obj, n) // lower_damagesourceplayers.go
				}
				return refuse("DamageSource$ to a target slot that may name a player")
			}
		}
		// "Target creature you control deals damage equal to its power":
		// the kernel's Bite (one-sided fight).
		if n.Kind != vPowerOf || n.Ref != dmgSrc || n.Mul != 0 || n.Add != 0 || n.Neg {
			if obj.Kind != rTarget || dmgSrc.Kind != rTarget {
				return refuse("DamageSource$ with an amount other than the source's power")
			}
			lc.emit(lInstr{Op: "DealDamageFrom", Obj: dmgSrc, A: n, C: obj.N, Note: "source deals A"})
			return nil
		}
		if obj.Kind != rTarget {
			// the kernel's Bite names one object target slot; a source's
			// power dealt to a player or each opponent has no lowering
			return refuse("DamageSource$ to %s", obj)
		}
		in = lInstr{Op: "Bite", Obj: dmgSrc, A: lValue{}, Note: "source deals its power"}
		in.B = lValue{Kind: vNone}
		in.CS = "to=" + obj.String()
		in.C = obj.N
	}
	if d, ok := ps.get("DividedAsYouChoose"); ok {
		dv, err := lc.value(d)
		if err != nil {
			return err
		}
		if obj.Kind != rTarget {
			return refuse("divided damage without targets")
		}
		if _, ok := ps.get("Announce"); ok {
			// Nahiri's Sacrifice announces X for its "sacrifice a permanent of
			// mana value X" cost (CR 601.2b); gorge announces no X for a cost
			// without {X} and reads the sacrifice's X as 0, so the card casts
			// only against a mana-value-0 permanent
			return refuse("divided damage over an Announce$ X total: gorge announces no X for a cost without {X}")
		}
		if dv.isConst() {
			lc.ab.Targets[obj.N].Divided = dv.N
		} else if mn := lc.ab.Targets[obj.N].Min; !mn.isConst() || mn.N != 0 {
			// gorge aborts a cast whose total is below the slot's minimum
			// (cast_targets.go); the kernel's clamp assumes "any number"
			return refuse("divided over a value total with a TargetMin$ above 0")
		} else {
			// A value total (Ureni's land count, a spell's paid X) is read where
			// the ability is put on the stack, exactly where gorge's
			// divisionTotalFor reads DividedAsYouChoose$ (CR 601.2d, 603.3d).
			// gorge clamps a divided slot's target count to that total (each
			// target takes at least 1; cast_targets.go, target_ask.go) whatever
			// TargetMax$ says (Rolling Thunder's all-targets count, Shatterskull
			// Smashing's 2); the kernel clamps the slot's evaluated maximum the
			// same way (tgtBoundsV).
			lc.ab.Targets[obj.N].Divided = kop.DivValue
			lc.ab.Targets[obj.N].DividedV = dv
		}
		in.Op = "DealDamageDivided"
	}
	if de, ok := ps.get("DivideEvenly"); ok {
		if err := lc.divideEvenly(ps, de, &in); err != nil { // lower_divideevenly.go
			return err
		}
	}
	if ps.is("DamageMap", "True") {
		in.Note = "damage map: dealt together with the following DealDamage nodes"
	}
	if es, ok := ps.get("ExcessSVar"); ok {
		// "" and Card.targetedBy both hold for the damaged target (gorge
		// excessConditionHolds); anything else refuses
		if c := ps.str("ExcessSVarCondition"); c != "" && c != "Card.targetedBy" {
			return refuse("ExcessSVar condition")
		}
		if err := lc.dealExcess(obj, dmgSrc); err != nil {
			return err
		}
		in.CS = "TrackExcess"
		lc.excess = strings.TrimSpace(es)
	}
	if rd, ok := ps.get("ReplaceDyingDefined"); ok {
		// Remembered is the set this same node damaged (gorge
		// registerReplaceDying builds its ids from the damaged slice)
		if rd != "Targeted" && !(rd == "Remembered" && ps.is("RememberDamaged", "True")) {
			return refuse("ReplaceDyingDefined %q", rd)
		}
		in.CS = "ExileIfDiesThisTurn"
		if rd == "Remembered" {
			in.CS += "|ExileIfDealt" // only a recipient actually dealt damage (gorge damage.go)
		}
	}
	if rd, ok := ps.get("RememberDamaged"); ok {
		if err := lc.rememberDamaged(rd, &in); err != nil {
			return err
		}
	}
	if err := lc.noPrevention(ps, &in); err != nil { // lower_noprevention.go
		return err
	}
	if err := lc.damageDecider(ps, &in); err != nil { // lower_optional.go
		return err
	}
	if err := lc.excessDamage(ps, &in, dmgSrc); err != nil { // lower_excessredirect.go
		return err
	}
	lc.emit(in)
	return nil
}

// rememberDamaged reads DealDamage RememberDamaged$ (gorge effDealDamage:
// each object target on the battlefield is remembered when damage above 0 is
// actually dealt to it, CR 120.1; a player target is remembered as a
// player). The kernel's remembered set
// holds objects only (kop.DmgRemember), so a later node that can observe a
// remembered player refuses when the damage can go to a player.
func (lc *lowerCtx) rememberDamaged(rd string, in *lInstr) error {
	if rd != "True" {
		return refuse("RememberDamaged$ %q", rd)
	}
	if in.Op != "DealDamage" || strings.HasPrefix(in.Note, "damage map") {
		return refuse("RememberDamaged$ on a %s", in.Op)
	}
	admitsPlayer := in.Obj.Kind != rTarget
	if in.Obj.Kind == rTarget {
		f := lc.ab.Targets[in.Obj.N].F
		admitsPlayer = f == nil || f.Players != 0
	}
	if admitsPlayer && lc.restObservesRememberedPlayer() {
		return refuse("RememberDamaged$ with a remembered player observed")
	}
	in.CS = strings.TrimPrefix(in.CS+"|Remember", "|")
	return nil
}

// restObservesRememberedPlayer reports whether a later node of the chain
// could tell a remembered player apart from none: a Player-based
// ConditionPresent$, Defined$ Player.IsRemembered, or a count of the set
// (Remembered$Amount, Count$RememberedSize), an SVar's body included.
func (lc *lowerCtx) restObservesRememberedPlayer() bool {
	if lc.curNode == nil {
		return false
	}
	reads := func(v string) bool {
		return strings.Contains(v, "Player.IsRemembered") || strings.Contains(v, "Remembered$Amount") ||
			strings.Contains(v, "RememberedSize")
	}
	for n := lc.curNode.Sub; n != nil; n = n.Sub {
		for k, v := range n.Params {
			if k == "ConditionPresent" && strings.Contains(v, "Player") || reads(v) || reads(lc.face.SVars[strings.TrimSpace(v)]) {
				return true
			}
		}
	}
	return false
}

func (lc *lowerCtx) damageAll(ps *params) error {
	n, err := lc.amount(ps, "NumDmg", 0)
	if err != nil {
		return err
	}
	if ds, ok := ps.get("DamageSource"); ok {
		return lc.sweepFrom(ps, ds, n) // lower_sweepfrom.go
	}
	if vp := ps.str("ValidPlayers"); vp == "OppNonTriggeredTarget" && ps.m["ValidCards"] == "" {
		// every opponent but the damaged one (gorge validPlayers:
		// AliveFrom(0) less the controller and the trigger's target
		// player), at any seat count: OpDamageAll B.N bit 32 (kernel
		// damageOtherOpps), seat-exact. In a two-player game the damaged player is the
		// one opponent, so no player is left to hit (CR 102.2)
		if !lc.damagedPlayerIsTriggerPlayer() {
			return refuse("ValidPlayers$ OppNonTriggeredTarget outside a damage-to-player trigger")
		}
		ps.consume("ValidPlayers")
		lc.emit(lInstr{Op: "DamageAll", A: n, CS: "OtherOpps"})
		return nil
	}
	if strings.TrimSpace(ps.str("ValidPlayers")) == "Targeted" {
		ps.consume("ValidPlayers")
		return lc.damageAllTargetedPlayers(ps, n) // lower_damagealltargeted.go
	}
	in := lInstr{Op: "DamageAll", A: n}
	if vc, ok := ps.get("ValidCards"); ok {
		if in.F, err = lc.filter(vc); err != nil {
			return err
		}
	}
	if vp, ok := ps.get("ValidPlayers"); ok {
		pf, err := lc.filter(vp)
		if err != nil {
			return err
		}
		if in.F == nil {
			in.F = &lFilter{}
		}
		in.F.Players |= pf.Players
	}
	if in.F == nil {
		return refuse("DamageAll with no filter")
	}
	lc.emit(in)
	return nil
}

// chooseColorPrevent lowers a mid-resolution ChooseColor (Defined$ You)
// whose one sub-ability is an Effect registering only a bodyless DamageDone
// Prevent$ True replacement over sources of the chosen colour (Prismatic
// Strands: gorge effChooseColor stores the answer on the source card, whose
// Effect replacement reads it as ValidSource$ Card.ChosenColor for the turn,
// effect.go's bodyless prevent arm). Both nodes lower to one PreventColor
// (the choice is the op's own ask); any other ChooseColor chain refuses.
func (lc *lowerCtx) chooseColorPrevent(ps *params) error {
	if d, ok := ps.get("Defined"); ok && d != "You" {
		return refuse("ChooseColor outside an as-enters replacement (Defined$ %q)", d)
	}
	ps.consume("AILogic", "SubAbility")
	orColors, err := chooseColorOrColors(ps) // lower_choosecolor.go
	if err != nil {
		return err
	}
	n := lc.curNode.Sub
	animate := n != nil && n.API == "Animate" && strings.EqualFold(strings.TrimSpace(n.Params["Colors"]), "ChosenColor")
	if orColors && !animate {
		return refuse("ChooseColor OrColors$ outside an Animate Colors$ ChosenColor")
	}
	if n != nil && n.API == "Pump" && n.Params["DefinedKW"] == "ChosenColor" {
		// (Skrelv) the colour rides the resolution (res.chosen)
		// for the chosen-colour grants that follow
		lc.colGrant = true
		lc.emit(lInstr{Op: "ChooseColorRes", Note: "the controller chooses a colour for this resolution"})
		return nil
	}
	if chooseColorResSub(n) {
		// (Akroma's Blessing, Wash Out) lower_choosecolorres.go
		lc.colGrant = true
		lc.emit(lInstr{Op: "ChooseColorRes", Note: "the controller chooses a colour for this resolution"})
		return nil
	}
	if animate {
		// (Puca's Eye, Mondo Gecko) the Animate's
		// Colors$ ChosenColor reads the resolution's colour (res.chosen)
		// when the five-record override applies
		lc.colGrant = true
		in := lInstr{Op: "ChooseColorRes", Note: "the controller chooses a colour for this resolution"}
		if orColors {
			// (Quickchange, Shyft) gorge cannot ask OrColors$: white, no ask
			in.C, in.Note = 1, "OrColors$: gorge's no-ask white answer"
		}
		lc.emit(in)
		return nil
	}
	if n == nil || n.API != "Effect" || n.Sub != nil || lc.preventColorShape(n.Params) != "" {
		return refuse("ChooseColor outside an as-enters replacement%s", chooseColorRefusal(n))
	}
	lc.prevColor = true
	return nil
}

// preventColorShape names what keeps an Effect from being the
// prevent-the-chosen-colour shape ("" when it is).
func (lc *lowerCtx) preventColorShape(p map[string]string) string {
	for k := range p {
		switch k {
		case "ReplacementEffects", "SpellDescription", "StackDescription", "AILogic":
		default:
			return "Effect param " + k
		}
	}
	name := strings.TrimSpace(p["ReplacementEffects"])
	line, ok := lc.face.SVars[name]
	if !ok {
		return "replacement " + name
	}
	want := map[string]string{"Event": "DamageDone", "Prevent": "True", "ActiveZones": "Command", "ValidSource": "Card.ChosenColor"}
	seen := 0
	for _, part := range strings.Split(line, "|") {
		k, v, _ := strings.Cut(strings.TrimSpace(part), "$")
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		if k == "Description" {
			continue
		}
		if want[k] != v {
			return "replacement param " + k
		}
		seen++
	}
	if seen != len(want) {
		return "replacement shape"
	}
	return ""
}

// preventColorEffect is the Effect half of chooseColorPrevent: OpExt ExtPreventColor
// asks the controller for a colour and prevents all damage sources of that
// colour would deal this turn (kernel cast_altfree.go).
func (lc *lowerCtx) preventColorEffect(ps *params) error {
	lc.prevColor = false
	ps.consume("ReplacementEffects", "AILogic")
	lc.emit(lInstr{Op: "PreventColor", Note: "choose a colour; prevent all damage its sources would deal this turn"})
	return nil
}

// eachDamage lowers api:EachDamage as OpExt ExtEachDamage
// (Case of the Gateway Express; gorge effEachDamage). The damagers are
// either DefinedDamagers$ Valid <filter> (every battlefield permanent
// matching it) or a chain referent (Common Goal, Polukranos):
// ParentTarget (the nearest earlier link's target slot, gorge
// parentLinkTargets), Targeted (the union of the chain's earlier slots,
// refTargetUnion) or Targeted.YouCtrl (that union narrowed to the
// controller's permanents, gorge eachDamagerTargets). Each deals NumDmg$
// (a constant, or Count$CardPower: its own power) to the recipient: the
// node's own single object target, or Defined$ Self. A ValidCards$ <filter>
// without DefinedDamagers$ is gorge's same battlefield sweep as
// DefinedDamagers$ Valid <filter> (effEachDamage, Nissa's Judgment).
// EachToItself$, ToEachOther$, DamageSource$, division and player recipients
// refuse.
func (lc *lowerCtx) eachDamage(ps *params) error {
	for _, k := range [...]string{"EachToItself", "ToEachOther", "DamageSource", "DividedAsYouChoose"} {
		if _, ok := ps.get(k); ok {
			return refuse("EachDamage %s$", k)
		}
	}
	dd := strings.TrimSpace(ps.str("DefinedDamagers"))
	if vc, ok := ps.get("ValidCards"); ok {
		if dd != "" {
			return refuse("EachDamage ValidCards$ beside DefinedDamagers$")
		}
		dd = "Valid " + strings.TrimSpace(vc)
	}
	var f *lFilter
	mask := 0 // damager target slots (bit per slot); 0 = the Valid sweep
	if spec, ok := strings.CutPrefix(dd, "Valid "); ok {
		var err error
		if f, err = lc.filter(strings.TrimSpace(spec)); err != nil {
			return err
		}
		if f.Players != 0 || len(f.Clauses) == 0 {
			return refuse("EachDamage damagers %q: a player alternative", dd)
		}
	} else {
		// read before the node's own target slot is added: gorge's
		// parent record and union are the links resolved before this one
		var slots []int
		switch dd {
		case "ParentTarget":
			if t := lc.lastTarget(); t >= 0 {
				slots = []int{t}
			}
		case "Targeted", "Targeted.YouCtrl":
			if !(lc.depth == 1 && lc.mode < 0 || lc.depth == 2 && lc.mode >= 0) {
				return refuse("EachDamage DefinedDamagers$ %s in a nested chain", dd)
			}
			if len(lc.chainT) > 1 {
				// gorge's union over two earlier slots names only the
				// root's (Friendly Rivalry's legendary second target never
				// deals damage: eachdmgtargeted seeds 432, 696; reported to gorge)
				return refuse("EachDamage DefinedDamagers$ %s over %d slots (gorge reads only the root's)", dd, len(lc.chainT))
			}
			slots = append(slots, lc.chainT...)
		case "Remembered":
			var err error
			if slots, err = lc.eachDamageRemembered(); err != nil { // lower_remembered.go
				return err
			}
		default:
			return refuse("EachDamage DefinedDamagers$ %q", dd)
		}
		if len(slots) == 0 {
			return refuse("EachDamage DefinedDamagers$ %s with no earlier target slot", dd)
		}
		for _, t := range slots {
			pt := lc.ab.Targets[t]
			if pt.F == nil || pt.F.Players != 0 || len(pt.F.Clauses) == 0 || pt.AtRes || pt.Zone != zBattlefield {
				return refuse("EachDamage damager slot %d: not a battlefield object target", t)
			}
			mask |= 1 << t
		}
		if dd == "Targeted.YouCtrl" {
			var err error
			if f, err = lc.filter("Card.YouCtrl"); err != nil {
				return err
			}
		}
	}
	n, err := lc.amount(ps, "NumDmg", 1)
	if err != nil {
		return err
	}
	switch {
	case n.Kind == vPowerOf && n.Ref.Kind == rSelf && n.Mul == 0 && n.Add == 0 && !n.Neg:
		// Count$CardPower read per damager (gorge's per-damager Ctx anchors
		// Source on it): B = 0, the kernel's "its power"
		n = cst(0)
	case !n.isConst() || n.N < 1:
		return refuse("EachDamage NumDmg$ %v: not a positive constant or the damager's power", n)
	}
	obj, err := lc.objOrTarget(ps, "", zBattlefield)
	if err != nil {
		return err
	}
	switch obj.Kind {
	case rTarget:
		if t := lc.ab.Targets[obj.N]; t.F == nil || t.F.Players != 0 || !t.Max.isConst() || t.Max.N != 1 {
			return refuse("EachDamage target slot %d: not one object", obj.N)
		}
		if mask&(1<<obj.N) != 0 {
			return refuse("EachDamage recipient slot %d is a damager slot", obj.N)
		}
	case rSelf:
		if mask == 0 {
			return refuse("EachDamage Valid damagers to Self")
		}
	default:
		return refuse("EachDamage to %s", obj)
	}
	lc.emit(lInstr{Op: "EachDamage", F: f, B: n, Obj2: obj, C: mask})
	return nil
}
