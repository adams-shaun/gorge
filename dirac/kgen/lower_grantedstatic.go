package kgen

import (
	"fmt"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// grantedStaticKinds are the inner static kinds a Continuous static's
// AddStaticAbility$ may grant. A kind is wired once its kernel reader walks
// eachStaticG (dirac/static_granted.go) AND gorge reads the granted static the
// same way; the value is the reason a kind that is not open yet refuses ("" =
// open). A kind absent from the map has no kernel reader at all.
//
// NoMaxHandSize (Nerd Rage's "You have no maximum hand size") is open: the
// kernel reader (maxHand) walks eachStaticG, and gorge's maxHandSizeFor
// (engine/rules/combat.go) reads the AddStaticAbility$ grant's inner
// SetMaxHandSize$ against the recipient's controller.
var grantedStaticKinds = map[string]string{
	"NoMaxHandSize": "",
	// RaiseCost: raiseCostGen and the target-dependent raise walks read it per
	// recipient (a generic raise only: grantedRaiseShape refuses the rest).
	"RaiseCost": "",
	// Zurgo, Thunder's Decree: gorge sacrificeBlockedBy reads grantedStatics
	// ("CantSacrifice") per recipient, and the kernel's sacBlockedBy walks
	// eachStaticG.
	"CantSacrifice": "",
}

// grantWindow lowers the outer Continuous static's Phases$ window beside a
// granted static (Zurgo, Thunder's Decree: Phases$ End of Turn beside
// Condition$ PlayerTurn): gorge grantPhasesHold, the step test
// continuousGateHolds reads, ANDed with the turn condition into the outer
// static's Cond2 (the inner static carries no condition of its own).
func grantWindow(sp *params, st *lStatic) error {
	ph, ok := sp.get("Phases")
	if !ok {
		return nil
	}
	c, err := phasesCond("Phases", ph)
	if err != nil {
		return err
	}
	st.Cond = andCond(st.Cond, c)
	return nil
}

// grantedStaticParam lowers a Continuous static's AddStaticAbility$ when no
// handler took it as a self grant, and only for a static that stayed a plain
// Continuous one: a player static, a MayPlay or any other kind the Continuous
// body turned it into leaves the param unread, and the unread check refuses it.
func (lc *lowerCtx) grantedStaticParam(sp *params, st *lStatic) error {
	v, ok := sp.m["AddStaticAbility"]
	if !ok || sp.used["AddStaticAbility"] || st.Kind != "Continuous" {
		return nil
	}
	if err := grantWindow(sp, st); err != nil {
		return err
	}
	return lc.grantedStatic(sp, st, v)
}

// grantedStatic lowers AddStaticAbility$ <SVar> on a Mode$ Continuous static
// that affects ANOTHER object: the inner static is lowered by staticBody and
// kept apart from the card's own statics (lCard.GrantedStatics), and the
// outer static names it (lStatic.GrantSt). gorge's layer walk
// (engine/rules/layers.go AddStaticAbility) registers the inner static once
// per recipient with the recipient as its Self and its controller as "you",
// so the kernel yields it per recipient (static_granted.go).
func (lc *lowerCtx) grantedStatic(sp *params, st *lStatic, name string) error {
	name = strings.TrimSpace(name)
	if strings.ContainsAny(name, ",&") {
		return refuse("AddStaticAbility %q: more than one static", name)
	}
	body, ok := lc.face.SVars[name]
	if !ok {
		return refuse("AddStaticAbility %q: no such SVar", name)
	}
	if st.GrantSt != 0 {
		return refuse("AddStaticAbility %q beside another granted static", name)
	}
	inner, ok := cards.ParseStaticLines(body)
	if !ok || len(inner) != 1 {
		return refuse("AddStaticAbility %q: not one static", name)
	}
	// the inner static's source is the recipient, not the card being lowered
	exw, before := lc.exwStatic, len(lc.out.Statics)
	lc.exwStatic = false
	sts, err := lc.staticBody(inner[0])
	lc.exwStatic = exw
	if err == nil && len(lc.out.Statics) != before {
		err = refuse("lowered a static of its own")
	}
	if err != nil {
		return refuse("AddStaticAbility %s: %v", name, err)
	}
	if len(sts) != 1 {
		return refuse("AddStaticAbility %s: lowered to %d statics", name, len(sts))
	}
	in := sts[0]
	if len(in.Grant) != 0 || in.GrantSt != 0 {
		return refuse("AddStaticAbility %s: granted static grants a static", name)
	}
	why, known := grantedStaticKinds[in.Kind]
	if !known {
		return refuse("AddStaticAbility %s: granted %s static has no kernel reader", name, in.Kind)
	}
	if why != "" {
		return refuse("AddStaticAbility %s: granted %s static: %s", name, in.Kind, why)
	}
	if in.Cond.Kind != cNone {
		return refuse("AddStaticAbility %s: granted static with a condition", name)
	}
	if err := grantedRaiseShape(name, in); err != nil {
		return err
	}
	lc.out.GrantedStatics = append(lc.out.GrantedStatics, in)
	st.GrantSt = len(lc.out.GrantedStatics)
	sp.consume("AddStaticAbility")
	return nil
}

// grantStatic names the built ability's granted static: the outer Continuous
// static's CardDef.Granted index (Static.GrantSt, 1-based).
func (cb *cardBuilder) grantStatic(ka *kop.Ability, s lStatic) error {
	if s.GrantSt == 0 {
		return nil
	}
	if s.GrantSt < 0 || s.GrantSt > len(cb.c.GrantedStatics) || ka.Static.Kind != kop.StatContinuous {
		return fmt.Errorf("granted static %d not emitted", s.GrantSt)
	}
	ka.Static.GrantSt = uint8(s.GrantSt)
	return nil
}

// grantedRaiseShape: the granted RaiseCost shapes the kernel prices per
// recipient are the generic raise on the battlefield, with or without a
// ValidTarget$. A life raise (raiseLifeFor), a per-target raise, and the
// card's own or stack-source raise (Zone$) read printed statics only.
func grantedRaiseShape(name string, in lStatic) error {
	if in.Kind != "RaiseCost" {
		return nil
	}
	for _, f := range in.Flags {
		if f == "RaiseLife" || f == "RaisePerTarget" {
			return refuse("AddStaticAbility %s: granted RaiseCost %s", name, f)
		}
	}
	if in.Zone != 0 {
		return refuse("AddStaticAbility %s: granted RaiseCost with an EffectZone$", name)
	}
	return nil
}
