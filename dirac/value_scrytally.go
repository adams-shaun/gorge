package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The per-player scried / surveilled this-turn tallies:
// player.scry holds the scry instructions in bits 0-3 and the surveil
// instructions in bits 4-7 (one byte per player: it fills the struct's
// padding, where a [2]uint8 grew State by 8 B). A sixteenth instruction of
// one kind in a turn is a loud StallTables, never a silent wrap. gorge
// (rules/scry_surveil_this_turn.go) folds the log's events.Scry /
// events.Surveil records naming the player since the last TurnChange; those
// are emitted at exactly the moments TrigScry / TrigSurveil fire
// (trig_scrysurveil.go): a surveil as it starts, before the look (also
// on an empty library); a scry once its arrangement is answered, or at once
// when no ask is posed (an empty library). A scry/surveil of 0 is none
// genPrep.anyScryTally gates the writes: some table reads
// ExScriedThisTurn.

// DebugScryTallyNoTurnReset (TestScrytallyMutation): beginTurn keeps the
// scried / surveilled tallies.
var DebugScryTallyNoTurnReset bool

// noteScry records one scry (op OpScry) or surveil (op OpSurveil)
// instruction by player p.
//
//go:noinline
func (s *State) noteScry(k *prep, op kop.Op, p uint8) {
	if !k.gen.anyScryTally {
		return
	}
	sh := uint8(0)
	if op == kop.OpSurveil {
		sh = 4
	}
	pl := &s.pl[SeatIx(p)]
	if pl.scry>>sh&15 == 15 {
		s.capFull(StallTables) // loud: the nibble holds fifteen
		return
	}
	pl.scry += 1 << sh
}

// scryAnswered is the answered scry arrangement of player p: the tally,
// then "whenever you scry".
//
//go:noinline
func (s *State) scryAnswered(k *prep, p uint8) {
	s.noteScry(k, kop.OpScry, p)
	if k.gen.anyScryTrig {
		s.fireScrySurveil(k, kop.TrigScry, p)
	}
}
