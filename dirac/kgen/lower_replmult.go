package kgen

import (
	"slices"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// damageDoubler lowers a filtered DamageDone doubler (Gratuitous Violence,
// Neriv, Trance Kuja): DB$ ReplaceEffect | VarName$ DamageAmount with a
// VarValue$ (an SVar or inline) of ReplaceCount$DamageAmount/Twice or /Thrice
// (City on Fire, Fiery Emancipation; the Thrice lowers to Repl.A 3), a
// ValidSource$ every alternative of which is a non-self object you control
// (Fire Servant: Instant.YouCtrl,Sorcery.YouCtrl), names no controller
// (Dictate of the Twin Gods, Furnace of Rath: Card,Emblem, any source's) or
// Card.Self alone (Charging Tuskodon), the ValidTarget$ words as kop.ReplDmg*
// bits (absent: any player or permanent; You and Card.Self: Goldnight
// Castigator), IsCombat$ False as ReplDmgNonCombat and True as
// ReplDmgCombat, and inZoneBattlefield as ReplDmgSrcInPlay (gorge
// replacement_damage.go damageReplacementMatches). The kernel reads it as a
// ReplDoubleDamageToOpps with Filter/C set (dirac/repl_multipliers.go).
// damageMinusRepl lowers a printed damage minus (Benevolent Unicorn,
// Lashknife Barrier; gorge replacement_damage.go's Updated DamageAmount
// rewrites): DB$ ReplaceEffect | VarName$ DamageAmount with a VarValue$ (an
// SVar or inline) of ReplaceCount$DamageAmount/Minus.<n>, a ValidSource$ of
// Spell (the resolving spell; ReplDmgSrcSpell) and the ValidTarget$ words as
// kop.ReplDmg* bits or a single object filter alternative (Lashknife
// Barrier's Creature.YouCtrl). The event's damage is that much minus n,
// floored at none; the kernel reads it beside the plus-N and doubler
// rewrites (dirac/repl_dmgminus.go). false: not this shape.
func (lc *lowerCtx) damageMinusRepl(rp *params, w *cards.SA, rl *lRepl) (bool, error) {
	if w == nil || !withIs(w, "ReplaceEffect", map[string]string{"VarName": "DamageAmount", "VarValue": w.Params["VarValue"]}) {
		return false, nil
	}
	vv := strings.TrimSpace(w.Params["VarValue"])
	if sv, ok := lc.face.SVars[vv]; ok {
		vv = strings.TrimSpace(sv)
	}
	half := vv == "ReplaceCount$DamageAmount/HalfDown" // Ghosts of the Innocent: half of it, rounded down
	ns, okN := strings.CutPrefix(vv, "ReplaceCount$DamageAmount/Minus.")
	nn, errN := strconv.Atoi(ns)
	if half {
		nn, okN, errN = 1, true, nil // Repl.A is unused beside ReplDmgHalfDown
	}
	if !okN || errN != nil || nn < 1 {
		return false, nil // the set-to-N shape (damageSetRepl) and the doubler read their own values
	}
	if az := rp.str("ActiveZones"); az != "Battlefield" {
		return true, refuse("repl-hard: DamageDone minus ActiveZones %q", az)
	}
	bp := newParams(w.Params)
	bp.consume("VarName", "VarValue")
	if left := bp.unread(); len(left) > 0 {
		return true, refuse("repl-hard: DamageDone minus body params %v", left)
	}
	var bits int
	if vs, ok := rp.get("ValidSource"); ok && strings.TrimSpace(vs) != "Spell" {
		return true, refuse("repl-hard: DamageDone minus ValidSource %q", vs)
	} else if ok {
		bits |= kop.ReplDmgSrcSpell
	}
	targets := "Player,Permanent" // no ValidTarget$: any recipient
	if vt, ok := rp.get("ValidTarget"); ok {
		targets = vt
	}
	var tf *lFilter
	for _, t := range strings.Split(targets, ",") {
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
			if tf != nil {
				return true, refuse("repl-hard: DamageDone minus ValidTarget %q", targets)
			}
			var err error
			if tf, err = lc.filter(t); err != nil {
				return true, err
			}
			if tf.Players != 0 || len(tf.Clauses) == 0 {
				// a player-spec word (You): dmgMinusN reads the filter on
				// permanent recipients only, so the line would never apply
				return true, refuse("repl-hard: DamageDone minus ValidTarget %q", targets)
			}
		}
	}
	if half {
		bits |= kop.ReplDmgHalfDown
	}
	rl.Kind, rl.Bits, rl.N, rl.F = "DamageMinus", bits, cst(nn), tf
	return true, nil
}

func (lc *lowerCtx) damageDoubler(rp *params, w *cards.SA, rl *lRepl) error {
	if w == nil || !withIs(w, "ReplaceEffect", map[string]string{"VarName": "DamageAmount", "VarValue": w.Params["VarValue"]}) {
		return refuse("repl-hard: DamageDone replacement shape")
	}
	vv := strings.TrimSpace(w.Params["VarValue"])
	if sv, ok := lc.face.SVars[vv]; ok {
		vv = strings.TrimSpace(sv)
	}
	mult := 2
	switch vv {
	case "ReplaceCount$DamageAmount/Twice":
	case "ReplaceCount$DamageAmount/Thrice":
		mult = 3 // City on Fire, Fiery Emancipation
	default:
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
	// Card.Self alone is the host's own damage (Charging Tuskodon): its
	// controller is the source's, and the source must be the host on the
	// battlefield (ReplDmgSrcInPlay; the kernel's FSelf reads the host slot)
	self := len(rl.F.Clauses) == 1 && rl.F.Clauses[0] == (lClause{Flags: fSelf})
	for _, c := range rl.F.Clauses {
		// the kernel walks the source controller's doublers, both seats' when
		// an alternative names no controller (Card,Emblem: Dictate of the Twin
		// Gods, Furnace of Rath); any other controller word (an opponent's
		// source) is not modelled
		if !self && (c.Flags&fSelf != 0 || c.Ctrl != whoYou && c.Ctrl != whoAny) {
			return refuse("repl-hard: DamageDone doubler ValidSource %q", vs)
		}
	}
	bits := 0
	targets := "Player,Permanent" // no ValidTarget$: any recipient (gorge damageReplacementMatches; Fire Servant)
	if vt, ok := rp.get("ValidTarget"); ok {
		targets = vt
	}
	for _, t := range strings.Split(targets, ",") {
		switch strings.TrimSpace(t) {
		case "Opponent":
			bits |= kop.ReplDmgOppPlayer
		case "Permanent.OppCtrl":
			bits |= kop.ReplDmgOppPerm
		case "Player":
			bits |= kop.ReplDmgPlayer
		case "Permanent":
			bits |= kop.ReplDmgPerm
		case "You":
			// the replacement's controller (Goldnight Castigator's "damage to
			// you": any source's, so any player's damage to its controller)
			bits |= kop.ReplDmgYou
		case "Card.Self":
			// the replacement's host itself (Goldnight Castigator's second
			// line: any source's damage to CARDNAME)
			bits |= kop.ReplDmgSelf
		default:
			return refuse("repl-hard: DamageDone doubler ValidTarget %q", targets)
		}
	}
	if c, ok := rp.get("IsCombat"); ok {
		switch strings.TrimSpace(c) {
		case "False":
			bits |= kop.ReplDmgNonCombat
		case "True":
			bits |= kop.ReplDmgCombat
		default:
			return refuse("repl-hard: DamageDone doubler IsCombat %q", c)
		}
	}
	if in || self {
		bits |= kop.ReplDmgSrcInPlay
	}
	rl.Kind, rl.Bits = "DoubleDamageToOpps", bits
	if mult != 2 {
		rl.N = cst(mult)
	}
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
//   - the Phantom creatures (Phantom Tiger, Unbreathing Horde): the same
//     prevention with a DB$ RemoveCounter | Defined$ Self | CounterType$
//     P1P1 | CounterNum$ 1 body (preventRemoveCounter).
//   - Wolverine, Fierce Fighter: ReplacementResult$ Updated with a DB$
//     HealDamage | Defined$ ReplacedTarget body: the marked damage is healed
//     and the held event is still dealt (the damage, its lifelink and its
//     triggers all happen).
//
// false: not this shape (ValidTarget$ is not Card.Self).
func (lc *lowerCtx) selfDamageRepl(rp *params, w *cards.SA, rl *lRepl) (bool, error) {
	gated := rp.peek("ValidTarget") == "Card.Self+counters_GE1_P1P1" && w != nil && w.API == "RemoveCounter"
	if rp.peek("ValidTarget") != "Card.Self" && !gated || w == nil || w.API != "PutCounter" && w.API != "HealDamage" && w.API != "RemoveCounter" {
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
	if w.API == "RemoveCounter" {
		if err := lc.preventRemoveCounter(w, rl); err != nil {
			return true, err
		}
		if gated {
			rl.Bits |= kop.ReplPrevSelfIfCounter // only while it has a +1/+1 counter (Undergrowth Champion)
		}
		return true, nil
	}
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
// copy source needs (an Equipment attaches to creatures only): it lowers to
// Repl.C = kop.ReplCopyFirstAttached, so an unattached Crown is never a
// candidate of gorge's CR 616.1 order ask.
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
		// gorge reads the presence test at every match: a Crown on no
		// creature is no candidate at all (Repl.C, kop.ReplCopyFirstAttached)
		rl.Bits = kop.ReplCopyFirstAttached
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

// preventRemoveCounter lowers the Phantom body: "if damage would be dealt to
// CARDNAME, prevent that damage. Remove a +1/+1 counter from CARDNAME"
// (DB$ RemoveCounter | Defined$ Self or ReplacedTarget, both the host here
// since ValidTarget$ is Card.Self | CounterType$ P1P1 | CounterNum$ 1). gorge
// runs it as the prevent-and-counters body: the body at the proposal, a
// stopped event, the body still run and the damage standing under a
// can't-prevent static (CR 615.12). It lowers to ReplPreventDamageCounters
// with Repl.C = kop.ReplPrevSelfRemoveOne.
func (lc *lowerCtx) preventRemoveCounter(w *cards.SA, rl *lRepl) error {
	bp := newParams(w.Params)
	bp.consume("Defined", "CounterType", "CounterNum")
	cn := bp.str("CounterNum")
	if sv, ok := lc.face.SVars[cn]; ok {
		cn = strings.TrimSpace(sv)
	}
	if d := bp.str("Defined"); w.Sub != nil || d != "Self" && d != "ReplacedTarget" || bp.str("CounterType") != "P1P1" ||
		cn != "1" && cn != "ReplaceCount$DamageAmount" || len(bp.unread()) != 0 {
		return refuse("repl-hard: DamageDone prevent-and-remove-counter body")
	}
	rl.Kind, rl.Bits = "PreventDamageCounters", kop.ReplPrevSelfRemoveOne
	if cn == "ReplaceCount$DamageAmount" {
		// "remove that many +1/+1 counters" (Polukranos, Unchained)
		rl.Bits = kop.ReplPrevSelfRemoveDmg
	}
	return nil
}

// damageSetRepl lowers a printed damage set (Divine Presence, Forethought
// Amulet; gorge replacement_damage.go's Updated DamageAmount rewrites):
// DB$ ReplaceEffect | VarName$ DamageAmount with a literal VarValue$, a
// DamageAmount$ GE<k> gate and the ValidTarget$ words as kop.ReplDmg* bits
// (You: ReplDmgPlayer; the player/permanent words their bits, any other word
// refuses — no corpus carrier names a filter recipient). The ValidSource$
// alternatives are the object filter read against the damage source (a
// resolving spell's card for Instant,Sorcery: the read spMatch's departed
// source arm makes). The event's damage becomes the set amount when it is k
// or more (the gate at match time); the gate value rides C's high byte (the
// low bits stay the ReplDmg* words). The kernel reads it beside the minus
// rewrite (dirac/repl_dmgminus.go). false: not this shape.
func (lc *lowerCtx) damageSetRepl(rp *params, w *cards.SA, rl *lRepl) (bool, error) {
	if w == nil || !withIs(w, "ReplaceEffect", map[string]string{"VarName": "DamageAmount", "VarValue": w.Params["VarValue"]}) {
		return false, nil
	}
	vv := strings.TrimSpace(w.Params["VarValue"])
	if sv, ok := lc.face.SVars[vv]; ok {
		vv = strings.TrimSpace(sv)
	}
	n, errN := strconv.Atoi(vv)
	if errN != nil || n < 0 || n > 1<<14 {
		return false, nil // the minus rewrite and the doubler read their own values
	}
	ge, ok := rp.get("DamageAmount")
	k, errK := strconv.Atoi(strings.TrimPrefix(strings.TrimSpace(ge), "GE"))
	if !ok || errK != nil || k < 1 || k > 15 || k <= n {
		return true, refuse("repl-hard: DamageDone set gate %q over %d", ge, n)
	}
	if az := rp.str("ActiveZones"); az != "Battlefield" {
		return true, refuse("repl-hard: DamageDone set ActiveZones %q", az)
	}
	bp := newParams(w.Params)
	bp.consume("VarName", "VarValue")
	if left := bp.unread(); len(left) > 0 {
		return true, refuse("repl-hard: DamageDone set body params %v", left)
	}
	var bits int
	if vs, ok := rp.get("ValidSource"); ok {
		var err error
		if rl.F2, err = lc.filter(vs); err != nil {
			return true, err
		}
		if rl.F2.Players != 0 || len(rl.F2.Clauses) == 0 {
			return true, refuse("repl-hard: DamageDone set ValidSource %q", vs)
		}
	}
	targets := "Player,Permanent" // no ValidTarget$: any recipient
	if vt, ok := rp.get("ValidTarget"); ok {
		targets = vt
	}
	for _, t := range strings.Split(targets, ",") {
		switch strings.TrimSpace(t) {
		case "Opponent":
			bits |= kop.ReplDmgOppPlayer
		case "Permanent.OppCtrl":
			bits |= kop.ReplDmgOppPerm
		case "Player": // "You" is the controller alone (gorge MatchesPlayerSpecFrom): no kernel bit, so it refuses
			bits |= kop.ReplDmgPlayer
		case "Permanent":
			bits |= kop.ReplDmgPerm
		default:
			return true, refuse("repl-hard: DamageDone set ValidTarget %q", targets)
		}
	}
	rl.Kind, rl.Bits, rl.N = "DamageSet", bits|kop.ReplDmgSetGE(k), cst(n)
	return true, nil
}
