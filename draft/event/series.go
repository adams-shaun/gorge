package event

// The series rules shared by the Apply mutators and the read accessors: a
// best-of-N series is decided when a side has need wins, full when every
// game of its length has been played, and (Bo3 only) between games while
// the sideboarding window is open.

// need is the number of wins one side needs to decide a best-of-N series:
// the majority, bestOf/2+1.
func need(bestOf int) int { return bestOf/2 + 1 }

// wins counts the games each side of st won (draws count for neither).
func wins(st *seriesState, a, b int) (int, int) {
	wa, wb := 0, 0
	for _, g := range st.games {
		switch {
		case g < 0:
		case int(g) == a:
			wa++
		default:
			wb++
		}
	}
	return wa, wb
}

// decided reports whether either side has the wins it needs.
func decided(st *seriesState, a, b int) bool {
	wa, wb := wins(st, a, b)
	return wa >= need(st.bestOf) || wb >= need(st.bestOf)
}

// full reports whether the series has played its whole length.
func full(st *seriesState) bool { return len(st.games) == st.bestOf }

// between reports a best-of-three series in the sideboarding window: a game
// has ended, the next has not started, no attempt is in flight, and the
// series is neither decided nor full.
func between(st *seriesState, a, b int) bool {
	return st.started && !st.ended && st.bestOf == 3 && st.inFlight == nil &&
		len(st.games) >= 1 && !decided(st, a, b) && !full(st)
}

// expectedChooser reports the play/draw chooser the series' next game (2 or
// later) must name (MTR 2.2): the loser of the last game, or — when that
// game was drawn — the chooser that game had, who chooses again. -1 when no
// game was played, and -1 when a drawn game's chooser is unknown (any seat
// may then choose).
func expectedChooser(st *seriesState, a, b int) int {
	if len(st.games) == 0 {
		return -1
	}
	last := st.games[len(st.games)-1]
	if last >= 0 {
		if int(last) == a {
			return b
		}
		return a
	}
	if len(st.choosers) == len(st.games) {
		return int(st.choosers[len(st.choosers)-1])
	}
	return -1
}

// SeriesView is a read-only snapshot of one series of the current round.
// Every slice in it is a copy; the view never aliases the event's state.
type SeriesView struct {
	Round, A, B, BestOf int
	Started, Ended      bool
	// Games is the winner seat per played game, -1 for a draw.
	Games []int
	// Choosers is the play/draw chooser per played game, -1 unknown.
	Choosers    []int
	Wins        [2]int // games won by A, B
	NextGame    int    // len(Games)+1 while neither decided nor full, else 0
	Between     bool   // in the Bo3 sideboarding window
	Locked      [2]int // revisions A, B locked for NextGame; 0 = not yet
	PlayRevs    [2]int // revisions A, B play in the in-flight (or last) game
	NextChooser int    // MTR 2.2 chooser for NextGame >= 2, else -1
	InFlight    *GameAttempt
}

// SeriesState reports the current round's (a, b) series; false when (a, b)
// is not a current pairing. It is read-only: unlike the Apply path it never
// creates series state. A pairing no entry has touched yet reports Started
// false, NextGame 1 and NextChooser -1.
func (e *Event) SeriesState(a, b int) (SeriesView, bool) {
	if e.round < 1 {
		return SeriesView{}, false
	}
	for _, p := range e.pairings {
		if p[0] == a && p[1] == b {
			return seriesView(e.round, a, b, e.series[p]), true
		}
	}
	return SeriesView{}, false
}

// SeriesOf reports the current-round pairing that contains seat.
func (e *Event) SeriesOf(seat int) (a, b int, ok bool) {
	if e.round < 1 || seat < 0 || seat >= e.NumSeats() {
		return 0, 0, false
	}
	for _, p := range e.pairings {
		if p[0] == seat || p[1] == seat {
			return p[0], p[1], true
		}
	}
	return 0, 0, false
}

// seriesView builds the snapshot of one series' state (nil for a pairing
// no entry has touched yet).
func seriesView(round, a, b int, st *seriesState) SeriesView {
	v := SeriesView{Round: round, A: a, B: b, NextChooser: -1}
	if st == nil {
		v.NextGame = 1
		return v
	}
	v.BestOf = st.bestOf
	v.Started = st.started
	v.Ended = st.ended
	v.Games = make([]int, len(st.games))
	for i, g := range st.games {
		v.Games[i] = int(g)
	}
	v.Choosers = make([]int, len(st.choosers))
	for i, c := range st.choosers {
		v.Choosers[i] = int(c)
	}
	v.Wins[0], v.Wins[1] = wins(st, a, b)
	if st.started && !decided(st, a, b) && !full(st) {
		v.NextGame = len(st.games) + 1
	}
	v.Between = between(st, a, b)
	v.Locked = st.locked
	v.PlayRevs = st.playRevs
	if v.NextGame >= 2 {
		v.NextChooser = expectedChooser(st, a, b)
	}
	if st.inFlight != nil {
		at := *st.inFlight
		v.InFlight = &at
	}
	return v
}
