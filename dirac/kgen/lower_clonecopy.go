package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// DB$ Clone on the battlefield (gorge effects/clone.go
// effClone), one of the six shapes the kernel's copy machinery holds exactly:
//
//	own ValidTgts$ template, CloneTarget$ ParentTarget   Fleeting Reflection, Shuri
//	Choices$ template (Optional$), the source becomes    Deepfathom Echo
//	own ValidTgts$ template, the source becomes          Oko the Ringleader, Silent Hallcreeper
//	Defined$ Targeted template, CloneTarget$ Valid ...+NotDefinedTargeted   Naga Fleshcrafter
//
// Duration$ absent or Permanent (until the becomer leaves) or UntilEndOfTurn;
// the template a creature or artifact (the kernel copies those only); the
// copy exceptions NonLegendary$ and AddKeywords$. Everything else refuses.

// cloneCopyRefused are the Clone params the lowering does not read: each
// refuses by name before copyExceptions (which reads some of them for
// CopyPermanent) can lower it.
var cloneCopyRefused = []string{
	"NewName", "AddTypes", "AddColors", "SetColor", "SetPower", "SetToughness", "RemoveCost",
	"Controller", "GainThisAbility", "AddSVars", "AddTriggers", "AddAbilities", "AddStaticAbilities",
	"ExcludeChosen", "ChoiceZone", "TgtZone", "CloneZone", "ChoiceOptional", "PumpKeywords",
	"PumpDuration", "RemoveCardTypes", "RemoveCreatureTypes", "RemoveSubTypes", "CopyFromChosenName",
	"FaceDown", "KeepFacedownFalse", "AttachedTo", "IntoPlayTapped", "RememberCloneOrigin", "IfNew",
}

func (lc *lowerCtx) cloneCopy(ps *params) error {
	for _, k := range cloneCopyRefused {
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
	_, hasVT := ps.m["ValidTgts"]
	_, hasCh := ps.m["Choices"]
	d, hasD := ps.get("Defined")
	ct, hasCT := ps.get("CloneTarget")
	ct = strings.TrimSpace(ct)
	switch {
	case !hasCT:
		// the source becomes the copy: a creature or planeswalker (a land that
		// becomes an artifact, The Mycosynth Gardens, is not in a lockstep pool)
		bodied := false
		for _, t := range lc.face.Types {
			bodied = bodied || t == "Creature" || t == "Planeswalker"
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
			return refuse("Clone CloneTarget$ Valid %q without NotDefinedTargeted", spec)
		}
		var err error
		if valid, err = lc.filter(spec); err != nil {
			return err
		}
		flags |= kop.CloneExclTmpl
	default:
		return refuse("Clone CloneTarget$ %q", ct)
	}

	// the template
	switch {
	case hasVT && !hasCh && !hasD:
		if valid != nil {
			return refuse("Clone CloneTarget$ Valid with its own ValidTgts$")
		}
		tr, err := lc.objOrTarget(ps, "", zBattlefield)
		if err != nil {
			return err
		}
		if err := lc.cloneTemplate(tr, &flags); err != nil {
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
		if err := lc.cloneTemplate(tr, &flags); err != nil {
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
		flags |= kop.CloneTmplPick
	default:
		return refuse("Clone template: need exactly one of ValidTgts$, Defined$, Choices$")
	}
	if valid == nil && hasCT && ct != "ParentTarget" {
		return refuse("Clone CloneTarget$ %q", ct)
	}

	exc, hasExc, err := lc.copyExceptions(ps)
	if err != nil {
		return err
	}
	in := lInstr{Op: "CloneCopy", Obj: becomer, F: valid, B: cst(flags)}
	if hasExc {
		lc.out.overrides = append(lc.out.overrides, exc)
		in.C = len(lc.out.overrides) // index+1
		in.Note = "except " + overrideString(exc)
	}
	if optional {
		lc.emit(lInstr{Op: "MayAsk", C: kop.MayGuardCloneCopy, Skip: 1, Note: "you may"})
	}
	lc.emit(in)
	return nil
}

// cloneTemplate checks template ref tr (a target slot) and packs its slot
// into flags.
func (lc *lowerCtx) cloneTemplate(tr lRef, flags *int) error {
	if tr.Kind != rTarget || tr.N < 0 || tr.N > 2 || tr.N >= len(lc.ab.Targets) {
		return refuse("Clone template is not a target slot 0..2")
	}
	t := &lc.ab.Targets[tr.N]
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
