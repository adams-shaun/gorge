package kgen

import (
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// staticPrevAmountLE is DamageAmount$ LE<n>, the only comparison a line is
// lowered with (Callous Giant).
var staticPrevAmountLE = regexp.MustCompile(`^LE(\d+)$`)

// staticDamagePrevent lowers a printed static prevention of damage that is not
// a prevention of damage to the permanent itself (preventToSelf) or Crystal
// Barricade's: R:Event$ DamageDone | Prevent$ True (the full kind), or a
// ReplaceWith$ DB$ ReplaceDamage | Amount$ <N> body (the prevent-N shield
// kind: Sphere of Law, Urza's Armor, Hyperion's all-but-one). The line's
// ValidTarget$ (the player words You, Player and Opponent become bits, any other
// alternative the recipient filter, absent: every player and permanent),
// ValidSource$, IsCombat$, PlayerTurn$, DamageAmount$ LEn, CheckSVar$ /
// IsPresent$ and Optional$ lower as kop.ReplSP* bits, filters and a gate; any
// other key refuses by name (lowerRepl's unread check). It lowers to
// ReplPreventDamageFilt / ReplPreventDamageN (dirac/repl_staticprevent.go).
// RelativeToSource$ is read by nothing in gorge's damage matching
// (damageReplacementMatches), so it lowers as gorge plays it. false: not this
// shape.
func (lc *lowerCtx) staticDamagePrevent(rp *params, w *cards.SA, rl *lRepl) (bool, error) {
	full := w == nil && strings.EqualFold(rp.peek("Prevent"), "True")
	shield := w != nil && w.API == "ReplaceDamage"
	if !full && !shield {
		return false, nil
	}
	if full && rp.peek("ValidTarget") == "Creature.YouCtrl+Other" && rp.peek("IsCombat") == "False" && rp.peek("ValidSource") == "" {
		return false, nil // Crystal Barricade's own kind (PreventNoncombatToOthers)
	}
	if az := rp.peek("ActiveZones"); az != "Battlefield" {
		// An empty ActiveZones$ leaves the line live in every zone of gorge's
		// replacement scan (forEachReplacementSource visits hand, graveyard
		// and library); the kernel's prevention scan is the battlefield, so
		// the line lowers only when the match cannot hold while the printed
		// permanent is off the battlefield: a filter whose every alternative
		// names the printed permanent itself (Self) or a permanent it is
		// attached to (EnchantedBy / EquippedBy — nothing is attached to a
		// card in hand, in a graveyard or in a library) can never match
		// there, while a player scope, an absent ValidTarget$ beside an
		// unanchored ValidSource$ and every plain type or controller word
		// can (Gaseous Form, Candletrap, Demonic Torment, Armament of Nyx,
		// Camel; Bronze Horse's line also refuses, its ValidCause$ read has
		// no kernel damage-site read).
		if az != "" || !prevAnchored(rp.peek("ValidTarget"), rp.peek("ValidSource")) {
			return true, refuse("repl-hard: DamageDone static prevention ActiveZones %q", az)
		}
	}
	rp.consume("Prevent", "PreventionEffect", "ActiveZones", "RelativeToSource", "AlwaysReplace", "Secondary")
	if err := lc.staticPrevRecipient(rp, rl); err != nil {
		return true, err
	}
	if err := lc.staticPrevSource(rp, rl); err != nil {
		return true, err
	}
	if err := lc.staticPrevGates(rp, rl); err != nil {
		return true, err
	}
	rl.Kind = "PreventDamageFilt"
	if shield {
		rl.Kind = "PreventDamageN"
		if err := lc.staticPrevAmount(w, rl); err != nil {
			return true, err
		}
	}
	if v, ok := rp.get("DamageAmount"); ok {
		m := staticPrevAmountLE.FindStringSubmatch(strings.TrimSpace(v))
		if m == nil || shield {
			return true, refuse("repl-hard: DamageDone prevention DamageAmount$ %q", v)
		}
		n, _ := strconv.Atoi(m[1])
		rl.N, rl.Bits = cst(n), rl.Bits|kop.ReplSPDmgLE
	}
	return true, nil
}

// staticPrevRecipient reads ValidTarget$: You, Player and Opponent are the
// player bits, "Permanent" alone every permanent, any other alternative the
// permanent filter. No ValidTarget$ is every player and permanent.
func (lc *lowerCtx) staticPrevRecipient(rp *params, rl *lRepl) error {
	vt, ok := rp.get("ValidTarget")
	if !ok {
		rl.Bits |= kop.ReplSPYou | kop.ReplSPOpp | kop.ReplSPAnyPerm
		return nil
	}
	var perm []string
	for _, alt := range strings.Split(vt, ",") {
		switch alt = strings.TrimSpace(alt); alt {
		case "You":
			rl.Bits |= kop.ReplSPYou
		case "Opponent":
			rl.Bits |= kop.ReplSPOpp
		case "Player":
			rl.Bits |= kop.ReplSPYou | kop.ReplSPOpp
		case "Permanent":
			rl.Bits |= kop.ReplSPAnyPerm
		case "Creature.Snow":
			// the printed Snow supertype is no filter word (unmodeledSupertype): a creature
			// recipient whose card is Snow (CardDef.Snow), alone in the list
			rl.Bits |= kop.ReplSPRecipSnow
			perm = append(perm, "Creature")
		default:
			perm = append(perm, alt)
		}
	}
	if len(perm) == 0 {
		return nil
	}
	if rl.Bits&kop.ReplSPRecipSnow != 0 && len(perm) != 1 {
		return refuse("repl-hard: DamageDone prevention ValidTarget %q", vt)
	}
	if rl.Bits&kop.ReplSPAnyPerm != 0 {
		return refuse("repl-hard: DamageDone prevention ValidTarget %q", vt)
	}
	f, err := lc.filter(strings.Join(perm, ","))
	if err != nil {
		return err
	}
	if f.Players != 0 || len(f.Clauses) == 0 {
		return refuse("repl-hard: DamageDone prevention ValidTarget %q", vt)
	}
	rl.F = f
	return nil
}

// staticPrevSource reads ValidSource$ into the source filter and
// ReplSPSrcInPlay (inZoneBattlefield on every alternative).
func (lc *lowerCtx) staticPrevSource(rp *params, rl *lRepl) error {
	vs, ok := rp.get("ValidSource")
	if !ok {
		return nil
	}
	if strings.TrimSpace(vs) == "Spell" {
		rl.Bits |= kop.ReplSPSrcSpell // a spell on the stack: the resolving entry is one, with no battlefield source
		return nil
	}
	in, err := srcInPlay(vs)
	if err != nil {
		return err
	}
	f, err := lc.filter(vs)
	if err != nil {
		return err
	}
	if f.Players != 0 || len(f.Clauses) == 0 {
		return refuse("repl-hard: DamageDone prevention ValidSource %q", vs)
	}
	rl.F2 = f
	if in {
		rl.Bits |= kop.ReplSPSrcInPlay
	}
	return nil
}

// staticPrevGates reads IsCombat$, PlayerTurn$, Optional$ and the card's
// CheckSVar$ / IsPresent$ gate (a lowered Repl.Cond).
func (lc *lowerCtx) staticPrevGates(rp *params, rl *lRepl) error {
	if v, ok := rp.get("IsCombat"); ok {
		switch strings.TrimSpace(v) {
		case "True":
			rl.Bits |= kop.ReplSPCombat
		case "False":
			rl.Bits |= kop.ReplSPNonCombat
		default:
			return refuse("repl-hard: DamageDone prevention IsCombat %q", v)
		}
	}
	if v, ok := rp.get("PlayerTurn"); ok {
		if strings.TrimSpace(v) != "True" {
			return refuse("repl-hard: DamageDone prevention PlayerTurn %q", v)
		}
		rl.Bits |= kop.ReplSPYourTurn
	}
	if v, ok := rp.get("Optional"); ok {
		if strings.TrimSpace(v) != "True" || rp.str("OptionalDecider") != "You" {
			return refuse("repl-hard: DamageDone prevention Optional %q", v)
		}
		rp.consume("OptionalDecider")
		rl.Bits |= kop.ReplSPOptional
	}
	if cs, ok := rp.get("CheckSVar"); ok {
		v, err := lc.value(cs)
		if err != nil {
			return err
		}
		c := lCond{Kind: cValue, A: v, Op: opGE, B: cst(1)}
		if cmp, ok := rp.get("SVarCompare"); ok {
			op, b, err := lc.cmpSpec(cmp)
			if err != nil {
				return err
			}
			c.Op, c.B = op, b
		}
		rl.Cond = andCond(rl.Cond, c)
	}
	if ip, ok := rp.get("IsPresent"); ok {
		f, err := lc.filter(ip)
		if err != nil {
			return err
		}
		c := lCond{Kind: cPresent, F: f, Op: opGE, A: cst(1)}
		if pc, ok := rp.get("PresentCompare"); ok {
			op, v, err := lc.cmpSpec(pc)
			if err != nil {
				return err
			}
			c.Op, c.A = op, v
		}
		rl.Cond = andCond(rl.Cond, c)
	}
	return nil
}

// staticPrevAmount reads a ReplaceDamage body's Amount$: a literal, a count
// (Count$Valid Cleric.YouCtrl), ReplaceCount$DamageAmount/Minus.N (all but N)
// or Rock Hydra's SVar$NumDmg/LimitMax.Y (the damage up to the host's +1/+1
// counters; gorge never runs the body's counter removal).
func (lc *lowerCtx) staticPrevAmount(w *cards.SA, rl *lRepl) error {
	bp := newParams(w.Params)
	raw := bp.str("Amount")
	body := raw
	if sv, ok := lc.face.SVars[raw]; ok {
		body = strings.TrimSpace(sv)
	}
	hydra := raw != "" && body == "SVar$NumDmg/LimitMax.Y" && lc.hydraBody(w)
	bp.consume("Amount")
	if hydra {
		bp.consume("SubAbility")
	}
	if left := bp.unread(); len(left) > 0 || w.Sub != nil && !hydra {
		return refuse("repl-hard: DamageDone ReplaceDamage body params %v", left)
	}
	switch rest, ok := strings.CutPrefix(body, "ReplaceCount$DamageAmount/Minus."); {
	case hydra:
		rl.Bits |= kop.ReplSPLimitCtr
		rl.Ctr = "P1P1"
	case ok:
		n, err := strconv.Atoi(rest)
		if err != nil || n < 0 {
			return refuse("repl-hard: DamageDone ReplaceDamage Amount$ %q", raw)
		}
		rl.Bits |= kop.ReplSPAllBut
		rl.N = cst(n)
	default:
		v, err := lc.value(body)
		if err != nil {
			return err
		}
		rl.N = v
		if !v.isConst() {
			rl.Bits |= kop.ReplSPAmtV
		} else if v.N <= 0 || v.N > 1<<14 {
			return refuse("repl-hard: DamageDone ReplaceDamage Amount$ %q", raw)
		}
	}
	return nil
}

// hydraBody: Rock Hydra's ReplaceDamage body (SubAbility$ RemoveCounter of
// NumDmg +1/+1 counters from itself, NumDmg the damage, Y its counters).
func (lc *lowerCtx) hydraBody(w *cards.SA) bool {
	sv := lc.face.SVars
	sub := w.Sub
	if sub == nil || sub.API != "RemoveCounter" || sub.Sub != nil || strings.TrimSpace(sv["NumDmg"]) != "ReplaceCount$DamageAmount" ||
		strings.TrimSpace(sv["Y"]) != "Count$CardCounters.P1P1" {
		return false
	}
	sp := newParams(sub.Params)
	sp.consume("Defined", "CounterType", "CounterNum")
	return sp.str("Defined") == "Self" && sp.str("CounterType") == "P1P1" && sp.str("CounterNum") == "NumDmg" && len(sp.unread()) == 0
}

// staticPrevRepl fills rp for a static prevention (cardBuilder.repl).
func (cb *cardBuilder) staticPrevRepl(rp *kop.Repl, r lRepl) (err error) {
	rp.C = int16(r.Bits)
	if r.F != nil {
		if rp.Filter, err = cb.filter(r.F, kop.ZoneBattlefield); err != nil {
			return err
		}
	}
	if r.F2 != nil {
		if rp.Filter2, err = cb.filter(r.F2, kop.ZoneBattlefield); err != nil {
			return err
		}
	}
	if r.Kind == "PreventDamageFilt" {
		rp.Kind = kop.ReplPreventDamageFilt
		if r.Bits&kop.ReplSPDmgLE != 0 {
			rp.A = int16(r.N.N)
		}
		return nil
	}
	rp.Kind = kop.ReplPreventDamageN
	switch {
	case r.Bits&kop.ReplSPLimitCtr != 0:
		k, err := counterKind(r.Ctr)
		if err != nil {
			return err
		}
		rp.A = int16(k)
	case r.Bits&kop.ReplSPAmtV != 0:
		rp.V, err = cb.value(r.N)
	default:
		rp.A = int16(r.N.N)
	}
	return err
}

// dumpRepl writes one replacement of the LIR dump: the line every kind has,
// and the second line a static prevention adds (its source filter, bits and
// amount).
func dumpRepl(w io.Writer, r lRepl) {
	fmt.Fprintf(w, "  repl %s F=%s cond=%s\n", r.Kind, r.F, r.Cond)
	if r.Kind == "PreventDamageFilt" || r.Kind == "PreventDamageN" {
		fmt.Fprintf(w, "    source=%s bits=%#x n=%v ctr=%q\n", r.F2, r.Bits, r.N, r.Ctr)
	}
	if r.Kind == "GraveToLibrary" {
		fmt.Fprintf(w, "    lib bottom=%v shuffle=%v fromBF=%v\n", r.LibBottom, r.LibShuffle, r.LibFromBattlefield)
	}
}

// prevAnchored reports whether one of the printed prevention's two filter
// specs keeps its match off the battlefield of gorge's zone-any replacement
// scan: every alternative of the spec names the printed permanent itself
// (Self — a card in hand, in a graveyard or in a library is no damage event's
// recipient or source) or a permanent the printed permanent itself is
// attached to (EnchantedBy / EquippedBy — no attachment exists off the
// battlefield). An absent spec matches every candidate, a player scope
// matches every player and every plain type or controller word matches a
// battlefield object, so none of those anchors.
func prevAnchored(vt, vs string) bool {
	return prevAnchoredSpec(vt) || prevAnchoredSpec(vs)
}

func prevAnchoredSpec(spec string) bool {
	if strings.TrimSpace(spec) == "" {
		return false
	}
	for _, alt := range strings.Split(spec, ",") {
		anch := false
		for _, q := range strings.Split(alt, "+") {
			for _, w := range strings.Split(q, ".") {
				switch strings.TrimSpace(w) {
				case "Self", "EnchantedBy", "EquippedBy":
					anch = true
				}
			}
		}
		if !anch {
			return false
		}
	}
	return true
}
