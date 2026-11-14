package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// onePoisonRepl lowers Melira's poison replacement (ReplOnePoison):
// ValidPlayer$ You, ValidCounterType$ POISON, DB$ ReplaceCounter Amount$ 1
// whose sub-ability is the static ImmediateTrigger creating the turn's
// CantPutCounter POISON effect on you.
func (lc *lowerCtx) onePoisonRepl(rp *params, w *cards.SA, rl *lRepl) error {
	if rp.str("ValidPlayer") != "You" || rp.str("ValidCounterType") != "POISON" {
		return refuse("repl-hard: AddCounter ValidPlayer shape")
	}
	for _, k := range []string{"ValidCard", "ValidObject", "ValidSource", "EffectOnly"} {
		if _, ok := rp.m[k]; ok {
			return refuse("repl-hard: AddCounter ValidPlayer with %s$", k)
		}
	}
	if w == nil || w.API != "ReplaceCounter" || w.Params["ValidCounterType"] != "POISON" || w.Params["Amount"] != "1" || w.Sub == nil {
		return refuse("repl-hard: AddCounter one-poison body")
	}
	it := w.Sub
	if it.API != "ImmediateTrigger" || it.Sub != nil || it.Params["Static"] != "True" || it.Params["AfterReplacement"] != "True" {
		return refuse("repl-hard: AddCounter one-poison rider")
	}
	eff := cards.ResolveSVar(lc.face.SVars, it.Params["Execute"])
	if eff == nil || eff.API != "Effect" || eff.Sub != nil {
		return refuse("repl-hard: AddCounter one-poison effect")
	}
	st := eff.Params["StaticAbilities"]
	if len(eff.Params) != 1 || strings.Contains(st, ",") {
		return refuse("repl-hard: AddCounter one-poison effect params")
	}
	sv := strings.Join(strings.Fields(lc.face.SVars[st]), " ")
	if !strings.HasPrefix(sv, "Mode$ CantPutCounter | ValidPlayer$ You | CounterType$ POISON |") {
		return refuse("repl-hard: AddCounter one-poison static %q", sv)
	}
	rp.consume("ValidPlayer", "ValidCounterType")
	rl.Kind = "OnePoison"
	return nil
}

// youDoubleCountersRepl lowers Innkeeper's Talent's "If you would put one or
// more counters on a permanent or player, put twice that many of each of
// those kinds of counters on that permanent or player instead"
// (ReplDoubleCountersYou): ValidSource$ You, ValidObject$
// Permanent.inZoneBattlefield,Player, DB$ ReplaceCounter ChooseCounter$
// True Amount$ X, X = ReplaceCount$CounterNum/Twice. gorge reads the putter
// from its in-flight counter adder (rules/engine_damage.go).
func (lc *lowerCtx) youDoubleCountersRepl(rp *params, w *cards.SA, rl *lRepl) error {
	if rp.str("ValidSource") != "You" || rp.str("ValidObject") != "Permanent.inZoneBattlefield,Player" {
		return refuse("repl-hard: AddCounter putter shape")
	}
	for _, k := range []string{"ValidCard", "ValidCounterType", "EffectOnly", "ValidCause"} {
		if _, ok := rp.m[k]; ok {
			return refuse("repl-hard: AddCounter putter with %s$", k)
		}
	}
	if w == nil || w.API != "ReplaceCounter" || w.Sub != nil || len(w.Params) != 2 ||
		w.Params["ChooseCounter"] != "True" || lc.face.SVars[w.Params["Amount"]] != "ReplaceCount$CounterNum/Twice" {
		return refuse("repl-hard: AddCounter putter body")
	}
	rp.consume("ValidSource", "ValidObject")
	rl.Kind = "DoubleCountersYou"
	return nil
}

// AddCounter replacements that name no ValidCounterType$.
// gorge's matcher admits every kind for an absent one
// (rules/replacement_match.go, "Winding Constrictor's one or more counters"),
// and the ReplaceCounter body resolves its Amount$ against each kind's own
// held amount, so "each of those kinds" is one rewrite per kind placed. The
// kernel reads Repl.C = kop.ReplCtrAny / ReplCtrAnyYou in place of a kind.

// putterDoubleCountersRepl lowers "If you would put one or more counters on a
// <recipient> you control, put twice that many of each of those kinds of
// counters on it instead" (Dr. Joseph M'Benga: ValidObject$ names the
// recipient, no counter type) and the one-kind form (Lt. Cmdr. Geordi La
// Forge: ValidCard$ names the recipient, ValidCounterType$ CHARGE). Both read
// the putter, ValidSource$ You, from gorge's in-flight counter adder, which the
// kernel publishes as it does for Doc Samson's plus one (Repl.C ReplCtrAnyYou
// for every kind; Repl.A 1 for a single kind).
func (lc *lowerCtx) putterDoubleCountersRepl(rp *params, w *cards.SA, rl *lRepl) error {
	if rp.str("ValidSource") != "You" {
		return refuse("repl-hard: AddCounter putter shape")
	}
	for _, k := range []string{"EffectOnly", "ValidCause", "ValidPlayer"} {
		if _, ok := rp.m[k]; ok {
			return refuse("repl-hard: AddCounter putter with %s$", k)
		}
	}
	vc, vo := rp.str("ValidCard"), rp.str("ValidObject")
	if (vc == "") == (vo == "") {
		return refuse("repl-hard: AddCounter putter recipient")
	}
	spec := vc + vo
	ck := rp.str("ValidCounterType")
	if w == nil || w.API != "ReplaceCounter" || w.Sub != nil {
		return refuse("repl-hard: AddCounter body")
	}
	bp := newParams(w.Params)
	if ck != "" {
		if _, err := counterKind(ck); err != nil {
			return refuse("repl-hard: AddCounter counter type %q", ck)
		}
		if bp.str("ValidCounterType") != ck {
			return refuse("repl-hard: AddCounter body counter type")
		}
	}
	if s, ok := bp.get("ValidSource"); ok && s != "You" {
		return refuse("repl-hard: AddCounter body ValidSource %q", s)
	}
	amt := bp.str("Amount")
	if cc, ok := bp.get("ChooseCounter"); ok && cc != "True" {
		return refuse("repl-hard: AddCounter ChooseCounter %q", cc)
	}
	bp.consume("Amount", "ChooseCounter", "ValidSource", "ValidCounterType")
	if left := bp.unread(); len(left) > 0 {
		return refuse("repl-hard: AddCounter body params %v", left)
	}
	// Twice reads the putter-gated doubler (ReplCtrAnyYou doubling); plus one
	// reads the putter-gated plus one of each kind (Repl.A 1), the same
	// counters the ValidCounterType$ one-kind form lowers (Lae'zel, Vlaakith's
	// Champion beside Lt. Cmdr. Geordi La Forge).
	switch amount := lc.face.SVars[amt]; amount {
	case "ReplaceCount$CounterNum/Twice":
		f, err := lc.filter(spec)
		if err != nil {
			return err
		}
		rp.consume("ValidSource", "ValidCard", "ValidObject", "ValidCounterType")
		rl.Kind, rl.F, rl.Ctr, rl.Putter = "DoubleCountersOn", f, ck, true
		return nil
	case "ReplaceCount$CounterNum/Plus.1":
		if ck != "" {
			// the one-kind plus shape is the ValidCounterType$ arm of
			// addCounterRepl (lower_trigger.go), not a putter line
			return refuse("repl-hard: AddCounter body counter type")
		}
		f, err := lc.filter(spec)
		if err != nil {
			return err
		}
		rp.consume("ValidSource", "ValidCard", "ValidObject", "ValidCounterType")
		rl.Kind, rl.N, rl.F, rl.Ctr, rl.Putter = "AddCountersOn", cst(1), f, "", true
		return nil
	default:
		return refuse("repl-hard: AddCounter amount %q", lc.face.SVars[amt])
	}
}

// youPlusObject is Doc Samson's ValidObject$: a permanent its controller
// controls (the recipient filter; the putter is ValidSource$ You).
const youPlusObject = "Permanent.YouCtrl+inZoneBattlefield"

// anyKindCountersRepl lowers an unfiltered-kind R:Event$ AddCounter line whose
// ValidCard$ vc scopes the recipient and whose body doubles (Loading Zone:
// ReplaceCount$CounterNum/Twice) or adds one of each kind (Pir, Imaginative
// Rascal and Winding Constrictor's permanent line:
// ReplaceCount$CounterNum/Plus.1).
func (lc *lowerCtx) anyKindCountersRepl(rp *params, w *cards.SA, rl *lRepl, vc string) error {
	amount, err := lc.anyKindBody(rp, w)
	if err != nil {
		return err
	}
	f, err := lc.filter(vc)
	if err != nil {
		return err
	}
	switch amount {
	case "ReplaceCount$CounterNum/Twice":
		rl.Kind, rl.F, rl.Ctr = "DoubleCountersOn", f, ""
	case "ReplaceCount$CounterNum/Plus.1":
		rl.Kind, rl.N, rl.F, rl.Ctr = "AddCountersOn", cst(1), f, ""
	default:
		return refuse("repl-hard: AddCounter amount %q", amount)
	}
	return nil
}

// youPlusCountersRepl lowers Doc Samson's "If you would put one or more
// counters on a permanent you control, put that many plus one of each of
// those kinds of counters on that permanent instead": ValidSource$ You (the
// putter, gorge's in-flight counter adder) and ValidObject$ the recipient.
func (lc *lowerCtx) youPlusCountersRepl(rp *params, w *cards.SA, rl *lRepl) error {
	if rp.str("ValidSource") != "You" {
		return refuse("repl-hard: AddCounter putter shape")
	}
	for _, k := range []string{"ValidCard", "ValidCounterType", "EffectOnly", "ValidCause", "ValidPlayer"} {
		if _, ok := rp.m[k]; ok {
			return refuse("repl-hard: AddCounter putter with %s$", k)
		}
	}
	if _, err := lc.anyKindBody(rp, w); err != nil {
		return err
	}
	f, err := lc.filter(youPlusObject)
	if err != nil {
		return err
	}
	rp.consume("ValidSource", "ValidObject")
	rl.Kind, rl.N, rl.F, rl.Ctr, rl.Putter = "AddCountersOn", cst(1), f, "", true
	return nil
}

// anyKindBody checks the DB$ ReplaceCounter body of an any-kind line: no
// ValidCounterType$, Amount$ naming an SVar of the shape (ChooseCounter$ True
// is gorge-inert) and returns that shape.
func (lc *lowerCtx) anyKindBody(rp *params, w *cards.SA) (string, error) {
	if w == nil || w.API != "ReplaceCounter" || w.Sub != nil {
		return "", refuse("repl-hard: AddCounter body")
	}
	bp := newParams(w.Params)
	amt := bp.str("Amount")
	if cc, ok := bp.get("ChooseCounter"); ok && cc != "True" {
		return "", refuse("repl-hard: AddCounter ChooseCounter %q", cc)
	}
	bp.consume("Amount", "ChooseCounter")
	if left := bp.unread(); len(left) > 0 {
		return "", refuse("repl-hard: AddCounter body params %v", left)
	}
	switch amount := lc.face.SVars[amt]; amount {
	case "ReplaceCount$CounterNum/Twice", "ReplaceCount$CounterNum/Plus.1":
		return amount, nil
	default:
		return "", refuse("repl-hard: AddCounter amount %q", amount)
	}
}

// replCtrKind is the Repl.C of a counter replacement: its kind, or the
// any-kind wildcard.
func replCtrKind(r lRepl) (int16, error) {
	if r.Ctr == "" {
		if r.Putter {
			return kop.ReplCtrAnyYou, nil
		}
		return kop.ReplCtrAny, nil
	}
	k, err := counterKind(r.Ctr)
	return int16(k), err
}
