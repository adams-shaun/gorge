package azmcts

import "fmt"

// Decided-game searches (KataGo's reduced visits once a game is decided,
// arXiv:1902.10565; DraftZero research memo D7): once a seat's last
// SeatConfig.DecidedN completed searches all valued its position above
// DecidedHigh, or all below 1 - DecidedHigh, the game is as good as decided
// (DraftZero's offline count: 18.4% of recorded decisions, whose side agreed
// with the result 98% of the time), so its later full searches are cheaper
// -- DecidedSims, no root noise, the most-visited candidate -- and their
// records carry the training weight DecidedWeight (VisitRecord.Weight). A
// searched root value outside that side's band ends the run, and the next
// full search is full again. The run is the seat's own and a pure function
// of its searches, so it never depends on the worker count.

// decidedState is a seat's run of decided root values and its counts.
type decidedState struct {
	// side is the run's side: +1 its values are above DecidedHigh (the seat
	// is winning), -1 below 1 - DecidedHigh (losing), 0 no run.
	side int8
	// run counts the consecutive completed searches on side.
	run int
	// searched and winning count the decided searches run and those on the
	// winning side (DecidedCounts).
	searched, winning int
}

// validateDecided refuses decided-game settings a seat must not run with:
// with DecidedHigh > 0 (0 is off, the others then ignored), DecidedHigh in
// (0.5, 1), DecidedN >= 1, DecidedSims >= 1 and DecidedWeight in [0, 1].
func (c *SeatConfig) validateDecided() error {
	if c.DecidedHigh == 0 {
		return nil
	}
	switch {
	case !(c.DecidedHigh > 0.5 && c.DecidedHigh < 1):
		return fmt.Errorf("azmcts: DecidedHigh %g must be in (0.5, 1) (0 is off)", c.DecidedHigh)
	case c.DecidedN < 1:
		return fmt.Errorf("azmcts: DecidedN %d must be >= 1 with DecidedHigh %g", c.DecidedN, c.DecidedHigh)
	case c.DecidedSims < 1:
		return fmt.Errorf("azmcts: DecidedSims %d must be >= 1 with DecidedHigh %g", c.DecidedSims, c.DecidedHigh)
	case !(c.DecidedWeight >= 0 && c.DecidedWeight <= 1):
		return fmt.Errorf("azmcts: DecidedWeight %g must be in [0,1] (0 records nothing)", c.DecidedWeight)
	}
	return nil
}

// decidedSide is root value v's side of the band hi: +1 above hi, -1 below
// 1 - hi, 0 between (both bounds strict).
func decidedSide(v, hi float64) int8 {
	switch {
	case v > hi:
		return 1
	case v < 1-hi:
		return -1
	}
	return 0
}

// observe folds one completed search's root value v into the run: a value
// on the run's side extends it, one on the other side starts a new run, and
// one inside the band ends it.
func (d *decidedState) observe(v, hi float64) {
	switch sd := decidedSide(v, hi); {
	case sd == 0:
		d.side, d.run = 0, 0
	case sd == d.side:
		d.run++
	default:
		d.side, d.run = sd, 1
	}
}

// decided reports whether the seat's next full search is a decided one:
// decided-game searches are on and the run holds at least DecidedN values.
func (s *Seat) decided() bool {
	return s.cfg.DecidedHigh > 0 && s.dec.side != 0 && s.dec.run >= s.cfg.DecidedN
}

// decidedOptions turns a full search's options into a decided one's:
// DecidedSims simulations, no root noise and no visit sampling; the seed is
// the full search's.
func (s *Seat) decidedOptions(opts Options) Options {
	opts.Sims, opts.Noise, opts.Sample = s.cfg.DecidedSims, false, false
	return opts
}

// DecidedCounts are the seat's decided-game searches (a tree was built) and
// how many of them were on the winning side (root values above
// DecidedHigh); both 0 with DecidedHigh 0.
func (s *Seat) DecidedCounts() (decided, winning int) { return s.dec.searched, s.dec.winning }
