package kgen

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// stationgrant: a Station threshold's self grant (CR 702.184, 613.1f).
//
//	S:Mode$ Continuous | AffectedDefined$ Self | Affected$ Card.counters_GE<N>_CHARGE |
//	    AddTrigger$ / AddStaticAbility$ <svar> | ...
//
// gorge's layer walk (engine/rules/layers.go AddStaticAbility, AddTrigger)
// queues the grant while the outer static is live, and the outer static is
// live exactly while the Spacecraft has at least N charge counters. A
// granted trigger is matched when its event happens and is not rechecked on
// resolution; it has no intervening-if of its own. So the grant lowers as
// the card's own trigger gated on the source's CHARGE count (kop.Gate with
// AfCheckOnTrigger, the Saga chapter's semantics), and an AddStaticAbility$
// inner static lowers as that static AND the same count (the max-speed
// template: maxSpeedGrantedStatic). Everything else on the outer static
// (AddType, AddKeyword) lowers as it does today.

// stationThreshold reports the N of a filter that is exactly Card.Self plus
// one counters_GE<N>_CHARGE compare (what selfOnlyCond reads), or ok false
// when the filter has another shape.
func stationThreshold(f *lFilter) (n int, ok bool) {
	if f == nil || f.Players != 0 || len(f.Clauses) != 1 {
		return 0, false
	}
	c := f.Clauses[0]
	if c.Flags != fSelf || c.CmpStat != cmpCounters || c.Types != 0 || c.TypesAll != 0 || c.NotTypes != 0 ||
		c.Subtype != "" || c.NotSubtype != "" || c.Colors != "" || c.NotColors != "" || c.Ctrl != 0 || c.Owner != 0 {
		return 0, false
	}
	return c.CmpN, true
}

// stationSelfGrant lowers the AddTrigger$ / AddStaticAbility$ / AddAbility$
// of a static whose Affected$ is the Station threshold on the source itself
// (any other static is left to lower as before). The
// lowered trigger is appended to the face's abilities; inner statics are
// left in lc.grantInner for staticBody to emit after the outer static. A
// granted mana ability lowers as the card's own, gated on the same count
// (stationGrantMana).
func (lc *lowerCtx) stationSelfGrant(sp *params, st *lStatic) error {
	tr, hasT := sp.m["AddTrigger"]
	inner, hasS := sp.m["AddStaticAbility"]
	mana, hasA := sp.m["AddAbility"]
	if !hasT && !hasS && !hasA {
		return nil
	}
	n, ok := stationThreshold(st.F)
	if !ok {
		return nil
	}
	if c := st.F.Clauses[0]; c.CmpOp != opGE || c.CmpCounter != "CHARGE" {
		return refuse("station grant on counter %q op %d", c.CmpCounter, c.CmpOp)
	}
	if n < 1 || n > 255 {
		return refuse("station grant threshold %d", n)
	}
	if hasT && hasS || hasA && (hasT || hasS) {
		return refuse("station static with both AddTrigger$ and AddStaticAbility$")
	}
	if _, ok := sp.m["AddReplacementEffect"]; ok {
		return refuse("station static grants AddReplacementEffect$ beside its threshold")
	}
	if _, ok := sp.m["AddSVar"]; ok && !sp.used["AddSVar"] {
		return refuse("station static grant with AddSVar$")
	}
	if hasA {
		return lc.stationGrantMana(sp, mana, n)
	}
	if hasT {
		return lc.stationGrantTrigger(sp, tr, n)
	}
	return lc.stationGrantStatic(sp, inner, n)
}

// stationGrantMana lowers AddAbility$ as the card's own mana ability whose
// activation condition is the source holding n charge counters (CR 702.184,
// 602.5b). gorge's census lists a granted ability only while the grant is
// live, which is the gate-on view of a gated mana ability (kernel
// prepManaGate). Only a one-pip fee of the product's colour is taken here
// (pipFeeMana), and only the shape the kernel models is lowered
// (fixedColourFiltMana): that filter beside the card's own plain {T}
// ability. Any other AddAbility$ stays on the grant path it always took.
func (lc *lowerCtx) stationGrantMana(sp *params, name string, n int) error {
	sa := cards.ResolveSVar(lc.face.SVars, name)
	if sa == nil || sa.Kind != "AB" || !pipFeeMana(sa) {
		return nil
	}
	idx := len(lc.out.Abilities)
	if err := lc.withAbility(lc.ab, func() error { return lc.lowerAbility(sa) }); err != nil {
		return err
	}
	if len(lc.out.Abilities) != idx+1 || lc.out.Abilities[idx].Kind != akMana {
		return refuse("station AddAbility %q is not one mana ability", name)
	}
	a := lc.out.Abilities[idx]
	if a.Latent || a.ActCond.Kind != cNone || a.Gate != nil {
		return refuse("station AddAbility %q with a condition of its own", name)
	}
	f, err := lc.filter("Card.Self+counters_GE" + strconv.Itoa(n) + "_CHARGE")
	if err != nil {
		return err
	}
	a.ActCond = lCond{Kind: cPresent, F: f, Op: opGE, A: cst(1)}
	var first *lAbility
	for _, o := range lc.out.Abilities[:idx] {
		if o.Kind == akMana && !o.Latent && o.Mana != nil {
			first = o
			break
		}
	}
	if first == nil || !fixedColourFiltMana(first, a) {
		return refuse("station AddAbility %q: a coloured-fee mana ability the kernel does not model", name)
	}
	sp.consume("AddAbility")
	return nil
}

// pipFeeMana reports whether sa is "{c}, {T}: Add {c} ..." with an Amount$
// (Evendo's and Uthros's Station grant): a single coloured pip and {T}, the
// product that colour.
func pipFeeMana(sa *cards.SA) bool {
	cost := strings.Fields(sa.Params["Cost"])
	if len(cost) != 2 || sa.Params["Amount"] == "" {
		return false
	}
	if cost[0] == "T" {
		cost[0], cost[1] = cost[1], cost[0]
	}
	return cost[1] == "T" && len(cost[0]) == 1 && strings.Contains("WUBRG", cost[0]) && sa.Params["Produced"] == cost[0]
}

// stationGrantTrigger lowers AddTrigger$ as the card's own battlefield
// trigger gated on n charge counters.
func (lc *lowerCtx) stationGrantTrigger(sp *params, name string, n int) error {
	return lc.gatedGrantTrigger(sp, name, "station", &lGate{Ctr: "CHARGE", Level: n}, "StationGrant", false)
}

// gatedGrantTrigger lowers a self-grant static's AddTrigger$ name as the
// card's own battlefield trigger carrying gate (what prefixes the refusal
// reasons). A granted trigger is matched when its event happens and not
// rechecked on resolution; allowCond lets it keep its own intervening-if
// (CheckSVar$), which is rechecked as a printed one is.
func (lc *lowerCtx) gatedGrantTrigger(sp *params, name, what string, gate *lGate, abName string, allowCond bool) error {
	if strings.ContainsAny(name, "&,") {
		return refuse("%s AddTrigger %q: more than one trigger", what, name)
	}
	t, ok := cards.ParseTriggerLine(lc.face.SVars[name])
	if !ok {
		return refuse("%s AddTrigger %q", what, name)
	}
	if t.Effect == nil {
		t.Effect = cards.ResolveSVar(lc.face.SVars, t.Params["Execute"])
	}
	lc.granted++
	defer func() { lc.granted-- }()
	idx := len(lc.out.Abilities)
	if err := lc.withAbility(lc.ab, func() error { return lc.lowerTriggered(t, "") }); err != nil {
		return err
	}
	if len(lc.out.Abilities) != idx+1 {
		return refuse("%s AddTrigger %q lowering", what, name)
	}
	a := lc.out.Abilities[idx]
	if a.Kind != akTriggered || a.Trigger == nil || a.Gate != nil || a.Latent {
		return refuse("%s AddTrigger %q is not a plain triggered ability", what, name)
	}
	if a.Trigger.Zone != 0 && a.Trigger.Zone != zBattlefield {
		return refuse("%s AddTrigger %q outside the battlefield", what, name)
	}
	if a.Trigger.Cond.Kind != cNone && !allowCond {
		return refuse("%s AddTrigger %q with an intervening-if", what, name)
	}
	a.Gate = gate
	a.Name = abName
	sp.consume("AddTrigger")
	return nil
}

// stationGrantStatic lowers AddStaticAbility$ as the inner Continuous static
// ANDed with the source holding n charge counters.
func (lc *lowerCtx) stationGrantStatic(sp *params, name string, n int) error {
	if strings.ContainsAny(name, "&,") {
		return refuse("station AddStaticAbility %q: more than one static", name)
	}
	inners, ok := cards.ParseStaticLines(lc.face.SVars[name])
	if !ok || len(inners) != 1 || inners[0].Mode != "Continuous" {
		return refuse("station AddStaticAbility %q", name)
	}
	in := inners[0]
	for _, k := range []string{"Condition", "CheckSVar", "IsPresent", "ClassBand"} {
		if _, ok := in.Params[k]; ok {
			return refuse("station granted static beside its own %s$", k)
		}
	}
	lc.granted++
	defer func() { lc.granted-- }()
	sts, err := lc.staticBody(in)
	if err != nil {
		return err
	}
	f, err := lc.filter("Card.Self+counters_GE" + strconv.Itoa(n) + "_CHARGE")
	if err != nil {
		return err
	}
	have := lCond{Kind: cPresent, F: f, Op: opGE, A: cst(1)}
	var out []lStatic
	for _, s := range sts {
		if s.Kind == "" {
			continue
		}
		s.Cond = andCond(s.Cond, have)
		out = append(out, s)
	}
	if len(out) == 0 {
		return refuse("station AddStaticAbility %q lowered to nothing", name)
	}
	lc.grantInner = out
	sp.consume("AddStaticAbility")
	return nil
}
