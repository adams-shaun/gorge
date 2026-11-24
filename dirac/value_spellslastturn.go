package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// ExSpellsCastLastTurn is the Innistrad werewolf's intervening-if (CR 603.4;
// gorge trigmatch phaseTriggerMatches over SpellsCastLastTurn): the front
// face transforms at an upkeep "if no spells were cast last turn" and the
// back face "if a player cast two or more spells last turn". Last turn is the
// turn before the current one, whoever's it was, and every player's casts
// count (not only the active player's, as CR 731.2's day/night count does).
//
// The kgen conditions are CondValue over the expression: N 0 is whether any
// player cast a spell (0 or 1; == 0), N 1 the most one player cast, at most 3
// (>= 2). beginTurn rolls the history bank's spell tallies into genState's
// dayNight byte (bits 4-6, above the designation and CR 731.2's count) before
// the bank resets (prep.anyLastSpells, set where a table reads the
// expression).

const (
	lsAny  uint8 = 1 << 4 // some player cast a spell last turn
	lsMost       = 5      // shift of the most one player cast last turn (2 bits)
	lsMask uint8 = 0x7 << 4
)

// spellsLastTurnRoll keeps the outgoing turn's spell tallies.
func (s *State) spellsLastTurnRoll() {
	var v, most uint8
	for p := range s.g.hist {
		n := s.g.hist[p][histSpells]
		if n != 0 {
			v = lsAny
		}
		most = max(most, n)
	}
	s.g.dayNight = s.g.dayNight&^lsMask | v | min(most, 3)<<lsMost
}

// spellsCastLastTurn reads the tally: n == 1 the most one player cast
// (saturating at 3), else whether any player cast a spell (0 or 1).
func (s *State) spellsCastLastTurn(n int16) int {
	if n == 1 {
		return int(s.g.dayNight >> lsMost & 3)
	}
	return int(s.g.dayNight >> 4 & 1)
}

// noteLastSpells sets prep.anyLastSpells when x is the expression and reports
// it: the tallies are the history bank's spell counts, so the reader also
// turns genPrep.anyHist on (prepareGen).
func (p *prep) noteLastSpells(x *kop.Expr) bool {
	if x.Kind != kop.ExSpellsCastLastTurn {
		return false
	}
	p.anyLastSpells = true
	return true
}
