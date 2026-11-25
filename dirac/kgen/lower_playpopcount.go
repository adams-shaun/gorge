package kgen

import (
	"strconv"
	"strings"
)

// playRemNotChosen is Plargg and Nassari's population: the remembered cards
// but the one the opponent chose (gorge DefinedSpec's Remembered with the
// !ChosenCard filter).
const playRemNotChosen = "Remembered.!ChosenCard"

// playUpToOK reports whether a Play's Amount$ (neither 1 nor All) lowers onto
// CastFree's UpTo bit: gorge effPlay's one ask with Max N, the picks begun in
// answer order (engine/effects/play.go). A literal N >= 2 over the remembered
// exile (Doom Reigns Supreme's "up to two spells") or over Plargg and
// Nassari's remembered-but-chosen set. A counted Amount$ (Talent of the
// Telepath's X, Chandra Ablaze's ChandraX) and a copy cast keep the refusal:
// gorge offers nothing at a count of 0, which the kernel's ask close does not
// read, and no carrier lowers whole to prove it.
func playUpToOK(ps *params, amount string, copyCard bool) bool {
	if copyCard {
		return false
	}
	def, valid, vzone := ps.m["Defined"], ps.m["Valid"], ps.m["ValidZone"]
	if n, err := strconv.Atoi(amount); err != nil || n < 2 {
		return false
	}
	return def == "" && vzone == "Exile" && strings.Contains(valid, "IsRemembered") ||
		def == playRemNotChosen && valid == "" && vzone == ""
}

// playDefinedPop lowers the Defined$ populations play does not read inline:
// Plargg and Nassari's Remembered.!ChosenCard, the remembered cards where they
// lie with the chosen card filtered out. Every other Defined$ refuses by name;
// Harness the Storm's Defined$ Targeted (the chain's TargetOnly graveyard
// card, a paid cast) stays refused: two of its triggers on one target diverge
// in lockstep after the first cast.
func (lc *lowerCtx) playDefinedPop(ps *params, def, saExpr string, emit func(lRef, int, string) error) error {
	_, hasValid := ps.m["Valid"]
	_, hasVZ := ps.m["ValidZone"]
	_, forget := ps.m["ForgetPlayed"]
	_, replace := ps.m["ReplaceGraveyard"]
	if def != playRemNotChosen || hasValid || hasVZ || forget || replace {
		return refuse("Play Defined$ %q", def)
	}
	expr, err := andFilterExpr("Card.nonChosenCard", saExpr)
	if err != nil {
		return err
	}
	return emit(lRef{Kind: rRemembered}, lc.playRememberedZone(), expr)
}
