package kgen

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// cantAttack lowers S:Mode$ CantAttack (gorge combat.AttackBlocked's face
// static walk) to kop.StatCantAttack. The kernel is two-player, so the
// defender-dependent parameters map exactly only in these shapes:
//   - Target$ absent: every defender.
//   - Target$ You,Planeswalker.YouCtrl: every attack target the creature
//     could have belongs to the static's controller exactly when the
//     creature is an opponent's, so the restriction is ValidCard$ +OppCtrl.
//   - UnlessDefender$ [!]controls<spec> on ValidCard$ Card.Self: the
//     defender is the opponent of the static's controller, so the gate is
//     IsPresent$ <spec>+OppCtrl (EQ0 for the positive form, GE1 negated).
//
// Any other Target$ / UnlessDefender$ refuses.
func (lc *lowerCtx) cantAttack(sp *params, st *lStatic) error {
	vc := sp.str("ValidCard")
	if vc == "" {
		return refuse("CantAttack without ValidCard$")
	}
	oppOnly := false
	if t, ok := sp.get("Target"); ok {
		if strings.ReplaceAll(t, " ", "") != "You,Planeswalker.YouCtrl" {
			return refuse("CantAttack Target$ %q", t)
		}
		oppOnly = true
		lc.twoSeat("CantAttack Target$ you and your planeswalkers read as OppCtrl")
	}
	if ud, ok := sp.get("UnlessDefender"); ok {
		if vc != "Card.Self" {
			return refuse("UnlessDefender$ on ValidCard$ %q", vc)
		}
		lc.twoSeat("UnlessDefender$ read as the static controller's opponent")
		ud = strings.TrimSpace(ud)
		neg := strings.HasPrefix(ud, "!")
		ud = strings.TrimPrefix(ud, "!")
		spec, ok := strings.CutPrefix(ud, "controls")
		if !ok || spec == "" || strings.ContainsAny(spec, "_") || strings.Contains(spec, "Ctrl") {
			return refuse("UnlessDefender$ %q", ud)
		}
		f, err := lc.filter(addFilterProp(spec, "OppCtrl"))
		if err != nil {
			return err
		}
		cmp := "EQ0"
		if neg {
			cmp = "GE1"
		}
		op, v, err := lc.cmpSpec(cmp)
		if err != nil {
			return err
		}
		st.Cond = andCond(st.Cond, lCond{Kind: cPresent, F: f, Op: op, A: v})
	}
	f, err := lc.filter(vc)
	if err != nil {
		return err
	}
	if oppOnly {
		// every clause also requires an opponent's creature, set on the
		// lowered filter (not as a textual suffix, which a spaced referent
		// such as EnchantedBy <Aura.YouCtrl> makes ambiguous)
		if f.Players != 0 {
			return refuse("CantAttack Target$ with a player filter %q", vc)
		}
		for i := range f.Clauses {
			if c := &f.Clauses[i]; c.Ctrl != whoAny && c.Ctrl != whoOpp {
				return refuse("CantAttack Target$ with a controller filter %q", vc)
			}
			f.Clauses[i].Ctrl = whoOpp
		}
	}
	st.F = f
	sp.consume("Secondary")
	return nil
}

// addFilterProp conjoins prop with every comma alternative of a Valid
// expression ("Creature" -> "Creature.prop", "Creature.X" -> "Creature.X+prop").
func addFilterProp(expr, prop string) string {
	parts := strings.Split(expr, ",")
	for i, p := range parts {
		p = strings.TrimSpace(p)
		if strings.Contains(p, ".") {
			parts[i] = p + "+" + prop
		} else {
			parts[i] = p + "." + prop
		}
	}
	return strings.Join(parts, ",")
}

// evasionBit maps a printed blocking-restriction keyword to its
// CardDef.Evasion bit.
func evasionBit(head string) uint8 {
	if head == "Horsemanship" {
		return kop.EvHorsemanship
	}
	if head == "Skulk" {
		return kop.EvSkulk
	}
	return kop.EvIntimidate
}

// minMaxBlockerStatic lowers S:Mode$ MinMaxBlocker | ValidCard$ Creature.Self
// | Min$ N (CR 509.1b, Troll of Khazad-dum) to CardDef
// .MinBlockers: gorge MinMaxBlockerBounds reads the literal Min$ against the
// attacker. A Max$, Min$ All, another scope or a gate is refused.
func (lc *lowerCtx) minMaxBlockerStatic(s cards.Static) error {
	ps := newParams(s.Params)
	ps.consume("Mode", "Description")
	if v := ps.str("ValidCard"); v != "Creature.Self" && v != "Card.Self" {
		return refuse("MinMaxBlocker ValidCard %q", v)
	}
	ps.consume("ValidCard")
	n, err := strconv.Atoi(ps.str("Min"))
	if err != nil || n < 2 || n > 8 {
		return refuse("MinMaxBlocker Min %q", ps.str("Min"))
	}
	ps.consume("Min")
	if left := ps.unread(); len(left) > 0 {
		return refuse("MinMaxBlocker unread params %v", left)
	}
	lc.out.MinBlockers = n
	return nil
}
