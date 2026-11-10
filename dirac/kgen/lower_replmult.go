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
	if w != nil && w.API == "ReplaceToken" && isCopyFirstBody(w) {
		return lc.tokenCopyFirst(rp, w, rl)
	}
	for _, k := range []string{"Layer", "Optional", "IsPresent"} {
		if _, ok := rp.get(k); ok {
			return refuse("repl-hard: CreateToken %s", k)
		}
	}
	// ValidPlayer$ You is the creating player being the replacement's
	// controller, which the kernel's token replacements read already (they walk
	// the creator's battlefield); any other player is not modelled.
	youOnly := false
	if vp, ok := rp.get("ValidPlayer"); ok {
		if strings.TrimSpace(vp) != "You" {
			return refuse("repl-hard: CreateToken ValidPlayer %q", vp)
		}
		youOnly = true
	}
	if e, ok := rp.get("EffectOnly"); ok && strings.TrimSpace(e) != "True" {
		return refuse("repl-hard: CreateToken EffectOnly %q", e)
	}
	if w == nil || w.API != "ReplaceToken" {
		return refuse("repl-hard: CreateToken replacement shape")
	}
	if t := strings.TrimSpace(w.Params["Type"]); t == "ReplaceToken" {
		return lc.tokenRewrite(rp, w, rl)
	} else if t == "AddToken" {
		return lc.tokenAdd(rp, w, rl, youOnly)
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

// tokenAdd lowers a CreateToken adder: DB$ ReplaceToken | Type$ AddToken |
// Amount$ 1 | TokenScript$ one fixed script, ValidPlayer$ You, ValidToken$ a
// filter on the token created (Worldwalker Helm: Artifact). Beside the tokens
// the controller creates, one token of the script is created too, once per
// creation event (gorge applyTokenReplacementToPlan: the fixed Amount$ block is
// claimed once per Token resolution, claimTokenCreationAdd). The script rides
// the card's token list. The kernel makes the extra token at a CreateToken op
// and stalls where it cannot (repl_tokorder.go mintTokAdds).
func (lc *lowerCtx) tokenAdd(rp *params, w *cards.SA, rl *lRepl, youOnly bool) error {
	if !youOnly {
		return refuse("repl-hard: CreateToken AddToken without ValidPlayer$ You")
	}
	script := strings.TrimSpace(w.Params["TokenScript"])
	if script == "" || strings.Contains(script, ",") || strings.EqualFold(script, "Chosen") {
		return refuse("repl-hard: CreateToken adder TokenScript %q", script)
	}
	if !withIs(w, "ReplaceToken", map[string]string{"Type": "AddToken", "Amount": "1", "TokenScript": w.Params["TokenScript"]}) {
		return refuse("repl-hard: CreateToken adder shape")
	}
	vt := rp.str("ValidToken")
	f, err := lc.filter(vt)
	if err != nil {
		return err
	}
	if f.Players != 0 || len(f.Clauses) == 0 {
		return refuse("repl-hard: CreateToken adder ValidToken %q", vt)
	}
	for _, c := range f.Clauses {
		// the kernel reads the token's card and its creator's own battlefield
		if c.Flags&fSelf != 0 || (c.Ctrl != whoYou && c.Ctrl != whoAny) {
			return refuse("repl-hard: CreateToken adder ValidToken %q", vt)
		}
	}
	if _, ok := lc.p.reg.Token(script); !ok {
		return refuse("token script %q not in the corpus", script)
	}
	if !slices.Contains(lc.out.Tokens, script) {
		lc.out.Tokens = append(lc.out.Tokens, script)
	}
	rl.F, rl.Kind, rl.S = f, "AddToken", script
	return nil
}

// preventMillRepl lowers The Mindskinner's prevent-and-mill replacement:
// ValidSource$ Card.YouCtrl,Emblem.YouCtrl | ValidTarget$ Opponent |
// ReplaceWith$ DB$ Mill | Defined$ Opponent | NumCards$ ReplaceCount$DamageAmount.
// gorge (replacement.go applyNonMoveReplacements) runs the Mill body in the
// replacement's frame and hands back a stopped damage event, one application
// per damage event (combat damage is one event per source and recipient, so
// ExecuteMode$ PerSource is what gorge does anyway); AlwaysReplace$ and
// PreventionEffect$ are not read there. false: not this shape.
func (lc *lowerCtx) preventMillRepl(rp *params, w *cards.SA, rl *lRepl) (bool, error) {
	if w == nil || w.API != "Mill" || rp.peek("ValidTarget") != "Opponent" {
		return false, nil
	}
	if rp.str("ValidSource") != "Card.YouCtrl,Emblem.YouCtrl" || !rp.is("AlwaysReplace", "True") ||
		!rp.is("PreventionEffect", "True") || !rp.is("ExecuteMode", "PerSource") || rp.str("ValidTarget") != "Opponent" {
		return true, refuse("repl-hard: DamageDone prevent-and-mill shape")
	}
	bp := newParams(w.Params)
	bp.consume("Defined", "NumCards")
	nc := strings.TrimSpace(w.Params["NumCards"])
	if sv, ok := lc.face.SVars[nc]; ok {
		nc = strings.TrimSpace(sv)
	}
	if w.Sub != nil || bp.str("Defined") != "Opponent" || nc != "ReplaceCount$DamageAmount" || len(bp.unread()) != 0 {
		return true, refuse("repl-hard: DamageDone prevent-and-mill body")
	}
	rl.Kind = "PreventMillOpps"
	return true, nil
}

// selfDamageRepl lowers the two DamageDone replacements of a permanent on
// damage to itself (ValidTarget$ Card.Self, no ValidSource$ or IsCombat$):
//
//   - Anti-Venom, Horrifying Healer: PreventionEffect$ True | AlwaysReplace$
//     True with a DB$ PutCounter | Defined$ ReplacedTarget | CounterType$ P1P1
//     | CounterNum$ ReplaceCount$DamageAmount body. gorge
//     (rules/replacement.go applyNonMoveReplacements) runs the body at the
//     proposal and hands back a stopped event (no damage, no lifelink, no
//     damage trigger); under a can't-prevent effect the counters are put and
//     the damage stands (CR 615.12). AlwaysReplace$ is not read there.
//   - Wolverine, Fierce Fighter: ReplacementResult$ Updated with a DB$
//     HealDamage | Defined$ ReplacedTarget body: the marked damage is healed
//     and the held event is still dealt (the damage, its lifelink and its
//     triggers all happen).
//
// false: not this shape (ValidTarget$ is not Card.Self).
func (lc *lowerCtx) selfDamageRepl(rp *params, w *cards.SA, rl *lRepl) (bool, error) {
	if rp.peek("ValidTarget") != "Card.Self" || w == nil || w.API != "PutCounter" && w.API != "HealDamage" {
		return false, nil
	}
	rp.consume("ValidTarget")
	if _, ok := rp.get("ValidSource"); ok {
		return true, refuse("repl-hard: DamageDone self replacement ValidSource")
	}
	if _, ok := rp.get("IsCombat"); ok {
		return true, refuse("repl-hard: DamageDone self replacement IsCombat")
	}
	bp := newParams(w.Params)
	if w.API == "HealDamage" {
		if _, ok := rp.get("PreventionEffect"); ok || rp.peek("ReplacementResult") != "Updated" {
			return true, refuse("repl-hard: DamageDone heal replacement shape")
		}
		bp.consume("Defined")
		if w.Sub != nil || bp.str("Defined") != "ReplacedTarget" || len(bp.unread()) != 0 {
			return true, refuse("repl-hard: DamageDone heal replacement body")
		}
		rl.Kind = "DamageHealFirst"
		return true, nil
	}
	if !rp.is("PreventionEffect", "True") || rp.peek("ReplacementResult") != "" {
		return true, refuse("repl-hard: DamageDone prevent-and-counters shape")
	}
	rp.consume("AlwaysReplace") // not read by gorge
	bp.consume("Defined", "CounterType", "CounterNum")
	cn := strings.TrimSpace(w.Params["CounterNum"])
	if sv, ok := lc.face.SVars[cn]; ok {
		cn = strings.TrimSpace(sv)
	}
	if w.Sub != nil || bp.str("Defined") != "ReplacedTarget" || bp.str("CounterType") != "P1P1" ||
		cn != "ReplaceCount$DamageAmount" || len(bp.unread()) != 0 {
		return true, refuse("repl-hard: DamageDone prevent-and-counters body")
	}
	rl.Kind = "PreventDamageCounters"
	return true, nil
}

// copyFirstGate is the SVar both copy-instead replacements read: the tokens its
// controller owns that entered the battlefield this turn (gorge's per-add
// zone-entry list), none of them yet.
const copyFirstGate = "Count$ThisTurnEntered_Battlefield_Card.tokenCreated+YouOwn"

// isCopyFirstBody reports a ReplaceToken body whose copy source is the
// permanent its replacement is attached to (Type$ ReplaceToken, TokenScript$
// Chosen, ValidChoices$ Card.EquippedBy / Card.EnchantedBy): the
// Mirrormind Crown / Moonlit Meditation shape. Esix and Fractal Bloom choose
// among other creatures and stay refused (tokenRepl's own checks).
func isCopyFirstBody(w *cards.SA) bool {
	vc := strings.TrimSpace(w.Params["ValidChoices"])
	return strings.TrimSpace(w.Params["Type"]) == "ReplaceToken" && strings.EqualFold(strings.TrimSpace(w.Params["TokenScript"]), "Chosen") &&
		(vc == "Card.EquippedBy" || vc == "Card.EnchantedBy")
}

// tokenCopyFirst lowers the optional copy-instead CreateToken replacement of an
// Equipment or Aura: "the first time you would create one or more tokens each
// turn, you may instead create that many tokens that are copies of the
// equipped creature / enchanted permanent" (R:Event$ CreateToken | ValidPlayer$
// You | Optional$ True | Layer$ Copy | CheckSVar$ X | SVarCompare$ EQ0, X the
// tokens its controller owns that entered this turn: copyFirstGate). gorge
// (replacement_token.go poseChosenTokenReplacement) asks the controller for each
// token's creation while the gate is open, decline first; one accepted token
// closes the gate, so a creation of several tokens makes one copy and plain
// tokens after it. The gate lowers to the replacement's Repl.Cond.
// The Crown's IsPresent$ Card.Self+AttachedTo Creature is the attachment the
// copy source needs (an Equipment attaches to creatures only).
func (lc *lowerCtx) tokenCopyFirst(rp *params, w *cards.SA, rl *lRepl) error {
	vc := strings.TrimSpace(w.Params["ValidChoices"])
	if !withIs(w, "ReplaceToken", map[string]string{"Type": "ReplaceToken", "ValidChoices": w.Params["ValidChoices"], "TokenScript": w.Params["TokenScript"]}) {
		return refuse("repl-hard: CreateToken copy replacement shape")
	}
	if rp.str("Layer") != "Copy" || !rp.is("Optional", "True") {
		return refuse("repl-hard: CreateToken copy replacement Layer/Optional")
	}
	if vp := rp.str("ValidPlayer"); vp != "You" {
		return refuse("repl-hard: CreateToken copy ValidPlayer %q", vp)
	}
	if e, ok := rp.get("EffectOnly"); ok && strings.TrimSpace(e) != "True" {
		return refuse("repl-hard: CreateToken EffectOnly %q", e)
	}
	if ip, ok := rp.get("IsPresent"); ok {
		if vc != "Card.EquippedBy" || strings.TrimSpace(ip) != "Card.Self+AttachedTo Creature" {
			return refuse("repl-hard: CreateToken copy IsPresent %q", ip)
		}
	}
	cs, ok := rp.get("CheckSVar")
	if !ok || strings.TrimSpace(cs) != "X" || strings.TrimSpace(lc.face.SVars["X"]) != copyFirstGate || strings.TrimSpace(rp.str("SVarCompare")) != "EQ0" {
		return refuse("repl-hard: CreateToken copy gate")
	}
	// gorge's tokenCreated is the token flag (effects/filter.go), the word
	// kgen lowers as "token"; no other card's tokenCreated is read here
	v, err := lc.value(strings.Replace(copyFirstGate, "tokenCreated", "token", 1))
	if err != nil {
		return err
	}
	op, b, err := lc.cmpSpec("EQ0")
	if err != nil {
		return err
	}
	rl.Cond = lCond{Kind: cValue, A: v, Op: op, B: b}
	rl.Kind = "CopyFirstToken"
	return nil
}
