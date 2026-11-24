package kgen

import (
	"github.com/adams-shaun/gorge/dirac/kop"
	"strconv"
	"strings"
)

// earthbend lowers Earthbend N (CR 701.66; gorge effEarthbend). gorge's
// parse step writes ValidTgts$ Land.YouCtrl into every Earthbend without
// one (normalizeImplicitTarget), which the IR already carries; any other
// ValidTgts$ is not that shape.
func (lc *lowerCtx) earthbend(ps *params) error {
	if v, ok := ps.m["ValidTgts"]; !ok {
		ps.m["ValidTgts"] = "Land.YouCtrl"
	} else if v != "Land.YouCtrl" {
		return refuse("Earthbend ValidTgts$ %q", v)
	}
	obj, err := lc.objOrTarget(ps, "Self", zBattlefield)
	if err != nil {
		return err
	}
	n, err := lc.amount(ps, "Num", 0)
	if err != nil {
		return err
	}
	ks, err := pumpKeywords("Haste")
	if err != nil {
		return err
	}
	// a 0/0 Creature land with haste for as long as it stays (gorge's
	// animateGrant, DurationPermanent, endOnLeave)
	o := &lOverride{AddTypes: typeBits["Creature"], HasSetPT: true, AddKw: ks, Dedup: true}
	lc.out.overrides = append(lc.out.overrides, o)
	lc.emit(lInstr{Op: "Earthbend", Obj: obj, A: n, C: len(lc.out.overrides) - 1})
	return nil
}

func (lc *lowerCtx) putCounter(ps *params) error {
	if lc.digImprintFoundMode {
		return lc.digImprintFoundPut(ps) // lower_digimprint.go
	}
	if _, ok := ps.m["DividedAsYouChoose"]; ok {
		if ok, err := lc.dividedCountersN(ps); ok || err != nil {
			return err
		}
		if ok, err := lc.dividedCountersAnyX(ps); ok || err != nil {
			return err
		}
		if ok, err := lc.dividedCountersPick(ps); ok || err != nil { // (Vastwood Hydra)
			return err
		}
		if ok, err := lc.dividedCountersRemLKI(ps); ok || err != nil { // (Yannik, Scavenging Sentinel)
			return err
		}
		return lc.dividedCountersLKI(ps) // (Quirion Beastcaller)
	}
	if ps.is("ETB", "True") {
		return refuse("ETB counters outside a replacement")
	}
	if _, ok := ps.m["Bolster"]; ok {
		return lc.bolster(ps)
	}
	if _, ok := ps.m["Support"]; ok {
		return lc.support(ps)
	}
	if _, ok := ps.m["Choices"]; ok {
		return lc.putCounterChoices(ps)
	}
	obj, err := lc.objOrTarget(ps, "Self", zBattlefield)
	if err != nil {
		return err
	}
	if _, ok := ps.m["Adapt"]; ok {
		return lc.adapt(ps, obj)
	}
	if mono, ok := ps.get("Monstrosity"); ok {
		return lc.monstrosity(ps, obj, mono)
	}
	if rn, ok := ps.get("Renown"); ok {
		return lc.renown(ps, obj, rn) // lower_renown.go
	}
	n, err := lc.amount(ps, "CounterNum", 1)
	if err != nil {
		return err
	}
	ct := ps.str("CounterType")
	cts := ps.str("CounterTypes")
	if cts != "" {
		if ct != "" {
			return refuse("PutCounter with both CounterType$ %q and CounterTypes$ %q", ct, cts)
		}
		return lc.putCounterKinds(obj, n, cts)
	}
	if ct == "" {
		return refuse("PutCounter without CounterType")
	}
	if ct == "ENERGY" {
		return lc.gainEnergy(obj, n, kop.PlCtrEnergy)
	}
	if ct == "Experience" {
		return lc.gainEnergy(obj, n, kop.PlCtrExperience) // Otharri, Suns' Glory; Ezuri, Claw of Progress
	}
	if ct == contractCounter {
		return lc.putContract(obj, n) // lower_contract.go
	}
	if ct == "EachFromSource" {
		return lc.putCounterEachFromSource(ps, obj)
	}
	if ct == "ExistingCounter" {
		return lc.putCounterExisting(ps, obj, n)
	}
	if strings.Contains(ct, ",") {
		return lc.putCounterChoice(obj, n, ct) // lower_counterchoice.go
	}
	in := lInstr{Op: "PutCounter", Obj: obj, A: n, CS: ct}
	if err := putCounterRememberPut(ps, &in); err != nil { // lower_putcounterparams.go
		return err
	}
	if err := putCounterRememberAmount(ps, &in); err != nil { // lower_remembered.go
		return err
	}
	lc.emit(in)
	return nil
}

// putCounterKinds lowers PutCounter's CounterTypes$ (plural) -- a DIFFERENT
// key from CounterType$ (gorge putcounter_params.go): one counter of each
// listed kind, repeats accumulating, each of CounterNum$ (default 1). Gorge
// emits one CounterChange per listed kind, in list order, per recipient
// (engine/effects/counters_put.go's OneOfEach arm); each kernel PutCounter
// instruction already applies its own replacement and counter-added
// triggers, so kgen emits one PutCounter per entry in list order. An entry
// gorge would keep in SpecialTypes (contains '_', ChosenFromList) or a kind
// the kernel's counter table does not know refuses, loudly.
func (lc *lowerCtx) putCounterKinds(obj lRef, n lValue, cts string) error {
	if obj.Kind == rRemembered {
		return refuse("PutCounter CounterTypes$ on the remembered set")
	}
	var kinds []string
	for _, k := range strings.Split(cts, ",") {
		k = strings.TrimSpace(k)
		if k == "" {
			continue // gorge splitCounterKinds drops empty entries
		}
		if strings.Contains(k, "_") {
			return refuse("PutCounter CounterTypes$ special entry %q", k)
		}
		if strings.EqualFold(k, "ChosenFromList") {
			return refuse("PutCounter CounterTypes$ ChosenFromList")
		}
		if _, err := counterKind(k); err != nil {
			return refuse("PutCounter CounterTypes$ kind %q: %v", k, err)
		}
		kinds = append(kinds, k)
	}
	for i, k := range kinds {
		in := lInstr{Op: "PutCounter", Obj: obj, A: n, CS: k}
		// one put action, one CounterPlayerAddedAll trigger per recipient
		// (gorge's counterBatch latch; kop.PutCtrCellFirst)
		if len(kinds) > 1 {
			in.B = cst(kop.PutCtrCellNext)
			if i == 0 {
				in.B = cst(kop.PutCtrCellFirst)
			}
		}
		lc.emit(in)
	}
	return nil
}

// adapt lowers Adapt$ N (CR 702.35a, gorge effPutCounter's AdaptSet arm):
// N +1/+1 counters on the source if it has none, read at resolution; an AB$
// root also gets gorge adaptGateOK's offer gate (activate only while the
// source has no +1/+1 counter). Only the source itself, the corpus's only
// recipient: a Defined$/ValidTgts$ recipient or a CounterNum$/CounterType$
// beside it refuses.
func (lc *lowerCtx) adapt(ps *params, obj lRef) error {
	if obj.Kind != rSelf {
		return refuse("Adapt$ recipient %v", obj)
	}
	if _, ok := ps.m["CounterNum"]; ok {
		return refuse("Adapt$ with CounterNum$")
	}
	if _, ok := ps.m["CounterType"]; ok {
		return refuse("Adapt$ with CounterType$")
	}
	n, err := lc.amount(ps, "Adapt", 1)
	if err != nil {
		return err
	}
	f, err := lc.filter("Card.Self+counters_GE1_P1P1")
	if err != nil {
		return err
	}
	none := lCond{Kind: cPresent, F: f, Op: opEQ, A: cst(0)}
	if lc.head && lc.depth == 1 && lc.ab.Kind == akActivated && lc.ab.Trigger == nil {
		if lc.ab.ActCond.Kind != cNone {
			return refuse("two activation conditions")
		}
		lc.ab.ActCond = none
	}
	return lc.guardBlock(none, func() error {
		lc.emit(lInstr{Op: "PutCounter", Obj: obj, A: n, CS: "P1P1"})
		return nil
	})
}

// monstrosity lowers Monstrosity$ N (CR 701.37a; gorge effPutCounter's
// MonstrositySet arm): N +1/+1 counters on the source if it is not
// monstrous, then the monstrous designation and the BecomeMonstrous event
// carrying N. The resolution guard lives in the op (gorge's resolve-time
// skip); an AB$ root also gets monstrosityGateOK's offer gate (activate
// only while the source is not monstrous). Only the source itself, the
// corpus's only recipient: a Defined$/ValidTgts$ recipient or a
// CounterNum$/CounterType$ beside it refuses.
func (lc *lowerCtx) monstrosity(ps *params, obj lRef, raw string) error {
	if obj.Kind != rSelf {
		return refuse("Monstrosity$ recipient %v", obj)
	}
	if _, ok := ps.m["CounterNum"]; ok {
		return refuse("Monstrosity$ with CounterNum$")
	}
	if _, ok := ps.m["CounterType"]; ok {
		return refuse("Monstrosity$ with CounterType$")
	}
	n, err := lc.value(raw)
	if err != nil {
		return err
	}
	f, err := lc.filter("Card.Self+IsMonstrous")
	if err != nil {
		return err
	}
	none := lCond{Kind: cPresent, F: f, Op: opEQ, A: cst(0)}
	if lc.head && lc.depth == 1 && lc.ab.Kind == akActivated && lc.ab.Trigger == nil {
		if lc.ab.ActCond.Kind != cNone {
			return refuse("two activation conditions")
		}
		lc.ab.ActCond = none
	}
	lc.emit(lInstr{Op: "Monstrosity", Obj: obj, A: n})
	return nil
}

// dividedCountersN lowers "distribute N +1/+1 counters among one to N target
// creatures" (N a literal 2..8; gorge counters_put.go AnnouncedDivision): a
// 1..N (TargetMin$ 0: 0..N) target slot with Divided N. ok=false leaves any other shape to
// dividedCountersLKI.
func (lc *lowerCtx) dividedCountersN(ps *params) (bool, error) {
	name := ps.str("DividedAsYouChoose")
	n, err := strconv.Atoi(name)
	ct := ps.str("CounterType")
	if err != nil || n < 2 || n > 8 || strconv.Itoa(n) != name || ps.str("CounterNum") != name || ps.str("TargetMax") != name ||
		(ps.str("TargetMin") != "1" && ps.str("TargetMin") != "0") || ct == "" || strings.ContainsAny(ct, ",") {
		return false, nil
	}
	// gorge's announced division is kind-agnostic (counters_put.go
	// putCounterShares emits CounterType$ as written): any one object
	// counter kind divides the same way (Contagion's -2/-1, Splendid
	// Agony's -1/-1); the build refuses a kind the kernel does not keep.
	if ct == "P1P1" {
		ct = ""
	}
	// gorge announces a divided primitive's shares only when it is the
	// resolving object's own root target declaration (effects/division.go
	// AnnouncedDivision matches Ctx.OfferedSA by Line, the same SA
	// rules/cast_division.go castDivisionAPI reads). A SubAbility$ link (Rhino,
	// Terrible Trampler) divides at resolution, so lowering it as an announced
	// division diverges. The root is the ability's chain head, or a chosen
	// charm mode's head (offeredTargetSA).
	if !lc.head || !(lc.depth == 1 || (lc.depth == 2 && lc.mode >= 0)) {
		return true, refuse("divided PutCounter outside the ability's root target declaration")
	}
	vt, ok := ps.get("ValidTgts")
	if !ok {
		return true, refuse("divided PutCounter without ValidTgts$")
	}
	for _, k := range []string{"TargetUnique", "TargetsWithSameController", "TargetsWithDifferentControllers", "TgtZone", "Defined", "Choices"} {
		if _, ok := ps.get(k); ok {
			return true, refuse("divided PutCounter with %s$", k)
		}
	}
	f, err := lc.filter(vt)
	if err != nil {
		return true, err
	}
	if f.Players != 0 {
		return true, refuse("divided PutCounter target filter %q names players", vt)
	}
	ps.consume("TgtPrompt")
	minT := 1
	if ps.str("TargetMin") == "0" {
		minT = 0 // castable with no target, which then puts nothing
	}
	t := lc.addTarget(lTarget{F: f, Zone: zBattlefield, Min: cst(minT), Max: cst(n), Divided: n})
	lc.emit(lInstr{Op: "DividedCounters", Obj: lRef{Kind: rTarget, N: t}, CS: ct, Note: "distribute N counters among min..N targets"})
	return true, nil
}

// dividedCountersLKI lowers the dies trigger "distribute X +1/+1 counters
// among any number of target creatures you control, where X is the number of
// +1/+1 counters on CARDNAME" (Quirion Beastcaller; gorge
// cr-dies-trigger-divided-counters-lki): a Dies Self trigger whose PutCounter
// P1P1 reads CounterNum$, DividedAsYouChoose$ and TargetMax$ as one SVar
// TriggeredCard$CardCounters.P1P1 (the last-known counters), TargetMin$ 0.
// OpExt ExtDividedCounters; any other divided counter shape refuses.
func (lc *lowerCtx) dividedCountersLKI(ps *params) error {
	tr := lc.ab.Trigger
	if tr == nil || tr.Event != "Dies" || lc.depth != 1 || tr.F == nil || len(tr.F.Clauses) != 1 || tr.F.Players != 0 ||
		tr.F.Clauses[0] != (lClause{Flags: fSelf}) {
		return refuse("DividedAsYouChoose$ PutCounter outside a Dies Self trigger")
	}
	name := strings.TrimSpace(ps.str("DividedAsYouChoose"))
	if name == "" || ps.str("CounterNum") != name || ps.str("TargetMax") != name || ps.str("TargetMin") != "0" || ps.str("CounterType") != "P1P1" {
		return refuse("divided PutCounter shape (CounterNum$ %q, TargetMax$ %q, TargetMin$ %q, CounterType$ %q)", ps.str("CounterNum"), ps.str("TargetMax"), ps.str("TargetMin"), ps.str("CounterType"))
	}
	if lc.face.SVars[name] != "TriggeredCard$CardCounters.P1P1" {
		return refuse("divided PutCounter total SVar %s=%q", name, lc.face.SVars[name])
	}
	vt, ok := ps.get("ValidTgts")
	if !ok {
		return refuse("divided PutCounter without ValidTgts$")
	}
	f, err := lc.filter(vt)
	if err != nil {
		return err
	}
	for _, k := range []string{"TargetUnique", "TargetsWithSameController", "TargetsWithDifferentControllers", "TgtZone", "Defined"} {
		if _, ok := ps.get(k); ok {
			return refuse("divided PutCounter with %s$", k)
		}
	}
	if f.Players != 0 {
		return refuse("divided PutCounter target filter %q names players", vt)
	}
	t := lc.addTarget(lTarget{F: f, Zone: zBattlefield, Min: cst(0), Max: cst(1), LKIDiv: true})
	lc.emit(lInstr{Op: "DividedCounters", Obj: lRef{Kind: rTarget, N: t}, Note: "distribute the source's last-known +1/+1 counters among 0..X targets"})
	return nil
}

// counterLoseEffect lowers the Effect after a Counter RememberCountered$:
// one Continuous static, Affected$ Card.IsRemembered, RemoveAllAbilities$
// True, kept for as long as the host stays (Duration$ AsLongAsInPlay,
// ForgetOnMoved$ Battlefield) and applied only when the remembered source
// is still an artifact, creature or planeswalker on the battlefield
// (ConditionPresent$). The CounterLose instruction carries it as a
// host-bound LoseAbilities override (Tishana's Tidebinder).
func (lc *lowerCtx) counterLoseEffect(ps *params) error {
	at := lc.clAt - 1
	lc.clAt = -1 // nodeCondition skips this node's Condition*$ once
	// an ordered list, not a map: the first mismatch names the refusal, and
	// map order would make the census nondeterministic
	for _, kv := range [...][2]string{
		{"RememberObjects", "Remembered"},
		{"ForgetOnMoved", "Battlefield"},
		{"ConditionDefined", "Remembered"},
		{"ConditionPresent", "Artifact.inZoneBattlefield,Creature.inZoneBattlefield,Planeswalker.inZoneBattlefield"},
		{"Duration", "AsLongAsInPlay"},
	} {
		if ps.str(kv[0]) != kv[1] {
			return refuse("Counter-lose Effect %s$ %q", kv[0], ps.str(kv[0]))
		}
	}
	name := strings.TrimSpace(ps.str("StaticAbilities"))
	line, ok := lc.face.SVars[name]
	if !ok || strings.Contains(name, ",") {
		return refuse("Counter-lose Effect statics %q", name)
	}
	sw := map[string]string{"Mode": "Continuous", "Affected": "Card.IsRemembered", "RemoveAllAbilities": "True", "AffectedZone": "Battlefield"}
	seen := 0
	for _, part := range strings.Split(line, "|") {
		k, v, _ := strings.Cut(strings.TrimSpace(part), "$")
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		if k == "Description" {
			continue
		}
		if sw[k] != v {
			return refuse("Counter-lose Effect static param %s", k)
		}
		seen++
	}
	if seen != len(sw) {
		return refuse("Counter-lose Effect static shape")
	}
	lc.out.overrides = append(lc.out.overrides, &lOverride{LoseAbilities: true, WhileHost: true})
	lc.ab.Prog[at].C = len(lc.out.overrides) - 1
	return nil
}

// putCounterExisting lowers CounterType$ ExistingCounter (gorge
// putCounterExisting): the controller chooses a kind of counter already on
// the recipient and puts CounterNum$ more of it (Dathon, His Heart Open).
// Only a target or Self recipient and a constant amount of at least 1; the
// each-kind, remember, placer and remembered-set forms refuse by name.
func (lc *lowerCtx) putCounterExisting(ps *params, obj lRef, n lValue) error {
	if ps.is("EachExistingCounter", "True") {
		return refuse("PutCounter ExistingCounter with EachExistingCounter$")
	}
	for _, k := range []string{"RememberPut", "RememberAmount", "Placer"} {
		if _, ok := ps.m[k]; ok {
			return refuse("PutCounter ExistingCounter with %s$", k)
		}
	}
	if obj.Kind == rRemembered {
		return refuse("PutCounter ExistingCounter on the remembered set")
	}
	if !n.isConst() || n.N < 1 {
		return refuse("PutCounter ExistingCounter CounterNum %v", n)
	}
	lc.emit(lInstr{Op: "PutCounterExisting", Obj: obj, B: n})
	return nil
}
