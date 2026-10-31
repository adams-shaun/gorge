package event

// GameAttempt is one started-but-not-yet-resulted game of a series: the
// host table it is being played on and its 1-based attempt number. A game
// replayed after a crash or restart gets the next attempt and a new seed.
type GameAttempt struct {
	Round   int
	A, B    int
	Game    int
	Attempt int
	Table   string
	Seed    uint64
	// Chooser is the game's play/draw chooser seat, or -1 when the
	// GameStarted entry carried none (every Bo1 game).
	Chooser int
}

// SeriesOutcome is one ended series: the round, the two seats, the final
// score and the winning seat (-1 for a drawn series).
type SeriesOutcome struct {
	Round  int
	A, B   int
	Score  [2]int
	Winner int // -1 for a drawn series
	// Games is the winner seat per played game, -1 for a draw.
	Games []int
}

// InFlight returns the current round's in-flight game attempts in pairing
// order, or nil when none are in flight or no round is live. It is
// read-only: unlike the Apply path it never creates series state.
func (e *Event) InFlight() []GameAttempt {
	if e.round < 1 {
		return nil
	}
	var out []GameAttempt
	for _, p := range e.pairings {
		st := e.series[p]
		if st != nil && st.inFlight != nil {
			out = append(out, *st.inFlight)
		}
	}
	return out
}

// NextAttempt reports the attempt number the current round's (a, b) series'
// next GameStarted must carry, or 0 when (a, b) is not a pairing of the
// current round or the series has already ended.
func (e *Event) NextAttempt(a, b int) int {
	if e.round < 1 {
		return 0
	}
	for _, p := range e.pairings {
		if p[0] != a || p[1] != b {
			continue
		}
		st := e.series[p]
		if st == nil {
			return 1
		}
		if st.ended {
			return 0
		}
		return st.attempts + 1
	}
	return 0
}

// Started reports whether a SeriesStarted has been logged for the current
// round's (a, b) series. It is read-only and never creates series state.
func (e *Event) Started(a, b int) bool {
	if e.round < 1 {
		return false
	}
	for _, p := range e.pairings {
		if p[0] == a && p[1] == b {
			st := e.series[p]
			return st != nil && st.started
		}
	}
	return false
}

// Results returns every ended series in log order, each an independent
// value copy.
func (e *Event) Results() []SeriesOutcome {
	out := make([]SeriesOutcome, 0, len(e.results))
	for _, f := range e.results {
		games := make([]int, len(f.games))
		for i, g := range f.games {
			games[i] = int(g)
		}
		out = append(out, SeriesOutcome{
			Round: f.round, A: f.a, B: f.b, Score: f.score, Winner: f.winner,
			Games: games,
		})
	}
	return out
}

// SavedRev reports the seat's latest saved deck revision, or 0 when the
// seat is out of range or has saved no deck.
func (e *Event) SavedRev(seat int) int {
	if seat < 0 || seat >= e.NumSeats() || len(e.decks[seat]) == 0 {
		return 0
	}
	return e.decks[seat][len(e.decks[seat])-1].rev
}

// ReadyRev reports the revision the seat has readied, or 0 when the seat is
// out of range or not ready.
func (e *Event) ReadyRev(seat int) int {
	if seat < 0 || seat >= e.NumSeats() {
		return 0
	}
	return e.ready[seat]
}

// Queued reports how many packs are waiting in the seat's draft queue, or 0
// when the event is not drafting (including a sealed event, which has no
// draft) or the pod is not built yet.
func (e *Event) Queued(seat int) int {
	if e.phase != PhaseDraft || e.mode == ModeSealed {
		return 0
	}
	e.ensurePod()
	if e.pod == nil || seat < 0 || seat >= e.NumSeats() {
		return 0
	}
	return e.pod.QueueLen(seat)
}

// LogFor returns the log as a viewer sees it. Once the event is COMPLETE
// the whole log is public history: a deep copy of every entry. Before that
// it is redacted per seat: for every seat other than viewer (a viewer
// outside 0..n-1 is a spectator, for whom every seat is "other"), a
// PackOpened's card list, a Picked's card ID and a DeckSaved's deck list are
// hidden, and every GameStarted's seed is hidden from everyone. The event's
// own log is never mutated.
func (e *Event) LogFor(viewer int) []Entry {
	out := make([]Entry, 0, len(e.log))
	for i := range e.log {
		en := cloneEntry(e.log[i])
		if !e.Done() {
			switch en.Kind {
			case KindPackOpened:
				if en.Seat != viewer {
					en.Cards = nil
				}
			case KindPicked:
				if en.Seat != viewer {
					en.ID = ""
				}
			case KindDeckSaved:
				if en.Seat != viewer {
					en.Deck = nil
				}
			case KindGameStarted:
				en.GameSeed = 0
			}
		}
		out = append(out, en)
	}
	return out
}
