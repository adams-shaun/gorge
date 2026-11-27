package kgen

import (
	"fmt"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
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
	if in.Op == "Return" && in.C == zBattlefield && in.Obj.Kind == rDelayedRemembered && !lc.impPersist {
		if err := lc.openImprint("ChangeZone Imprint$"); err != nil {
			return err
		}
		lc.retImprint = true // returnImprint brackets the move
		return nil
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

// returnImprint emits a Return whose ChangeZone carries Imprint$ True
// (Phelia, Exuberant Shepherd's delayed return of DelayTriggerRememberedLKI):
// gorge imprints the card when its post-move zone is the battlefield and it
// is not a token (settleChangeZoneMoveAs). OpReturnEx's flag space is full,
// so the kernel brackets the move instead: the delayed program's one object
// joins the imprinted set before the Return (ImprintRef, a token never), and
// after it ForgetImprinted drops it again when the ref still names the same
// incarnation, that is when the Return did not move it (an Aura with nothing
// to enchant stays in exile, CR 303.4g). A moved object is a new one (CR
// 400.7), so RefDelayedRemembered reads nothing after the move and the
// imprint stays. Only the plain single-object move lowers.
func (lc *lowerCtx) returnImprint(in lInstr, optional, bracket, fd bool) error {
	lc.retImprint = false
	if optional || bracket || fd || in.Obj.Kind != rDelayedRemembered {
		return refuse("ChangeZone Imprint$ on a Return beside Optional$, a library placement or a face-down exile")
	}
	lc.emit(lInstr{Op: "ImprintRef", Obj: in.Obj, Note: "imprints the object the Return is about to move"})
	lc.emit(in)
	lc.emit(lInstr{Op: "ForgetImprinted", Obj: in.Obj, Note: "forgets it again when the Return did not move it"})
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

// handMoveImprint reads Imprint$ True on a hand pick: each card the pick
// moves joins the resolution's imprinted set (gorge settleChangeZoneMoveAs: a
// non-token whose post-move zone is the destination; an Aura that stays in
// the hand is not imprinted). The kernel reads kop.SrImprint on the
// ChooseMove's B (dirac/zone_forget.go imprintFound). Two shapes lower: a
// put onto the battlefield in one resolution (Eureka, Hypergenesis), and an
// exile on a face that keeps its Imprinted on the source (Parallax Nexus),
// where the kernel's imprint also records the card in the source's store.
func (lc *lowerCtx) handMoveImprint(ps *params, dest int) (bool, error) {
	if lc.imprintMox {
		return false, nil // the mox's own imprint (lower_imprintmox.go) read it
	}
	v, ok := ps.get("Imprint")
	if !ok {
		return false, nil
	}
	if v != "True" || dest == zBattlefield && lc.impPersist || dest == zExile && !lc.impPersist || dest != zBattlefield && dest != zExile {
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

// imprintNameSweep lowers Eye of Singularity's "each permanent with the same
// name as another permanent": a RepeatEach over RepeatCards$ R whose one body
// imprints `Valid X.sharesNameWith Remembered+!IsRemembered` (each other X
// sharing the repeated card's name), followed by a DestroyAll of
// Card.IsImprinted. The imprinted set is then exactly the X permanents that
// share a name with another battlefield permanent matching R, which is the
// filter X.sharesNameWith Valid R (gorge sharesNameValidMatches: another
// battlefield permanent, not the candidate, matching R and sharing its
// name). The loop asks nothing and the sweep is one destroy, so the
// DestroyAll reads that filter (lc.impSweep) and no set is kept. Any other
// body, rider or follower keeps its refusal.
func (lc *lowerCtx) imprintNameSweep(ps *params, cardsSpec string) (bool, error) {
	if cardsSpec == "" || strings.ContainsAny(cardsSpec, "+,") || lc.impPersist {
		return false, nil
	}
	body := cards.ResolveSVar(lc.face.SVars, ps.peek("RepeatSubAbility"))
	if body == nil || body.Sub != nil || body.API != "Pump" && body.API != "Bookkeeping" || len(body.Params) != 1 {
		return false, nil
	}
	x, ok := strings.CutPrefix(strings.TrimSpace(body.Params["ImprintCards"]), "Valid ")
	if !ok {
		return false, nil
	}
	x, ok = strings.CutSuffix(x, ".sharesNameWith Remembered+!IsRemembered")
	if !ok || strings.ContainsAny(x, ".+,") {
		return false, nil
	}
	next := lc.curNode.Sub
	if next == nil || next.API != "DestroyAll" || strings.TrimSpace(next.Params["ValidCards"]) != "Card.IsImprinted" {
		return false, nil
	}
	for k := range ps.m {
		switch k {
		case "RepeatCards", "RepeatSubAbility", "SubAbility":
		default:
			return false, nil
		}
	}
	ps.consume("RepeatCards", "RepeatSubAbility")
	if err := lc.openImprint("RepeatEach ImprintCards$ sharesNameWith"); err != nil {
		return true, err
	}
	lc.impSweep = x + ".sharesNameWith Valid " + cardsSpec
	return true, nil
}

// imprintedReturnAll lowers ChangeZone Defined$ Imprinted Origin$ Exile
// Destination$ Hand on a face that keeps its Imprinted on the source
// (Parallax Nexus's leave trigger: "each player returns to their hand all
// cards they own exiled with it"). The kernel's Bounce moves one object, the
// first of a set ref, so the move is the exile sweep of Card.IsImprinted to
// its owners' hands (OpChangeZoneAllEx): the same cards, in exile order,
// which is the order they were imprinted. Any other rider keeps the plain
// path.
func (lc *lowerCtx) imprintedReturnAll(ps *params) (bool, error) {
	if !lc.impPersist || ps.peek("Defined") != "Imprinted" || ps.peek("Origin") != "Exile" || ps.peek("Destination") != "Hand" {
		return false, nil
	}
	for k := range ps.m {
		switch k {
		case "Defined", "Origin", "Destination", "SubAbility":
		default:
			return false, nil
		}
	}
	delete(ps.m, "Defined")
	ps.m["ChangeType"] = "Card.IsImprinted"
	return true, lc.apiChangeZoneAll(lc.curNode, ps)
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
	case "ImprintTriggered", "ImprintRef":
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
