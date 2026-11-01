package kgen

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// two ReduceCost shapes on a spell's own static.
//
//   - ValidSpell$ Spell.Bargain on a self static (Hamlet Glutton, Ice Out,
//     Johann's Stopgap): gorge spellConstraintMatchesBargain reads the cast
//     MODE, so the discount prices only the bargained cast. Flag "Bargained".
//   - Color$ (Brush Off's "Color$ 1 U | Amount$ 1"): gorge reads each token
//     reduced by Amount$ (statics_costmods.go): a numeric token is that many
//     generic pips times Amount$, a letter in WUBRGC is Amount$ pips of that
//     colour. Flag "ReduceColor"; st.Amount is the generic part and
//     lStatic.RedCol the coloured counts.

func init() {
	mask[kop.StaticFlags]("kop.StBargained", kop.StBargained, "kop.StReduceColor", kop.StReduceColor)
}

// reduceSpellShape reads Color$ and a self ValidSpell$ Spell.Bargain off a
// ReduceCost static whose Type$ Spell, ValidCard$, Amount$ and EffectZone$
// were already read into st.
func (lc *lowerCtx) reduceSpellShape(sp *params, st *lStatic) error {
	// a non-self ValidSpell$ is read elsewhere (OnlyFirstSpell$) or stays unread
	if vs := strings.TrimSpace(sp.m["ValidSpell"]); vs != "" && sp.str("ValidCard") == "Card.Self" {
		if vs != "Spell.Bargain" || st.Zone != zAny {
			return refuse("ReduceCost ValidSpell %q (only a self Spell.Bargain static with EffectZone$ All)", vs)
		}
		sp.consume("ValidSpell")
		st.Flags = append(st.Flags, "Bargained")
	}
	if sp.m["ValidSpell"] != "" && !sp.used["ValidSpell"] {
		return nil // a ValidSpell$ nothing reads (Head of the Class): the unread-params refusal names it
	}
	col, ok := sp.get("Color")
	if !ok {
		return nil
	}
	if !st.Amount.isConst() || st.Amount.N < 1 {
		return refuse("ReduceCost Color$ with a non-literal Amount$")
	}
	if _, ig := sp.get("IgnoreGeneric"); ig {
		return refuse("ReduceCost Color$ with IgnoreGeneric$")
	}
	if _, mm := sp.get("MinMana"); mm {
		return refuse("ReduceCost Color$ with MinMana$")
	}
	amt, generic := st.Amount.N, 0
	var cnt [kop.NumManaKinds]int
	for _, tok := range strings.Fields(col) {
		if n, err := strconv.Atoi(tok); err == nil && n >= 0 && tok[0] != '+' && tok[0] != '-' {
			generic += n * amt
			continue
		}
		i := strings.Index("WUBRGC", tok)
		if len(tok) != 1 || i < 0 {
			return refuse("ReduceCost Color$ token %q", tok)
		}
		cnt[i] += amt
	}
	if generic > 255 {
		return refuse("ReduceCost Color$ generic part %d", generic)
	}
	for i, n := range cnt {
		if n > 3 {
			return refuse("ReduceCost Color$ colour count %d", n)
		}
		st.RedCol |= uint16(n) << (kop.ReduceColorBits * i)
	}
	st.Amount = cst(generic)
	st.Flags = append(st.Flags, "ReduceColor")
	return nil
}
