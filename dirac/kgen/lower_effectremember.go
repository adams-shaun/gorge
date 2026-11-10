package kgen

import (
	"reflect"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// effMemKind is how an Effect's trigger binds the set RememberObjects$ names.
type effMemKind uint8

const (
	memNone  effMemKind = iota // the Targeted-over-DamageDone capture (emblemCapture) or no RememberObjects$
	memCount                   // the set's size, frozen at the Effect and read back as Remembered$Amount
	memPair                    // both of the chain's two target slots
	memList                    // the resolution's remembered set, up to four objects
)

// effMem is an Effect trigger's remembered-set binding (kop.Emblem's
// Capture, Capture2, CaptureRem, NumAmount, Once and Source).
type effMem struct {
	kind       effMemKind
	cap1, cap2 lRef
}

// effectRemember reads an Effect's RememberObjects$ for the three shapes the
// kernel's emblem carries beyond emblemCapture's one Targeted creature:
//
//   - Remembered over a one-off next-upkeep trigger (The Eagles Are Coming!):
//     gorge registers the Effect's trigger with the resolution's remembered
//     set, and the trigger's Remembered$Amount counts it. The emblem freezes
//     the count at the Effect and carries it as the trigger amount.
//   - Targeted over both of the chain's target slots beside a this-turn dies
//     trigger of those creatures (T'Pring, Never Parted).
//   - RememberedLKI over a permanent leaves trigger of the Effect's own
//     source (Mysterio, Master of Illusion's tokens).
//
// Any other RememberObjects$ is emblemCapture's, which refuses what it does
// not read. The returned params of the trigger drop OneOff$, which the
// emblem's Once flag reads.
func (lc *lowerCtx) effectRemember(ps *params, t *cards.Trigger, eot, next bool) (effMem, error) {
	ro, ok := ps.m["RememberObjects"]
	if !ok {
		return effMem{}, nil
	}
	switch strings.TrimSpace(ro) {
	case "Remembered":
		return lc.effectRememberCount(ps, t, eot, next)
	case "Targeted":
		if eot && t.Mode == "ChangesZone" {
			return lc.effectRememberPair(ps, t)
		}
	case "RememberedLKI":
		return lc.effectRememberList(ps, t, eot, next)
	}
	return effMem{}, nil
}

// effectRememberList is the RememberedLKI shape: see effectRemember. The
// Effect is permanent and forgets a remembered object once it leaves the
// battlefield (ForgetOnMoved$ Battlefield), its one trigger is "when the
// Effect's source leaves the battlefield" (ValidCard$ Card.EffectSource), and
// the set is the tokens the resolution made, so every object is a battlefield
// permanent when the Effect is created.
func (lc *lowerCtx) effectRememberList(ps *params, t *cards.Trigger, eot, next bool) (effMem, error) {
	why := func(more string) (effMem, error) {
		return effMem{}, refuse("Effect trigger RememberObjects$ RememberedLKI: %s", more)
	}
	if eot || next {
		return why("not a permanent Effect")
	}
	if ps.str("ForgetOnMoved") != "Battlefield" {
		return why("no ForgetOnMoved$ Battlefield")
	}
	if t.Mode != "ChangesZone" || strings.TrimSpace(t.Params["Origin"]) != "Battlefield" || strings.TrimSpace(t.Params["Destination"]) != "Any" ||
		strings.TrimSpace(t.Params["ValidCard"]) != "Card.EffectSource" {
		return why("the trigger is not a leave of Card.EffectSource")
	}
	writers, token := 0, false
	if lc.ab != nil {
		for _, in := range lc.ab.Prog {
			if kremWrites(in) {
				writers++
				token = in.Op == "CreateToken"
			}
		}
	}
	if writers != 1 || !token {
		return why("the set is not the tokens this resolution made")
	}
	cp := make(map[string]string, len(t.Params))
	for k, v := range t.Params {
		if k != "ValidCard" {
			cp[k] = v // the emblem's Source flag reads Card.EffectSource
		}
	}
	t.Params = cp
	ps.consume("RememberObjects", "ForgetOnMoved")
	return effMem{kind: memList}, nil
}

// effectRememberPair is the Targeted-over-both-slots shape: see effectRemember.
// The chain declares exactly two single battlefield object target slots and the
// Effect none of its own; gorge remembers the union of the chain's targets, so
// both are the set the trigger's ValidCard$ Creature.IsRemembered reads.
func (lc *lowerCtx) effectRememberPair(ps *params, t *cards.Trigger) (effMem, error) {
	why := func(more string) (effMem, error) {
		return effMem{}, refuse("Effect trigger RememberObjects$ Targeted over a pair: %s", more)
	}
	if _, own := ps.m["ValidTgts"]; own {
		return why("the Effect has a ValidTgts$ of its own")
	}
	if lc.ab == nil || len(lc.chainT) != 2 {
		return why("the chain does not declare exactly two target slots")
	}
	var refs [2]lRef
	for i, ti := range lc.chainT {
		ts := lc.ab.Targets[ti]
		if ts.F == nil || ts.F.Players != 0 || len(ts.F.Clauses) == 0 || ts.Zone != zBattlefield || !ts.Max.isConst() || ts.Max.N != 1 {
			return why("a target slot is not one battlefield object")
		}
		refs[i] = lRef{Kind: rTarget, N: ti}
	}
	ps.consume("RememberObjects")
	return effMem{kind: memPair, cap1: refs[0], cap2: refs[1]}, nil
}

// effectRememberFilter strips the Remembered flag from a pair or list
// emblem's dies trigger: the kernel gates the trigger on the emblem's own
// remembered objects (State.g.embN and its slots), so the filter keeps the type
// check only. The trigger carries nothing else the hook would drop.
func effectRememberFilter(ab *lAbility, mem effMem) error {
	t := ab.Trigger
	if t == nil || t.F == nil || len(t.F.Clauses) != 1 || t.F.Players != 0 || t.F.Clauses[0].Flags&fRemembered == 0 {
		return refuse("Effect RememberObjects$ beside a trigger that is not ValidCard$ ...IsRemembered")
	}
	if mem.kind == memPair && t.Event != "Dies" {
		return refuse("Effect RememberObjects$ Targeted over a pair beside a %s trigger", t.Event)
	}
	f := *t.F
	f.Clauses = append([]lClause(nil), f.Clauses...)
	f.Clauses[0].Flags &^= fRemembered
	t.F = &f
	z := *t
	z.Event, z.F, z.Zone, z.Origin, z.Dest, z.Cond = "", nil, 0, 0, 0, lCond{}
	if !reflect.DeepEqual(z, lTrigger{}) {
		return refuse("Effect %s trigger rider beside RememberObjects$", t.Event)
	}
	for _, in := range ab.Prog {
		if kremReads(in.String()) {
			return refuse("Effect trigger body reads Remembered beside RememberObjects$ Targeted over a pair")
		}
	}
	return nil
}

// effectRememberCount is the Remembered shape: see effectRemember.
func (lc *lowerCtx) effectRememberCount(ps *params, t *cards.Trigger, eot, next bool) (effMem, error) {
	why := func(more string) (effMem, error) {
		return effMem{}, refuse("Effect trigger RememberObjects$ Remembered: %s", more)
	}
	if eot || next {
		return why("not a permanent Effect")
	}
	if t.Mode != "Phase" || strings.TrimSpace(t.Params["Phase"]) != "Upkeep" || strings.TrimSpace(t.Params["OneOff"]) != "True" {
		return why("the trigger is not a one-off Phase$ Upkeep")
	}
	if lc.ab == nil || lc.ab.Kind != akSpell && lc.ab.Kind != akActivated || !lc.hasRememberWriter() {
		// a triggered ability's Remembered is its fire-time capture, which the
		// kernel does not seed: only a set this resolution wrote counts
		return why("the set is not one this resolution wrote")
	}
	cp := make(map[string]string, len(t.Params))
	for k, v := range t.Params {
		if k != "OneOff" {
			cp[k] = v
		}
	}
	t.Params = cp
	ps.consume("RememberObjects")
	return effMem{kind: memCount}, nil
}

// effectRememberBody checks a lowered emblem trigger against its memory: a
// count emblem's body reads the set only as the trigger amount (a Remembered
// read left in the body would see the kernel's empty set).
func effectRememberBody(ab *lAbility, mem effMem) error {
	if mem.kind == memPair {
		return effectRememberFilter(ab, mem)
	}
	if mem.kind == memList {
		return effectRememberListBody(ab)
	}
	for _, in := range ab.Prog {
		if kremReads(in.String()) {
			return refuse("Effect trigger body reads Remembered beside RememberObjects$ Remembered")
		}
	}
	return nil
}

// apply sets the emblem's flags and the CreateEmblem's operand for the binding.
func (m effMem) apply(em *lEmblem, ce *lInstr) {
	switch m.kind {
	case memCount:
		em.NumAmount, em.Once = true, true
		ce.A = lValue{Kind: vRememberedCount, S: "ctx"}
	case memPair:
		em.Capture, em.CaptureSet, em.Capture2, em.Capture2Set = m.cap1, true, m.cap2, true
	case memList:
		em.CaptureRem, em.CaptureLapse, em.Source, em.NotHere = true, true, true, true
	}
}

// effectPairCondition lowers the gate of an Effect that remembers both of the
// chain's targets (T'Pring, Never Parted: ConditionDefined$ Targeted |
// ConditionPresent$ Creature | ConditionCompare$ EQ2): the Effect is created
// only while both targets are still creatures, each read as gorge reads a
// defined card (a moved card as a card in its new zone, countRefMatches).
// found is false for any other gate.
func (lc *lowerCtx) effectPairCondition(ps *params) (c lCond, found bool, err error) {
	if strings.TrimSpace(ps.m["Triggers"]) == "" || strings.TrimSpace(ps.m["RememberObjects"]) != "Targeted" ||
		strings.TrimSpace(ps.m["ConditionDefined"]) != "Targeted" || strings.TrimSpace(ps.m["ConditionCompare"]) != "EQ2" {
		return lCond{}, false, nil
	}
	if len(lc.chainT) != 2 || strings.TrimSpace(ps.m["ConditionPresent"]) != "Creature" {
		return lCond{}, false, refuse("Effect ConditionDefined$ Targeted over %d target slots with ConditionPresent$ %q", len(lc.chainT), ps.m["ConditionPresent"])
	}
	f, err := lc.filter("Creature")
	if err != nil {
		return lCond{}, false, err
	}
	ps.consume("ConditionDefined", "ConditionPresent", "ConditionCompare")
	both := make([]lCond, 2)
	for i, ti := range lc.chainT {
		both[i] = lCond{Kind: cRefMatches, Ref: lRef{Kind: rTarget, N: ti}, F: f, Op: opGE, A: cst(1)}
	}
	return lCond{Kind: cAnd, Sub: both}, true, nil
}

// effectRememberListBody checks a list emblem's trigger: a leave of any card
// (the kernel's Source flag narrows it to the emblem's creator) whose body is
// exactly the exile of the remembered set from the battlefield, rewritten to
// the op that reads the emblem's own set.
func effectRememberListBody(ab *lAbility) error {
	t := ab.Trigger
	if t == nil || t.Event != "Leaves" || t.F != nil || t.F2 != nil {
		return refuse("Effect RememberObjects$ RememberedLKI beside a trigger that is not a bare leave")
	}
	z := *t
	z.Event, z.Zone, z.Origin, z.Dest, z.From, z.Cond = "", 0, 0, 0, 0, lCond{}
	if !reflect.DeepEqual(z, lTrigger{}) {
		return refuse("Effect %s trigger rider beside RememberObjects$ RememberedLKI", t.Event)
	}
	if len(ab.Prog) != 1 || ab.Prog[0].Op != "Exile" || ab.Prog[0].Obj.Kind != rRemembered || ab.Prog[0].CS != "from=Battlefield" {
		return refuse("Effect RememberObjects$ RememberedLKI beside a body that is not the exile of the remembered set from the battlefield")
	}
	ab.Prog[0] = lInstr{Op: "ExileEmbRem", Obj: lRef{Kind: rSelf}}
	return nil
}
