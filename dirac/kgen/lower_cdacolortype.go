package kgen

import "strings"

// cdaColorType folds a characteristic-defining colour or type setter on the
// card itself into the card's printed characteristics (CR 604.3, 613.3: a CDA
// works in every zone and applies before every other layer-4/5 effect, which
// is exactly the place the printed record holds). The printed colours, types
// and subtypes are read in every zone by every filter, count and cast check,
// so no static and no kernel state is needed; a copy copies them (CR 707.2),
// as gorge's copiable CDA does.
//
//	Transguild Courier  SetColor$ All
//	Ghostfire           SetColor$ Colorless
//	Stonework Packbeast AddType$ Cleric & Rogue & Warrior & Wizard
//	Mistform Ultimus    AddAllCreatureTypes$ True (the Changeling record)
//
// folded reports whether any of the three keys was read. The exact Self,
// unconditional shape only; SetColor$ ChosenColor stays refused by name (the
// kernel keeps the chosen colour on no card record, and gorge's CDA carrier
// has no recorded choice, so it keeps its printed colours).
func (lc *lowerCtx) cdaColorType(sp *params, st *lStatic, aff string) (folded bool, err error) {
	col, hasCol := sp.get("SetColor")
	add, hasAdd := sp.get("AddType")
	_, hasAll := sp.get("AddAllCreatureTypes")
	if !hasCol && !hasAdd && !hasAll {
		return false, nil
	}
	if aff != "Card.Self" {
		return false, refuse("CDA colour/type Affected %q", aff)
	}
	if st.Cond.Kind != cNone {
		return false, refuse("CDA colour/type static with a condition")
	}
	if hasAll && !sp.is("AddAllCreatureTypes", "True") {
		return false, refuse("AddAllCreatureTypes %q", sp.str("AddAllCreatureTypes"))
	}
	var cols string
	if hasCol {
		var ok bool
		if cols, ok = colorWordSet(col); !ok {
			return false, refuse("CDA SetColor %q", col)
		}
	}
	var bits uint16
	var subs []string
	if hasAdd {
		for _, w := range strings.Split(add, "&") {
			w = strings.TrimSpace(w)
			if b, ok := typeAddBit(w); ok {
				bits |= b
				continue
			}
			if !lc.known(w) {
				return false, refuse("CDA AddType %q", w)
			}
			subs = append(subs, w)
		}
	}
	if lc.out.CDAFace == nil {
		lc.out.CDAFace = &lCDAFace{Types: lc.out.Types, Subtypes: append([]string(nil), lc.out.Subtypes...),
			Colors: lc.out.Colors, Changeling: lc.out.Changeling}
	}
	if hasCol {
		lc.out.Colors = cols
	}
	lc.out.Types |= bits
	for _, w := range subs {
		dup := false
		for _, have := range lc.out.Subtypes {
			dup = dup || have == w
		}
		if !dup {
			lc.out.Subtypes = append(lc.out.Subtypes, w)
		}
	}
	if hasAll {
		lc.out.Changeling = true
	}
	return true, nil
}

// lCDAFace is a card's own type line, colours and changeling bit as the
// record held them before cdaColorType's first fold. A phased-out permanent
// shows them (kop.CardDef.CDAFace): gorge applies no static of a phased-out
// permanent, its CDA included (rules/layers.go, matchesWithCharsPT).
type lCDAFace struct {
	Types      uint16
	Subtypes   []string
	Colors     string
	Changeling bool
}
