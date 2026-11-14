package kgen

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// putCounterAll lowers api:PutCounterAll (gorge effPutCounterAll,
// engine/effects/counters.go:251): every battlefield permanent matching
// ValidCards$ (default Permanent) gets CounterNum$ counters of CounterType$
// (default P1P1), scoped to a chosen player target when ValidTgts$ Player
// is present (putCounterAllSweep, :295). A second batch
// (ValidCards2$/CounterType2$/CounterNum2$) runs after the first with its
// own filter (default: the first ValidCards$), kind (default P1P1) and
// count (default 1), and the same player scope. Placer$,
// AmountByChosenMap$, ValidZone$, TargetUnique$ and Defined$ stay refused:
// gorge's effPutCounterAll treats each as an exotic shape and places
// nothing at all (TargetUnique$: counters.go:261).
func (lc *lowerCtx) putCounterAll(ps *params) error {
	for _, k := range []string{"Placer", "AmountByChosenMap", "ValidZone", "Defined"} {
		if _, ok := ps.get(k); ok {
			return refuse("PutCounterAll %s$", k)
		}
	}
	// gorge checks TargetUnique$ presence (TgtUniqueSet), not its value:
	// any non-empty spelling makes effPutCounterAll emit an exotic Note and
	// return without placing a single counter.
	if v, ok := ps.get("TargetUnique"); ok && strings.TrimSpace(v) != "" {
		return refuse("PutCounterAll TargetUnique$")
	}
	// ValidCardsDesc$ is description text: gorge's effPutCounterAll never
	// reads it (Katilda, Dawnhart Prime's "creature you control")
	ps.consume("ValidCardsDesc")
	raw := ps.str("ValidCards")
	if raw == "" {
		return refuse("PutCounterAll without ValidCards$")
	}
	spec, err := lc.sweepTgtPlayer(ps, raw)
	if err != nil {
		return err
	}
	ct := ps.str("CounterType")
	if ct == "" {
		ct = "P1P1"
	}
	n, err := lc.amount(ps, "CounterNum", 1)
	if err != nil {
		return err
	}
	f, err := lc.filter(spec)
	if err != nil {
		return err
	}
	note := ""
	if _, ok := ps.get("ValidTgts"); ok {
		note = sweepTgtPlayerNote
	}
	first := lInstr{Op: "PutCounterAll", F: f, A: n, CS: ct, Note: note}
	remAmount, err := rememberParam(ps, "RememberAmount")
	if err != nil {
		return err
	}

	vc2, _ := ps.get("ValidCards2")
	ct2, _ := ps.get("CounterType2")
	vc2, ct2 = strings.TrimSpace(vc2), strings.TrimSpace(ct2)
	if vc2 == "" && ct2 == "" {
		// gorge only enters the second sweep when a non-empty ValidCards2$
		// or CounterType2$ is present.
		if _, ok := ps.get("CounterNum2"); ok {
			return refuse("PutCounterAll CounterNum2$ without ValidCards2$/CounterType2$")
		}
		if remAmount {
			// RememberAmount$ True: gorge appends each recipient once per
			// counter that landed, so Count$RememberedNumber reads the
			// sweep's landed total (Guinan, Ancient Advisor's "two or more
			// +1/+1 counters"); the kernel adds it to the remembered count,
			// and refuseRememberAmountObjects refuses an object read
			if note != "" {
				// the note marks the targeted-player sweep (lower_sweeptargetplayer.go)
				return refuse("PutCounterAll RememberAmount$ with ValidTgts$ Player")
			}
			first.B, first.Note = cst(kop.PutCtrRemAmount), notePutRememberAmount
		}
		lc.emit(first)
		return nil
	}
	if remAmount {
		return refuse("PutCounterAll RememberAmount$ with a second batch")
	}
	// one action across both sweeps: one CounterPlayerAddedAll trigger per
	// recipient (gorge's counterBatch latch; kop.PutCtrCellFirst)
	first.B = cst(kop.PutCtrCellFirst)
	lc.emit(first)
	// gorge's second sweep reads its own ValidCards2$ (default: the first
	// batch's ValidCards$) but the SAME ValidTgts$ Player scope, because
	// putCounterAllSweep reads TargetsOf(sa) on both calls.
	raw2 := vc2
	if raw2 == "" {
		raw2 = raw
	}
	if _, ok := ps.get("ValidTgts"); ok {
		if raw2, err = lc.sweepTgtPlayerSpec(raw2); err != nil {
			return err
		}
	}
	ctSecond := ct2
	if ctSecond == "" {
		ctSecond = "P1P1"
	}
	if k1, err := counterKind(ct); err == nil {
		if k2, err := counterKind(ctSecond); err == nil && k1 == k2 {
			// gorge keeps both batches inside one beginActionBatch bracket,
			// so a CounterAddedOnce listener fires once per permanent for
			// the whole action. The kernel fires its trigger per
			// instruction, so a second batch of the SAME kind on a
			// permanent both filters admit would fire twice. Different
			// kinds fire a per-kind listener once each, and a
			// CounterPlayerAddedAll listener once through the cell
			// (kop.PutCtrCellFirst), so only this shape refuses.
			return refuse("PutCounterAll second batch of the same counter kind %q", ctSecond)
		}
	}
	n2, err := lc.amount(ps, "CounterNum2", 1)
	if err != nil {
		return err
	}
	f2, err := lc.filter(raw2)
	if err != nil {
		return err
	}
	lc.emit(lInstr{Op: "PutCounterAll", F: f2, A: n2, B: cst(kop.PutCtrCellNext), CS: ctSecond, Note: note})
	return nil
}

// gainEnergy lowers PutCounter CounterType$ ENERGY on a player (gorge: the
// player's ENERGY counter) to OpGainEnergy. An object recipient refuses.
func (lc *lowerCtx) gainEnergy(obj lRef, n lValue) error {
	switch obj.Kind {
	case rYou, rEachOpp, rEachPlayer:
	default:
		return refuse("energy counters on %s", obj)
	}
	lc.emit(lInstr{Op: "GainEnergy", Obj: obj, A: n})
	return nil
}

// moveCounter lowers api:MoveCounter (gorge effMoveCounter) with one origin
// (Source$ Self or ParentTarget) and one destination (the node's target or
// a Defined$ object), one named CounterType$ and CounterNum$ a value
// (default 1) or All. Any / All / EachNotOn kinds, CounterNum$ Any (an
// ask), ValidSource$ / ValidDefined$ sweeps and the remember riders
// refuse.
func (lc *lowerCtx) moveCounter(ps *params) error {
	for _, k := range []string{"ValidSource", "ValidDefined", "RememberPut", "RememberAmount", "NonZero"} {
		if _, ok := ps.get(k); ok {
			return refuse("MoveCounter %s$", k)
		}
	}
	kind := ps.str("CounterType")
	switch kind {
	case "", "Any", "All", "EachNotOn":
		return refuse("MoveCounter CounterType %q", kind)
	}
	var src lRef
	var dst lRef
	dstDone := false
	switch sv := ps.str("Source"); sv {
	case "":
		// No Source$ and a Defined$ destination: gorge's moveCounterOrigin
		// takes the chosen target as the origin (moveCounterNamesDestination:
		// the whole chosen set, one slot here) and Defined$ is where it goes
		// (Cytoplast Root-Kin: Defined$ Self, ValidTgts$ Creature.YouCtrl).
		// Without Defined$ the 2-target shape stays refused.
		if _, ok := ps.get("Defined"); !ok {
			return refuse("MoveCounter Source %q", sv)
		}
		if _, ok := ps.get("ValidTgts"); !ok {
			return refuse("MoveCounter Source %q", sv)
		}
		d, err := lc.objOrTarget(ps, "", zBattlefield)
		if err != nil {
			return err
		}
		dst, dstDone = d, true
		src = lRef{Kind: rTarget, N: lc.lastTarget()}
	case "Self":
		src = lRef{Kind: rSelf}
	case "ParentTarget":
		r, err := lc.defined(sv)
		if err != nil {
			return err
		}
		src = r
	default:
		return refuse("MoveCounter Source %q", sv)
	}
	var n lValue
	switch cn := ps.str("CounterNum"); cn {
	case "All":
		n = cst(-1)
	case "Any":
		return refuse("MoveCounter CounterNum Any")
	default:
		var err error
		if n, err = lc.amount(ps, "CounterNum", 1); err != nil {
			return err
		}
		if n.isConst() && n.N < 0 {
			return refuse("MoveCounter CounterNum %v", n)
		}
	}
	if !dstDone {
		var err error
		if dst, err = lc.objOrTarget(ps, "", zBattlefield); err != nil {
			return err
		}
	}
	for _, r := range []lRef{src, dst} {
		switch r.Kind {
		case rSelf, rTriggered:
		case rTarget:
			t := lc.ab.Targets[r.N]
			if t.F == nil || len(t.F.Clauses) == 0 || t.Zone != zBattlefield || !t.Max.isConst() || t.Max.N > 1 {
				return refuse("MoveCounter target slot %d", r.N)
			}
		default:
			return refuse("MoveCounter of %s", r)
		}
	}
	if src == dst {
		return refuse("MoveCounter onto its origin")
	}
	lc.emit(lInstr{Op: "MoveCounter", Obj: dst, Obj2: src, A: n, CS: kind})
	return nil
}

// support lowers PutCounter Support$ N (CR 701.41a; gorge
// putCounterSupport): one counter (CounterType$, default P1P1) on each of
// up to N creatures the resolving controller picks, other than the source
// unless the card is an instant or sorcery (gorge's Creature.+Other /
// Creature split by the source's face). A Defined$/ValidTgts$/Choices$/
// CounterNum$ beside it refuses.
func (lc *lowerCtx) support(ps *params) error {
	for _, k := range []string{"Defined", "ValidTgts", "Choices", "CounterNum", "Adapt", "RememberCards", "RememberPut"} {
		if _, ok := ps.m[k]; ok {
			return refuse("Support$ with %s$", k)
		}
	}
	n, err := lc.amount(ps, "Support", 1)
	if err != nil {
		return err
	}
	ct := "P1P1"
	if c, ok := ps.get("CounterType"); ok {
		ct = c
	}
	other := 1
	if lc.face.IsInstant() || lc.face.IsSorcery() {
		other = 0
	}
	lc.emit(lInstr{Op: "Support", A: n, C: other, CS: ct})
	return nil
}

// putCounterUnion lowers a PutCounter whose Defined$ is `<A> & <B>` (gorge
// knownDefinedTargets, context.go: the selectors join in script order with no
// dedupe, so one object named twice is hit twice). It lowers the node once per
// part, in order. Each part must be a bare selector naming a single object;
// the node must repeat safely (no Condition*$, a constant or absent
// CounterNum$). handled reports whether the node was this shape.
func (lc *lowerCtx) putCounterUnion(sa *cards.SA, ps *params) (handled bool, err error) {
	d, ok := ps.m["Defined"]
	if sa.API != "PutCounter" || !ok || !strings.Contains(d, " & ") {
		return false, nil
	}
	parts := strings.Split(d, " & ")
	if len(parts) != 2 {
		return true, refuse("PutCounter Defined$ %q: not two selectors", d)
	}
	for k := range ps.m {
		if strings.HasPrefix(k, "Condition") {
			return true, refuse("PutCounter Defined$ %q beside %s$", d, k)
		}
	}
	if n, ok := ps.m["CounterNum"]; ok {
		for _, r := range strings.TrimSpace(n) {
			if r < '0' || r > '9' {
				return true, refuse("PutCounter Defined$ %q with CounterNum$ %q", d, n)
			}
		}
	}
	for i, p := range parts {
		p = strings.TrimSpace(p)
		parts[i] = p
		if p == "TriggeredCards" || strings.ContainsAny(p, ".&") {
			return true, refuse("PutCounter Defined$ %q: part %q is not a bare single object", d, p)
		}
		r, err := lc.defined(p)
		if err != nil {
			return true, err
		}
		switch r.Kind {
		case rSelf, rTriggered, rTriggeredNew, rHost, rEquipped:
		default:
			return true, refuse("PutCounter Defined$ %q: part %q is not a single object", d, p)
		}
	}
	for _, p := range parts {
		ps.m["Defined"] = p
		if err := lc.node(sa, ps); err != nil {
			return true, err
		}
	}
	return true, nil
}

// bolster lowers PutCounter's Bolster$ N (CR 701.39a; gorge
// counters_put.go putCounterBolster): the resolving controller chooses a
// creature they control with the least CURRENT (layer-derived) toughness
// and puts N counters of CounterType$ (default P1P1) on it; a tie is a
// counter_pick ask, a single candidate is taken unasked, N <= 0 or no
// creature does nothing. Bolster$ names no recipient: a Defined$ or
// ValidTgts$ beside it refuses, as does any other unread parameter.
func (lc *lowerCtx) bolster(ps *params) error {
	for _, k := range []string{"Defined", "ValidTgts", "Choices", "Support", "Adapt", "CounterNum"} {
		if _, ok := ps.m[k]; ok {
			return refuse("Bolster$ with %s$", k)
		}
	}
	n, err := lc.amount(ps, "Bolster", 1)
	if err != nil {
		return err
	}
	ct := "P1P1"
	if c, ok := ps.get("CounterType"); ok {
		ct = c
	}
	lc.emit(lInstr{Op: "Bolster", A: n, CS: ct})
	return nil
}

// proliferate lowers api:Proliferate (CR 701.34a; gorge
// effProliferate): the resolving controller proliferates Amount$ times
// (default 1). A literal Amount$ must be at least 1; any other Amount$ is
// lowered through lc.value (an SVar count, Tam, the Possibility's distinct
// planeswalker types), and refuses as lc.value does. A paid X (vX), a
// negated amount and an inline Number$ expression still refuse: gorge's Num
// read of an X without an announcement loud-degrades to nothing, and its
// read of Expand the Sphere's Number$2/Minus.<remembered> still proliferates
// after two lands, neither of which a lowered value would copy. A count of
// zero proliferates nothing on both seats. RememberPut$, Defined$ and
// ValidTgts$ refuse.
func (lc *lowerCtx) proliferate(ps *params) error {
	for _, k := range []string{"RememberPut", "Defined", "ValidTgts"} {
		if _, ok := ps.m[k]; ok {
			return refuse("Proliferate %s$", k)
		}
	}
	amt := cst(1)
	if a, ok := ps.get("Amount"); ok {
		if v, err := strconv.Atoi(a); err == nil {
			if v < 1 {
				return refuse("Proliferate Amount$ %q", a)
			}
			amt = cst(v)
		} else if strings.Contains(a, "$") {
			return refuse("Proliferate Amount$ %q", a)
		} else {
			val, err := lc.value(a)
			if err != nil {
				return refuse("Proliferate Amount$ %q: %v", a, err)
			}
			if val.Kind == vX || val.Neg {
				return refuse("Proliferate Amount$ %q", a)
			}
			amt = val
		}
	}
	lc.emit(lInstr{Op: "Proliferate", A: amt})
	return nil
}

// putCounterEachFromSource lowers PutCounter CounterType$ EachFromSource
// (gorge putCounterEachFromSource): the recipient takes a counter of each
// kind the EachFromSource$ source had, read through gorge's LKI ladder
// (live positive counters, else the trigger's or the cost sacrifice's
// snapshot), CounterNum$ times (default 1). The referent is lowered as the
// kernel reads it: TriggeredCard(LKICopy) is the trigger object (its
// snapshot rides the resolving entry), Self the source (a cost-sacrificed
// source's snapshot rides the entry too, Zack Fair). Remembered (Dominion
// Saboteur) and any other spelling refuse.
func (lc *lowerCtx) putCounterEachFromSource(ps *params, obj lRef) error {
	var src lRef
	switch ref := ps.str("EachFromSource"); ref {
	case "TriggeredCardLKICopy", "TriggeredCard":
		if lc.ab.Trigger == nil {
			return refuse("EachFromSource$ %s outside a trigger", ref)
		}
		src = lRef{Kind: rTriggered}
	case "Self":
		src = lRef{Kind: rSelf}
	default:
		return refuse("EachFromSource$ %q", ref)
	}
	n, err := lc.amount(ps, "CounterNum", 1)
	if err != nil {
		return err
	}
	lc.emit(lInstr{Op: "PutCountersFrom", Obj: obj, A: n, Obj2: src})
	return nil
}

// clearCounters lowers RemoveCounter CounterType$ All CounterNum$ All
// (gorge effRemoveCounter's AllKinds + NumAll arm: every counter of every
// kind the object holds, one removal per kind) to OpClearCounters. Only a
// battlefield object subject: a player subject (Final Act's "each opponent
// loses all counters", Suncleanser's opponent arm) or a target slot that
// admits players refuses (the kernel's player counters are poison and
// energy, and no liftable carrier removes them). RememberRemoved$ /
// RememberAmount$ beside it stay unread and refuse.
func (lc *lowerCtx) clearCounters(ps *params, obj lRef) error {
	switch obj.Kind {
	case rYou, rEachOpp, rEachPlayer, rTargetController, rTargetOwner, rTriggeredPlayer:
		return refuse("RemoveCounter All of every kind on a player (%s)", obj)
	case rTarget:
		if f := lc.ab.Targets[obj.N].F; f != nil && f.Players != 0 {
			return refuse("RemoveCounter All of every kind on a player (%s)", obj)
		}
	}
	lc.emit(lInstr{Op: "ClearCounters", Obj: obj})
	return nil
}

// removeCounterAll lowers api:RemoveCounterAll (gorge effRemoveCounterAll:
// every battlefield permanent matching ValidCards$, default Permanent,
// loses CounterNum$ counters of CounterType$, default 1, or every counter
// of that kind when AllCounters$ is True). gorge returns without acting
// when CounterType$ is absent, so Aether Snap's AllCounterTypes$ shape
// (no CounterType$) is refused rather than lowered to a no-op; ValidZone$,
// Defined$ and the rest stay unread and refuse.
func (lc *lowerCtx) removeCounterAll(ps *params) error {
	ct := ps.str("CounterType")
	if ct == "" {
		return refuse("RemoveCounterAll without CounterType$")
	}
	spec := ps.str("ValidCards")
	if spec == "" {
		spec = "Permanent"
	}
	f, err := lc.filter(spec)
	if err != nil {
		return err
	}
	n, err := lc.amount(ps, "CounterNum", 1)
	if err != nil {
		return err
	}
	in := lInstr{Op: "RemoveCounterAll", F: f, A: n, CS: ct}
	if ps.is("AllCounters", "True") {
		in.B = cst(1)
		in.Note = "all of the kind"
	}
	lc.emit(in)
	return nil
}
