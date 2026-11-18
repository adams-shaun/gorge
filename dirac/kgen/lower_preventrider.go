package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// effectPreventRider lowers the this-turn scoped prevention Effect whose
// replacement carries a body (gorge's PreventionEffect$ True with ReplaceWith$,
// damageReplacementPreventsWithRider): Chant of Vitu-Ghazi's "prevent all
// damage that would be dealt by creatures this turn; you gain life equal to
// the damage prevented". The line is Event$ DamageDone | PreventionEffect$
// True | ReplaceWith$ <svar> (no Prevent$), in the bit forms of
// effectPreventScope (ActiveZones$ absent or Command, IsCombat$ absent or
// True, ValidTarget$/ValidSource$ of scopeTargetBits/scopeSourceBits), and
// the body is exactly DB$ GainLife | Defined$ You | LifeAmount$ X with
// X = ReplaceCount$DamageAmount. A Duration$ UntilEndOfCombat on the Effect
// marks the record kop.PrevScopeEndCombat instead of the turn (prevstate-04's
// shapes). One OpExt PreventDamageScope with kop.PrevScopeGain. It reports
// false, consuming nothing, for any other shape.
func (lc *lowerCtx) effectPreventRider(ps *params, eoc bool) (bool, error) {
	for _, k := range []string{"RememberObjects", "ValidTgts"} {
		if _, ok := ps.m[k]; ok {
			return false, nil
		}
	}
	if _, ok := ps.get("Duration"); ok && !eoc {
		return false, nil
	}
	name := strings.TrimSpace(ps.peek("ReplacementEffects"))
	if name == "" || strings.Contains(name, ",") {
		return false, nil
	}
	r, ok := cards.ParseReplacementLine(lc.face.SVars[name])
	if !ok || r.Event != "DamageDone" {
		return false, nil
	}
	rm := r.Params
	if _, has := rm["Prevent"]; has || strings.TrimSpace(rm["PreventionEffect"]) != "True" {
		return false, nil
	}
	if az, has := rm["ActiveZones"]; has && strings.TrimSpace(az) != "Command" {
		return false, nil
	}
	bits := kop.PrevScopeGain
	if eoc {
		bits |= kop.PrevScopeEndCombat
	}
	if v, has := rm["IsCombat"]; has {
		if strings.TrimSpace(v) != "True" {
			return false, nil
		}
		bits |= kop.PrevScopeCombat
	}
	tb, tok := scopeTargetBits(strings.TrimSpace(rm["ValidTarget"]))
	sb, sok := scopeSourceBits(strings.TrimSpace(rm["ValidSource"]))
	if !tok || !sok {
		return false, nil
	}
	bits |= tb | sb
	if !lc.riderBodyIsGain(strings.TrimSpace(rm["ReplaceWith"])) {
		return false, nil
	}
	rp := newParams(rm)
	rp.consume("Event", "PreventionEffect", "ActiveZones", "IsCombat", "ValidTarget", "ValidSource", "ReplaceWith")
	if left := rp.unread(); len(left) > 0 {
		return false, nil
	}
	ps.consume("ReplacementEffects", "AILogic")
	if eoc {
		ps.consume("Duration", "UntilEndOfCombat")
	}
	lc.emit(lInstr{Op: "PreventDamageScope", C: bits, Note: "prevent the damage and gain that much life this turn"})
	return true, nil
}

// riderBodyIsGain reports whether SVar name is exactly DB$ GainLife | Defined$
// You | LifeAmount$ <svar> with that SVar exactly ReplaceCount$DamageAmount.
func (lc *lowerCtx) riderBodyIsGain(name string) bool {
	m := etbParams(lc.face.SVars[name])
	if name == "" || len(m) != 3 || m["DB"] != "GainLife" || m["Defined"] != "You" {
		return false
	}
	amt := m["LifeAmount"]
	return amt != "" && strings.TrimSpace(lc.face.SVars[amt]) == "ReplaceCount$DamageAmount"
}

// riderBodyIsGainSub is riderBodyIsGain with one more key: SubAbility$ <sub>,
// whose SVar is exactly the Effect's self-exile (Awe Strike's ExileEffect,
// DB$ ChangeZone | Defined$ Self | Origin$ Command | Destination$ Exile): the
// body runs once and the Effect exiles itself.
func (lc *lowerCtx) riderBodyIsGainSub(name string) bool {
	m := etbParams(lc.face.SVars[name])
	if name == "" || len(m) != 4 || m["DB"] != "GainLife" || m["Defined"] != "You" {
		return false
	}
	amt := m["LifeAmount"]
	if amt == "" || strings.TrimSpace(lc.face.SVars[amt]) != "ReplaceCount$DamageAmount" {
		return false
	}
	sub := etbParams(lc.face.SVars[m["SubAbility"]])
	return len(sub) == 4 && sub["DB"] == "ChangeZone" && sub["Defined"] == "Self" &&
		sub["Origin"] == "Command" && sub["Destination"] == "Exile"
}

// effectPreventRiderFrom lowers the remembered-source shield shapes: an Effect
// on one remembered target creature (ValidTgts$ Creature, RememberObjects$
// Targeted, ExileOnMoved$ Battlefield) whose DamageDone replacement is keyed
// on the source (ValidSource$ Card.IsRemembered, no Prevent$), carries
// PreventionEffect$ True and ReplaceWith$ <svar>. Two bodies:
//
//   - the prevstate-01 GainLife form plus the self-exile sub
//     (riderBodyIsGainSub, Awe Strike): "the next time target creature would
//     deal damage this turn, prevent that damage. You gain life equal to the
//     damage prevented this way" (CR 615.5, 615.8). No Duration$ and no
//     ValidTarget$ (prevstate-02 routes an end-of-combat duration to the
//     scoped arm). kop.PrevNextGain.
//
//   - the token form plus the self-exile sub (riderBodyIsTokenSub, Ria Ivor,
//     prevstate-04): "the next time target creature would deal combat damage
//     to one or more players this combat, prevent that damage. If damage is
//     prevented this way, create that many <token>s". IsCombat$ absent
//     matches combat and noncombat alike, True matches combat only
//     (kop.PrevNextCombat); ValidTarget$ Player matches a player recipient
//     only (kop.PrevNextPlayers); Duration$ UntilEndOfCombat ends the record
//     with the combat (kop.PrevNextEndCombat).
//
// It emits one Ext PreventNext with kop.PrevNextAll|kop.PrevNextFrom and the
// rider bits: the source-keyed shield of dirac/repl_prevnext.go, spent by the
// first event from the creature (the body's self-exile); the creature leaving
// the battlefield first ends it by ExileOnMoved$ (the unbounded shield's
// prevAllLeft). The token key goes in the lInstr CS, mapped to the pool index
// by opPreventNext, and is registered in the card's Tokens so the pool lowers
// it. A remembered-source line with no self-exile sub (an "every time" form)
// returns false, nil with nothing consumed; Inkshield's shape stays refused.
func (lc *lowerCtx) effectPreventRiderFrom(ps *params) (bool, error) {
	eoc := ps.is("Duration", "UntilEndOfCombat")
	if d := ps.peek("Duration"); d != "" && !eoc {
		return false, nil
	}
	if strings.TrimSpace(ps.peek("ExileOnMoved")) != "Battlefield" {
		return false, nil
	}
	if strings.TrimSpace(ps.peek("ValidTgts")) != "Creature" || ps.peek("RememberObjects") != "Targeted" {
		return false, nil
	}
	if _, many := ps.m["TargetMax"]; many {
		return false, nil
	}
	if _, many := ps.m["TargetMin"]; many {
		return false, nil
	}
	name := strings.TrimSpace(ps.peek("ReplacementEffects"))
	if name == "" || strings.Contains(name, ",") {
		return false, nil
	}
	r, ok := cards.ParseReplacementLine(lc.face.SVars[name])
	if !ok || r.Event != "DamageDone" {
		return false, nil
	}
	rm := r.Params
	if _, has := rm["Prevent"]; has || strings.TrimSpace(rm["PreventionEffect"]) != "True" {
		return false, nil
	}
	players := false
	if _, has := rm["ValidTarget"]; has {
		if strings.TrimSpace(rm["ValidTarget"]) != "Player" {
			return false, nil
		}
		players = true
	}
	if vs := strings.TrimSpace(rm["ValidSource"]); vs != "Card.IsRemembered" {
		return false, nil
	}
	bits := kop.PrevNextAll | kop.PrevNextFrom
	if v, has := rm["IsCombat"]; has {
		if strings.TrimSpace(v) != "True" {
			return false, nil
		}
		bits |= kop.PrevNextCombat
	}
	body := strings.TrimSpace(rm["ReplaceWith"])
	var tok string
	switch {
	case !eoc && !players && lc.riderBodyIsGainSub(body):
		bits |= kop.PrevNextGain
	default:
		key, ok := lc.riderBodyIsTokenSub(body)
		if !ok {
			return false, nil
		}
		bits |= kop.PrevNextTokens
		if eoc {
			bits |= kop.PrevNextEndCombat
		}
		if players {
			bits |= kop.PrevNextPlayers
		}
		tok = key
	}
	rp := newParams(rm)
	rp.consume("Event", "PreventionEffect", "ValidSource", "IsCombat", "ValidTarget", "ReplaceWith", "Description")
	if left := rp.unread(); len(left) > 0 {
		return false, nil
	}
	obj, err := lc.objOrTarget(ps, "", zBattlefield)
	if err != nil {
		return true, err
	}
	in := lInstr{Op: "PreventNext", Obj: obj, C: bits, CS: tok,
		Note: "the next damage the remembered creature deals is prevented and its body runs"}
	if err := lc.tokRiderAgrees(in); err != nil {
		return true, err
	}
	ps.consume("ReplacementEffects", "AILogic", "ValidTgts", "RememberObjects", "ExileOnMoved", "Name", "Duration")
	if tok != "" {
		lc.out.Tokens = append(lc.out.Tokens, tok)
	}
	lc.emit(in)
	return true, nil
}

// riderBodyIsTokenSub reports whether SVar name is exactly DB$ Token |
// TokenAmount$ <svar> | TokenScript$ <one script> | TokenOwner$ You |
// SubAbility$ <sub>, with that SVar exactly ReplaceCount$DamageAmount and the
// sub exactly the Effect's self-exile (Awe Strike's ExileEffect, DB$
// ChangeZone | Defined$ Self | Origin$ Command | Destination$ Exile): the
// body runs once and the Effect exiles itself. It reports the script key,
// which must be in the corpus.
func (lc *lowerCtx) riderBodyIsTokenSub(name string) (string, bool) {
	m := etbParams(lc.face.SVars[name])
	if name == "" || len(m) != 5 || m["DB"] != "Token" || m["TokenOwner"] != "You" {
		return "", false
	}
	amt := m["TokenAmount"]
	if amt == "" || strings.TrimSpace(lc.face.SVars[amt]) != "ReplaceCount$DamageAmount" {
		return "", false
	}
	script := strings.TrimSpace(m["TokenScript"])
	if script == "" || strings.Contains(script, ",") {
		return "", false
	}
	if _, ok := lc.p.reg.Token(script); !ok {
		return "", false
	}
	sub := etbParams(lc.face.SVars[m["SubAbility"]])
	if len(sub) != 4 || sub["DB"] != "ChangeZone" || sub["Defined"] != "Self" ||
		sub["Origin"] != "Command" || sub["Destination"] != "Exile" {
		return "", false
	}
	return script, true
}

// tokRiderAgrees refuses a card whose program already holds a token-rider
// PreventNext that differs from in: the kernel finds a record's token from
// the Effect's source card (prevTokInstr), so one card carries one token
// there.
func (lc *lowerCtx) tokRiderAgrees(in lInstr) error {
	same := func(o lInstr) bool { return o.C == in.C && o.CS == in.CS }
	progs := [][]lInstr{lc.ab.Prog}
	for _, a := range lc.out.Abilities {
		if a != lc.ab {
			progs = append(progs, a.Prog)
		}
	}
	for _, p := range progs {
		for _, o := range p {
			if o.Op == "PreventNext" && o.C&kop.PrevNextTokens != 0 && !same(o) {
				return refuse("Effect prevention rider: two token riders")
			}
		}
	}
	return nil
}
