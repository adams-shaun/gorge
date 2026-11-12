package kgen

import (
	"strconv"
	"strings"
)

// cantBeCast lowers S:Mode$ CantBeCast (gorge castRestricted): spells
// matching ValidCard$ (absent: every spell) can't be cast by the casters
// Caster$ names (You / Opponent / Player; absent: anyone), under the
// static's conditions (Condition$ PlayerTurn: during its controller's
// turn). The kernel withholds the cast offers (hand, flashback, play
// permissions, free casts). NumLimitEachTurn$ N (a positive literal, an
// unfiltered ValidCard$) binds once the caster has cast N spells this turn
// OnlySorcerySpeed$ True (Teferi, Time Raveler) binds only outside the
// caster's sorcery-speed window (flag SorcerySpeed, kop CantCastSorcery).
// Refused: a filtered NumLimitEachTurn$, Origin$, Phases$, a mana-value filter (CR 601.3a's X
// lookahead). ValidCard$ Card.Self | EffectZone$ All is the card's own "you
// can't cast this spell": it binds only the card itself,
// in any zone, with no filter; Caster$ Player.Active (the active player) is
// read on that shape only. ValidCard$ Card.NamedCard | Caster$ Opponent over
// the source's as-enters chosen name (Alhammarret, High Arbiter) binds only
// the spell with that name (flag NamedCard, kop CantCastNamed).
func (lc *lowerCtx) cantBeCast(sp *params, st *lStatic) error {
	vc := sp.str("ValidCard")
	self := vc == "Card.Self"
	if vc == "Card.NamedCard" {
		// the source's as-enters chosen name (Alhammarret, High Arbiter): the
		// kernel compares the cast card with the chosen byte (kop CantCastNamed)
		if !entersNamingCard(lc.face) {
			return refuse("CantBeCast Card.NamedCard beside no as-enters NameCard")
		}
		// only the opponents-can't-cast shape is lockstep-proven
		// (alhammarret_test.go); the all-casters ones (Meddling Mage) stay refused
		if c := sp.str("Caster"); c != "Opponent" && c != "Player.Opponent" {
			return refuse("CantBeCast Card.NamedCard Caster %q", c)
		}
		f, err := lc.filter("Card")
		if err != nil {
			return err
		}
		st.F = f
		st.Flags = append(st.Flags, "NamedCard")
	} else if vc != "" {
		if strings.Contains(vc, "cmc") || strings.Contains(vc, "Self") && !self {
			return refuse("CantBeCast ValidCard %q", vc)
		}
		if self {
			if sp.str("EffectZone") != "All" {
				return refuse("CantBeCast Card.Self without EffectZone$ All")
			}
			sp.consume("EffectZone")
			st.Zone = zAny
		} else {
			f, err := lc.cantBeCastFilter(vc)
			if err != nil {
				return err
			}
			st.F = f
		}
	}
	switch c := sp.str("Caster"); c {
	case "", "Player":
	case "You":
		st.Flags = append(st.Flags, "ActivatorYou")
	case "Opponent", "Player.Opponent":
		st.Flags = append(st.Flags, "ActivatorOpp")
	case "Player.Active":
		if !self {
			return refuse("CantBeCast Caster %q", c)
		}
		st.Flags = append(st.Flags, "CasterActive")
	default:
		return refuse("CantBeCast Caster %q", c)
	}
	if sp.is("OnlySorcerySpeed", "True") {
		// gorge castSorceryTimingExempt: the prohibition binds only while the
		// caster could not cast a sorcery (Teferi, Time Raveler: "each
		// opponent can cast spells only any time they could cast a sorcery");
		// the kernel reads it with its own sorcerySpeed (kop CantCastSorcery)
		if self {
			return refuse("CantBeCast OnlySorcerySpeed$ on a Card.Self static")
		}
		st.Flags = append(st.Flags, "SorcerySpeed")
		sp.consume("OnlySorcerySpeed")
	}
	if raw, ok := sp.get("NumLimitEachTurn"); ok {
		// gorge castLimitBinds (rules/statics_cast.go:463-484): binds once the
		// caster already cast N spells this turn. Exact only for an
		// unfiltered ValidCard$: the kernel's count is histSpells.
		raw = strings.TrimSpace(raw)
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 255 || strconv.Itoa(n) != raw {
			return refuse("CantBeCast NumLimitEachTurn %q", raw)
		}
		if vc != "" && vc != "Card" {
			return refuse("CantBeCast NumLimitEachTurn with ValidCard %q", vc)
		}
		st.Amount = cst(n)
	}
	sp.consume("Secondary")
	return nil
}

// cantBeCastFilter lowers a CantBeCast ValidCard$. The cast-provenance words
// (Drannith Magistrate's, Avatar's Wrath's Card.!wasCastFromTheirHand) are
// admitted here only: the kernel reads them at the cast offer from the
// candidate's current zone (matchSpellCardOffer), which is exact for the
// hand (1), exile (3) and cast-not-from-hand (4) families. The ones that
// need a stack entry (ByYou, MayPlaySource, Mayhem, bare wasCast) refuse.
func (lc *lowerCtx) cantBeCastFilter(vc string) (*lFilter, error) {
	save, saveC := lc.spellProvOK, lc.cantCastProv
	lc.spellProvOK, lc.cantCastProv = true, true
	defer func() { lc.spellProvOK, lc.cantCastProv = save, saveC }()
	f, err := lc.filter(vc)
	if err != nil {
		return nil, err
	}
	for _, c := range f.Clauses {
		if c.CmpStat != cmpWasCastSpell {
			continue
		}
		if n := c.CmpN & 15; n != spellHand && n != spellExile && n != spellNotHand {
			return nil, refuse("CantBeCast ValidCard %q: cast provenance %s outside the origin families", vc, vc)
		}
	}
	return f, nil
}
