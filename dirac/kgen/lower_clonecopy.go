package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// DB$ Clone on the battlefield (gorge effects/clone.go
// effClone), one of the six shapes the kernel's copy machinery holds exactly:
//
//	own ValidTgts$ template, CloneTarget$ ParentTarget   Fleeting Reflection, Shuri
//	Choices$ template (Optional$), the source becomes    Deepfathom Echo
//	own ValidTgts$ template, the source becomes          Oko the Ringleader, Silent Hallcreeper
//	Defined$ Targeted template, CloneTarget$ Valid ...+NotDefinedTargeted   Naga Fleshcrafter
//	Defined$ Targeted template, CloneTarget$ Valid ..., until end of turn   Gwyndala, the Progeny
//
// Duration$ absent or Permanent (until the becomer leaves) or UntilEndOfTurn;
// the template a creature or artifact (the kernel copies those only); the
// copy exceptions NonLegendary$ and AddKeywords$. Everything else refuses.

// cloneCopyRefused are the Clone params the lowering does not read: each
// refuses by name before copyExceptions (which reads some of them for
// CopyPermanent) can lower it.
var cloneCopyRefused = []string{
	"NewName", "AddColors", "SetColor", "SetPower", "SetToughness", "RemoveCost",
	"Controller", "AddAbilities", "AddStaticAbilities",
	"ExcludeChosen", "ChoiceZone", "CloneZone", "ChoiceOptional", "PumpKeywords",
	"PumpDuration", "RemoveCardTypes", "RemoveCreatureTypes", "RemoveSubTypes", "CopyFromChosenName",
	"FaceDown", "KeepFacedownFalse", "AttachedTo", "IntoPlayTapped", "RememberCloneOrigin", "IfNew",
}

func (lc *lowerCtx) cloneCopy(ps *params) error {
	batch := ps.str("Choices") == "Card.TriggeredCards+Creature"
	if batch && !(lc.batchPoolOK && ps.is("ChoiceZone", "Exile")) {
		return refuse("Clone Choices$ %s outside the exile batch trigger", ps.str("Choices"))
	}
	for _, k := range cloneCopyRefused {
		if k == "ChoiceZone" && batch {
			continue // the batch pool's zone, read below
		}
		if _, ok := ps.m[k]; ok {
			return refuse("Clone %s$ on the battlefield", k)
		}
	}
	if lc.ab == nil {
		return refuse("Clone outside a resolving ability")
	}
	var flags int
	switch d := ps.str("Duration"); d {
	case "", "Permanent":
	case "UntilEndOfTurn":
		flags |= kop.CloneEOT
	default:
		return refuse("Clone Duration$ %q", d)
	}
	optional := false
	if v, ok := ps.get("Optional"); ok {
		if strings.TrimSpace(v) != "True" {
			return refuse("Clone Optional$ %q", v)
		}
		optional = true
	}
	ps.consume("TgtPrompt", "ValidTgtsDesc", "ChoiceTitle", "AILogic", "Mandatory")

	// the becomer, read before the node's own target slot is declared
	var becomer lRef
	var valid *lFilter
	selfCopy := false
	if batch {
		// the pick is a creature card among the exiled batch; the node's own
		// target (the token) is the becomer (gorge ChoicesTargetBecome)
		_, hasCT := ps.m["CloneTarget"]
		_, hasD := ps.m["Defined"]
		if !flagsEOT(flags) || optional || hasCT || hasD || ps.m["ValidTgts"] == "" {
			return refuse("Clone Choices$ batch pool without the Kaya shape")
		}
		tr, err := lc.objOrTarget(ps, "", zBattlefield)
		if err != nil {
			return err
		}
		if tr.Kind != rTarget || tr.N != 0 || len(lc.ab.Targets) != 1 {
			return refuse("Clone Choices$ batch pool: becomer is not the first target slot")
		}
		f, err := lc.filter("Card.Creature")
		if err != nil {
			return err
		}
		if err := cloneTemplateFilter(f); err != nil {
			return err
		}
		exc, hasExc, _, err := lc.cloneExceptions(ps, &flags, true)
		if err != nil {
			return err
		}
		if exc.AllCreatureTypes {
			return refuse("Clone AddKeywords Changeling")
		}
		lc.emit(lInstr{Op: "CloneCopy", F: f, B: cst(kop.ClonePick | kop.CloneBatch), Note: "pick the batch's exiled creature card"})
		in := lInstr{Op: "CloneCopy", Obj: tr, B: cst(flags | kop.CloneTmplPick | kop.CloneBatch)}
		if hasExc {
			lc.out.overrides = append(lc.out.overrides, exc)
			in.C = len(lc.out.overrides)
			in.Note = "except " + overrideString(exc)
		}
		lc.emit(in)
		return nil
	}
	d, _ := ps.get("Defined")
	ct, hasCT := ps.get("CloneTarget")
	ct = strings.TrimSpace(ct)
	switch {
	case !hasCT:
		// the source becomes the copy: a creature or planeswalker (a land that
		// becomes an artifact, The Mycosynth Gardens, is not in a lockstep pool)
		bodied := false
		for _, t := range lc.face.Types {
			// a land is one only for a graveyard card's copy (Shifting
			// Woodland, lower_clonecard.go)
			bodied = bodied || t == "Creature" || t == "Planeswalker" || t == "Land" && ps.is("TgtZone", "Graveyard")
		}
		if !bodied {
			return refuse("Clone: the source is not a creature or planeswalker")
		}
		becomer = lRef{Kind: rSelf}
	case ct == "ParentTarget":
		if n := len(lc.parentChain()); n != 1 {
			return refuse("Clone CloneTarget$ ParentTarget over %d earlier target slots", n)
		}
		var err error
		if becomer, err = lc.defined("ParentTarget"); err != nil {
			return err
		}
	case strings.HasPrefix(ct, "Valid "):
		spec := strings.TrimSpace(strings.TrimPrefix(ct, "Valid "))
		spec, cut := cutNotDefinedTargeted(spec)
		if !cut {
			// the template may be one of the becomers: a copy of itself
			// with no exception changes nothing (CR 707.2), and its
			// until-end-of-turn marker ends with the others, so the kernel
			// skips it (Gwyndala, the Progeny). A permanent self-copy stays
			// in gorge's layer-1 markers, and a template that left (Absorb
			// Identity's ParentTarget) is copied from its new zone: refused.
			if ps.str("Duration") != "UntilEndOfTurn" || d != "Targeted" {
				return refuse("Clone CloneTarget$ Valid %q without NotDefinedTargeted", spec)
			}
			selfCopy = true
		}
		var err error
		if valid, err = lc.filter(spec); err != nil {
			return err
		}
		if cut {
			flags |= kop.CloneExclTmpl
		}
	default:
		return refuse("Clone CloneTarget$ %q", ct)
	}

	if err := lc.cloneCopyTemplate(ps, &flags, becomer, valid, selfCopy, optional); err != nil {
		return err
	}
	if valid == nil && hasCT && ct != "ParentTarget" {
		return refuse("Clone CloneTarget$ %q", ct)
	}

	exc, hasExc, grant, err := lc.cloneExceptions(ps, &flags, false)
	if err != nil {
		return err
	}
	if exc.AllCreatureTypes {
		// gorge's Clone keeps a changeling grant a continuous effect with
		// the copy's duration (clone.go), not a copiable value
		return refuse("Clone AddKeywords Changeling")
	}
	if selfCopy && hasExc {
		return refuse("Clone CloneTarget$ Valid without NotDefinedTargeted beside copy exceptions")
	}
	in := lInstr{Op: "CloneCopy", Obj: becomer, F: valid, B: cst(flags)}
	if hasExc {
		lc.cloneExceptionRecords(&in, exc, grant)
	}
	if optional {
		lc.emit(lInstr{Op: "MayAsk", C: kop.MayGuardCloneCopy, Skip: 1, Note: "you may"})
	}
	lc.emit(in)
	return nil
}

// cloneCopyTemplate selects the template the copy takes: the node's own
// ValidTgts$, a Defined$ target (Targeted, ParentTarget, or the exile batch's
// Exiled card), or a Choices$ pick. It packs the target slot into flags.
func (lc *lowerCtx) cloneCopyTemplate(ps *params, flags *int, becomer lRef, valid *lFilter, selfCopy, optional bool) error {
	_, hasVT := ps.m["ValidTgts"]
	_, hasCh := ps.m["Choices"]
	d, hasD := ps.get("Defined")
	ct, hasCT := ps.get("CloneTarget")
	ct = strings.TrimSpace(ct)
	switch {
	case hasVT && !hasCh && !hasD:
		if valid != nil {
			return refuse("Clone CloneTarget$ Valid with its own ValidTgts$")
		}
		tr, err := lc.objOrTarget(ps, "", zBattlefield)
		if err != nil {
			return err
		}
		if err := lc.cloneTemplate(tr, flags); err != nil {
			return err
		}
	case selfCopy && hasVT && !hasCh:
		// Defined$ Targeted names the node's own ValidTgts$ target
		tr, err := lc.objOrTarget(newParams(map[string]string{"ValidTgts": ps.str("ValidTgts")}), "", zBattlefield)
		if err != nil {
			return err
		}
		ps.consume("ValidTgts", "Defined")
		if err := lc.cloneTemplate(tr, flags); err != nil {
			return err
		}
	case hasD && d == "Exiled" && !hasCh && !hasVT && !hasCT:
		if optional {
			return refuse("Clone Defined$ Exiled with Optional$")
		}
		if err := lc.cloneExiledTemplate(flags); err != nil {
			return err
		}
	case hasD && !hasCh && !hasVT:
		if d != "Targeted" && d != "ParentTarget" {
			return refuse("Clone Defined$ %q", d)
		}
		if valid == nil {
			return refuse("Clone Defined$ %q without a Valid CloneTarget$", d)
		}
		tr, err := lc.defined(d)
		if err != nil {
			return err
		}
		if err := lc.cloneTemplate(tr, flags); err != nil {
			return err
		}
	case hasCh && !hasVT && !hasD:
		if valid != nil {
			return refuse("Clone CloneTarget$ Valid with Choices$")
		}
		if becomer.Kind != rSelf {
			return refuse("Clone Choices$ with CloneTarget$ %q", ct)
		}
		f, err := lc.filter(ps.str("Choices"))
		if err != nil {
			return err
		}
		if err := cloneTemplateFilter(f); err != nil {
			return err
		}
		lc.emit(lInstr{Op: "CloneCopy", F: f, B: cst(kop.ClonePick), Note: "pick the Choices$ template"})
		*flags |= kop.CloneTmplPick
	default:
		return refuse("Clone template: need exactly one of ValidTgts$, Defined$, Choices$")
	}
	return nil
}

// cloneTemplate checks template ref tr (a target slot) and packs its slot
// into flags.
func (lc *lowerCtx) cloneTemplate(tr lRef, flags *int) error {
	if tr.Kind != rTarget || tr.N < 0 || tr.N > 2 || tr.N >= len(lc.ab.Targets) {
		return refuse("Clone template is not a target slot 0..2")
	}
	t := &lc.ab.Targets[tr.N]
	if t.Zone == zGraveyard && !t.GraveExileCard && !t.PermOrGraveCard {
		return lc.cloneGraveTemplate(tr, flags) // lower_clonecard.go
	}
	if t.Zone != zBattlefield || t.F == nil || t.F.Players != 0 {
		return refuse("Clone template target is not a battlefield permanent")
	}
	if err := cloneTemplateFilter(t.F); err != nil {
		return err
	}
	*flags |= tr.N << kop.CloneTmplShift
	return nil
}

// cloneTemplateFilter: the kernel copies creature and artifact templates
// only (a copied Aura must attach, a planeswalker enters with its loyalty,
// a land may be one: lower_trigger.go cloneEnter).
func cloneTemplateFilter(f *lFilter) error {
	if f == nil || len(f.Clauses) == 0 {
		return refuse("Clone template filter names no permanent")
	}
	for _, c := range f.Clauses {
		if c.Types&^(tCreature|tArtifact) != 0 || c.Types == 0 && c.Subtype != "Equipment" {
			return refuse("Clone template that is not a creature or artifact")
		}
	}
	return nil
}

// cutNotDefinedTargeted strips a trailing +NotDefinedTargeted /
// .NotDefinedTargeted from a Valid spec (the kernel drops the template slot
// itself, B.N&CloneExclTmpl).
func cutNotDefinedTargeted(spec string) (string, bool) {
	for _, sep := range []string{"+", "."} {
		if s, ok := strings.CutSuffix(spec, sep+"NotDefinedTargeted"); ok {
			return s, true
		}
	}
	return spec, false
}

// flagsEOT: the copy ends at cleanup.
func flagsEOT(flags int) bool { return flags&kop.CloneEOT != 0 }

// cloneGrant reads the CR 707.9b exception copyExceptions does not:
// GainThisAbility$ True (the copy has the resolving ability: the printed
// ability this Clone is the body of) returns a grant-only override, which the
// kernel applies as the copy's granted ability (it follows exc in the override
// pool, kop.CloneGrant). NewName$ stays refused: the kernel's copy exceptions
// carry no name. AddSVars$ only carries the SVars that ability's chain reads, which the
// kernel's own ability already has, so it lowers only beside GainThisAbility$.
// A grant makes *hasExc true: the pair stays together.
func (lc *lowerCtx) cloneGrant(ps *params, flags *int, hasExc *bool) (*lOverride, error) {
	if _, ok := ps.get("AddTriggers"); ok {
		return lc.cloneTriggerGrant(ps, flags, hasExc) // lower_clonecard.go
	}
	v, ok := ps.get("GainThisAbility")
	if !ok {
		if _, ok := ps.get("AddSVars"); ok {
			return nil, refuse("Clone AddSVars$ without GainThisAbility$")
		}
		return nil, nil
	}
	if strings.TrimSpace(v) != "True" {
		return nil, refuse("Clone GainThisAbility$ %q", v)
	}
	if lc.ab == nil || lc.ab.Latent || lc.granted > 0 {
		return nil, refuse("Clone GainThisAbility$ outside a printed ability")
	}
	if sv, ok := ps.get("AddSVars"); ok {
		for _, n := range strings.Split(sv, ",") {
			if cards.ResolveSVar(lc.face.SVars, strings.TrimSpace(n)) == nil {
				return nil, refuse("Clone AddSVars$ %q is no SVar of the card", n)
			}
		}
	}
	if flagsEOT(*flags) {
		// an until-end-of-turn copy keeps no grant record of its own
		return nil, refuse("Clone GainThisAbility$ with Duration$ UntilEndOfTurn")
	}
	*flags |= kop.CloneGrant
	*hasExc = true
	return &lOverride{GrantAbility: []int{lc.ab.Index}}, nil
}

// cloneBatchGrantRefused: the exile batch pool's Clone keeps no ability.
func cloneBatchGrantRefused(ps *params) error {
	for _, k := range []string{"GainThisAbility", "AddSVars"} {
		if _, ok := ps.m[k]; ok {
			return refuse("Clone %s$ in the exile batch pool", k)
		}
	}
	return nil
}

// cloneExceptionRecords puts the copy exceptions' override, and the grant-only
// record the kernel reads right after it (kop.CloneGrant), in the pool and
// names them in in.
func (lc *lowerCtx) cloneExceptionRecords(in *lInstr, exc, grant *lOverride) {
	lc.out.overrides = append(lc.out.overrides, exc)
	in.C = len(lc.out.overrides) // index+1
	in.Note = "except " + overrideString(exc)
	if grant != nil {
		lc.out.overrides = append(lc.out.overrides, grant)
		in.Note += " +" + overrideString(grant)
	}
}

// cloneExceptions reads a Clone's copy exceptions: copyExceptions' and the
// grant-only override of GainThisAbility$ (cloneGrant). The exile batch
// pool's Clone keeps no ability.
func (lc *lowerCtx) cloneExceptions(ps *params, flags *int, batch bool) (exc *lOverride, has bool, grant *lOverride, err error) {
	if batch {
		if err = cloneBatchGrantRefused(ps); err != nil {
			return
		}
	}
	if exc, has, err = lc.copyExceptions(ps); err != nil {
		return
	}
	grant, err = lc.cloneGrant(ps, flags, &has)
	return
}
