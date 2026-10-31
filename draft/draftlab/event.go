package draftlab

import (
	"bytes"
	"fmt"

	"github.com/adams-shaun/gorge/draft/event"
	"github.com/adams-shaun/gorge/draft/limited/build"
	"github.com/adams-shaun/gorge/draft/limited/setdata"
)

// EventSpec shapes one full draft-lab event: the supply (set, collation,
// mode, packs), the shape (seats, structure, rounds) and the resolved
// agents each seat plays. Zero fields mean the event's own defaults.
type EventSpec struct {
	// Set is the set code the packs come from.
	Set string
	// Collation is the booster collation, "" = setdata's default.
	Collation setdata.Collation
	// Mode is "" (draft), event.ModeDraft or event.ModeSealed.
	Mode string
	// Seats is the seat count: 4 or 8.
	Seats int
	// Packs is the packs-per-seat supply; 0 = the mode's default (3 draft,
	// 6 sealed).
	Packs int
	// Structure is "" (round robin at 4 seats, Swiss at 8), RoundRobin,
	// Swiss or NoGames.
	Structure Structure
	// Rounds is the round count; 0 = the structure's default (round robin
	// n-1, Swiss ceil(log2 n), none 0).
	Rounds int
	// Drafters, Builders and Players each have one entry (every seat plays
	// it) or Seats entries. Drafters are ignored in sealed mode.
	Drafters []NamedDrafter
	Builders []NamedBuilder
	Players  []NamedPlayer
	// Seed is the event seed: the pod's packs, each seat's pick rng, the
	// Swiss tie-breaks and every game's engine seed derive from it.
	Seed uint64
}

// EventResult is one finished event: the replayable event log (JSON lines,
// EventCreated through EventEnded), every seat's built deck, every game
// played (round then pairing order), the final standings and the run's
// manifest.
type EventResult struct {
	// Log is the event log, marshalable and re-drivable event-for-event.
	Log []byte
	// Decks is each seat's built deck, in seat order.
	Decks []Deck
	// Games is every played game in round then pairing order.
	Games []GameResult
	// Standings is the final standings table, event.Standings order.
	Standings []event.Standing
	// Manifest is the run's provenance (kind "event") with the event
	// fields filled.
	Manifest Manifest
}

// defaultRounds is a structure's default round count at n seats: the
// event's own defaults (round robin n-1, Swiss ceil(log2 n), none 0), so
// the EventCreated entry names Rounds only when the spec named one too.
func defaultRounds(structure Structure, n int) int {
	switch structure {
	case Swiss:
		return swissRounds(n)
	case NoGames:
		return 0
	}
	return n - 1
}

// RunEvent runs one full event through the event state machine: an
// EventCreated entry, the pod's packs (one OpenPod call, applied
// seat-major), the draft or the sealed pools, every seat's build and
// ready, each round's series played with the event's seeds and logged
// (RoundPaired for Swiss rounds, SeriesStarted, GameEnded, SeriesEnded in
// pairing order), then EventEnded. The result is a pure function of the
// spec; no result depends on Options.Workers. A stall counts as a drawn
// game, as cmd/draftsim logs it. The refusal list mirrors the event's own
// validation: a seat count that is not 4 or 8, an unknown mode or
// structure, a round or pack count the event would refuse, an agent list
// that is neither 1 nor Seats, and an Env player (the same gate playGame
// applies).
func (l *Lab) RunEvent(s EventSpec) (EventResult, error) {
	n := s.Seats
	if n != 4 && n != 8 {
		return EventResult{}, fmt.Errorf("draftlab: RunEvent: got %d seats, want 4 or 8", n)
	}
	mode := s.Mode
	if mode == "" {
		mode = event.ModeDraft
	}
	if mode != event.ModeDraft && mode != event.ModeSealed {
		return EventResult{}, fmt.Errorf("draftlab: RunEvent: unknown mode %q, want %q or %q", s.Mode, event.ModeDraft, event.ModeSealed)
	}
	structure := s.Structure
	if structure == "" {
		if n == 8 {
			structure = Swiss
		} else {
			structure = RoundRobin
		}
	}
	switch structure {
	case RoundRobin, Swiss, NoGames:
	default:
		return EventResult{}, fmt.Errorf("draftlab: RunEvent: unknown structure %q, want %q, %q or %q", s.Structure, RoundRobin, Swiss, NoGames)
	}
	if structure == NoGames && s.Rounds != 0 {
		return EventResult{}, fmt.Errorf("draftlab: RunEvent: structure none takes no rounds, got %d", s.Rounds)
	}
	defRounds := defaultRounds(structure, n)
	if s.Rounds < 0 || (s.Rounds != 0 && (s.Rounds < 1 || s.Rounds >= n)) {
		return EventResult{}, fmt.Errorf("draftlab: RunEvent: %d rounds for %d seats, want 1..%d (0 = the structure's default %d)", s.Rounds, n, n-1, defRounds)
	}
	defPacks := event.DraftPacks
	if mode == event.ModeSealed {
		defPacks = event.DefaultSealedPacks
	}
	packsPer := s.Packs
	if packsPer == 0 {
		packsPer = defPacks
	}
	if mode == event.ModeDraft && s.Packs != 0 && s.Packs != event.DraftPacks {
		return EventResult{}, fmt.Errorf("draftlab: RunEvent: %d packs in draft mode, want %d (0 = the default)", s.Packs, event.DraftPacks)
	}
	if mode == event.ModeSealed && s.Packs != 0 && (s.Packs < 1 || s.Packs > event.MaxSealedPacks) {
		return EventResult{}, fmt.Errorf("draftlab: RunEvent: %d packs in sealed mode, want 1..%d (0 = the default %d)", s.Packs, event.MaxSealedPacks, event.DefaultSealedPacks)
	}
	if mode == event.ModeDraft && len(s.Drafters) != 1 && len(s.Drafters) != n {
		return EventResult{}, fmt.Errorf("draftlab: RunEvent: got %d drafters for %d seats, want 1 or %d", len(s.Drafters), n, n)
	}
	if len(s.Builders) != 1 && len(s.Builders) != n {
		return EventResult{}, fmt.Errorf("draftlab: RunEvent: got %d builders for %d seats, want 1 or %d", len(s.Builders), n, n)
	}
	if len(s.Players) != 1 && len(s.Players) != n {
		return EventResult{}, fmt.Errorf("draftlab: RunEvent: got %d players for %d seats, want 1 or %d", len(s.Players), n, n)
	}
	if err := l.envGateList(s.Players, nil); err != nil {
		return EventResult{}, fmt.Errorf("draftlab: RunEvent: %w", err)
	}
	spec, err := l.loadSpec(s.Set, s.Collation)
	if err != nil {
		return EventResult{}, err
	}
	set := spec.Code

	drafterOf := func(i int) NamedDrafter {
		if len(s.Drafters) == 1 {
			return s.Drafters[0]
		}
		return s.Drafters[i]
	}
	builderOf := func(i int) NamedBuilder {
		if len(s.Builders) == 1 {
			return s.Builders[0]
		}
		return s.Builders[i]
	}
	playerOf := func(i int) NamedPlayer {
		if len(s.Players) == 1 {
			return s.Players[0]
		}
		return s.Players[i]
	}

	seats := make([]event.SeatSpec, n)
	for i := range seats {
		persona := fmt.Sprintf("%s/sealed/%s/%s", set, builderOf(i).Ref, playerOf(i).Ref)
		if mode == event.ModeDraft {
			persona = fmt.Sprintf("%s/%s/%s/%s", set, drafterOf(i).Ref, builderOf(i).Ref, playerOf(i).Ref)
		}
		seats[i] = event.SeatSpec{Kind: "bot", Name: playerOf(i).Ref.String(), Persona: persona}
	}
	en := event.Entry{Kind: event.KindEventCreated, Seed: s.Seed, Set: set, Seats: seats}
	if mode != event.ModeDraft {
		en.Mode = mode
	}
	if string(structure) != event.StructureRoundRobin {
		en.Structure = string(structure)
	}
	if s.Packs != 0 && packsPer != defPacks {
		en.Packs = packsPer
	}
	if s.Rounds != 0 && s.Rounds != defRounds {
		en.Rounds = s.Rounds
	}
	if s.Collation != "" {
		en.Collation = string(s.Collation)
	}
	e := event.New()
	if err := e.Apply(en); err != nil {
		return EventResult{}, fmt.Errorf("draftlab: RunEvent: EventCreated: %w", err)
	}

	packs, err := l.OpenPod(s.Set, s.Collation, s.Seed, n, packsPer)
	if err != nil {
		return EventResult{}, fmt.Errorf("draftlab: RunEvent: %w", err)
	}
	for st := 0; st < n; st++ {
		for p := 0; p < packsPer; p++ {
			if err := e.Apply(event.Entry{
				Kind: event.KindPackOpened, Round: p + 1, Seat: st, Cards: packs[st][p],
			}); err != nil {
				return EventResult{}, fmt.Errorf("draftlab: RunEvent: pack round %d seat %d: %w", p+1, st, err)
			}
		}
	}
	if mode == event.ModeDraft {
		if err := runDraftLoop(e, n, drafterOf, s.Seed); err != nil {
			return EventResult{}, fmt.Errorf("draftlab: RunEvent: %w", err)
		}
	}

	decks := make([]Deck, n)
	for st := 0; st < n; st++ {
		d, err := l.buildNamed(e.Pool(st), builderOf(st), fmt.Sprintf("p%d %s %s", st, set, mode))
		if err != nil {
			return EventResult{}, fmt.Errorf("draftlab: RunEvent: seat %d: %w", st, err)
		}
		d.DraftID = fmt.Sprintf("%d-s%d", s.Seed, st)
		if err := e.Apply(event.Entry{Kind: event.KindDeckSaved, Seat: st, Rev: 1, Deck: d.Build}); err != nil {
			return EventResult{}, fmt.Errorf("draftlab: RunEvent: seat %d: DeckSaved: %w", st, err)
		}
		if err := e.Apply(event.Entry{Kind: event.KindReady, Seat: st, Rev: 1}); err != nil {
			return EventResult{}, fmt.Errorf("draftlab: RunEvent: seat %d: Ready: %w", st, err)
		}
		decks[st] = d
	}

	var games []GameResult
	for {
		if round, ok := e.AwaitingPairing(); ok {
			pairs, err := e.SuggestedPairings()
			if err != nil {
				return EventResult{}, fmt.Errorf("draftlab: RunEvent: swiss round %d: %w", round, err)
			}
			if err := e.Apply(event.Entry{Kind: event.KindRoundPaired, Round: round, Pairs: pairs}); err != nil {
				return EventResult{}, fmt.Errorf("draftlab: RunEvent: RoundPaired round %d: %w", round, err)
			}
			continue
		}
		pending := e.PendingSeries()
		if len(pending) == 0 {
			break
		}
		round := pending[0].Round
		roundGames, err := l.runGames(len(pending), func(i int) (GameResult, error) {
			sp := pending[i]
			gs := gameSeed(s.Seed, sp.Round, sp.A, sp.B, 1)
			res, err := l.playGame(
				Entrant{Deck: decks[sp.A], Player: playerOf(sp.A)},
				Entrant{Deck: decks[sp.B], Player: playerOf(sp.B)},
				[2]string{fmt.Sprintf("p%d", sp.A), fmt.Sprintf("p%d", sp.B)}, gs, false)
			if err != nil {
				return GameResult{}, err
			}
			res.Round, res.Game, res.A, res.B = sp.Round, 1, sp.A, sp.B
			return res, nil
		})
		if err != nil {
			return EventResult{}, fmt.Errorf("draftlab: RunEvent: round %d: %w", round, err)
		}
		for i, sp := range pending {
			g := roundGames[i]
			if err := e.Apply(event.Entry{Kind: event.KindSeriesStarted, Round: sp.Round, A: sp.A, B: sp.B, BestOf: 1}); err != nil {
				return EventResult{}, fmt.Errorf("draftlab: RunEvent: round %d: SeriesStarted: %w", sp.Round, err)
			}
			ge := event.Entry{Kind: event.KindGameEnded, Round: sp.Round, A: sp.A, B: sp.B, Game: 1, Head: g.Head}
			if g.Winner >= 0 {
				w := sp.A
				if g.Winner == 1 {
					w = sp.B
				}
				ge.Winner = &w
			}
			if err := e.Apply(ge); err != nil {
				return EventResult{}, fmt.Errorf("draftlab: RunEvent: round %d: GameEnded: %w", sp.Round, err)
			}
			score := [2]int{}
			switch g.Winner {
			case 0:
				score[0] = 1
			case 1:
				score[1] = 1
			}
			if err := e.Apply(event.Entry{Kind: event.KindSeriesEnded, Round: sp.Round, A: sp.A, B: sp.B, Score: score}); err != nil {
				return EventResult{}, fmt.Errorf("draftlab: RunEvent: round %d: SeriesEnded: %w", sp.Round, err)
			}
			games = append(games, g)
		}
	}
	if err := e.Apply(event.Entry{Kind: event.KindEventEnded}); err != nil {
		return EventResult{}, fmt.Errorf("draftlab: RunEvent: EventEnded: %w", err)
	}
	log, err := event.MarshalLog(e.Log())
	if err != nil {
		return EventResult{}, fmt.Errorf("draftlab: RunEvent: %w", err)
	}
	res := EventResult{Log: log, Decks: decks, Games: games, Standings: e.Standings()}
	res.Manifest = l.eventManifest(spec, mode, structure, n, s, games)
	return res, nil
}

// eventManifest fills a "games" manifest for an event and then its event
// fields: the resolved set/collation, the supply mode, the series
// structure and the seat count.
func (l *Lab) eventManifest(spec setdata.Spec, mode string, structure Structure, n int, s EventSpec, games []GameResult) Manifest {
	agents := make([]RoleRef, 0, len(s.Drafters)+len(s.Builders)+len(s.Players))
	if mode == event.ModeDraft {
		for _, d := range s.Drafters {
			agents = append(agents, RoleRef{Role: RoleDrafter, Ref: d.Ref})
		}
	}
	for _, b := range s.Builders {
		agents = append(agents, RoleRef{Role: RoleBuilder, Ref: b.Ref})
	}
	for _, p := range s.Players {
		agents = append(agents, RoleRef{Role: RolePlayer, Ref: p.Ref})
	}
	m := l.NewManifest("event", s.Seed, agents, games)
	m.Set = spec.Code
	m.Collation = string(spec.Collation)
	m.Mode = mode
	m.Structure = string(structure)
	m.Seats = n
	return m
}

// VerifyEvent re-checks a finished event result the way cmd/draftsim's
// -verify does: the log replays event-for-event and re-marshals to the
// identical bytes, and every logged GameEnded re-drives from the replayed
// event's own saved decks and pools with the same seeds — the replayed
// chain head must equal the logged one. players are the player policies
// to re-drive with, one entry or one per seat (the same rule RunEvent
// takes). A stalled replay is an error: a capped game's chain is not
// verified.
func (l *Lab) VerifyEvent(r EventResult, players []NamedPlayer) error {
	ev, err := verifyEventLog(r)
	if err != nil {
		return fmt.Errorf("draftlab: VerifyEvent: %w", err)
	}
	n := ev.NumSeats()
	if len(players) != 1 && len(players) != n {
		return fmt.Errorf("draftlab: VerifyEvent: %d players for %d seats, want 1 or %d", len(players), n, n)
	}
	mode, set := ev.Mode(), ev.SetCode()
	for _, en := range ev.Log() {
		if en.Kind != event.KindGameEnded {
			continue
		}
		head, err := l.replayGameHead(ev, set, mode, players, en.Round, en.A, en.B, en.Game)
		if err != nil {
			return fmt.Errorf("draftlab: VerifyEvent: game round %d seats %d–%d: %w", en.Round, en.A, en.B, err)
		}
		if head != en.Head {
			return fmt.Errorf("draftlab: VerifyEvent: game round %d seats %d–%d: replayed chain head %s, log has %s", en.Round, en.A, en.B, head, en.Head)
		}
	}
	return nil
}

// verifyEventLog is the log half of VerifyEvent, shared with
// VerifyEventTapes: the log replays event-for-event and re-marshals to the
// identical bytes, and the replayed event carries the manifest seed.
func verifyEventLog(r EventResult) (*event.Event, error) {
	ev, err := event.ReplayLog(r.Log)
	if err != nil {
		return nil, fmt.Errorf("event log replay: %w", err)
	}
	again, err := event.MarshalLog(ev.Log())
	if err != nil {
		return nil, fmt.Errorf("event log re-marshal: %w", err)
	}
	if !bytes.Equal(again, r.Log) {
		return nil, fmt.Errorf("event log replay diverges at %s", firstDiffLine(r.Log, again))
	}
	if ev.Seed() != r.Manifest.Seed {
		return nil, fmt.Errorf("event log replay carries seed %d, manifest says %d", ev.Seed(), r.Manifest.Seed)
	}
	return ev, nil
}

// eventDeck rebuilds seat st's deck from the replayed event's own state: its
// saved deck rewritten as a deck file over the event's pool (the way
// RunEvent's build stage did). Shared by VerifyEvent's re-drive and
// VerifyEventTapes.
func eventDeck(ev *event.Event, set, mode string, st int) (Deck, error) {
	d, ok := ev.SavedDeck(st)
	if !ok {
		return Deck{}, fmt.Errorf("seat %d has no saved deck in the log", st)
	}
	f := build.ToDeckFile(ev.Pool(st), d, fmt.Sprintf("p%d %s %s", st, set, mode))
	return Deck{Name: f.Name, File: f}, nil
}

// replayGameHead re-drives one logged event game from the replayed event's
// own state: seat seat's saved deck is rewritten as a deck file over the
// event's pool (the way RunEvent's build stage did), the two files are
// resolved against the lab's registry, and the game plays with the event's
// own game seed (gameSeed(event seed, round, a, b, game)).
func (l *Lab) replayGameHead(ev *event.Event, set, mode string, players []NamedPlayer, round, a, b, game int) (string, error) {
	da, err := eventDeck(ev, set, mode, a)
	if err != nil {
		return "", err
	}
	db, err := eventDeck(ev, set, mode, b)
	if err != nil {
		return "", err
	}
	gs := gameSeed(ev.Seed(), round, a, b, game)
	res, err := l.playGame(
		Entrant{Deck: da, Player: players[a%len(players)]},
		Entrant{Deck: db, Player: players[b%len(players)]},
		[2]string{fmt.Sprintf("p%d", a), fmt.Sprintf("p%d", b)}, gs, false)
	if err != nil {
		return "", err
	}
	if res.Stall != "" {
		return "", fmt.Errorf("seed %d: replay ended as a %s stall", gs, res.Stall)
	}
	return res.Head, nil
}

// firstDiffLine names the first JSON log line where want and got part
// ways, for the -verify failure message. It is cmd/draftsim's run.go
// firstDiffLine, copied verbatim.
func firstDiffLine(want, got []byte) string {
	wl := bytes.Split(bytes.TrimRight(want, "\n"), []byte("\n"))
	gl := bytes.Split(bytes.TrimRight(got, "\n"), []byte("\n"))
	for i := 0; i < len(wl) && i < len(gl); i++ {
		if !bytes.Equal(wl[i], gl[i]) {
			return fmt.Sprintf("log line %d", i+1)
		}
	}
	if len(wl) != len(gl) {
		return fmt.Sprintf("log length %d vs %d", len(gl), len(wl))
	}
	return "an unknown position"
}
