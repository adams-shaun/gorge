package kgen

import (
	"github.com/adams-shaun/gorge/dirac/kop"
	"strconv"
	"strings"
)

func (lc *lowerCtx) fight(ps *params) error {
	var a lRef
	var err error
	d, hasD := ps.get("Defined")
	if hasD {
		if d == "DelayTriggerRemembered" && lc.ab != nil && lc.ab.ReflexObj {
			a = lRef{Kind: rDelayedRemembered} // the reflexive trigger's captured target (lower_remembered.go)
		} else if a, err = lc.defined(d); err != nil {
			return err
		}
	}
	f, err := lc.filter(ps.str("ValidTgts"))
	if err != nil {
		return err
	}
	lt := lTarget{F: f, Zone: zBattlefield}
	// "fights up to one target creature": gorge fightPairs (engine/effects/
	// damage.go) pairs Defined$ with each chosen target, and the placement
	// ask honours TargetMin$ 0 / TargetMax$ 1 (engine/rules/fight_test.go:235,
	// fight_cr70114_test.go:38). Only that spelling is read; any other
	// TargetMin/TargetMax shape is refused.
	tmin, hasMin := ps.get("TargetMin")
	tmax, hasMax := ps.get("TargetMax")
	if hasMin || hasMax {
		if !hasD || tmin != "0" || tmax != "1" {
			return refuse("Fight TargetMin$ %q TargetMax$ %q: only Defined$ with 0/1 is read", tmin, tmax)
		}
		lt.Min, lt.Max = cst(0), cst(1)
	}
	t := lc.addTarget(lt)
	if !hasD {
		return refuse("Fight without Defined$")
	}
	if a.Kind == rTriggered {
		// Affectionate Indrik's ETB "it fights": the source itself, only
		// when the trigger is a self-trigger (ValidCard$ Card.Self). A
		// non-self trigger (Tolsimir's entering Wolf) keeps the trigger
		// object.
		if tr := lc.ab.Trigger; tr != nil && tr.F != nil && len(tr.F.Clauses) == 1 && tr.F.Players == 0 &&
			(tr.F.Clauses[0] == lClause{Flags: fSelf} || tr.F.Clauses[0] == lClause{Types: tCreature, Flags: fSelf}) {
			a = lRef{Kind: rSelf}
		}
	}
	track, err := lc.fightExcess(ps)
	if err != nil {
		return err
	}
	cs := "with=Target" + strconv.Itoa(t)
	if track {
		cs += "|TrackExcess" // the hit dealt to the C creature records ExExcessDamage
	}
	lc.emit(lInstr{Op: "Fight", Obj: a, C: t, CS: cs})
	return lc.replaceDyingMark(ps, lRef{Kind: rTarget, N: t}, a) // only the fought target (Targeted), not Defined$
}

func (lc *lowerCtx) attach(ps *params) error {
	if v, ok := ps.get("Unattach"); ok {
		return lc.unattachSelf(ps, v) // lower_reconfigure.go
	}
	if o, ok := ps.get("Object"); ok && o != "Self" {
		return lc.attachObject(ps, o)
	}
	// A player DESTINATION (Archnemesis's "you may attach Archnemesis to
	// that player": DB$ Attach | Defined$ TriggeredAttackingPlayer |
	// Object$ Self): gorge's effAttach EnchantSelf branch
	// (engine/effects/attach.go) walks the Defined$ player set and attaches
	// the aura to the first legal seat — alive, not lost, an Opponent
	// Enchant word excluding the aura's controller — emitting the "attach to
	// player" event, the no-legal-player Note when none is. The kernel's
	// OpAttach W arm is that walk, gated by attachPlayerLegal
	// (interp_gen.go). Only a triggered player referent is accepted here;
	// any other Defined$ falls through to the ordinary object walk.
	// Peek (not get) and gate on the one spelling: every other Defined$ (the
	// ordinary Targeted attach object) must still resolve in the walk below,
	// after its own target slot exists.
	// The chosen player (Maddening Hex: DB$ Attach | Defined$ ChosenPlayer,
	// after the random-pool ChoosePlayer): gorge's effAttach EnchantSelf
	// branch walks the Defined$ player set — the resolution's chosen player —
	// and attaches to the first legal seat (alive, the aura's Enchant word;
	// the random pick already excluded the aura's bearer). The kernel's W arm
	// is that walk over kop RefResChosenPlayer; a resolution with no pick (the
	// random ChoosePlayer's empty pool) attaches nothing.
	if d, ok := ps.m["Defined"]; ok && strings.TrimSpace(d) == "ChosenPlayer" &&
		lc.chosenPlAb != nil && lc.chosenPlAb == lc.ab {
		ps.consume("Defined")
		in := lInstr{Op: "Attach", W: lRef{Kind: rResChosenPlayer}}
		if v, ok := ps.m["RememberAttached"]; ok {
			if v = strings.TrimSpace(v); v != "True" {
				return refuse("Attach RememberAttached$ %q", v)
			}
			ps.consume("RememberAttached")
			in.CS = "Remember"
		}
		lc.emit(in)
		return nil
	}
	if d, ok := ps.m["Defined"]; ok && strings.TrimSpace(d) == "TriggeredAttackingPlayer" {
		r, err := lc.defined("TriggeredAttackingPlayer")
		if err != nil {
			return err
		}
		if r.Kind == rTriggeredPlayer {
			ps.consume("Defined")
			in := lInstr{Op: "Attach", W: r}
			if v, ok := ps.m["RememberAttached"]; ok {
				// gorge attachTo: the attaching object joins the remembered set
				// when the attach happens
				if v = strings.TrimSpace(v); v != "True" {
					return refuse("Attach RememberAttached$ %q", v)
				}
				ps.consume("RememberAttached")
				in.CS = "Remember"
			}
			lc.emit(in)
			return nil
		}
	}
	if _, ok := ps.get("Choices"); ok {
		if _, ok := ps.get("Object"); !ok {
			return lc.attachChoices(ps)
		}
	}
	lc.reconfigureOther(ps) // lower_reconfigure.go
	obj, err := lc.objOrTarget(ps, "", zBattlefield)
	if err != nil {
		return err
	}
	lc.attachSelfOther(obj) // lower_animateaura.go
	if lc.ab.Kind == akActivated && ps.is("Keyword", "Equip") {
		lc.ab.Name = "Equip"
	}
	if err := lc.mayGate(ps, kop.MayGuardAttach, obj); err != nil {
		return err
	}
	in := lInstr{Op: "Attach", Obj: obj}
	if v, ok := ps.m["RememberAttached"]; ok {
		// gorge attachTo: the attaching object joins the remembered set
		// when the attach happens
		if v = strings.TrimSpace(v); v != "True" {
			return refuse("Attach RememberAttached$ %q", v)
		}
		ps.consume("RememberAttached")
		in.CS = "Remember"
	}
	lc.emit(in)
	return nil
}
