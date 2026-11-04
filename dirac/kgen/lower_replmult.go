package kgen

import (
	"slices"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// damageDoubler lowers a filtered DamageDone doubler (Gratuitous Violence,
// Neriv, Trance Kuja): DB$ ReplaceEffect | VarName$ DamageAmount with a
// VarValue$ (an SVar or inline) of ReplaceCount$DamageAmount/Twice, a
// ValidSource$ every alternative of which is a non-self object you control,
// the ValidTarget$ words as kop.ReplDmg* bits, IsCombat$ False as
// ReplDmgNonCombat and inZoneBattlefield as ReplDmgSrcInPlay (gorge
// replacement_damage.go damageReplacementMatches). The kernel reads it as a
// ReplDoubleDamageToOpps with Filter/C set (dirac/repl_multipliers.go).
func (lc *lowerCtx) damageDoubler(rp *params, w *cards.SA, rl *lRepl) error {
	if w == nil || !withIs(w, "ReplaceEffect", map[string]string{"VarName": "DamageAmount", "VarValue": w.Params["VarValue"]}) {
		return refuse("repl-hard: DamageDone replacement shape")
	}
	vv := strings.TrimSpace(w.Params["VarValue"])
	if sv, ok := lc.face.SVars[vv]; ok {
		vv = strings.TrimSpace(sv)
	}
	if vv != "ReplaceCount$DamageAmount/Twice" {
		return refuse("repl-hard: DamageDone replacement shape")
	}
	vs, ok := rp.get("ValidSource")
	if !ok {
		return refuse("repl-hard: DamageDone doubler without ValidSource")
	}
	in, err := srcInPlay(vs)
	if err != nil {
		return err
	}
	if rl.F, err = lc.filter(vs); err != nil {
		return err
	}
	if rl.F.Players != 0 || len(rl.F.Clauses) == 0 {
		return refuse("repl-hard: DamageDone doubler ValidSource %q", vs)
	}
	for _, c := range rl.F.Clauses {
		// the kernel walks the source controller's doublers only (YouCtrl)
		if c.Flags&fSelf != 0 || c.Ctrl != whoYou {
			return refuse("repl-hard: DamageDone doubler ValidSource %q", vs)
		}
	}
	bits := 0
	for _, t := range strings.Split(rp.str("ValidTarget"), ",") {
		switch strings.TrimSpace(t) {
		case "Opponent":
			bits |= kop.ReplDmgOppPlayer
		case "Permanent.OppCtrl":
			bits |= kop.ReplDmgOppPerm
		case "Player":
			bits |= kop.ReplDmgPlayer
		case "Permanent":
			bits |= kop.ReplDmgPerm
		default:
			return refuse("repl-hard: DamageDone doubler ValidTarget %q", rp.str("ValidTarget"))
		}
	}
	if c, ok := rp.get("IsCombat"); ok {
		if strings.TrimSpace(c) != "False" {
			return refuse("repl-hard: DamageDone doubler IsCombat %q", c)
		}
		bits |= kop.ReplDmgNonCombat
	}
	if in {
		bits |= kop.ReplDmgSrcInPlay
	}
	rl.Kind, rl.Bits = "DoubleDamageToOpps", bits
	return nil
}

// tokenRepl lowers a CreateToken multiplier: DB$ ReplaceToken | Type$ Amount
// with Amount$ absent or Twice (x2) or Thrice (x3), ValidToken$ matched per
// token (Card.YouCtrl is every token: no filter). EffectOnly$ True is
// vacuous for tokens (gorge replacement_match.go: every token creation is an
// effect). Doubling Season, Exalted Sunborn, Ojer Taq.
func (lc *lowerCtx) tokenRepl(rp *params, w *cards.SA, rl *lRepl) error {
	if rp.str("ActiveZones") != "Battlefield" {
		return refuse("repl-hard: CreateToken ActiveZones")
	}
	for _, k := range []string{"ValidPlayer", "Layer", "Optional", "IsPresent"} {
		if _, ok := rp.get(k); ok {
			return refuse("repl-hard: CreateToken %s", k)
		}
	}
	if e, ok := rp.get("EffectOnly"); ok && strings.TrimSpace(e) != "True" {
		return refuse("repl-hard: CreateToken EffectOnly %q", e)
	}
	if w == nil || w.API != "ReplaceToken" {
		return refuse("repl-hard: CreateToken replacement shape")
	}
	if t := strings.TrimSpace(w.Params["Type"]); t == "ReplaceToken" {
		return lc.tokenRewrite(rp, w, rl)
	} else if t != "Amount" {
		return refuse("repl-hard: CreateToken Type %q", t)
	}
	want := map[string]string{"Type": "Amount"}
	n := 2
	if a, ok := w.Params["Amount"]; ok {
		want["Amount"] = a
		switch strings.TrimSpace(a) {
		case "Twice":
		case "Thrice":
			n = 3
		default:
			return refuse("repl-hard: CreateToken Amount %q", a)
		}
	}
	if !withIs(w, "ReplaceToken", want) {
		return refuse("repl-hard: CreateToken replacement shape")
	}
	vt := rp.str("ValidToken")
	if vt != "Card.YouCtrl" {
		var err error
		if rl.F, err = lc.filter(vt); err != nil {
			return err
		}
		if rl.F.Players != 0 || len(rl.F.Clauses) == 0 {
			return refuse("repl-hard: CreateToken ValidToken %q", vt)
		}
		for _, c := range rl.F.Clauses {
			// the kernel walks the token controller's multipliers only
			if c.Flags&fSelf != 0 || c.Ctrl != whoYou {
				return refuse("repl-hard: CreateToken ValidToken %q", vt)
			}
		}
	}
	rl.Kind = "DoubleTokens"
	if n != 2 {
		rl.N = cst(n)
	}
	return nil
}

// tokenRewrite lowers a CreateToken rewriter: DB$ ReplaceToken | Type$
// ReplaceToken with one fixed TokenScript$ (the token every token its
// controller would create is instead; the count stays), ValidToken$
// Permanent.YouCtrl or Card.YouCtrl (every token the controller creates: a
// token is always a permanent). Holo-Engineer. The script rides the card's
// token list so the matchup holds it; gorge replacement_token.go
// applyTokenReplacementToPlan rewrites each matching mint, a copy mint too,
// and a second matching replacement poses the CR 616.1 order ask, which the
// kernel stalls on (dirac/repl_tokrewrite.go).
func (lc *lowerCtx) tokenRewrite(rp *params, w *cards.SA, rl *lRepl) error {
	script := strings.TrimSpace(w.Params["TokenScript"])
	if script == "" || strings.Contains(script, ",") || strings.EqualFold(script, "Chosen") {
		return refuse("repl-hard: CreateToken rewriter TokenScript %q", script)
	}
	if !withIs(w, "ReplaceToken", map[string]string{"Type": "ReplaceToken", "TokenScript": w.Params["TokenScript"]}) {
		return refuse("repl-hard: CreateToken replacement shape")
	}
	if vt := rp.str("ValidToken"); vt != "Permanent.YouCtrl" && vt != "Card.YouCtrl" {
		return refuse("repl-hard: CreateToken rewriter ValidToken %q", vt)
	}
	if _, ok := lc.p.reg.Token(script); !ok {
		return refuse("token script %q not in the corpus", script)
	}
	if !slices.Contains(lc.out.Tokens, script) {
		lc.out.Tokens = append(lc.out.Tokens, script)
	}
	rl.Kind, rl.S = "RewriteTokens", script
	return nil
}
