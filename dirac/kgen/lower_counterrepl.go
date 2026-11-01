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

// youPlusObject is Doc Samson's ValidObject$: a permanent its controller
// controls (the recipient filter; the putter is ValidSource$ You).
const youPlusObject = "Permanent.YouCtrl+inZoneBattlefield"

// anyKindCountersRepl lowers an unfiltered-kind R:Event$ AddCounter line whose
// ValidCard$ vc scopes the recipient and whose body only doubles (Loading
// Zone: ReplaceCount$CounterNum/Twice).
func (lc *lowerCtx) anyKindCountersRepl(rp *params, w *cards.SA, rl *lRepl, vc string) error {
	if err := lc.anyKindBody(rp, w, "ReplaceCount$CounterNum/Twice"); err != nil {
		return err
	}
	f, err := lc.filter(vc)
	if err != nil {
		return err
	}
	rl.Kind, rl.F, rl.Ctr = "DoubleCountersOn", f, ""
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
	if err := lc.anyKindBody(rp, w, "ReplaceCount$CounterNum/Plus.1"); err != nil {
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
// ValidCounterType$, Amount$ naming an SVar of exactly amount (ChooseCounter$
// True is gorge-inert).
func (lc *lowerCtx) anyKindBody(rp *params, w *cards.SA, amount string) error {
	if w == nil || w.API != "ReplaceCounter" || w.Sub != nil {
		return refuse("repl-hard: AddCounter body")
	}
	bp := newParams(w.Params)
	amt := bp.str("Amount")
	if cc, ok := bp.get("ChooseCounter"); ok && cc != "True" {
		return refuse("repl-hard: AddCounter ChooseCounter %q", cc)
	}
	bp.consume("Amount", "ChooseCounter")
	if left := bp.unread(); len(left) > 0 {
		return refuse("repl-hard: AddCounter body params %v", left)
	}
	if lc.face.SVars[amt] != amount {
		return refuse("repl-hard: AddCounter amount %q", lc.face.SVars[amt])
	}
	return nil
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
