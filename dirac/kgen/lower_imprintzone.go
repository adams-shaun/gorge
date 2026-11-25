package kgen

import (
	"fmt"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
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
// lowered. One chain per card, except for a face that keeps its Imprinted on
// the source (lower_imppersist.go): there every writer may sit in its own
// chain, as gorge's list does.
func (lc *lowerCtx) openImprint(what string) error {
	if lc.chainRoot == nil {
		return refuse("%s outside one chain", what)
	}
	if !lc.impPersist && lc.digImprintRoot != nil && lc.digImprintRoot != lc.chainRoot {
		return refuse("%s outside one chain", what)
	}
	lc.digImprintRoot = lc.chainRoot
	return nil
}

// definedImprinted lowers Defined$ Imprinted: inside a RepeatEach iteration
// gorge's spelling names the loop's current subject before the source's
// persistent pile (effects/context.go definedSpecImprinted; Sivriss, Nightmare
// Speaker's Mill body names the repeated opponent, so its chained ChangeZone
// moves no card). A face that keeps its Imprinted on the source reads the
// persistent ref in any ability (lower_imppersist.go); otherwise the DigUntil
// ImprintFound$ chain reads the seek-found chosen set (lower_digimprint.go).
func (lc *lowerCtx) definedImprinted() (lRef, error) {
	if lc.impPersist && (lc.repeatPl > 0 || lc.repeatCard > 0) {
		// Inside a RepeatEach iteration gorge's definedSpecImprinted gives
		// the loop's subject precedence over the source's persistent pile.
		// Only the persist shape has a pile to fall back to; a repeat on the
		// one-resolution shapes stays refused.
		return lRef{Kind: rEach}, nil
	}
	if lc.impPersist || lc.digImprintSetLive() { // lower_digimprintset.go
		return lRef{Kind: rImprinted}, nil
	}
	if !lc.digImprintChosen {
		return lRef{}, refuse("Defined$ Imprinted outside a DigUntil ImprintFound$")
	}
	return lRef{Kind: rChosen}, nil
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
// Unimprint$ and ImprintLast$ on the same writer exile clear the source's
// imprint before the move (gorge clearChangeZoneImprint, from
// changeZonePrelude; at the move ImprintLast replaces rather than appends):
// the kernel emits the same ExtClearImprinted immediately before its exile
// instruction.
func (lc *lowerCtx) changeZoneImprint(ps *params, in *lInstr) error {
	if in.Op == "Exile" {
		if v, ok := ps.get("Unimprint"); ok {
			if v != "True" {
				return refuse("ChangeZone Unimprint$ %q", v)
			}
			if err := lc.changeZoneUnimprint("Unimprint$"); err != nil {
				return err
			}
		}
		if v, ok := ps.get("ImprintLast"); ok {
			if v != "True" {
				return refuse("ChangeZone ImprintLast$ %q", v)
			}
			if err := lc.changeZoneUnimprint("ImprintLast$"); err != nil {
				return err
			}
		}
	}
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

// changeZoneUnimprint emits the pre-move clear of a writer ChangeZone's
// Unimprint$/ImprintLast$: the resolution's imprinted set empties and the
// source's kept records the resolving entry can read go with it (gorge
// clearChangeZoneImprint uses the same event as Cleanup's ClearImprinted$
// rider). Only the persist shape lowers it; anywhere else the params stay
// unread and refuse.
func (lc *lowerCtx) changeZoneUnimprint(what string) error {
	if !lc.impPersist {
		return refuse("ChangeZone %s outside the persist shape", what)
	}
	lc.emit(lInstr{Op: "ClearImprinted"})
	return nil
}

// handMoveImprint reads Imprint$ True on a hand pick onto the battlefield
// (Eureka, Hypergenesis): each card the pick puts onto the battlefield joins
// the resolution's imprinted set (gorge settleChangeZoneMoveAs: a non-token
// whose post-move zone is the destination; an Aura that stays in the hand is
// not imprinted). The kernel reads kop.SrImprint on the ChooseMove's B
// (dirac/zone_forget.go imprintFound). A face that keeps its Imprinted on the
// source stays refused: the pick's writes do not reach the store.
func (lc *lowerCtx) handMoveImprint(ps *params, dest int) (bool, error) {
	v, ok := ps.get("Imprint")
	if !ok {
		return false, nil
	}
	if v != "True" || dest != zBattlefield || lc.impPersist {
		return false, refuse("hand move Imprint$ %q to %s", v, zoneLabel[dest])
	}
	if err := lc.openImprint("ChangeZone Imprint$"); err != nil {
		return false, err
	}
	return true, nil
}

// repeatGateImprinted reports whether the ability being lowered is the body
// of a Repeat whose RepeatCheckSVar$ gate counts the imprinted set (Eureka's
// PutCount: Count$Valid Card.IsImprinted). The body's per-pass Cleanup
// ClearImprinted$ then opens the set its writer fills and the gate reads,
// all in one resolution.
func (lc *lowerCtx) repeatGateImprinted() bool {
	return lc.repeatCheckAb != nil && lc.repeatCheckAb == lc.ab && lc.repeatCheckSVar != "" && !lc.impPersist &&
		strings.Contains(lc.face.SVars[lc.repeatCheckSVar], "IsImprinted")
}

// millImprint reads Mill's Imprint$ True: each milled card joins the imprinted
// set (kop.MvImprint on OpMill C), wherever a replacement sends it. The reads
// that follow sit in the same chain (Patient Naturalist's IsImprinted pick).
func (lc *lowerCtx) millImprint(ps *params, in *lInstr) error {
	v, ok := ps.get("Imprint")
	if !ok {
		return nil
	}
	if v != "True" {
		return refuse("Mill Imprint$ %q", v)
	}
	if err := lc.openImprint("Mill Imprint$"); err != nil {
		return err
	}
	in.CS = strings.TrimPrefix(in.CS+"|ImprintMove", "|")
	return nil
}

// imprintedFilterGate refuses a filter naming IsImprinted outside a chain
// that opened the imprinted set: the kernel's set lives for one resolution.
// A face that keeps its Imprinted on the source reads it in any chain.
func (lc *lowerCtx) imprintedFilterGate(expr string) error {
	if strings.Contains(expr, "IsImprinted") && !lc.impPersist && !lc.digImprintSetLive() {
		return refuse("filter %q: IsImprinted outside a chain that writes the imprinted set", expr)
	}
	return nil
}

// filterReadsImprinted reports whether f carries the IsImprinted word.
func filterReadsImprinted(f *lFilter) bool {
	if f == nil {
		return false
	}
	for _, c := range f.Clauses {
		if c.CmpStat == cmpWord7 && c.CmpN == int(kop.W7Imprinted) {
			return true
		}
	}
	return false
}

// imprintedPTValue is the Imprinted$CardPower/CardToughness head: the imprint
// pile's objects' property (gorge count_ref_property.go; the persist shape
// holds one), the kernel's ExPowerOf/ExToughnessOf over RefImprinted.
func imprintedPTValue(rest string) lValue {
	kind := vPowerOf
	if rest == "CardToughness" {
		kind = vToughnessOf
	}
	return lValue{Kind: kind, Ref: lRef{Kind: rImprinted}}
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
	case "Mill":
		return strings.Contains(in.CS, "ImprintMove")
	case "ChooseMove":
		return strings.Contains(in.CS, "from=Hand") && strings.Contains(in.CS, "|Imprint")
	case "ImprintTriggered":
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
		writes = writes || in.Op == "CreateToken" && in.C&2048 != 0 || in.Op == "Exile" && strings.Contains(in.CS, "ImprintMove") || in.Op == "Mill" && strings.Contains(in.CS, "ImprintMove") || in.Op == "ChooseMove" && imprintWriter(in)
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
// Bookworm, Superior Foes of Spider-Man). A face that keeps its Imprinted on
// the source reads another ability's write, so neither refusal applies.
func (lc *lowerCtx) refuseCrossAbilityImprinted() {
	if lc.impPersist {
		return
	}
	for _, a := range lc.out.Abilities {
		reads, writes := false, false
		for _, in := range a.Prog {
			reads = reads || in.Obj.Kind == rImprinted || in.Obj2.Kind == rImprinted || filterReadsImprinted(in.F)
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
