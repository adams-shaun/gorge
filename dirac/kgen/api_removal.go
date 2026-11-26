package kgen

import (
	"maps"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The api: effects for destroy, counter and sacrifice (lower_api.go).
func init() {
	apiOp("Destroy", (*lowerCtx).apiDestroy)
	apiOp("DestroyAll", (*lowerCtx).apiDestroyAll)
	apiOp("Counter", (*lowerCtx).apiCounter)
	apiOp("Sacrifice", (*lowerCtx).apiSacrifice)
	apiOp("SacrificeAll", (*lowerCtx).apiSacrificeAll)
}

func (lc *lowerCtx) apiDestroy(sa *cards.SA, ps *params) error {
	if d, ok := ps.get("Defined"); ok && strings.TrimSpace(d) == "ChosenCard" && lc.multiChosenPick() {
		// a destruction of a counted ChooseCard's whole answer (The Horus
		// Heresy): the one-object Destroy reads only RefChosen, so spell
		// it as the sweep over Card.ChosenCard
		cp := &params{m: maps.Clone(ps.m), used: map[string]bool{}}
		delete(cp.m, "Defined")
		cp.m["ValidCards"] = "Card.ChosenCard"
		if err := lc.apiDestroyAll(sa, cp); err != nil {
			return err
		}
		n := len(lc.ab.Prog) - 1
		if n >= 0 && lc.ab.Prog[n].Op == "DestroyAll" {
			lc.ab.Prog[n].C |= kop.DestroyAllChosenOrder
		}
		return nil
	}
	obj, err := lc.objOrTarget(ps, "", zBattlefield)
	if err != nil {
		return err
	}
	nr, err := noRegen(ps)
	if err != nil {
		return err
	}
	in := lInstr{Op: "Destroy", Obj: obj, C: nr}
	if done, err := lc.destroyRememberLKIDestroyed(ps, &in); err != nil { // lower_rememberwritersmass.go
		return err
	} else if !done {
		if err := destroyRememberLKI(ps, &in); err != nil { // lower_handmove.go
			return err
		}
	}
	if err := destroyRememberDestroyed(ps, &in); err != nil { // lower_remembered.go
		return err
	}
	lc.emit(in)
	return nil
}

func (lc *lowerCtx) apiDestroyAll(sa *cards.SA, ps *params) error {
	if ps.peek("RememberAllObjects") != "" {
		// Angrath, Minotaur Pirate's -11 remembers the destroyed set for
		// a following non-LKI power total; gorge models neither half
		return refuse("DestroyAll RememberAllObjects$: the remembered destroyed set is not modelled")
	}
	if vt := ps.peek("ValidTgts"); vt != "" {
		// gorge's targetPlayerKindScope scopes a DestroyAll to the
		// target's permanents; the kernel instruction is one filter
		return refuse("DestroyAll ValidTgts$ %q: a target-scoped sweep is not modelled", vt)
	}
	f, err := lc.destroyAllFilter(ps) // lower_destroyalltargeted.go
	if err != nil {
		return err
	}
	in := lInstr{Op: "DestroyAll", F: f}
	if v, ok := ps.get("Hidden"); ok && v != "True" {
		// Forge's UI flag: gorge's effDestroyAll (zone_bulk.go) never reads
		// Hidden$ (PKHidden is read only by ChangeZone); Steel Hellkite
		return refuse("DestroyAll Hidden %q", v)
	}
	if v, ok := ps.get("Zone"); ok && v != "Battlefield" {
		return refuse("DestroyAll Zone %q: only permanents are destroyed", v)
	}
	if v, ok := ps.get("RememberDestroyed"); ok {
		if v != "True" {
			return refuse("RememberDestroyed %q", v)
		}
		in.C = 1 // the destroyed cards put into a graveyard become the remembered set (Starfall Invocation)
	}
	nr, err := noRegen(ps)
	if err != nil {
		return err
	}
	in.C |= nr << 1
	lc.emit(in)
	return nil
}

func (lc *lowerCtx) apiCounter(sa *cards.SA, ps *params) error {
	if _, ok := ps.get("TargetType"); !ok {
		if d, ok := ps.get("Defined"); ok {
			if spec, ok := strings.CutPrefix(d, "ValidStack "); ok {
				return lc.counterAll(ps, strings.TrimSpace(spec)) // lower_stackrewrite.go
			}
			return lc.counterTriggered(d)
		}
	}
	tt := ps.str("TargetType")
	listF, trigOnly := "", false
	if tt != "Spell" && tt != "Spell,Activated,Triggered" && tt != "Activated,Triggered" {
		var terr error
		if listF, trigOnly, terr = counterTypeList(tt); terr != nil { // lower_counterspell.go
			return terr
		}
	}
	var vtg, vspell *lFilter
	vyou := false
	if v, ok := ps.get("TargetValidTargeting"); ok {
		if tt != "Spell" && tt != "Spell,Activated,Triggered" {
			return refuse("Counter TargetValidTargeting beside TargetType %q: only a spell, or a spell or ability (Diplomatic Escort), has a lowered targeting clause", tt)
		}
		var verr error
		if vtg, vyou, verr = lc.counterValidTargeting(strings.TrimSpace(v)); verr != nil {
			return verr
		}
		if vt := ps.str("ValidTgts"); tt == "Spell" && vt != "Card" {
			// the spell side's own filter (Avoid Fate: Instant,Aura)
			if vspell, verr = lc.filter(vt); verr != nil {
				return verr
			}
		}
	}
	if v, ok := ps.get("TgtZone"); ok && (tt != "Spell" || v != "Stack") {
		return refuse("Counter TgtZone$ %q beside TargetType %q", v, tt)
	}
	remMV := 0
	if v, ok := ps.get("RememberCounteredCMC"); ok {
		if v != "True" || tt != "Spell" {
			return refuse("Counter RememberCounteredCMC$ %q beside TargetType %q", v, tt)
		}
		remMV = kop.CtrRemMV
	}
	if v, ok := ps.get("RememberForCounter"); ok && v != "True" {
		return refuse("Counter RememberForCounter$ %q", v)
	}
	remTargets := false
	if v, ok := ps.get("RememberTargets"); ok {
		// Arcane Denial: gorge remembers the countered spell like
		// RememberForCounter$ (lower_stacktgt.go)
		if v != "True" || tt != "Spell" {
			return refuse("Counter RememberTargets$ %q beside TargetType %q", v, tt)
		}
		remTargets = true
	}
	cl := false
	if v, ok := ps.get("RememberCountered"); ok {
		// Tishana's Tidebinder: the countered ability's source card
		// is remembered for the loss-of-abilities Effect after it
		if v != "True" || tt != "Activated,Triggered" || ps.str("Destination") != "" || ps.str("SubAbility") == "" {
			return refuse("Counter RememberCountered$ shape")
		}
		cl = true
	}
	vt := ps.str("ValidTgts")
	if tt != "Spell" {
		// gorge's stack census: a spell matching the spec, any ability
		// (Card,Emblem names every source; lower_stacktgt.go)
		if vt != "Card,Emblem" && vt != "Card" {
			return refuse("Counter TargetType %q ValidTgts %q", tt, vt)
		}
		vt = "Card"
	}
	if listF != "" {
		vt = listF
	}
	f, err := lc.counterTgtFilter(tt, vt) // lower_countertgtprov.go
	if err != nil {
		return err
	}
	lt := lTarget{F: f, Zone: zStack, Spell: true, StackAb: tt != "Spell", AbOnly: tt == "Activated,Triggered", TrigOnly: trigOnly, VTgt: vtg, VYou: vyou, VSpellF: vspell}
	if v, ok := ps.get("TargetMin"); ok {
		if lt.Min, err = lc.value(v); err != nil {
			return err
		}
	}
	if v, ok := ps.get("TargetMax"); ok {
		if lt.Max, err = lc.value(v); err != nil {
			return err
		}
	}
	t := lc.addTarget(lt)
	if cl {
		lc.emit(lInstr{Op: "CounterLose", Obj: lRef{Kind: rTarget, N: t}, C: -1, Note: "counter the ability; its source permanent loses all abilities while the host stays"})
		lc.clAt = len(lc.ab.Prog)
		return nil
	}
	dest, err := counterDestination(ps)
	if err != nil {
		return err
	}
	in := lInstr{Op: "Counter", Obj: lRef{Kind: rTarget, N: t}, C: dest}
	if remMV != 0 {
		in.B = cst(remMV)
	}
	if ps.is("RememberForCounter", "True") || remTargets {
		in.Note = rememberCounterNote // its controller is the chain's RememberedController
	}
	lc.emit(in)
	return nil
}

func (lc *lowerCtx) apiSacrifice(sa *cards.SA, ps *params) error {
	if ps.str("Defined") == "DelayTriggerRememberedLKI" {
		// CR 702.181a: the group program's body (lowerCtx.groupDelayed
		// checked its shape); gorge sacrifices the survivors as one batch
		if !lc.inGroup {
			return refuse("Sacrifice Defined$ DelayTriggerRememberedLKI outside a Mobilize group")
		}
		lc.emitSacDelayed()
		return nil
	}
	if ps.str("Mode") == "TgtChoose" {
		// gorge effSacrifice never reads Mode$: a targeted player
		// chooses what to sacrifice whether or not it says so
		ps.consume("Mode")
	}
	if sv := strings.TrimSpace(ps.str("SacValid")); strings.Contains(sv, " & ") && ps.is("SacEachValid", "True") {
		// SacEachValid$ True ("an artifact, a creature, and a land", Tomb of
		// Annihilation's Oubliette room; Catch // Release, Perilous
		// Predicament, To the Slaughter and Structural Collapse carry the
		// same shape): the SacValid$ list splits on " & " into one ordinary
		// Sacrifice per listed type, part by part (gorge effSacrifice's
		// sacEachValidParts recursion). Each part is this same lowering over
		// a fresh params copy carrying the part as SacValid$ and no
		// SacEachValid$, so every other parameter is checked once per part
		// and the parts resolve in list order -- a part's picks leave the
		// battlefield before the next part's pool is taken, and a permanent
		// answering two parts leaves with the first part that takes it and
		// is never taken again. An empty entry is not a list: it refuses,
		// the same fail-closed shape gorge's sacEachValidParts keeps the
		// whole spec at.
		for i, part := range strings.Split(sv, " & ") {
			part = strings.TrimSpace(part)
			if part == "" {
				return refuse("SacEachValid$ list with an empty entry")
			}
			m := make(map[string]string, len(ps.m))
			for k, v := range ps.m {
				m[k] = v
			}
			m["SacValid"] = part
			delete(m, "SacEachValid")
			if i > 0 {
				// gorge reads the same cast-time targets for every part (its
				// Defined() resolves a later part's ValidTgts$ to the targets
				// already chosen), so a later part must not declare the
				// target slot a second time: it names the first part's slot
				// with Defined$ Targeted. A node carrying both spellings has
				// no such subject to share; it refuses.
				if _, ok := m["ValidTgts"]; ok {
					if _, ok := m["Defined"]; ok {
						return refuse("SacEachValid$ part with both ValidTgts$ and Defined$")
					}
					m["Defined"] = "Targeted"
					delete(m, "ValidTgts")
				}
			}
			if err := lc.apiSacrifice(sa, newParams(m)); err != nil {
				return err
			}
		}
		// The per-part lowerings (fresh params over the same keys) checked
		// every parameter; this node's own check must see them read too.
		for _, k := range sortedParamKeys(ps.m) {
			ps.consume(k)
		}
		return nil
	}
	_, hasD := ps.get("Defined")
	_, hasT := ps.get("ValidTgts")
	sv := strings.TrimSpace(ps.str("SacValid"))
	selfSac := sv == "" || sv == "Self" || sv == "Card.Self"
	if selfSac && !hasT && (!hasD || ps.str("Defined") == "Self") {
		// gorge effSacrifice: no Defined$/ValidTgts$ (or Defined$ Self)
		// and no SacValid$ other than Self resolves Defined() to the
		// source object, which is sacrificed: its controller sacrifices
		// the one permanent "Self" matches (no choice, count <= Amount).
		ps.consume("Defined")
		n, err := lc.amount(ps, "Amount", 1)
		if err != nil {
			return err
		}
		in := lInstr{Op: "Sacrifice", Obj: lRef{Kind: rYou}, A: n, F: &lFilter{Clauses: []lClause{{Flags: fSelf}}}}
		if err := lc.sacrificeOptional(ps, &in); err != nil {
			return err
		}
		if err := rememberSacrificed(ps, &in); err != nil {
			return err
		}
		lc.emit(in)
		return nil
	}
	who, err := lc.objOrTarget(ps, "You", zNone)
	if err != nil {
		return err
	}
	f, err := lc.filter(ps.str("SacValid"))
	if err != nil {
		return err
	}
	n, err := lc.amount(ps, "Amount", 1)
	if err != nil {
		return err
	}
	if ann := ps.str("Annihilator"); ann != "" {
		// CR 702.86a: the Annihilator expansion's count rides the
		// marker and replaces Amount$ (gorge zone_bulk.go effSacrifice)
		v, aerr := strconv.Atoi(ann)
		if aerr != nil || v < 0 {
			return refuse("Annihilator count %q", ann)
		}
		ps.consume("Annihilator")
		n = cst(v)
	}
	in := lInstr{Op: "Sacrifice", Obj: who, A: n, F: f}
	if err := lc.sacrificeOptional(ps, &in); err != nil {
		return err
	}
	if err := rememberSacrificed(ps, &in); err != nil {
		return err
	}
	lc.emit(in)
	return nil
}

func (lc *lowerCtx) apiSacrificeAll(sa *cards.SA, ps *params) error {
	if ps.str("Defined") == "DelayTriggerRememberedLKI" {
		// a remembered group's delayed body (lowerCtx.delayedGroupSac:
		// Will Riker, First in the Field); the same ExtSacDelayed batch
		// as Mobilize's group, emitted by Sacrifice below
		if !lc.inGroup {
			return refuse("SacrificeAll Defined$ DelayTriggerRememberedLKI outside a delayed group")
		}
		lc.emitSacDelayed()
		return nil
	}
	if _, ok := ps.get("ValidCards"); !ok {
		// gorge effSacrificeAll's Defined$ path sacrifices the named
		// objects that are on the battlefield: Defined$ Self is the
		// permanent "Self" matches; Defined$ Targeted (this node's
		// target slot) the permanent the resolving ability targets
		d := ps.str("Defined")
		_, hasT := ps.get("ValidTgts")
		var cl lClause
		switch {
		case d == "Self" && !hasT:
			cl.Flags = fSelf
		case (d == "TriggeredObject" || d == "TriggeredObjectLKICopy") && !hasT:
			// the Mode$ Unattached family (Grafted Exoskeleton, Grafted
			// Wargear, Stitcher's Graft): the FORMER BEARER the queue
			// bound, the trigger object (kernel isTriggerObj, the same
			// predicate ChooseFromDefined$ TriggeredCards pools read).
			// A bearer already off the battlefield matches nothing on
			// the battlefield: the body is a no-op, as gorge's
			// TriggeredObjectLKICopy reference is (CR 608.2h)
			cl = lClause{CmpStat: cmpTriggerObj, CmpN: 1}
		case (d == "Enchanted" || d == "Equipped") && !hasT:
			// gorge definedSpecEquipped: the source's host (its
			// AttachedTo), the permanent EnchantedBy / EquippedBy name
			// (Parasitic Implant, Slow Motion). A detached source names
			// nothing: the sacrifice is a no-op on both sides
			cl.Flags = fEnchantedBy
			if d == "Equipped" {
				cl.Flags = fEquippedBy
			}
		case d == "Targeted" && hasT:
			if _, err := lc.objOrTarget(newParams(map[string]string{"ValidTgts": ps.str("ValidTgts")}), "", zBattlefield); err != nil {
				return err
			}
			ps.consume("TgtPrompt")
			cl.Flags = fTargetedBy
		default:
			return refuse("SacrificeAll without ValidCards$ (Defined$ %q)", d)
		}
		if ctl, ok := ps.get("Controller"); ok {
			// gorge effSacrificeAll keeps only the Defined$
			// objects a Controller$ player controls, judged against
			// the board before the sacrifice (controlledOK); You is
			// the resolving controller (definedPlayerIDs: the
			// delayed trigger's controller, Fog Elemental's "sacrifice
			// it at end of combat"), the filter's ctrl=You. The other
			// corpus spellings (TriggeredPlayer, OriginalHostController)
			// name no kernel player selector: refused.
			if strings.TrimSpace(ctl) != "You" {
				return refuse("SacrificeAll Controller$ %q", ctl)
			}
			cl.Ctrl = whoYou
		}
		in := lInstr{Op: "SacrificeAll", F: &lFilter{Clauses: []lClause{cl}}}
		if rem, err := rememberParam(ps, "RememberSacrificed"); err != nil {
			return err
		} else if rem {
			in.CS = "Remember"
		}
		lc.emit(in)
		return nil
	}
	f, err := lc.filter(ps.str("ValidCards"))
	if err != nil {
		return err
	}
	in := lInstr{Op: "SacrificeAll", F: f}
	if rem, err := rememberParam(ps, "RememberSacrificed"); err != nil {
		return err
	} else if rem {
		in.CS = "Remember"
	}
	lc.emit(in)
	return nil
}
