package rules

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/state"
)

// AvailableMana is the engine's answer to the view's "what could this seat
// tap for right now" question (view.Chars). It is the mana the seat could
// produce this very moment by activating the free-to-tap mana abilities of
// untapped permanents it controls -- the "free mana one gains by tapping
// lands or other effects" the seat-box line 3 advertises, as distinct from
// the floating Pool (poolView), which is a between-decisions snapshot that
// CR 500.4 empties at every step's end.
//
// It is public information: it is derived entirely from the battlefield
// (a public zone), never from a hand, library or graveyard, so the same value
// is projected for every seat and for every visibility (seat, public,
// omniscient).
//
// The eligibility gate mirrors the engine's own tap-for-mana offer
// (rules/legal.go legalActions): an untapped permanent with at least one
// unrestrictable mana ability. Beyond that gate this is deliberately
// narrower, in three honest ways the projection must not paper over:
//
//   - Only a mana ability whose activation cost is a bare tap (or free) is
//     counted. manaFreeCost rejects any cost that also sacrifices, removes
//     a counter, pays life or mana -- activating such an ability is not
//     "free mana by tapping". A permanent that is a legitimate tap source
//     but whose only mana ability carries a paid cost contributes nothing.
//   - Ability-restricted mana abilities (CantBeActivated) are skipped, the
//     same restriction legalActions' offer gate consults, so a source a
//     static currently forbids is not advertised as available.
//   - A permanent with several remaining free mana abilities contributes
//     nothing. It can tap for one of those abilities, not their sum, and
//     state.Mana cannot represent that colour choice without falsely calling
//     it colourless or promising both colours. This keeps AvailableMana a
//     conservative fixed-colour lower bound rather than overstating payment.
//   - An Indeterminate Amount$ ("X", "Y", a Count$ expression) yields no
//     amount the seat is guaranteed to receive, so it contributes nothing.
//     (mirrors cards.ManaiProduction, which resolves it to zero rather than
//     claiming a count the pool is never promised.)
//
// A summoning-sick creature is not separately excluded: the engine's own
// tap-for-mana offer gate does not exclude one either (it checks only
// Tapped), and AvailableMana is intentionally consistent with the offer set
// the seat actually acts through rather than silently diverging from it.
// Like `Cards`' production, a Produced$ of "Any"/"Combo Any" resolves to the
// colourless the executor emits (effects/misc.go effMana) and carried into
// state.Mana's colourless slot -- the colour the engine does not model -- and
// CardView.Produces remains a per-face capability summary (it can list the
// alternatives a card has), while AvailableMana is deliberately stricter: it
// reports only the fixed mana that can be added together right now.
func (e *Engine) AvailableMana(p state.PlayerID) state.Mana {
	var out state.Mana
	for _, id := range e.G.Zone(state.ZBattlefield, p) {
		o := e.G.Obj(id)
		if o == nil || o.Tapped {
			continue
		}
		f := o.Face()
		if f == nil {
			continue
		}
		var free []*cards.SA
		for _, ma := range e.availableManaAbilities(p, id) {
			if manaFreeCost(ParseCost(ma.Params["Cost"])) {
				free = append(free, ma)
			}
		}
		// Tapping this permanent selects one ability. No state.Mana vector can
		// say "one U or one R" without asserting a colour that is not fixed,
		// so omit a multi-choice source from this conservative aggregate.
		if len(free) == 1 {
			addAvailable(&out, free[0])
		}
	}
	return out
}

// manaFreeCost reports whether a mana ability's activation cost is a bare
// tap (or empty -- an ability that produces mana for nothing): Tap may be
// true, but no Sac, no SubCounter, no generic/coloured mana, no variable X.
// A cost like "T, Sac <1/CARDNAME>", "T, PayLife<1>" (which ParseCost folds
// to a generic), "2 T" or "G T" is not free and must not be counted as
// available-by-tapping.
func manaFreeCost(c Cost) bool {
	return len(c.Sac) == 0 && len(c.SubCounter) == 0 &&
		c.Generic == 0 && c.Colored == (state.Mana{}) && c.X == 0
}

// addAvailable folds one free-to-tap mana ability into an available-mana
// accumulator, using exactly the rune rules cards.ManaiProduction.add uses
// so the aggregate agrees with the per-face projection for the pure-tap case:
// blank / "Any" / "Combo Any" Produced$ becomes one colourless (the executor's
// effMana resolution), every other brace/space-stripped rune adds its WUBRG
// colour (or colourless for an unrecognised rune). The amount comes from
// Amount$ with the executor's default of 1 and the T14-f negative clamp; an
// Indeterminate amount ("X", "Y", a Count$, "Sacrificed$...") resolves to
// zero, contributing nothing.
func addAvailable(m *state.Mana, ma *cards.SA) {
	raw := strings.TrimSpace(ma.Params["Produced"])
	if raw == "" || raw == "Any" || raw == "Combo Any" {
		raw = "C"
	}
	s := strings.NewReplacer("{", "", "}", "", " ", "").Replace(raw)
	amt := availableAmount(ma)
	for _, r := range s {
		m[state.ManaIndex(byte(r))] += amt
	}
}

// availableAmount is cards' manaAbilityAmount, re-derived here because that
// helper is unexported and this package needs a per-ability amount to fold a
// single ability into an accumulator. A blank Amount is the executor's own
// default of 1; a literal integer is used directly (negative clamped to 0);
// anything else is a value the projection cannot statically price, so it
// returns 0 -- never a count the pool is not guaranteed to receive.
func availableAmount(ma *cards.SA) int32 {
	raw := strings.TrimSpace(ma.Params["Amount"])
	if raw == "" {
		return 1
	}
	if v, err := strconv.Atoi(raw); err == nil {
		if v < 0 {
			return 0
		}
		return int32(v)
	}
	return 0
}
