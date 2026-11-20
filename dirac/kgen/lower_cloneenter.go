package kgen

import (
	"reflect"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// cloneEnterCounterRider accepts exactly Dominion Saboteur's rider chain: a
// Clone body with RememberCloneOrigin$ True, then one PutCounter with Defined$
// Self, ETB$ True, CounterType$ EachFromSource and EachFromSource$ Remembered
// (the copy origin is the remembered choice), then optionally the trailing
// Cleanup ClearRemembered$ True. Everything else about the body stays
// cloneEnter's. The chain is consumed here rather than lowered as separate
// instructions: OpBecomeCopy's BecomeCopyCtr flag carries the counters during
// the entry, where gorge's rider runs before the enters triggers match.
func (lc *lowerCtx) cloneEnterCounterRider(ps *params, sa *cards.SA) error {
	if !ps.is("RememberCloneOrigin", "True") {
		return refuse("ETB copy chained rider without RememberCloneOrigin$")
	}
	link := sa.Sub
	if link.API != "PutCounter" {
		return refuse("ETB copy chained rider %s", link.API)
	}
	lp := newParams(link.Params)
	if !lp.is("ETB", "True") || lp.str("Defined") != "Self" ||
		!strings.EqualFold(lp.str("CounterType"), "EachFromSource") ||
		lp.str("EachFromSource") != "Remembered" {
		return refuse("ETB copy chained rider shape")
	}
	lp.consume("SubAbility")
	if left := lp.unread(); len(left) > 0 {
		return refuse("ETB copy chained rider unread params %v", left)
	}
	if link.Sub == nil {
		return nil
	}
	cleanup := link.Sub
	if cleanup.API != "Cleanup" || cleanup.Sub != nil {
		return refuse("ETB copy chained rider tail %s", cleanup.API)
	}
	cp := newParams(cleanup.Params)
	if !cp.is("ClearRemembered", "True") {
		return refuse("ETB copy chained rider cleanup shape")
	}
	if left := cp.unread(); len(left) > 0 {
		return refuse("ETB copy chained rider cleanup unread params %v", left)
	}
	return nil
}

// cloneEnter lowers "you may have this enter as a copy of <Choices$>,
// except ..." (gorge's ETB clone: cast_etbchoice.go collectETBChoices
// kind "copy", effects/clone.go effClone with CloneEnter.ETB): the clone
// ask is posed while the spell is on the stack, over the battlefield
// permanents matching Choices$ (gorge etbOptions: seat 0's battlefield,
// then seat 1's), plus "enter as itself" when the keyword line is
// :Optional (or nothing matches). Exceptions: copyExceptions' list
// (AddTypes$, AddKeywords$, SetPower$/SetToughness$, colours...); NewName$
// and AddKeywords$ Changeling refuse by name.
func (lc *lowerCtx) cloneEnter(ps *params, node, sa *cards.SA) error {
	if node != sa || len(lc.ab.Prog) != 0 {
		return refuse("ETB copy in a chain")
	}
	// Dominion Saboteur's exact counter rider (remembered copy origin, then
	// PutCounter EachFromSource$ Remembered, then Cleanup): the whole chain is
	// folded into the BecomeCopy op's BecomeCopyCtr flag, so the kernel carries
	// the counters into the entering object before its enters triggers match.
	rememberCounters := false
	if node.Sub != nil {
		if err := lc.cloneEnterCounterRider(ps, sa); err != nil {
			return err
		}
		rememberCounters = true
	}
	if _, ok := ps.get("Defined"); ok {
		return refuse("ETB copy Defined$")
	}
	ch, ok := ps.get("Choices")
	if !ok {
		return refuse("ETB copy without Choices$")
	}
	lc.cloneRepl = true
	f, err := lc.filter(ch)
	lc.cloneRepl = false
	if err != nil {
		return err
	}
	for _, c := range f.Clauses {
		// a copied Aura must attach and a copied planeswalker enters with
		// its loyalty: neither is modelled, so the templates are creatures
		// and artifacts only
		if c.Types&^(tCreature|tArtifact) != 0 || c.Types == 0 && c.Subtype != "Equipment" {
			return refuse("ETB copy of %q (not a creature or artifact)", ch)
		}
	}
	// the copy exceptions (CR 707.9b), read through the shared
	// copyExceptions; a name change and the Changeling CDA are not copy
	// exceptions gorge applies the same way (clone.go L531), refused by name
	if _, ok := ps.get("NewName"); ok {
		return refuse("ETB copy NewName$")
	}
	if k, ok := ps.get("AddKeywords"); ok {
		for _, w := range strings.Split(k, "&") {
			if strings.TrimSpace(w) == "Changeling" {
				return refuse("ETB copy AddKeywords Changeling: a CDA, not an exception (gorge clone.go L531)")
			}
		}
	}
	embalm, err := cloneEnterEmbalmShape(ps)
	if err != nil {
		return err
	}
	exc, hasExc, myriads, err := lc.copyExceptionsMyriad(ps, true)
	if err != nil {
		return err
	}
	grant, err := lc.cloneEnterGrant(ps) // lower_cloneentergrant.go
	if err != nil {
		return err
	}
	if embalm && hasExc {
		if err := cloneEnterEmbalmGated(exc, grant, myriads); err != nil {
			return err
		}
	}
	if myriads > 0 {
		// "except it has myriad" (Auton Soldier; CR 707.9b, 702.116a-b):
		// one granted-Myriad trigger per instance rides in the grant record
		// beside any AddAbilities$ (lower_myriad.go); at most two grants
		if grant == nil {
			grant = &lOverride{}
		}
		for ; myriads > 0; myriads-- {
			idx, err := lc.myriadGrantTrigger()
			if err != nil {
				return err
			}
			grant.GrantAbility = append(grant.GrantAbility, idx)
		}
		if len(grant.GrantAbility) > 2 {
			return refuse("ETB copy grants %d abilities (Myriad included): more than two", len(grant.GrantAbility))
		}
	}
	in := lInstr{Op: "BecomeCopy", Obj: lRef{Kind: rReplaced}, F: f}
	if grant != nil {
		lc.out.overrides = append(lc.out.overrides, grant)
		in.A = cst(len(lc.out.overrides)) // index+1
	}
	flags := 0
	if rememberCounters {
		flags |= kop.BecomeCopyCtr
		in.Note = "with the copy origin's counters"
	}
	if embalm && hasExc {
		flags |= kop.BecomeCopyEmbalm
	}
	if flags != 0 {
		in.B = cst(flags)
	}
	if lc.replOptional {
		in.C = 1
	}
	if hasExc {
		lc.out.overrides = append(lc.out.overrides, exc)
		in.C |= len(lc.out.overrides) << 1 // index+1
		in.Note = "except " + overrideString(exc)
		if embalm {
			in.Note = "if embalmed, " + in.Note
		}
	}
	lc.emit(in)
	return nil
}

// cloneEnterEmbalmShape reads the Embalm$ gate of an as-enters copy body and
// holds gorge's shape of it (effects clone_params.go cloneETBShapeOK): an
// Embalm$ spelling other than True fails closed, and RemoveCost$ is lowered
// only as an Embalm$ True body's "no mana cost" rider, never as an
// unconditional one (it would rewrite a hard-cast carrier's mana cost).
func cloneEnterEmbalmShape(ps *params) (embalm bool, err error) {
	if v, ok := ps.get("Embalm"); ok {
		if !strings.EqualFold(strings.TrimSpace(v), "True") {
			return false, refuse("ETB copy Embalm$ %q (gorge fails an Embalm$ other than True closed)", v)
		}
		embalm = true
	}
	if v, ok := ps.get("RemoveCost"); ok && (!embalm || !strings.EqualFold(strings.TrimSpace(v), "True")) {
		return false, refuse("ETB copy RemoveCost$ %q without Embalm$ True (gorge withholds an unconditional no-mana-cost rider)", v)
	}
	return embalm, nil
}

// cloneEnterEmbalmGated holds an Embalm$ True body's exception override to the
// riders gorge gates (cloneEmbalmGate: AddTypes$, SetColor$, RemoveCost$): the
// kernel gate drops the whole override for a hard-cast carrier, so any other
// exception beside it (gorge applies those ungated) refuses.
func cloneEnterEmbalmGated(exc *lOverride, grant *lOverride, myriads int) error {
	gated := lOverride{AddTypes: exc.AddTypes, AddSubtypes: exc.AddSubtypes,
		SetColors: exc.SetColors, HasSetColors: exc.HasSetColors, NoCost: exc.NoCost}
	if grant != nil || myriads > 0 || !reflect.DeepEqual(*exc, gated) {
		return refuse("ETB copy Embalm$ True beside an exception gorge applies ungated (only AddTypes$, SetColor$, RemoveCost$ ride the gate)")
	}
	return nil
}
