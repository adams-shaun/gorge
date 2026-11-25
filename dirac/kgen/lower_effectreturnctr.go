package kgen

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// retCtrRider is an Effect's "if a <type> enters this way, it enters with N
// additional +1/+1 counters" rider, read by the return its Sub performs.
type retCtrRider struct {
	node *cards.SA // the ChangeZone node the rider rides on
	typ  string    // the entering card's type the counters need (Creature, Hero)
	n    int       // the counters
}

// effectReturnCtr lowers the Effect a graveyard return creates just before it
// moves the card (Recommission, Heroic Return, What Must Be Done, Winter
// Soldier): DB$ Effect | RememberObjects$ Targeted [& Self] | ExileOnMoved$
// Graveyard[,Stack] over one Event$ Moved | ValidCard$ <Type>.IsRemembered |
// Destination$ Battlefield | ReplacementResult$ Updated line whose body is DB$
// PutCounter | Defined$ ReplacedCard | CounterType$ P1P1 | ETB$ True [|
// CounterNum$ N], with the Effect's Sub the targeted Origin$ Graveyard |
// Destination$ Battlefield move. gorge effects/effect.go registers the live
// Moved replacement over the remembered target, and the Sub's move applies
// it: the card enters with the counters when it is of the type, and the
// Effect ends as the card leaves the graveyard (or the spell the stack), so
// it touches that one move only. Lowered onto the move: a two-arm branch on
// the target's type, the matching arm a WithCountersType$ return
// (retCtrMove). It reports false for any other shape so effectPrevent's own
// arms read it.
func (lc *lowerCtx) effectReturnCtr(ps *params) (bool, error) {
	ro := strings.TrimSpace(ps.m["RememberObjects"])
	if ro != "Targeted" && ro != "Targeted & Self" {
		return false, nil
	}
	eom := strings.TrimSpace(ps.m["ExileOnMoved"])
	if eom != "Graveyard" && eom != "Graveyard,Stack" {
		return false, nil
	}
	for _, k := range []string{"StaticAbilities", "Duration", "ValidTgts", "Triggers", "ForgetOnMoved"} {
		if _, ok := ps.m[k]; ok {
			return false, nil
		}
	}
	name := strings.TrimSpace(ps.m["ReplacementEffects"])
	r := etbParams(lc.face.SVars[name])
	if r == nil || r["Event"] != "Moved" || r["Destination"] != "Battlefield" || r["ReplacementResult"] != "Updated" {
		return false, nil
	}
	typ, ok := strings.CutSuffix(r["ValidCard"], ".IsRemembered")
	if !ok || typ == "" || strings.ContainsAny(typ, ".+,") {
		return false, nil
	}
	for k := range r {
		switch k {
		case "Event", "Destination", "ReplacementResult", "ValidCard", "ReplaceWith", "Description", "Secondary":
		default:
			return true, refuse("Effect return-counters replacement param %s", k)
		}
	}
	body := etbParams(lc.face.SVars[r["ReplaceWith"]])
	if body["DB"] != "PutCounter" || body["Defined"] != "ReplacedCard" || body["CounterType"] != "P1P1" || body["ETB"] != "True" {
		return true, refuse("Effect return-counters replacement body")
	}
	n := 1
	if v, ok := body["CounterNum"]; ok {
		var err error
		if n, err = strconv.Atoi(v); err != nil || n < 1 {
			return true, refuse("Effect return-counters CounterNum$ %q", v)
		}
	}
	if len(body) != 4 && !(len(body) == 5 && body["CounterNum"] != "") {
		return true, refuse("Effect return-counters replacement body")
	}
	sub := lc.curNode.Sub
	if sub == nil || sub.API != "ChangeZone" || sub.Params["Origin"] != "Graveyard" || sub.Params["Destination"] != "Battlefield" ||
		sub.Params["ValidTgts"] == "" || sub.Params["Defined"] != "" || sub.Params["SubAbility"] != "" {
		return true, refuse("Effect return-counters without its graveyard return")
	}
	if _, err := lc.filter(typ); err != nil {
		return true, err
	}
	ps.consume("RememberObjects", "ExileOnMoved", "ReplacementEffects")
	lc.retCtr = &retCtrRider{node: sub, typ: typ, n: n}
	return true, nil
}

// retCtrMove lowers the rider's return: SkipIf the target is not of the type
// (over the counters arm and the jump); the return with the counters (it
// declares the target slot); SkipIf never (over the plain arm); the plain
// return of that slot. The type is read before the move, on the graveyard
// card gorge's replacement matches.
func (lc *lowerCtx) retCtrMove(ps *params, r *retCtrRider) error {
	armMap := func() map[string]string {
		m := make(map[string]string, len(ps.m))
		for k, v := range ps.m {
			m[k] = v
		}
		return m
	}
	for _, k := range []string{"WithCountersType", "WithCountersAmount", "DestAltSVar"} {
		if _, ok := ps.m[k]; ok {
			return refuse("Effect return-counters move with %s$", k)
		}
	}
	nt := len(lc.ab.Targets)
	at := len(lc.ab.Prog)
	lc.emit(lInstr{Op: "SkipIf", Note: "only a card of the type enters with the counters"})
	withM := armMap()
	withM["WithCountersType"] = "P1P1"
	withM["WithCountersAmount"] = strconv.Itoa(r.n)
	if err := lc.changeZone(&params{m: withM, used: ps.used}); err != nil {
		return err
	}
	if len(lc.ab.Targets) != nt+1 || len(lc.chainT) != 1 {
		return refuse("Effect return-counters move does not declare one target slot")
	}
	f, err := lc.filter(r.typ)
	if err != nil {
		return err
	}
	lc.ab.Prog[at].Cond = lCond{Kind: cRefMatches, Ref: lRef{Kind: rTarget, N: nt}, F: f, Op: opGE, A: cst(1)}
	lc.ab.Prog[at].Skip = len(lc.ab.Prog) - at // over the counters arm and the jump
	primM := armMap()
	for _, k := range []string{"ValidTgts", "TgtPrompt", "TargetMin", "TargetMax", "TgtZone", "ValidTgtsDesc"} {
		delete(primM, k)
	}
	primM["Defined"] = "Targeted"
	jt := len(lc.ab.Prog)
	lc.emit(lInstr{Op: "SkipIf", Cond: lCond{Kind: cValue, A: cst(0), Op: opGE, B: cst(1)}, Note: "jump over the plain return"})
	if err := lc.changeZone(&params{m: primM, used: ps.used}); err != nil {
		return err
	}
	lc.ab.Prog[jt].Skip = len(lc.ab.Prog) - jt - 1
	return nil
}
