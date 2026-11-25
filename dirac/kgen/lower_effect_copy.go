package kgen

import (
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"strings"
)

// repeat lowers a gate-governed api:Repeat (gorge effRepeat): the body
// (RepeatSubAbility$) runs at most MaxRepeat$ times, again only while the
// RepeatCheckSVar$ gate compares true after a pass. The sibling
// RepeatDefined$/RepeatPresent$/RepeatCompare$ gate counts the remembered
// or imprinted set's matches (gorge repeatDefinedGateHolds). Both gates may
// be present (Tainted Pact): the loop continues only while both hold.
// RepeatCheckSVar$ bodies of the `Remembered$Valid <filter>` shape and of
// any Count$ expression are lowered (Sphinx's Tutelage, Tainted Pact).
func (lc *lowerCtx) repeat(ps *params) error {
	if _, ok := ps.get("RepeatOptional"); ok {
		if ps.str("RepeatCheckSVar") == "" && ps.str("RepeatDefined") == "" {
			return lc.repeatOptional(ps) // lower_repeatoptional.go
		}
		return refuse("Repeat RepeatOptional$ beside a RepeatCheckSVar$ or RepeatDefined$ gate")
	}
	if _, ok := ps.get("RepeatNum"); ok {
		return refuse("Repeat RepeatNum$")
	}
	if _, ok := ps.get("ValidTgts"); ok {
		// the targeted player the body's Defined$ Targeted names
		if _, err := lc.objOrTarget(ps, "", zNone); err != nil {
			return err
		}
	}
	check := strings.TrimSpace(ps.str("RepeatCheckSVar"))
	defined := strings.TrimSpace(ps.str("RepeatDefined"))
	gated := check != "" || defined != ""
	mx := cst(255) // the kernel's loopN is uint8; gorge clamps a gated loop to 1000
	if v, ok := ps.get("MaxRepeat"); ok {
		n, err := lc.value(v)
		if err != nil {
			return err
		}
		mx = n
	} else if !gated {
		return refuse("Repeat without MaxRepeat$")
	}
	if !gated {
		// gorge effRepeat's ungated loop: the body MaxRepeat$ times, no gate
		// (RepeatPresent$/RepeatCompare$ are read only beside RepeatDefined$,
		// so they stay unread and refuse the card by name)
		return lc.repeatFixed(ps, mx) // lower_repeatfixed.go
	}
	svarCmp := strings.TrimSpace(ps.str("RepeatSVarCompare"))
	var conds []lCond
	if defined != "" {
		var ref refKind
		switch defined {
		case "Remembered":
			ref = rRemembered
		case "Imprinted":
			ref = rImprinted
		default:
			return refuse("RepeatDefined$ %q", defined)
		}
		var f *lFilter
		if present := strings.TrimSpace(ps.str("RepeatPresent")); present != "" {
			var err error
			if f, err = lc.filter(present); err != nil {
				return err
			}
		}
		c := lCond{Kind: cRefMatches, Ref: lRef{Kind: ref}, F: f, Op: opGE, A: cst(1)}
		cmp := strings.TrimSpace(ps.str("RepeatCompare"))
		if cmp == "" && check == "" {
			// gorge: RepeatCompare$ overrides; with no check gate an absent
			// RepeatCompare$ falls back to RepeatSVarCompare$, default GE1
			cmp = svarCmp
		}
		if cmp != "" {
			var err error
			if c.Op, c.A, err = lc.cmpSpec(cmp); err != nil {
				return err
			}
		}
		conds = append(conds, c)
	}
	body := cards.ResolveSVar(lc.face.SVars, ps.str("RepeatSubAbility"))
	if body == nil {
		return refuse("Repeat body %q", ps.str("RepeatSubAbility"))
	}
	at := len(lc.ab.Prog)
	lc.emit(lInstr{Op: "RepeatWhile", A: mx})
	prevSVar, prevAb := lc.repeatCheckSVar, lc.repeatCheckAb
	prevGuard, prevImprint := lc.repeatStoredGuard, lc.repeatImprintBody
	lc.repeatCheckSVar, lc.repeatCheckAb = check, lc.ab
	lc.repeatStoredGuard, lc.repeatImprintBody = false, defined == "Imprinted"
	err := lc.chain(body, nil)
	guard := lc.repeatStoredGuard
	if prevAb != nil {
		// keep the gate SVar named for the rest of this ability, so the
		// post-loop reset (Tainted Pact's DBReset) still matches; a nested
		// repeat restores its outer one
		lc.repeatCheckSVar, lc.repeatCheckAb = prevSVar, prevAb
	}
	lc.repeatStoredGuard, lc.repeatImprintBody = prevGuard, prevImprint
	if err != nil {
		return err
	}
	if check != "" {
		// The check gate is built after the body: Tainted Pact's body Dig
		// opens the imprinted set its Count$ValidHand Card.IsImprinted gate
		// reads (lower_imprintzone.go's gate requires the writer lower first).
		gate := lc.face.SVars[check]
		if head, rest, _ := strings.Cut(strings.TrimSpace(gate), "$"); head == "Remembered" && strings.HasPrefix(rest, "Valid ") {
			f, err := lc.filter(strings.TrimPrefix(rest, "Valid "))
			if err != nil {
				return err
			}
			c := lCond{Kind: cRefMatches, Ref: lRef{Kind: rRemembered}, F: f, Op: opGE, A: cst(1)}
			if svarCmp != "" {
				if c.Op, c.A, err = lc.cmpSpec(svarCmp); err != nil {
					return err
				}
			}
			conds = append(conds, c)
		} else {
			// an arbitrary level-count gate (Tainted Pact's TaintedCheck:
			// Count$ValidHand Card.IsImprinted)
			v, err := lc.value(check)
			if err != nil {
				return err
			}
			c := lCond{Kind: cValue, A: v, Op: opGE, B: cst(1)}
			if svarCmp != "" {
				if c.Op, c.B, err = lc.cmpSpec(svarCmp); err != nil {
					return err
				}
			}
			conds = append(conds, c)
		}
	}
	if guard {
		// a StoreSVar$ in the body writes the gate SVar true when the
		// library is empty (Tainted Pact's LibraryCheck$): the printed
		// count alone would then re-run the empty body. Fold the write into
		// the gate; lower_storesvar.go drops the constant write itself.
		conds = append(conds, lCond{Kind: cValue, A: lValue{Kind: vLibrarySizeOf, Ref: lRef{Kind: rYou}}, Op: opGE, B: cst(1)})
	}
	cond := lCond{}
	for _, c := range conds {
		if cond.Kind == cNone {
			cond = c
		} else {
			cond = lCond{Kind: cAnd, Sub: []lCond{cond, c}}
		}
	}
	lc.ab.Prog[at].Cond = cond
	lc.ab.Prog[at].Skip = len(lc.ab.Prog) - at - 1
	return nil
}

// copySpell lowers a SpellCast trigger's "copy it" (gorge
// effCopySpellAbility, Defined$ TriggeredSpellAbility): Amount$ copies of
// the triggering instant or sorcery spell; MayChooseTarget$ True lets the
// copier choose new targets for each as it resolves (CR 707.10c).
func (lc *lowerCtx) copySpell(ps *params) error {
	if ps.str("Defined") == "Parent" {
		return lc.copyParentSpell(ps) // lower_choosetype.go
	}
	if _, ok := ps.get("Defined"); !ok {
		if tt := ps.str("TargetType"); tt == "Activated.YouCtrl,Triggered.YouCtrl" || tt == "Triggered.YouCtrl" {
			return lc.copyAbility(ps) // lower_copyab.go
		}
		if _, ok := ps.get("ValidTgts"); ok {
			return lc.copyTargetSpell(ps)
		}
	}
	if ps.str("Defined") == "Targeted" {
		return lc.copyTargetedSpell(ps)
	}
	tr := lc.ab.Trigger
	if tr != nil && tr.Event == "AbilityCast" && ps.str("Defined") == "TriggeredSpellAbility" && tr.AbClass == kop.TrigAbCastAny {
		return lc.copyTriggeringAbility(ps)
	}
	if tr == nil || tr.Event != "SpellCast" || ps.str("Defined") != "TriggeredSpellAbility" {
		return refuse("CopySpellAbility outside a SpellCast trigger's Defined$ TriggeredSpellAbility")
	}
	if tr.F == nil || len(tr.F.Clauses) == 0 || tr.F.Players != 0 {
		return refuse("CopySpellAbility of an unfiltered spell")
	}
	perm := false
	for _, c := range tr.F.Clauses {
		if c.Types != 0 && c.Types&^(tInstant|tSorcery) == 0 {
			continue // an instant or sorcery clause
		}
		if c == (lClause{Flags: fSelf}) && lc.out.Types&^(tInstant|tSorcery) == 0 {
			continue // the spell's own cast trigger on an instant or sorcery
		}
		perm = true // a permanent (or untyped) clause: the copy may be a permanent spell
	}
	n, err := lc.amount(ps, "Amount", 1)
	if err != nil {
		return err
	}
	in := lInstr{Op: "CopySpell", Obj: lRef{Kind: rTriggered}, A: n}
	if ps.is("MayChooseTarget", "True") {
		in.C = 1
	}
	if perm {
		in.C |= 2 // may copy a permanent spell: the copy becomes a token (CR 707.10f)
	}
	// Controller$ You is the copy controller gorge defaults to; any other
	// selector stays unread (and refuses) until a carrier needs it.
	if v, ok := ps.m["Controller"]; ok && strings.TrimSpace(v) == "You" {
		ps.consume("Controller")
	}
	if err := lc.copyNonLegendary(ps, &in); err != nil {
		return err
	}
	lc.emit(in)
	return nil
}

// copyNonLegendary reads a spell copy's NonLegendary$ True (The Sixth
// Doctor, Jackal, Double Major: "copy it, except the copy isn't legendary";
// gorge stackCopyRiders, Object.CopyNonLegendary): the copy's type line
// drops Legendary, so the token a copied legendary permanent spell becomes
// is not gathered by the legend rule against its original (CR 707.9b,
// 704.5j). B is the shared RemoveLegendary override's index+1; the kernel
// carries it on the copy's stack entry and pushes it as the token's copy
// exception as it enters (copy_spellnonlegendary.go).
func (lc *lowerCtx) copyNonLegendary(ps *params, in *lInstr) error {
	v, ok := ps.get("NonLegendary")
	if !ok {
		return nil
	}
	if strings.TrimSpace(v) != "True" {
		return refuse("CopySpellAbility NonLegendary %q", v)
	}
	lc.out.overrides = append(lc.out.overrides, &lOverride{RemoveLegendary: true, Dedup: true})
	in.B = cst(len(lc.out.overrides)) // index+1
	in.Note = "except it isn't legendary"
	return nil
}

// copyTargetSpell lowers "copy target instant or sorcery spell you
// control" (Kitsa, Otterball Elite: gorge effCopySpellAbility's targeted
// arm copies the first object target): one copy (Amount$) of the target
// spell, MayChooseTarget$ True as for the trigger form. A filter that can
// match a permanent spell sets C&2 (kernel anyPermCopy).
//
// An absent TargetType$ (Sigil Tracer, Echo Mage) is the same spell target
// in gorge: compileTargets defaults the stack kinds to spells and a
// ValidTgts$ Instant,Sorcery with no TgtZone$ targets the stack. It is
// admitted only when every clause is bounded to instant|sorcery; an untyped
// or permanent clause keeps the TargetType refusal.
func (lc *lowerCtx) copyTargetSpell(ps *params) error {
	tt, typed := ps.get("TargetType")
	if typed && strings.TrimSpace(tt) != "Spell" {
		return refuse("CopySpellAbility TargetType %q", ps.str("TargetType"))
	}
	f, err := lc.filter(ps.str("ValidTgts"))
	if err != nil {
		return err
	}
	if f == nil || len(f.Clauses) == 0 || f.Players != 0 {
		if !typed {
			return refuse("CopySpellAbility TargetType %q", "")
		}
		return refuse("CopySpellAbility of an unfiltered spell")
	}
	perm := false
	for _, c := range f.Clauses {
		if c.Types == 0 || c.Types&^(tInstant|tSorcery) != 0 {
			perm = true
		}
	}
	if perm && !typed {
		return refuse("CopySpellAbility TargetType %q", "")
	}
	if _, ok := ps.get("TargetMin"); ok {
		return refuse("CopySpellAbility TargetMin")
	}
	if _, ok := ps.get("TargetMax"); ok {
		return refuse("CopySpellAbility TargetMax")
	}
	n, err := lc.amount(ps, "Amount", 1)
	if err != nil {
		return err
	}
	t := lc.addTarget(lTarget{F: f, Zone: zStack, Spell: true})
	in := lInstr{Op: "CopySpell", Obj: lRef{Kind: rTarget, N: t}, A: n}
	if ps.is("MayChooseTarget", "True") {
		in.C = 1
	}
	if perm {
		in.C |= 2 // may copy a permanent spell: the copy becomes a token (CR 707.10f)
	}
	// Controller$ (gorge copyControllerFor): You is the default; the
	// target-derived family (Meletis Charlatan's "the controller of target
	// instant or sorcery spell copies it") gives the copy to the target
	// spell's controller (C&4), who also answers its new-target election.
	if v, ok := ps.get("Controller"); ok {
		switch strings.TrimSpace(v) {
		case "You":
		case "TargetedController", "TargetedOrController", "Targeted":
			in.C |= 4
		default:
			return refuse("CopySpellAbility Controller$ %q", v)
		}
	}
	if err := lc.copyNonLegendary(ps, &in); err != nil {
		return err
	}
	lc.emit(in)
	return nil
}

// copyTargetedSpell lowers Defined$ Targeted (Wild Ricochet's "Then copy
// that spell" under its ChangeTargets): gorge effCopySpellAbility's targeted
// arm copies the chain's first object target, here the one instant or
// sorcery spell slot the parent declared, with the targets it has now.
func (lc *lowerCtx) copyTargetedSpell(ps *params) error {
	t := lc.lastTarget()
	if len(lc.chainT) != 1 || t < 0 {
		return refuse("CopySpellAbility Defined$ Targeted over %d target slots", len(lc.chainT))
	}
	tg := lc.ab.Targets[t]
	if !tg.Spell || tg.Zone != zStack || tg.StackAb || tg.F == nil || len(tg.F.Clauses) == 0 {
		return refuse("CopySpellAbility Defined$ Targeted of a slot that is not a spell")
	}
	for _, c := range tg.F.Clauses {
		if c.Types == 0 || c.Types&^(tInstant|tSorcery) != 0 {
			return refuse("CopySpellAbility Defined$ Targeted of a permanent spell slot")
		}
	}
	if v, ok := ps.get("Controller"); ok && strings.TrimSpace(v) != "You" {
		return refuse("CopySpellAbility Defined$ Targeted Controller$ %q", v)
	}
	ps.consume("Controller")
	n, err := lc.amount(ps, "Amount", 1)
	if err != nil {
		return err
	}
	in := lInstr{Op: "CopySpell", Obj: lRef{Kind: rTarget, N: t}, A: n}
	if ps.is("MayChooseTarget", "True") {
		in.C = 1
	}
	lc.emit(in)
	return nil
}

// demonstrate lowers the body of the trigger gorge's Demonstrate keyword
// expansion appends (cards/kw_demonstrate.go): "when you cast this spell,
// copy it and you may choose a new target for the copy; an opponent copies
// it too" (CR 702.144a). The trigger must be the card's own SpellCast
// (ValidCard$ Card.Self) and the spell an instant or sorcery. The election
// and the two copies are the kernel's (OpDemonstrate).
func (lc *lowerCtx) demonstrate(ps *params) error {
	tr := lc.ab.Trigger
	if tr == nil || tr.Event != "SpellCast" || ps.str("Defined") != "TriggeredSpellAbility" {
		return refuse("Demonstrate outside a SpellCast trigger's Defined$ TriggeredSpellAbility")
	}
	if tr.F == nil || len(tr.F.Clauses) == 0 || tr.F.Players != 0 {
		return refuse("Demonstrate of an unfiltered spell")
	}
	for _, c := range tr.F.Clauses {
		if c.Flags != fSelf || c.Types != 0 {
			return refuse("Demonstrate of a spell other than the card itself")
		}
	}
	if lc.out.Types&(tInstant|tSorcery) == 0 {
		return refuse("Demonstrate on a permanent spell")
	}
	lc.emit(lInstr{Op: "Demonstrate", Obj: lRef{Kind: rTriggered}})
	return nil
}
