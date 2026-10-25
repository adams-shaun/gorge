package rules

import (
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// oracle_run_faces.go binds an oracle scenario's step ref that names an
// ALTERNATE spell face of a physical card. A scenario deals the physical
// card under its parent (front) name -- the name both engines deal it by --
// and a cast step may name the face it casts ("p0:Stomp" for a Bonecrusher
// Giant in hand). The Adventure (CR 715) and Omen (CR 720) spell faces are
// bound: each is cast from the hand through the engine's own "adventure_alt"
// / "omen_alt" offer (engine/rules/adventure.go), so the runner chooses
// exactly the offer a player would. Setup names stay physical-card names; a
// setup that names an Adventure or Omen face is still "not dealt". A Room's
// other door (CR 709.5) binds the same way and is cast through the engine's
// "room_alt" offer.

// oracleAltFaceMode returns the cast mode that casts name as an alternate
// spell face of o's card, or "" when name is not such a face of o (or is
// the face o already shows).
func oracleAltFaceMode(o *state.Object, name string) string {
	if o == nil || o.Face() == nil || o.Face().Name == name {
		return ""
	}
	if f := adventureSpellFace(o); f != nil && f.Name == name {
		return "adventure_alt"
	}
	if f := omenSpellFace(o); f != nil && f.Name == name {
		return "omen_alt"
	}
	if f := roomAlternateCastFace(o); f != nil && f.Name == name {
		return "room_alt"
	}
	return ""
}

// oracleCastWantMode is the cast option Mode a cast step on o asks for: "kicked" for
// a kicked cast, else its explicit cast_mode, else the alternate face's mode
// when the step's ref names one of o's alternate spell faces, else "" (the
// ordinary cast).
func oracleCastWantMode(st oracleStep, o *state.Object) string {
	if st.Kicked {
		return "kicked"
	}
	if st.CastMode != "" {
		return st.CastMode
	}
	_, name, token, _, _, err := splitRef(st.Card)
	if err != nil || token {
		return ""
	}
	return oracleAltFaceMode(o, name)
}

// oracleAltCostMode is the cast_mode that selects the first
// AlternativeCost-static cast option (Leyline of Mutation's "You may pay
// {W}{U}{B}{R}{G} rather than pay the mana cost"): the engine offers it as its
// own "cast" option with no Mode and AltCostIndex 1 (engine/rules/legal_walk_hand.go),
// so the Mode match alone can never tell it from the plain cast. A cast
// through a may-play permission from a non-hand zone offers its first
// alternative (Warped Space's "pay {0} rather than pay the mana cost for a
// spell you cast from exile") the same way under Mode "mayplay"
// (engine/rules/legal_walk_alt.go), so the mode names that option too.
const oracleAltCostMode = "alternative"

// oracleCastModeMatches reports whether a cast option is the one a step's
// wanted mode names: the alternative-cost option for oracleAltCostMode, the
// option whose Mode equals want for every other mode.
func oracleCastModeMatches(o decision.Option, want string) bool {
	if want == oracleAltCostMode {
		return (o.Mode == "" || o.Mode == "mayplay") && o.AltCostIndex == 1
	}
	return o.Mode == want
}
