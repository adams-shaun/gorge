package kgen

import (
	"fmt"
	"strings"
)

// lower_imprintzone.go: the Imprint$ writers of a Token (ImprintTokens$) and
// of an exile ChangeZone, beside the Dig writer of lower_digimprintset.go.
// gorge records the imprint on the source and keeps it until a Cleanup
// ClearImprinted$ or the source leaves the battlefield; the kernel keeps the
// set for one resolution (genState.imp), so the writer, every read and the
// clearing Cleanup must sit in one chain. The reads (Defined$ Imprinted) and
// the Cleanup consult digImprintSetLive for that; a read in another ability
// stays refused (refuseCrossAbilityImprinted).

// openImprint opens the resolution's imprinted set for the chain being
// lowered. One chain per card: a second chain with its own writer refuses.
func (lc *lowerCtx) openImprint(what string) error {
	if lc.chainRoot == nil || lc.digImprintRoot != nil && lc.digImprintRoot != lc.chainRoot {
		return refuse("%s outside one chain", what)
	}
	lc.digImprintRoot = lc.chainRoot
	return nil
}

// tokenImprint reads Token's ImprintTokens$ True: the created tokens join the
// imprinted set (kop.TokImprintObj). The destination Ref and the AtEOT kind
// share B.N's high byte with the flag's bit, so those riders refuse.
func (lc *lowerCtx) tokenImprint(ps *params, in *lInstr) error {
	v, ok := ps.get("ImprintTokens")
	if !ok {
		return nil
	}
	if v != "True" {
		return refuse("Token ImprintTokens$ %q", v)
	}
	if in.C&(32|512|1024) != 0 {
		return refuse("Token ImprintTokens$ beside AttachedTo$ or an extended AtEOT$")
	}
	if err := lc.openImprint("Token ImprintTokens$"); err != nil {
		return err
	}
	in.C |= 2048
	return nil
}

// changeZoneImprint reads ChangeZone's Imprint$ True on a targeted or defined
// exile (the kernel's ExileEx): each moved card joins the imprinted set. A
// hidden-zone pick, a hand return or any other destination stays refused.
func (lc *lowerCtx) changeZoneImprint(ps *params, in *lInstr) error {
	v, ok := ps.get("Imprint")
	if !ok {
		return nil
	}
	if v != "True" {
		return refuse("ChangeZone Imprint$ %q", v)
	}
	if in.Op != "Exile" {
		return refuse("ChangeZone Imprint$ on a %s move", in.Op)
	}
	if err := lc.openImprint("ChangeZone Imprint$"); err != nil {
		return err
	}
	in.CS = strings.TrimPrefix(in.CS+"|ImprintMove", "|")
	return nil
}

// imprintWriter reports whether in adds to the resolution's imprinted set.
func imprintWriter(in lInstr) bool {
	switch in.Op {
	case "CreateToken":
		return in.C&2048 != 0
	case "Exile":
		return strings.Contains(in.CS, "ImprintMove")
	case "DigImprint":
		return true
	}
	return false
}

// unclearedImprint reports whether prog adds to the imprinted set through a
// Token or exile writer but never clears it unconditionally. gorge keeps the
// imprint on the source across resolutions until a ClearImprinted$ Cleanup
// (or the source leaves the battlefield), the kernel empties its set at each
// resolution, so a repeatable ability without the Cleanup would see only the
// newest objects from its second resolution on.
func unclearedImprint(prog []lInstr) bool {
	writes, clears := false, false
	for j, in := range prog {
		writes = writes || in.Op == "CreateToken" && in.C&2048 != 0 || in.Op == "Exile" && strings.Contains(in.CS, "ImprintMove")
		if in.Op != "ClearImprinted" || in.Cond.Kind != cNone {
			continue
		}
		guarded := false
		for i := 0; i < j; i++ {
			guarded = guarded || prog[i].Skip > 0 && j <= i+prog[i].Skip
		}
		clears = clears || !guarded
	}
	return writes && !clears
}

// refuseCrossAbilityImprinted refuses each ability that reads the imprinted
// set but never writes it in its own program: gorge's imprint persists on the
// source, the kernel's set does not outlive the resolution (Oblivious
// Bookworm, Superior Foes of Spider-Man).
func (lc *lowerCtx) refuseCrossAbilityImprinted() {
	for _, a := range lc.out.Abilities {
		reads, writes := false, false
		for _, in := range a.Prog {
			reads = reads || in.Obj.Kind == rImprinted || in.Obj2.Kind == rImprinted
			writes = writes || imprintWriter(in)
		}
		if unclearedImprint(a.Prog) {
			lc.refuseNode(fmt.Sprintf("ab[%d]", a.Index), "imprints without an unconditional Cleanup ClearImprinted$ (gorge's imprint outlives the resolution; the kernel's set does not)")
		}
		if reads && !writes {
			lc.refuseNode(fmt.Sprintf("ab[%d]", a.Index), "reads Imprinted written by another ability (the kernel's imprinted set is per resolution; gorge's persists on the source)")
		}
	}
}
