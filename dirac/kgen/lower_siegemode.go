package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// ksiegemode: the Tarkir Dragonstorm Sieges' as-enters mode.
// The shape: an Other-kind ETBReplacement keyword names a GenericChoice SVar
// (SetChosenMode$ True, Defined$ You, ShowChoice$ ExceptSelf, Choices$ naming
// two label SVars, each a bare DB$ Pump carrying only a SpellDescription$). A
// Continuous static per mode (AffectedDefined$ Self, Affected$ Card.ChosenMode<X>,
// which gorge spells Card.Self+ChosenMode<X>) grants the mode's AddTrigger$ or
// AddStaticAbility$.
//
// gorge poses the choice as the permanent enters and folds the answer into
// the permanent's ChosenModes (events.Choose "mode", foldChoose); the filter
// Card.ChosenMode<X> holds while that is X (effects/filter.go; unanswered
// fails closed). The kernel keeps the answer in the permanent's chosen byte
// (the kernel's only per-permanent choice slot, which a Siege never uses for
// a colour or a type). The statics grant to the Siege itself while the
// filter matches, so they lower as the card's own ability / static gated on
// "my chosen mode is X" (the max-speed template, lower_speed.go).

func init() { exprKinds[vChosenMode] = kop.ExChosenMode }

// siegeChoiceNames reads a SetChosenMode GenericChoice's two choice names,
// each a bare `DB$ Pump | SpellDescription$` label SVar.
func (lc *lowerCtx) siegeChoiceNames(sa *cards.SA) ([]string, error) {
	ps := newParams(sa.Params)
	if sa.Sub != nil {
		return nil, refuse("SetChosenMode GenericChoice with a SubAbility")
	}
	if ps.str("Defined") != "You" {
		return nil, refuse("SetChosenMode GenericChoice Defined$ %q", ps.str("Defined"))
	}
	if !ps.is("SetChosenMode", "True") {
		return nil, refuse("GenericChoice without SetChosenMode$ True")
	}
	if v, ok := ps.get("ShowChoice"); ok && strings.TrimSpace(v) != "ExceptSelf" {
		// descriptive only (a UI notify; Forge ChooseGenericEffect)
		return nil, refuse("SetChosenMode GenericChoice ShowChoice$ %q", v)
	}
	ps.consume("AILogic", "SpellDescription")
	var names []string
	for _, n := range strings.Split(ps.str("Choices"), ",") {
		names = append(names, strings.TrimSpace(n))
	}
	if len(names) != 2 || names[0] == "" || names[1] == "" || names[0] == names[1] {
		return nil, refuse("SetChosenMode GenericChoice Choices$ %q", ps.str("Choices"))
	}
	for _, n := range names {
		c := cards.ResolveSVar(lc.face.SVars, n)
		if c == nil || c.API != "Pump" || c.Sub != nil {
			return nil, refuse("SetChosenMode choice %q is not a bare label", n)
		}
		cp := newParams(c.Params)
		cp.consume("SpellDescription")
		if left := cp.unread(); len(left) > 0 {
			return nil, refuse("SetChosenMode choice %q has effect params %v", n, left)
		}
	}
	ps.consume("Choices")
	if left := ps.unread(); len(left) > 0 {
		return nil, refuse("SetChosenMode GenericChoice unread params %v", left)
	}
	return names, nil
}

// etbSetChosenMode lowers the whole body of an ETB replacement that is a
// SetChosenMode GenericChoice: the as-enters ask, answered into the
// permanent's chosen byte (GenericChoice C 2).
func (lc *lowerCtx) etbSetChosenMode(node, sa *cards.SA, other bool, cond *lCond) error {
	if other || node != sa || sa.Sub != nil || cond.Kind != cNone || len(lc.ab.Prog) != 0 {
		return refuse("SetChosenMode GenericChoice beside another ETB replacement part")
	}
	if _, err := lc.siegeChoiceNames(node); err != nil {
		return err
	}
	lc.emit(lInstr{Op: "GenericChoice", Obj: lRef{Kind: rYou}, A: cst(2), C: 2})
	return nil
}

// siegeModeIndex is the index of mode name in the face's SetChosenMode
// choice (statics lower before replacements, so it reads the face's own
// replacement lines).
func (lc *lowerCtx) siegeModeIndex(name string) (int, error) {
	for _, r := range lc.face.Repls {
		if r.With == nil || r.With.API != "GenericChoice" || r.With.Params["SetChosenMode"] != "True" {
			continue
		}
		names, err := lc.siegeChoiceNames(r.With)
		if err != nil {
			return 0, err
		}
		for i, n := range names {
			if n == name {
				return i, nil
			}
		}
		return 0, refuse("ChosenMode%s is not among Choices$ %v", name, names)
	}
	return 0, refuse("ChosenMode%s without a SetChosenMode GenericChoice", name)
}

// chosenModeAffected is the Affected$ prefix gorge's parser leaves for
// `AffectedDefined$ Self | Affected$ Card.ChosenMode<X>` (it ANDs the
// defined object into the filter: Card.Self+ChosenMode<X>).
const chosenModeAffected = "Card.Self+ChosenMode"

// isChosenModeStatic reports a static whose Affected$ is Card.Self+ChosenMode<X>.
func isChosenModeStatic(s cards.Static) bool {
	return strings.HasPrefix(s.Params["Affected"], chosenModeAffected)
}

// chosenModeStatic lowers `Mode$ Continuous | AffectedDefined$ Self |
// Affected$ Card.ChosenMode<X> | AddTrigger$ / AddStaticAbility$ <svar>`.
func (lc *lowerCtx) chosenModeStatic(s cards.Static) error {
	sp := newParams(s.Params)
	sp.consume("Mode", "Description")
	if d, ok := sp.get("AffectedDefined"); s.Mode != "Continuous" || ok && strings.TrimSpace(d) != "Self" {
		return refuse("ChosenMode static shape (Mode %s, AffectedDefined$ %q)", s.Mode, d)
	}
	x := strings.TrimPrefix(sp.str("Affected"), chosenModeAffected)
	if x == "" || strings.ContainsAny(x, "+.!") {
		return refuse("ChosenMode static Affected$ %q", sp.str("Affected"))
	}
	idx, err := lc.siegeModeIndex(x)
	if err != nil {
		return err
	}
	sp.consume("Affected")
	tr, hasT := sp.get("AddTrigger")
	inner, hasS := sp.get("AddStaticAbility")
	if hasT == hasS {
		return refuse("ChosenMode static grants no single AddTrigger$ / AddStaticAbility$")
	}
	if hasT {
		if left := sp.unread(); len(left) > 0 {
			return refuse("ChosenMode static unread params %v", left)
		}
		if t, ok := cards.ParseTriggerLine(lc.face.SVars[tr]); ok {
			switch t.Mode {
			case "DamageDoneOnce", "DamageDealtOnce", "DamageAll", "ExcessDamageAll":
				// gorge's granted-trigger walk (rules/trigger_granted.go
				// checkGrantedStaticTriggersUsing) lacks the per-batch latch
				// of the printed walk (trigger_match.go): a granted "one or
				// more creatures deal combat damage" trigger fires once per
				// Damage event, so two attackers queue two (Frostcliff
				// Siege, Jeskai). Forge fires it once per batch; kept
				// refused, not emulated.
				return refuse("ChosenMode granted %s trigger: gorge fires it per damage event, not per batch", t.Mode)
			}
		}
		return lc.gatedGrantTrigger(sp, tr, "ChosenMode", &lGate{Ctr: "MODE", Level: idx + 1}, "SiegeMode", true)
	}
	if left := sp.unread(); len(left) > 0 {
		return refuse("ChosenMode static unread params %v", left)
	}
	if strings.ContainsAny(inner, "&,") {
		return refuse("ChosenMode AddStaticAbility %q: more than one static", inner)
	}
	inners, ok := cards.ParseStaticLines(lc.face.SVars[inner])
	if !ok || len(inners) != 1 {
		return refuse("ChosenMode AddStaticAbility %q", inner)
	}
	if inners[0].Mode != "Continuous" {
		// Windcrag Siege's Mardu mode is a Panharmonicon of attack-caused
		// triggers (ValidMode$ Attacks), which the kernel does not carry
		return refuse("ChosenMode AddStaticAbility %q Mode$ %s", inner, inners[0].Mode)
	}
	in := inners[0]
	ps := make(map[string]string, len(in.Params))
	for k, v := range in.Params {
		switch k {
		case "Secondary", "Description":
		case "Condition", "CheckSVar", "IsPresent", "ClassBand":
			return refuse("ChosenMode granted static beside its own %s$", k)
		default:
			ps[k] = v
		}
	}
	in.Params = ps
	lc.granted++
	defer func() { lc.granted-- }()
	sts, err := lc.staticBody(in)
	if err != nil {
		return err
	}
	gate := lCond{Kind: cValue, A: lValue{Kind: vChosenMode}, Op: opEQ, B: cst(idx)}
	n := 0
	for _, st := range sts {
		if st.Kind == "" {
			continue
		}
		st.Cond = andCond(st.Cond, gate)
		lc.out.Statics = append(lc.out.Statics, st)
		n++
	}
	if n == 0 {
		return refuse("ChosenMode AddStaticAbility %q lowered to nothing", inner)
	}
	return nil
}
