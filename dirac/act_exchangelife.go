package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// ExchangeLife (CR 701.12; gorge engine/effects/life.go effExchangeLife's
// one-player arm): the resolving controller and the one named player swap life
// totals. Both deltas come from the same snapshot (oldB-oldA to the controller,
// oldA-oldB to the player), a tie or the same player changes nothing, and each
// side goes through the ordinary life change (gainLife / loseLife: the "can't
// gain life" and the doubling replacements, the LifeGained / LifeLost triggers
// and the per-turn tallies), as the SetLife op (dispatch.go) does for gorge's
// LifeChange events. gorge's rules-side exchange transaction stages both sides
// and applies them together; no kernel replacement reads the other side's
// life, so the sequential apply is the same game.
//
// DebugExchangeLifeOneSide is the lockstep mutation: only the controller's
// side applies, so a game that exchanges must diverge at the next life read.

// DebugExchangeLifeOneSide applies only the controller's half of the exchange.
var DebugExchangeLifeOneSide bool

// exchangeLifeExt is ExtExchangeLife.
//
//kmanifest:ext ExtExchangeLife
func (s *State) exchangeLifeExt(k *prep, in *kop.Instr) (ok, done bool) {
	q := s.players(k, in.Obj)
	if q.n != 1 {
		return true, true // the target is gone or illegal: gorge's Note, no change
	}
	a, b := s.res.e.ctrl, q.s[0]
	if a == b {
		return true, true
	}
	oldA, oldB := int(s.pl[a].life), int(s.pl[b].life)
	if oldA == oldB {
		return true, true
	}
	s.shiftLife(k, a, oldB-oldA)
	if !DebugExchangeLifeOneSide {
		s.shiftLife(k, b, oldA-oldB)
	}
	return true, true
}

// shiftLife applies a signed life change as a gain or a loss.
func (s *State) shiftLife(k *prep, p uint8, d int) {
	if d > 0 {
		s.gainLife(k, p, d)
	} else {
		s.loseLife(k, p, -d)
	}
}
