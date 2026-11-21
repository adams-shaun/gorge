package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// K:Spectacle:<cost> (CR 702.137a; dirac/kw_spectacle.go) and the `spectacle`
// filter word.
//
// gorge offers a "spectacle" cast from the hand for the keyword's cost while an
// opponent of the caster lost life this turn (rules altcast_modes.go
// altSpectacle) and stamps FlagAltCostPaid on it, which the `spectacle` filter
// predicate reads on a face carrying K:Spectacle (effects altCostKeywordPaid).
// The kernel carries the cost in CardDef.HasSpectacle/SpectacleCost and the
// paid provenance as the permanent's kicked mark (F2Kicked, the Bargain and
// Teamwork shape), so the word lowers only on a Spectacle carrier, where no
// kicker shares the mark. A GRANTED Spectacle stays refused (no kwMask bit).

func init() {
	kwHandlers["Spectacle"] = (*lowerCtx).lowerSpectacle
}

// lowerSpectacle lowers a printed K:Spectacle:<cost> to LIR.Spectacle. X, a ":"
// or "<" rider and any non-mana part are refused; hybrid pips are announced as
// for any cost.
func (lc *lowerCtx) lowerSpectacle(node, param string) {
	mc, err := parseManaCost(param)
	if err != nil || param == "" || strings.ContainsAny(param, ":<") || mc.X || mc.None {
		lc.refuseNode(node, "spectacle cost %q", param)
		return
	}
	if lc.out.Spectacle != nil {
		lc.refuseNode(node, "two Spectacle keywords")
		return
	}
	lc.out.Spectacle = &mc
}

// surgeSpectacleDef sets the two conditional hand alternative costs on d: surge
// (CR 702.117a: once another spell was cast this turn; dirac/kw_surge.go) and
// spectacle (CR 702.137a: once an opponent lost life this turn;
// dirac/kw_spectacle.go).
func surgeSpectacleDef(d *kop.CardDef, c *lCard) (err error) {
	if c.Surge != nil {
		d.HasSurge = true
		if d.SurgeCost, err = manaCostOf(*c.Surge); err != nil {
			return err
		}
	}
	if c.Spectacle != nil {
		d.HasSpectacle = true
		if d.SpectacleCost, err = manaCostOf(*c.Spectacle); err != nil {
			return err
		}
	}
	return nil
}

// refuseSpectacleCombos refuses spectacle beside any other cast cost route (the
// kernel's spectacle cast pays one plain alternative mana cost of a
// single-face card), on an {X} spell, on a multi-face card and on a modal
// spell, the refuseSurgeCombos shape. The spectacle provenance shares
// stackEntry.mode's impending bit, which a modal spell's chosen modes also
// occupy, so a card with both would be ambiguous.
func (lc *lowerCtx) refuseSpectacleCombos() {
	out := lc.out
	if out.Spectacle == nil {
		return
	}
	if out.Kicker != nil || out.Warp != nil || out.Plot != nil || out.Flashback != nil || out.SpellCost != nil || out.AltAddCost != nil || out.Convoke || out.Improvise ||
		out.Offspring != nil || out.Gift || out.MorphCost != nil || out.Sneak != nil || out.Bestow != nil || out.Impending != nil || out.Mayhem != nil ||
		out.Harmonize != nil || out.Madness != nil || out.Casualty != 0 || out.Bargain || out.OptCost != nil || out.Teamwork != 0 || out.Spree || out.WebSling != nil ||
		out.Evoke != nil || out.Dash != nil || out.Overload != nil || out.MayFlash != nil || out.Retrace || out.Delve || out.BeamMeUp != nil || out.Escape != nil ||
		out.Surge != nil || out.Cleave != nil {
		lc.refuseNode("K:Spectacle", "Spectacle beside an alternative or additional cost")
	}
	if out.Cost.X {
		lc.refuseNode("K:Spectacle", "Spectacle on an {X} spell")
	}
	if len(lc.card.Faces) != 1 || lc.card.AlternateMode != "" {
		lc.refuseNode("K:Spectacle", "Spectacle on a multi-face card")
	}
	for _, a := range out.Abilities {
		if a.Kind == akSpell && a.Modes != 0 {
			lc.refuseNode("K:Spectacle", "Spectacle on a modal spell")
			break
		}
	}
}

// provenanceWordRefusal names why a cast-provenance filter word (bargained,
// spectacle, Teamwork) cannot lower on this card, or "". Each reads the
// permanent's kicked mark (F2Kicked), so it lowers only on its own keyword's
// carrier: on any other card it would read the kicker or offspring mark.
func (lc *lowerCtx) provenanceWordRefusal(flags uint64) string {
	switch {
	case flags&fBargained != 0 && !lc.out.Bargain:
		return "bargained without Bargain" // CR 702.166
	case flags&fSpectacle != 0 && lc.out.Spectacle == nil:
		return "spectacle without Spectacle" // CR 702.137a
	case flags&fTeamwork != 0 && lc.out.Teamwork == 0:
		return "Teamwork without Teamwork" // CR 702.194b
	}
	return ""
}
