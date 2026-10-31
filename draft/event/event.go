package event

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/adams-shaun/gorge/draft/limited"
	"github.com/adams-shaun/gorge/draft/limited/build"
	"github.com/adams-shaun/gorge/draft/limited/draft"
)

// Phase is the event's lifecycle stage. A round phase is spelled ROUNDn
// (ROUND1, ROUND2, ROUND3); use RoundPhase to build one.
type Phase string

// The lifecycle phases. LOBBY is a fresh event before its EventCreated;
// COMPLETE is reached only through the EventEnded entry; CANCELLED
// is reached only through the LobbyTimeout entry, when nobody joined an
// event within its lobby window.
//
// PhaseDraft is the supply phase of BOTH modes: a draft event drafts its
// pod there, and a sealed event only opens its packs into the seats' pools
// (the name predates the sealed mode and is kept so logs stay comparable).
const (
	PhaseLobby     Phase = "LOBBY"
	PhaseDraft     Phase = "DRAFT"
	PhaseBuild     Phase = "BUILD"
	PhaseComplete  Phase = "COMPLETE"
	PhaseCancelled Phase = "CANCELLED"
)

// RoundPhase spells the phase of event round r (1..the event's round
// count).
func RoundPhase(r int) Phase { return Phase("ROUND" + strconv.Itoa(r)) }

// SeatSpec describes one entrant. Kind is the entrant's broad type ("bot"
// or "human"), Name is its display name and Persona is the bot persona it
// plays as (a pickbot drafter/builder/player triple), empty for a human.
type SeatSpec struct {
	Kind    string `json:"kind"`
	Name    string `json:"name"`
	Persona string `json:"persona,omitempty"`
}

// SeriesSpec is one series the caller must play: the two seats and the
// round, with the series length.
type SeriesSpec struct {
	Round  int `json:"round"`
	A      int `json:"a"`
	B      int `json:"b"`
	BestOf int `json:"best_of"`
}

// bestOfOnly is the series length PendingSeries reports for a series not
// yet started: the caller passes its own best-of in its SeriesStarted.
const bestOfOnly = 1

// Event is one draft event's state machine. Every field change goes through
// Apply, which validates the entry against the current phase and appends it
// to the log; Replay rebuilds an Event from its log.
type Event struct {
	phase  Phase
	seed   uint64
	set    string
	seats  []SeatSpec
	cooked bool // an EventCreated has been applied

	// The EventCreated shape, normalized: the logged entry keeps the values
	// exactly as given (mode "" stays "", Packs 0 stays 0).
	mode      string // ModeDraft or ModeSealed; "" before EventCreated
	structure string // StructureRoundRobin, StructureSwiss or StructureNone; "" before EventCreated
	packsPer  int
	rounds    int // 0 for StructureNone
	collation string

	log []Entry

	packs      [][][]limited.CardRef // [pack-1][seat]
	packOpen   [][]bool              // [pack-1][seat]
	packCount  int
	packIDs    map[string]struct{} // every CardRef ID seen in a pack
	pod        *draft.Pod
	podStarted bool
	podErr     error // a draft.NewPod failure (impossible for a validated event)

	decks [][]savedDeck
	ready []int // the readied revision, 0 = not ready

	round    int // the current event round, 0 outside the rounds
	pairings [][2]int
	series   map[[2]int]*seriesState
	results  []finishedSeries
}

// savedDeck is one revision of a seat's deck list.
type savedDeck struct {
	rev  int
	deck build.Deck
}

// seriesState is the current round's record of one series.
type seriesState struct {
	started bool
	ended   bool
	bestOf  int
	games   []int8 // winner seat, or -1 for a draw
	// choosers is the play/draw chooser per played game, -1 unknown; kept
	// parallel with games.
	choosers []int8
	// playRevs is the deck revision each side played in the in-flight (or
	// last) game: game 1 plays the readied BUILD decks, later games the
	// sideboarded locks.
	playRevs [2]int
	// locked is the deck revision each side has Sideboarded for the series'
	// next game; 0 = not yet. A GameEnded clears it; a GameAborted keeps it,
	// so a replayed attempt plays the same decks.
	locked [2]int
	// inFlight is the started-but-not-yet-resulted game attempt, or nil.
	inFlight *GameAttempt
	// attempts is the highest attempt logged for the series' NEXT game; it
	// resets to 0 when a game ends, so each game's attempts start at 1.
	attempts int
}

// finishedSeries is a completed series, kept for standings and replay.
type finishedSeries struct {
	round  int
	a, b   int
	score  [2]int
	games  []int8 // winner seat, or -1 for a draw
	winner int    // -1 drawn, else the winning seat
}

// New returns a fresh event in the LOBBY phase. Its first entry must be
// EventCreated.
func New() *Event {
	return &Event{phase: PhaseLobby, packIDs: map[string]struct{}{}}
}

// Phase reports the event's current lifecycle phase.
func (e *Event) Phase() Phase { return e.phase }

// Round reports the current event round: 1..3 inside a round phase, 0
// otherwise.
func (e *Event) Round() int { return e.round }

// Seed reports the event seed from its EventCreated entry.
func (e *Event) Seed() uint64 { return e.seed }

// SetCode reports the drafted set code from its EventCreated entry.
func (e *Event) SetCode() string { return e.set }

// Seats reports the event's entrants from the EventCreated entry, as a
// copy.
func (e *Event) Seats() []SeatSpec {
	return append([]SeatSpec(nil), e.seats...)
}

// Log returns a copy of the append-only log.
func (e *Event) Log() []Entry {
	return append([]Entry(nil), e.log...)
}

// Done reports whether the event reached COMPLETE.
func (e *Event) Done() bool { return e.phase == PhaseComplete }

// Next returns seat's draft view when a pack waits for it. It is read-only:
// the actual pick goes through a Picked entry. A sealed event never offers
// one.
func (e *Event) Next(seat int) (draft.PickView, bool) {
	if e.mode == ModeSealed {
		return draft.PickView{}, false
	}
	e.ensurePod()
	if e.pod == nil || seat < 0 || seat >= e.NumSeats() {
		return draft.PickView{}, false
	}
	return e.pod.Next(seat)
}

// Pool returns seat's cards — a draft's picks through its pod, a sealed
// event's opened packs in pack order — or nil before any packs are open.
func (e *Event) Pool(seat int) []limited.CardRef {
	if e.mode == ModeSealed {
		return e.poolSealed(seat)
	}
	e.ensurePod()
	if e.pod == nil || seat < 0 || seat >= e.NumSeats() {
		return nil
	}
	return e.pod.Pool(seat)
}

// SavedDeck returns the seat's latest saved deck revision, if any.
func (e *Event) SavedDeck(seat int) (build.Deck, bool) {
	if seat < 0 || seat >= e.NumSeats() || len(e.decks[seat]) == 0 {
		return build.Deck{}, false
	}
	d := e.decks[seat][len(e.decks[seat])-1].deck
	d.Main = append([]string(nil), d.Main...)
	return d, true
}

// DeckRev returns the seat's saved deck at the given revision, if any
// (the pool view's Playing deck names a specific revision).
func (e *Event) DeckRev(seat, rev int) (build.Deck, bool) {
	if seat < 0 || seat >= e.NumSeats() || rev < 1 {
		return build.Deck{}, false
	}
	for _, sd := range e.decks[seat] {
		if sd.rev == rev {
			d := sd.deck
			d.Main = append([]string(nil), d.Main...)
			return d, true
		}
	}
	return build.Deck{}, false
}

// PendingSeries lists the current round's series that have not ended, in
// pairing order. The caller plays each one and reports its games back as
// GameEnded entries, then closes it with SeriesEnded.
func (e *Event) PendingSeries() []SeriesSpec {
	if e.round < 1 {
		return nil
	}
	out := make([]SeriesSpec, 0, len(e.pairings))
	for _, p := range e.pairings {
		st := e.series[p]
		if st != nil && st.ended {
			continue
		}
		bestOf := bestOfOnly
		if st != nil && st.started {
			bestOf = st.bestOf
		}
		out = append(out, SeriesSpec{Round: e.round, A: p[0], B: p[1], BestOf: bestOf})
	}
	return out
}

// inRound reports whether a round phase is live: the event round is set
// and the phase spells it.
func (e *Event) inRound() bool {
	return e.round >= 1 && e.phase == RoundPhase(e.round)
}

// Apply validates one entry against the current state, applies it and
// appends it to the log. It is the only mutator; an entry illegal in the
// current phase is rejected with an error and changes nothing.
func (e *Event) Apply(en Entry) error {
	en = cloneEntry(en)
	if err := e.apply(&en); err != nil {
		return err
	}
	e.log = append(e.log, en)
	return nil
}

// apply dispatches one entry to its kind's mutator. It may fill fields of
// the private clone (EventEnded's standings), which is then logged.
func (e *Event) apply(en *Entry) error {
	switch en.Kind {
	case KindEventCreated:
		return e.applyEventCreated(en)
	case KindPackOpened:
		return e.applyPackOpened(en)
	case KindPicked:
		return e.applyPicked(en)
	case KindDeckSaved:
		return e.applyDeckSaved(en)
	case KindReady:
		return e.applyReady(en)
	case KindSeriesStarted:
		return e.applySeriesStarted(en)
	case KindGameEnded:
		return e.applyGameEnded(en)
	case KindSeriesEnded:
		return e.applySeriesEnded(en)
	case KindEventEnded:
		return e.applyEventEnded(en)
	case KindGameStarted:
		return e.applyGameStarted(en)
	case KindGameAborted:
		return e.applyGameAborted(en)
	case KindSideboarded:
		return e.applySideboarded(en)
	case KindRoundPaired:
		return e.applyRoundPaired(en)
	case KindLobbyTimeout:
		return e.applyLobbyTimeout(en)
	case KindIdleTimeout:
		return e.applyIdleTimeout(en)
	default:
		return fmt.Errorf("event: unknown entry kind %q", en.Kind)
	}
}

// Replay rebuilds an event by applying entries in order; the first illegal
// entry is an error and the returned event is nil.
func Replay(entries []Entry) (*Event, error) {
	e := New()
	for i := range entries {
		if err := e.Apply(entries[i]); err != nil {
			return nil, fmt.Errorf("event: replay entry %d: %w", i, err)
		}
	}
	return e, nil
}

// ReplayLog parses a JSON-lines log and replays it.
func ReplayLog(data []byte) (*Event, error) {
	entries, err := ParseLog(data)
	if err != nil {
		return nil, err
	}
	return Replay(entries)
}

// applyEventCreated validates the EventCreated shape and stores the
// normalized values; the logged entry keeps the values exactly as given,
// so old logs re-marshal byte-identically.
func (e *Event) applyEventCreated(en *Entry) error {
	if e.cooked {
		return fmt.Errorf("event: EventCreated: event already created")
	}
	if en.Set == "" {
		return fmt.Errorf("event: EventCreated: set code is empty")
	}
	n := len(en.Seats)
	if n != 4 && n != 8 {
		return fmt.Errorf("event: EventCreated: got %d seats, want 4 or 8", n)
	}
	for i, s := range en.Seats {
		if s.Kind == "" {
			return fmt.Errorf("event: EventCreated: seat %d has no kind", i)
		}
		if s.Name == "" {
			return fmt.Errorf("event: EventCreated: seat %d has no name", i)
		}
	}
	mode := ModeDraft
	switch en.Mode {
	case "":
	case ModeDraft, ModeSealed:
		mode = en.Mode
	default:
		return fmt.Errorf("event: EventCreated: unknown mode %q", en.Mode)
	}
	packs := en.Packs
	switch {
	case mode == ModeDraft && packs == 0:
		packs = DraftPacks
	case mode == ModeDraft && packs != DraftPacks:
		return fmt.Errorf("event: EventCreated: draft packs %d, want %d", packs, DraftPacks)
	case packs == 0:
		packs = DefaultSealedPacks
	case packs < 1 || packs > MaxSealedPacks:
		return fmt.Errorf("event: EventCreated: sealed packs %d, want 1..%d", packs, MaxSealedPacks)
	}
	structure := StructureRoundRobin
	switch en.Structure {
	case "":
	case StructureRoundRobin, StructureNone, StructureSwiss:
		structure = en.Structure
	default:
		return fmt.Errorf("event: EventCreated: unknown structure %q", en.Structure)
	}
	rounds := 0
	switch structure {
	case StructureRoundRobin:
		rounds = n - 1
		if en.Rounds != 0 {
			if en.Rounds < 1 || en.Rounds >= n {
				return fmt.Errorf("event: EventCreated: rounds %d out of range 1..%d", en.Rounds, n-1)
			}
			rounds = en.Rounds
		}
	case StructureSwiss:
		// A Swiss event's default round count is ceil(log2 n): 2 for 4
		// seats, 3 for 8.
		rounds = swissDefaultRounds(n)
		if en.Rounds != 0 {
			if en.Rounds < 1 || en.Rounds >= n {
				return fmt.Errorf("event: EventCreated: rounds %d out of range 1..%d", en.Rounds, n-1)
			}
			rounds = en.Rounds
		}
	default: // StructureNone
		if en.Rounds != 0 {
			return fmt.Errorf("event: EventCreated: structure none takes no rounds")
		}
	}
	e.seed = en.Seed
	e.set = en.Set
	e.seats = append([]SeatSpec(nil), en.Seats...)
	e.mode = mode
	e.structure = structure
	e.packsPer = packs
	e.rounds = rounds
	e.collation = en.Collation
	e.cooked = true
	e.phase = PhaseDraft
	e.packs = make([][][]limited.CardRef, packs)
	e.packOpen = make([][]bool, packs)
	for i := range e.packs {
		e.packs[i] = make([][]limited.CardRef, n)
		e.packOpen[i] = make([]bool, n)
	}
	e.decks = make([][]savedDeck, n)
	e.ready = make([]int, n)
	return nil
}

func (e *Event) applyPackOpened(en *Entry) error {
	if !e.cooked {
		return fmt.Errorf("event: PackOpened: event not created")
	}
	if e.phase != PhaseDraft {
		return fmt.Errorf("event: PackOpened: illegal in phase %s", e.phase)
	}
	if en.Round < 1 || en.Round > e.packsPer {
		return fmt.Errorf("event: PackOpened: round %d out of range", en.Round)
	}
	if en.Seat < 0 || en.Seat >= e.NumSeats() {
		return fmt.Errorf("event: PackOpened: seat %d out of range", en.Seat)
	}
	if e.packOpen[en.Round-1][en.Seat] {
		return fmt.Errorf("event: PackOpened: round %d seat %d already opened", en.Round, en.Seat)
	}
	if len(en.Cards) == 0 {
		return fmt.Errorf("event: PackOpened: pack is empty")
	}
	inPack := map[string]struct{}{}
	for _, c := range en.Cards {
		if c.ID == "" || c.Name == "" {
			return fmt.Errorf("event: PackOpened: card with empty ID or name")
		}
		if _, dup := inPack[c.ID]; dup {
			return fmt.Errorf("event: PackOpened: card %q twice in one pack", c.ID)
		}
		if _, seen := e.packIDs[c.ID]; seen {
			return fmt.Errorf("event: PackOpened: card ID %q already used in another pack", c.ID)
		}
		inPack[c.ID] = struct{}{}
	}
	for _, c := range en.Cards {
		e.packIDs[c.ID] = struct{}{}
	}
	e.packs[en.Round-1][en.Seat] = append([]limited.CardRef(nil), en.Cards...)
	e.packOpen[en.Round-1][en.Seat] = true
	e.packCount++
	if e.sealedSupplyDone() {
		e.phase = PhaseBuild
	}
	return nil
}

// ensurePod builds the event's draft pod from the logged PackOpened entries
// once all of them are in. The driver opens every pack of the event at DRAFT
// start, so a pick cannot precede the pod. A sealed event never builds one.
func (e *Event) ensurePod() {
	n := e.NumSeats()
	if e.podStarted || e.mode != ModeDraft || e.packCount < n*DraftPacks {
		return
	}
	packs := make([][][]limited.CardRef, n)
	for s := 0; s < n; s++ {
		packs[s] = make([][]limited.CardRef, e.packsPer)
		for r := 0; r < e.packsPer; r++ {
			packs[s][r] = e.packs[r][s]
		}
	}
	p, err := draft.NewPod(e.seed, packs)
	if err != nil {
		e.podErr = err // impossible for a validated event
	} else {
		e.pod = p
	}
	e.podStarted = true
}

func (e *Event) applyPicked(en *Entry) error {
	if !e.cooked {
		return fmt.Errorf("event: Picked: event not created")
	}
	if e.mode == ModeSealed {
		return fmt.Errorf("event: Picked: a sealed event has no draft")
	}
	if e.phase != PhaseDraft {
		return fmt.Errorf("event: Picked: illegal in phase %s", e.phase)
	}
	e.ensurePod()
	if e.pod == nil {
		if e.podErr != nil {
			return fmt.Errorf("event: Picked: %w", e.podErr)
		}
		return fmt.Errorf("event: Picked: packs not opened yet")
	}
	if en.Seat < 0 || en.Seat >= e.NumSeats() {
		return fmt.Errorf("event: Picked: seat %d out of range", en.Seat)
	}
	v, ok := e.pod.Next(en.Seat)
	if !ok {
		return fmt.Errorf("event: Picked: seat %d has no pack to pick from: %w", en.Seat, draft.ErrNotYourTurn)
	}
	if v.Round != en.Round {
		return fmt.Errorf("event: Picked: entry round %d, seat's pack is from round %d", en.Round, v.Round)
	}
	if v.PickNo != en.PickNo {
		return fmt.Errorf("event: Picked: seat %d's next pick number is %d, got %d", en.Seat, v.PickNo, en.PickNo)
	}
	if err := e.pod.Pick(en.Seat, en.PickNo, en.ID); err != nil {
		return fmt.Errorf("event: Picked: seat %d pick %d: %w", en.Seat, en.PickNo, err)
	}
	if e.pod.Done() {
		e.phase = PhaseBuild
	}
	return nil
}

// applyDeckSaved saves one seat's deck revision. It is a BUILD-phase entry,
// and also legal in a round phase for a seat whose current series is
// between games with its side not yet sideboarded — a mid-series deck change
// only takes effect when the seat Sideboards the new revision, so a round
// phase DeckSaved does not touch the seat's readiness.
func (e *Event) applyDeckSaved(en *Entry) error {
	if !e.cooked {
		return fmt.Errorf("event: DeckSaved: event not created")
	}
	inRound := false
	switch e.phase {
	case PhaseBuild:
		// the ordinary BUILD-phase save
	default:
		if !e.inRound() {
			return fmt.Errorf("event: DeckSaved: illegal in phase %s", e.phase)
		}
		inRound = true
	}
	if en.Seat < 0 || en.Seat >= e.NumSeats() {
		return fmt.Errorf("event: DeckSaved: seat %d out of range", en.Seat)
	}
	if inRound {
		a, b, ok := e.SeriesOf(en.Seat)
		if !ok {
			return fmt.Errorf("event: DeckSaved: seat %d has no series in round %d", en.Seat, e.round)
		}
		st := e.series[[2]int{a, b}]
		if st == nil || !between(st, a, b) {
			return fmt.Errorf("event: DeckSaved: seat %d's series is not between games", en.Seat)
		}
		side := 0
		if en.Seat == b {
			side = 1
		}
		if st.locked[side] != 0 {
			return fmt.Errorf("event: DeckSaved: seat %d's side is already sideboarded for game %d", en.Seat, len(st.games)+1)
		}
	}
	if en.Rev != len(e.decks[en.Seat])+1 {
		return fmt.Errorf("event: DeckSaved: seat %d revision %d is not the next revision", en.Seat, en.Rev)
	}
	if en.Deck == nil {
		return fmt.Errorf("event: DeckSaved: seat %d has no deck", en.Seat)
	}
	if err := build.Validate(e.Pool(en.Seat), *en.Deck); err != nil {
		return fmt.Errorf("event: DeckSaved: seat %d: %w", en.Seat, err)
	}
	d := *en.Deck
	d.Main = append([]string(nil), d.Main...)
	e.decks[en.Seat] = append(e.decks[en.Seat], savedDeck{rev: en.Rev, deck: d})
	if !inRound {
		e.ready[en.Seat] = 0 // a new revision must be readied again
	}
	return nil
}

func (e *Event) applyReady(en *Entry) error {
	if !e.cooked {
		return fmt.Errorf("event: Ready: event not created")
	}
	if e.phase != PhaseBuild {
		return fmt.Errorf("event: Ready: illegal in phase %s", e.phase)
	}
	if en.Seat < 0 || en.Seat >= e.NumSeats() {
		return fmt.Errorf("event: Ready: seat %d out of range", en.Seat)
	}
	found := false
	for _, d := range e.decks[en.Seat] {
		if d.rev == en.Rev {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("event: Ready: seat %d has no saved deck revision %d", en.Seat, en.Rev)
	}
	if e.ready[en.Seat] != 0 {
		return fmt.Errorf("event: Ready: seat %d already ready", en.Seat)
	}
	e.ready[en.Seat] = en.Rev
	if e.allReady() {
		e.startRounds()
	}
	return nil
}

func (e *Event) allReady() bool {
	if len(e.ready) == 0 {
		return false // before EventCreated nothing is ready
	}
	for s := range e.ready {
		if e.ready[s] == 0 {
			return false
		}
	}
	return true
}

// startRounds opens the first event round: its pairings become the current
// series set. A StructureNone event opens no round and stays in BUILD. A
// Swiss event stays in BUILD too: it awaits the driver's first RoundPaired
// entry (see AwaitingPairing).
func (e *Event) startRounds() {
	if e.structure != StructureRoundRobin {
		return
	}
	e.round = 1
	e.pairings = RoundRobinPairings(e.NumSeats(), 1)
	e.series = map[[2]int]*seriesState{}
	e.phase = RoundPhase(1)
}

// seriesFor returns the state of the current round's (a, b) series, or nil
// when the pair is not a pairing of the current round.
func (e *Event) seriesFor(a, b int) *seriesState {
	for _, p := range e.pairings {
		if p[0] == a && p[1] == b {
			st := e.series[p]
			if st == nil {
				st = &seriesState{}
				e.series[p] = st
			}
			return st
		}
	}
	return nil
}

func (e *Event) applySeriesStarted(en *Entry) error {
	if err := e.seriesEntryOK(en); err != nil {
		return err
	}
	if en.BestOf != 1 && en.BestOf != 3 {
		return fmt.Errorf("event: SeriesStarted: best of %d unsupported (only Bo1 and Bo3)", en.BestOf)
	}
	st := e.seriesFor(en.A, en.B)
	if st.started {
		return fmt.Errorf("event: SeriesStarted: seats %d–%d already started", en.A, en.B)
	}
	st.started = true
	st.bestOf = en.BestOf
	return nil
}

func (e *Event) applyGameEnded(en *Entry) error {
	if err := e.seriesEntryOK(en); err != nil {
		return err
	}
	st := e.seriesFor(en.A, en.B)
	if !st.started {
		return fmt.Errorf("event: GameEnded: series seats %d–%d not started", en.A, en.B)
	}
	if st.ended {
		return fmt.Errorf("event: GameEnded: series seats %d–%d already ended", en.A, en.B)
	}
	if decided(st, en.A, en.B) || full(st) {
		return fmt.Errorf("event: GameEnded: series seats %d–%d is already decided or full", en.A, en.B)
	}
	if en.Game != len(st.games)+1 {
		return fmt.Errorf("event: GameEnded: game %d is not the series' next game (%d)", en.Game, len(st.games)+1)
	}
	if en.Winner != nil && *en.Winner != en.A && *en.Winner != en.B {
		return fmt.Errorf("event: GameEnded: winner seat %d is not a series seat", *en.Winner)
	}
	if en.Head == "" {
		return fmt.Errorf("event: GameEnded: game has no chain head")
	}
	ch := int8(-1)
	if st.inFlight != nil {
		if en.Game != st.inFlight.Game {
			return fmt.Errorf("event: GameEnded: game %d does not match the in-flight attempt's game %d", en.Game, st.inFlight.Game)
		}
		if en.Attempt != 0 && en.Attempt != st.inFlight.Attempt {
			return fmt.Errorf("event: GameEnded: attempt %d does not match the in-flight attempt %d", en.Attempt, st.inFlight.Attempt)
		}
		ch = int8(st.inFlight.Chooser)
		st.inFlight = nil
	}
	st.attempts = 0
	g := int8(-1)
	if en.Winner != nil {
		g = int8(*en.Winner)
	}
	st.games = append(st.games, g)
	st.choosers = append(st.choosers, ch)
	st.locked = [2]int{} // the locks served the game just ended
	return nil
}

// applyGameStarted records a game attempt: the host table an attempt of the
// series' next game is being played on. A crash or restart aborts the
// attempt with GameAborted and the retry logs the next attempt. A
// best-of-three GameStarted also carries the play/draw chooser: game 1 the
// CR 103.1 toss winner, later games MTR 2.2's loser of the previous game
// (or that game's chooser after a draw).
func (e *Event) applyGameStarted(en *Entry) error {
	if err := e.seriesEntryOK(en); err != nil {
		return err
	}
	st := e.seriesFor(en.A, en.B)
	if !st.started {
		return fmt.Errorf("event: GameStarted: series seats %d–%d not started", en.A, en.B)
	}
	if st.ended {
		return fmt.Errorf("event: GameStarted: series seats %d–%d already ended", en.A, en.B)
	}
	if decided(st, en.A, en.B) || full(st) {
		return fmt.Errorf("event: GameStarted: series seats %d–%d is already decided or full", en.A, en.B)
	}
	if st.inFlight != nil {
		return fmt.Errorf("event: GameStarted: series seats %d–%d already has a game attempt in flight (game %d attempt %d)",
			en.A, en.B, st.inFlight.Game, st.inFlight.Attempt)
	}
	if en.Game != len(st.games)+1 {
		return fmt.Errorf("event: GameStarted: game %d is not the series' next game (%d)", en.Game, len(st.games)+1)
	}
	if en.Attempt != st.attempts+1 {
		return fmt.Errorf("event: GameStarted: attempt %d is not the series' next attempt (%d)", en.Attempt, st.attempts+1)
	}
	if en.Table == "" {
		return fmt.Errorf("event: GameStarted: attempt has no table")
	}
	chooser := -1
	switch {
	case st.bestOf == 1:
		if en.Chooser != nil {
			return fmt.Errorf("event: GameStarted: a best-of-one game carries no play/draw chooser")
		}
	case en.Chooser == nil:
		return fmt.Errorf("event: GameStarted: a best-of-three game needs a play/draw chooser")
	case *en.Chooser != en.A && *en.Chooser != en.B:
		return fmt.Errorf("event: GameStarted: chooser seat %d is not a series seat", *en.Chooser)
	case len(st.games) >= 1:
		if st.locked[0] == 0 || st.locked[1] == 0 {
			return fmt.Errorf("event: GameStarted: game %d needs both sides sideboarded first", en.Game)
		}
		if exp := expectedChooser(st, en.A, en.B); exp >= 0 && *en.Chooser != exp {
			return fmt.Errorf("event: GameStarted: chooser seat %d, want seat %d (MTR 2.2)", *en.Chooser, exp)
		}
	}
	if en.Chooser != nil {
		chooser = *en.Chooser
	}
	if en.Game == 1 {
		st.playRevs = [2]int{e.ready[en.A], e.ready[en.B]}
	} else {
		st.playRevs = st.locked
	}
	st.inFlight = &GameAttempt{
		Round: en.Round, A: en.A, B: en.B, Game: en.Game, Attempt: en.Attempt,
		Table: en.Table, Seed: en.GameSeed, Chooser: chooser,
	}
	st.attempts = en.Attempt
	return nil
}

// applyGameAborted clears an in-flight game attempt without recording a
// result, so the game can be replayed as the series' next attempt.
func (e *Event) applyGameAborted(en *Entry) error {
	if err := e.seriesEntryOK(en); err != nil {
		return err
	}
	st := e.seriesFor(en.A, en.B)
	if st.inFlight == nil {
		return fmt.Errorf("event: GameAborted: series seats %d–%d has no game attempt in flight", en.A, en.B)
	}
	if en.Game != st.inFlight.Game || en.Attempt != st.inFlight.Attempt {
		return fmt.Errorf("event: GameAborted: names game %d attempt %d, but the in-flight attempt is game %d attempt %d",
			en.Game, en.Attempt, st.inFlight.Game, st.inFlight.Attempt)
	}
	if en.Reason == "" {
		return fmt.Errorf("event: GameAborted: no reason given")
	}
	st.inFlight = nil
	return nil
}

// applySideboarded locks one side's deck revision for its series' next
// game: the seat (or the server for an absent human, Auto) keeps the deck
// it played last game or plays a revision it saved between games. Both
// sides must be locked before the next GameStarted of a best-of-three
// series is legal.
func (e *Event) applySideboarded(en *Entry) error {
	if err := e.seriesEntryOK(en); err != nil {
		return err
	}
	st := e.seriesFor(en.A, en.B)
	if !between(st, en.A, en.B) {
		return fmt.Errorf("event: Sideboarded: series seats %d–%d is not between games", en.A, en.B)
	}
	if en.Game != len(st.games)+1 {
		return fmt.Errorf("event: Sideboarded: game %d is not the series' next game (%d)", en.Game, len(st.games)+1)
	}
	if en.Seat < 0 || en.Seat >= e.NumSeats() {
		return fmt.Errorf("event: Sideboarded: seat %d out of range", en.Seat)
	}
	if en.Seat != en.A && en.Seat != en.B {
		return fmt.Errorf("event: Sideboarded: seat %d is not a series seat", en.Seat)
	}
	side := 0
	if en.Seat == en.B {
		side = 1
	}
	if st.locked[side] != 0 {
		return fmt.Errorf("event: Sideboarded: seat %d already sideboarded for game %d", en.Seat, en.Game)
	}
	if en.Rev < 1 || en.Rev > e.SavedRev(en.Seat) {
		return fmt.Errorf("event: Sideboarded: seat %d revision %d is not a saved revision (1..%d)",
			en.Seat, en.Rev, e.SavedRev(en.Seat))
	}
	st.locked[side] = en.Rev
	return nil
}

func (e *Event) applySeriesEnded(en *Entry) error {
	if err := e.seriesEntryOK(en); err != nil {
		return err
	}
	st := e.seriesFor(en.A, en.B)
	if !st.started {
		return fmt.Errorf("event: SeriesEnded: series seats %d–%d not started", en.A, en.B)
	}
	if st.ended {
		return fmt.Errorf("event: SeriesEnded: series seats %d–%d already ended", en.A, en.B)
	}
	if st.inFlight != nil {
		return fmt.Errorf("event: SeriesEnded: series seats %d–%d has a game attempt in flight (game %d attempt %d)",
			en.A, en.B, st.inFlight.Game, st.inFlight.Attempt)
	}
	if len(st.games) != st.bestOf && !decided(st, en.A, en.B) {
		return fmt.Errorf("event: SeriesEnded: %d of %d games played", len(st.games), st.bestOf)
	}
	winsA, winsB := 0, 0
	for _, g := range st.games {
		switch {
		case g < 0:
		case int(g) == en.A:
			winsA++
		default:
			winsB++
		}
	}
	if en.Score[0] != winsA || en.Score[1] != winsB {
		return fmt.Errorf("event: SeriesEnded: score %d–%d does not match the games (%d–%d)",
			en.Score[0], en.Score[1], winsA, winsB)
	}
	st.ended = true
	games := append([]int8(nil), st.games...)
	e.results = append(e.results, finishedSeries{
		round: en.Round, a: en.A, b: en.B, score: en.Score, games: games,
		winner: seriesWinner(en.A, en.B, en.Score),
	})
	e.roundAdvance()
	return nil
}

// seriesWinner scores a series result: -1 for a drawn series, else the
// winning seat.
func seriesWinner(a, b int, score [2]int) int {
	switch {
	case score[0] > score[1]:
		return a
	case score[1] > score[0]:
		return b
	default:
		return -1
	}
}

// roundAdvance opens the next round once every current series has ended;
// after the event's last round it stays there until the EventEnded entry. A
// Swiss event stays in ROUND r instead: round r+1 opens when the driver
// logs its RoundPaired entry (see AwaitingPairing).
func (e *Event) roundAdvance() {
	for _, p := range e.pairings {
		st := e.series[p]
		if st == nil || !st.ended {
			return
		}
	}
	if e.structure == StructureSwiss {
		return
	}
	if e.round >= e.rounds {
		return
	}
	e.round++
	e.pairings = RoundRobinPairings(e.NumSeats(), e.round)
	e.series = map[[2]int]*seriesState{}
	e.phase = RoundPhase(e.round)
}

// applyEventEnded completes the event. A StructureNone event ends from
// BUILD once every seat is ready; a round-robin or Swiss event ends from
// its last round's phase (RoundPhase(rounds)) once every series of that
// round has ended.
func (e *Event) applyEventEnded(en *Entry) error {
	if !e.cooked {
		return fmt.Errorf("event: EventEnded: event not created")
	}
	if e.structure == StructureNone {
		// A no-games event ends from BUILD once every seat is ready.
		if e.phase != PhaseBuild {
			return fmt.Errorf("event: EventEnded: illegal in phase %s", e.phase)
		}
		for s := range e.ready {
			if e.ready[s] == 0 {
				return fmt.Errorf("event: EventEnded: seat %d is not ready", s)
			}
		}
	} else {
		if e.phase != RoundPhase(e.rounds) {
			return fmt.Errorf("event: EventEnded: illegal in phase %s", e.phase)
		}
		for _, p := range e.pairings {
			st := e.series[p]
			if st == nil || !st.ended {
				return fmt.Errorf("event: EventEnded: round %d series seats %d–%d not ended", e.round, p[0], p[1])
			}
		}
	}
	st := e.Standings()
	if len(en.Standings) > 0 && !standingsEqual(en.Standings, st) {
		return fmt.Errorf("event: EventEnded: caller standings do not match the event's results")
	}
	en.Standings = st
	e.phase = PhaseComplete
	return nil
}

// seriesEntryOK checks the guards shared by the three series entries: the
// event is created, a round phase is live, and the entry names a pairing of
// the current round in lower-seat-first order.
func (e *Event) seriesEntryOK(en *Entry) error {
	if !e.cooked {
		return fmt.Errorf("event: %s: event not created", en.Kind)
	}
	if e.round < 1 || e.phase != RoundPhase(e.round) {
		return fmt.Errorf("event: %s: illegal in phase %s", en.Kind, e.phase)
	}
	if en.Round != e.round {
		return fmt.Errorf("event: %s: round %d is not the current round %d", en.Kind, en.Round, e.round)
	}
	if en.A < 0 || en.B < 0 || en.A >= e.NumSeats() || en.B >= e.NumSeats() {
		return fmt.Errorf("event: %s: seats %d–%d out of range", en.Kind, en.A, en.B)
	}
	if en.A >= en.B {
		return fmt.Errorf("event: %s: seat a must be the lower seat (got %d–%d)", en.Kind, en.A, en.B)
	}
	if e.seriesFor(en.A, en.B) == nil {
		return fmt.Errorf("event: %s: seats %d–%d are not a pairing of round %d", en.Kind, en.A, en.B, en.Round)
	}
	return nil
}

// StateJSON marshals the event's derived state — phase, entrants, decks,
// pools, series results and standings — deterministically, so a replayed
// event can be compared byte for byte with the original.
func (e *Event) StateJSON() ([]byte, error) {
	type deckJSON struct {
		Rev  int        `json:"rev"`
		Deck build.Deck `json:"deck"`
	}
	type resultJSON struct {
		Round  int    `json:"round"`
		A      int    `json:"a"`
		B      int    `json:"b"`
		Score  [2]int `json:"score"`
		Games  []int8 `json:"games"`
		Winner int    `json:"winner"`
	}
	type snapshot struct {
		Phase Phase  `json:"phase"`
		Round int    `json:"round"`
		Seed  uint64 `json:"seed"`
		Set   string `json:"set"`
		// Mode, Structure, Packs and Rounds appear only when the event's
		// shape is not the default, so a 4-seat draft round-robin event
		// marshals without them.
		Mode      string              `json:"mode,omitempty"`
		Structure string              `json:"structure,omitempty"`
		Packs     int                 `json:"packs,omitempty"`
		Rounds    int                 `json:"rounds,omitempty"`
		Seats     []SeatSpec          `json:"seats"`
		Pools     [][]limited.CardRef `json:"pools"`
		Decks     [][]deckJSON        `json:"decks"`
		Ready     []int               `json:"ready"`
		Results   []resultJSON        `json:"results"`
		Standings []Standing          `json:"standings"`
	}
	n := e.NumSeats()
	s := snapshot{
		Phase: e.phase, Round: e.round, Seed: e.seed, Set: e.set,
		Seats:     append([]SeatSpec(nil), e.seats...),
		Ready:     append([]int(nil), e.ready...),
		Standings: e.Standings(),
	}
	if e.mode == ModeSealed {
		s.Mode = e.mode
	}
	if e.structure == StructureNone || e.structure == StructureSwiss {
		s.Structure = e.structure
	}
	if e.packsPer != e.defaultPacks() {
		s.Packs = e.packsPer
	}
	switch e.structure {
	case StructureRoundRobin:
		if n > 0 && e.rounds != n-1 {
			s.Rounds = e.rounds
		}
	case StructureSwiss:
		if n > 0 && e.rounds != swissDefaultRounds(n) {
			s.Rounds = e.rounds
		}
	}
	s.Pools = make([][]limited.CardRef, n)
	s.Decks = make([][]deckJSON, n)
	for seat := 0; seat < n; seat++ {
		if pool := e.Pool(seat); pool != nil {
			s.Pools[seat] = append([]limited.CardRef(nil), pool...)
		}
		for _, d := range e.decks[seat] {
			s.Decks[seat] = append(s.Decks[seat], deckJSON{Rev: d.rev, Deck: d.deck})
		}
	}
	for _, f := range e.results {
		s.Results = append(s.Results, resultJSON{
			Round: f.round, A: f.a, B: f.b, Score: f.score,
			Games: append([]int8(nil), f.games...), Winner: f.winner,
		})
	}
	return json.Marshal(&s)
}

// standingsEqual compares two standings tables field by field.
func standingsEqual(a, b []Standing) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
