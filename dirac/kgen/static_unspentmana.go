package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// S:Mode$ UnspentMana (ValidPlayer$ You, ManaType$ <colours>; Omnath, Locus of
// Mana, Upwelling; CR 500.4): the pool slots a live static names keep their
// unspent mana as each step and phase ends (gorge unspentManaKeep at the one
// ManaClear emit site). ValidPlayer$ absent protects every seat's (Upwelling's
// spelling, gorge unspentManaFold); ManaType$ absent protects every slot.
// The kernel reads it live at the boundary empties, gated by useUnspentMana
// (dirac/static_unspentmana.go), so the lowering carries the colour mask and
// the player scope only.
func init() {
	staticMode("UnspentMana", (*staticCtx).modeUnspentMana)
}

func (sc *staticCtx) modeUnspentMana() ([]lStatic, bool, error) {
	sp := sc.sp
	switch v := sp.str("ValidPlayer"); v {
	case "", "You":
		if v != "" {
			sc.st.Flags = append(sc.st.Flags, "You")
		}
	default:
		return nil, true, refuse("UnspentMana ValidPlayer %q", v)
	}
	sp.consume("ValidPlayer", "Secondary")
	mask := kop.ColW | kop.ColU | kop.ColB | kop.ColR | kop.ColG | kop.ColC
	if mt, ok := sp.get("ManaType"); ok {
		m, err := manaTypeMask(mt)
		if err != nil {
			return nil, true, err
		}
		mask = m
		sp.consume("ManaType")
	}
	sc.st.Amount = cst(int(mask))
	return nil, false, nil
}

// manaTypeMask is an UnspentMana static's ManaType$ as a colour mask: the
// colour-word list effects.ColorLetters parses (commas between entries,
// "Colorless" the {C} slot), fail-closed on a word it does not recognise
// (gorge leaves such a line out of the keep set, so the kernel must not
// lower it blanket).
func manaTypeMask(mt string) (kop.ColorMask, error) {
	var m kop.ColorMask
	for _, w := range strings.Split(mt, ",") {
		w = strings.TrimSpace(w)
		switch w {
		case "":
			return 0, refuse("UnspentMana ManaType %q", mt)
		case "Colorless":
			m |= kop.ColC
		default:
			letter, ok := colorWords[w]
			if !ok {
				return 0, refuse("UnspentMana ManaType %q", mt)
			}
			m |= kop.ColorMask(1) << strings.IndexByte("WUBRG", letter[0])
		}
	}
	return m, nil
}

// unspentManaAbility builds the UnspentMana static (kop.StatUnspentMana):
// Static.A the colour mask, StUnspentYou ValidPlayer$ You.
func (cb *cardBuilder) unspentManaAbility(s lStatic) (kop.Ability, []uint8, error) {
	ka := kop.Ability{Kind: kop.AbStatic}
	st := &ka.Static
	var err error
	if st.Cond2, err = cb.cond(s.Cond); err != nil {
		return ka, nil, err
	}
	st.OvGate = s.OvGate
	st.Kind = kop.StatUnspentMana
	if s.Amount.isConst() && s.Amount.N >= 0 && s.Amount.N <= 63 {
		st.A = int16(s.Amount.N)
	} else {
		return ka, nil, refuse("UnspentMana mask %v", s.Amount)
	}
	for _, fl := range s.Flags {
		if fl != "You" {
			return ka, nil, refuse("UnspentMana flag %q", fl)
		}
		st.StFlags |= kop.StUnspentYou
	}
	return ka, nil, nil
}
