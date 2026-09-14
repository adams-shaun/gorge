package rules

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/state"
)

// potentialUnbounded is the per-unit amount one indeterminate mana source
// contributes to the hypothetical potential pool. A source whose Amount$ is
// an X, a Y or a Count$ expression (an Urza land, Gaea's Cradle, Priest of
// Titania) could produce any amount this turn; the stop decision the
// projection serves must never lose an action to a source the engine cannot
// statically price, so such a source is priced unbounded rather than at zero.
// 99 is far past any real cost a game can present, while staying small enough
// that sums of a few sources cannot overflow int32.
const potentialUnbounded int32 = 99

// PotentialMana is the hypothetical pool the potential-action walk prices
// against: the seat's floating pool PLUS what every untapped mana source it
// controls could produce. It is deliberately an OVER-bound, the direction the
// auto-pass doctrine names safe (a wrongly withheld pass costs one idle stop;
// a wrongly eaten window loses the player's action):
//
//   - a source with a single fixed-colour production (a Plains) adds its
//     literal Amount (default 1);
//   - a source with several fixed productions (a Volcanic Island's two
//     abilities, a "Produced$ GW" line) adds every face -- the seat can only
//     tap for one at a time, but the bound must not lose a colour;
//   - a source with an alternative or variable production (Produced$ Any /
//     Combo Any / Chosen / an unknown token) or an indeterminate Amount$ (an
//     Urza land, Gaea's Cradle, Elvish Archdruid) contributes
//     potentialUnbounded to EVERY unit -- the one shape a fixed vector cannot
//     represent honestly, and the shape the old client-side bound priced at
//     zero (the Tron + Karn defect);
//   - a mana ability with a paid activation cost (Wasteland's
//     "T, Sac<1/CARDNAME>") still counts: the engine offers it as an activate
//     option once its cost is satisfiable, so the mana is genuinely reachable.
//
// Restriction-gated abilities (CantBeActivated) contribute nothing, the same
// gate the engine's tap-for-manability offer uses -- a source a static
// currently forbids is not advertised as producible. The aggregate is a pure
// read: no event is emitted and no state field is written; the pool lives
// only in this return value.
//
// Determinism: the walk is over zone order, never a map, so the aggregate is
// byte-stable run to run.
func (e *Engine) PotentialMana(p state.PlayerID) state.Mana {
	out := e.G.Players[p].Pool
	for _, id := range e.G.Zone(state.ZBattlefield, p) {
		o := e.G.Obj(id)
		if o == nil || o.Tapped {
			continue
		}
		if o.Face() == nil {
			continue
		}
		for _, ma := range e.availableManaAbilities(p, id) {
			addPotentialMana(&out, ma)
		}
	}
	return out
}

// addPotentialMana folds one mana ability into the potential accumulator. The
// amount and production parsing mirrors AvailableMana's addAvailable (the
// executor's own effMana resolutions: blank Produced$ is one colourless) with
// one divergence: anything the executor cannot statically price -- an
// alternative production or an indeterminate amount -- is UNBOUNDED here
// rather than zero, because this bound must never lose an action.
func addPotentialMana(m *state.Mana, ma *cards.SA) {
	amt, indeterminate := potentialAmount(ma)
	raw := strings.TrimSpace(ma.Params["Produced"])
	if indeterminate || producedOpen(raw) {
		for i := range m {
			m[i] += potentialUnbounded
		}
		return
	}
	if raw == "" {
		raw = "C"
	}
	s := strings.NewReplacer("{", "", "}", "", " ", "").Replace(raw)
	for _, r := range s {
		m[state.ManaIndex(byte(r))] += amt
	}
}

// producedOpen reports whether a Produced$ value names an ALTERNATIVE or
// unknown production the executor resolves at activation time rather than a
// fixed mana set: "Any"/"Combo Any" (any colour), "Chosen", or any token
// outside the WUBRGC faces. A fixed multi-face value ("GW") is not open --
// each face is folded additively as an over-bound.
func producedOpen(raw string) bool {
	switch raw {
	case "", "Any", "Combo Any", "Chosen":
		return true
	}
	s := strings.NewReplacer("{", "", "}", "", " ", "").Replace(raw)
	for _, r := range s {
		switch r {
		case 'W', 'U', 'B', 'R', 'G', 'C':
		default:
			return true
		}
	}
	return false
}

// potentialAmount is an ability's Amount$ as (literal, indeterminate): a
// blank Amount is the executor's own default of 1; a literal integer is used
// directly with the negative clamp the executor applies; anything else (an X,
// a Y, a Count$ expression, a Sacrificed$ reference) is indeterminate and
// prices unbounded upstream rather than at zero.
func potentialAmount(ma *cards.SA) (int32, bool) {
	raw := strings.TrimSpace(ma.Params["Amount"])
	if raw == "" {
		return 1, false
	}
	if v, err := strconv.Atoi(raw); err == nil {
		if v < 0 {
			return 0, false
		}
		return int32(v), false
	}
	return 0, true
}

// PotentialActions is the view's per-seat projection of every play the seat
// could still make after floating every mana its untapped sources could
// produce: the engine's own legal-offer walk (legalActionsPriced, the exact
// code that builds a priority decision's options) priced against
// PotentialMana. It carries only real plays -- "cast" (hand, command zone and
// flashback), "ability" (activated abilities, Equip among them) and
// "play_land" -- never the mana tap, pass or concede, which every priority
// window offers and which are never a play. The walk's own gates (timing,
// restrictions, targets, non-mana costs, live RaiseCost/ReduceCost, X at 0)
// are the engine's, so the projection cannot disagree with the engine the way
// a client-side re-derivation does.
//
// This is a pure read and never touches an event; callers project it ONLY for
// the viewer's own seat (view/view.go gates it on p.ID == viewer), because
// the walk reads that seat's hand, command zone and graveyard -- projecting
// another seat's would leak their hidden zones (CR 400.2).
//
// Determinism: the walk is zone-order, never a map range, so the list is
// byte-stable run to run.
func (e *Engine) PotentialActions(p state.PlayerID) []decision.PotentialAction {
	if e.G.Over {
		return nil
	}
	pool := e.PotentialMana(p)
	var out []decision.PotentialAction
	for _, o := range e.legalActionsPriced(p, &pool) {
		switch o.Kind {
		case "cast", "ability", "play_land":
			out = append(out, decision.PotentialAction{
				Kind: o.Kind, Obj: o.Obj, Ability: o.Ability, Mode: o.Mode, Label: o.Label,
			})
		}
	}
	return out
}
