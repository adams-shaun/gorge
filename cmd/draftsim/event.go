package main

import (
	"fmt"
	"math/rand/v2"

	"github.com/adams-shaun/gorge/draft/event"
	"github.com/adams-shaun/gorge/draft/limited"
	"github.com/adams-shaun/gorge/draft/limited/booster"
	"github.com/adams-shaun/gorge/draft/limited/build"
	"github.com/adams-shaun/gorge/draft/limited/draft"
	"github.com/adams-shaun/gorge/draft/limited/pickbot"
	"github.com/adams-shaun/gorge/engine/cards"
)

// pickSaltBase tags the per-seat pick-rng streams so they never collide
// with booster's own per-pack streams (booster salts seat<<32|pack).
const pickSaltBase = 0x5011C1

// gameSaltBase tags the per-game seed streams the same way.
const gameSaltBase = 0x6A111E

// pickRNG is a seat's pick-decision rng: seeded from the event seed,
// salted by seat, used in pick order only, so the picks are a pure
// function of (seed, seat, pick history).
func pickRNG(seed uint64, seat int) *rand.Rand {
	return rand.New(rand.NewPCG(seed, pickSaltBase<<32|uint64(seat)))
}

// gameSeed derives one game's engine seed from the event seed and the
// game's place in the event: a pure function of (seed, round, a, b, game).
func gameSeed(seed uint64, round, a, b, game int) uint64 {
	r := rand.New(rand.NewPCG(seed, gameSaltBase<<32|
		uint64(round)<<24|uint64(a)<<16|uint64(b)<<8|uint64(game)))
	return r.Uint64()
}

// gameRecord is one played game, in the shape runOneEvent reports and
// -verify re-checks.
type gameRecord struct {
	round    int
	a, b     int
	winner   *int // nil for a draw (including a watchdog stall)
	head     string
	turns    int
	intents  int
	stall    string // the watchdog that capped the game, "" for a finished one
	verified bool   // -verify replayed this game's chain against its log
}

// runOneEvent plays one full event: the packs, the draft (draft mode only),
// the build, then the structure's rounds as Bo1 series (the driver loop:
// a Swiss event pairs each round through a logged RoundPaired entry, a
// round-robin event's rounds are paired by the event itself, and a none
// event plays no series so EventEnded lands in BUILD). With verify set,
// every game is additionally replayed from its own chain the way
// cmd/mtgsim's -verify does. It returns the finished event and the per-
// game records.
func runOneEvent(r *runner, seed uint64, verify bool) (*event.Event, *eventResult, error) {
	sh := r.podShapeOr()
	n := sh.seats
	e := event.New()
	persona := drafterLabel(r)
	if sh.mode == event.ModeSealed {
		persona = "sealed"
	}
	seats := make([]event.SeatSpec, n)
	for i, p := range r.players {
		seats[i] = event.SeatSpec{
			Kind:    "bot",
			Name:    p,
			Persona: fmt.Sprintf("%s/%s/%s", r.spec.Code, persona, p),
		}
	}
	en := event.Entry{Kind: event.KindEventCreated, Seed: seed, Set: r.spec.Code, Seats: seats, Collation: sh.collation}
	// The shape fields are logged only when non-default, so the
	// default-flag logs stay byte-identical to the pre-shape-flag ones.
	if sh.mode != event.ModeDraft {
		en.Mode = sh.mode
	}
	if sh.structure != event.StructureRoundRobin {
		en.Structure = sh.structure
	}
	if sh.structure != event.StructureNone && sh.rounds != defaultRounds(sh.structure, n) {
		en.Rounds = sh.rounds
	}
	if err := e.Apply(en); err != nil {
		return nil, nil, err
	}

	packs, err := openPacks(r, seed)
	if err != nil {
		return nil, nil, err
	}
	for s := 0; s < n; s++ {
		for p := 0; p < sh.packs; p++ {
			if err := e.Apply(event.Entry{Kind: event.KindPackOpened, Round: p + 1, Seat: s, Cards: packs[s][p]}); err != nil {
				return nil, nil, fmt.Errorf("pack %d seat %d: %w", p+1, s, err)
			}
		}
	}

	// The draft: every seat picks whenever its queue head has a pack; the
	// pod's own turn order keeps the pools identical to any legal order. A
	// sealed event has no draft — its packs went straight into the pools.
	if sh.mode == event.ModeDraft {
		rngs := make([]*rand.Rand, n)
		for s := range rngs {
			rngs[s] = pickRNG(seed, s)
		}
		for e.Phase() == event.PhaseDraft {
			progressed := false
			for s := 0; s < n; s++ {
				v, ok := e.Next(s)
				if !ok {
					continue
				}
				id := r.drafter.Pick(v, rngs[s])
				if err := e.Apply(event.Entry{Kind: event.KindPicked, Round: v.Round, PickNo: v.PickNo, Seat: s, ID: id}); err != nil {
					return nil, nil, fmt.Errorf("pick seat %d round %d pick %d: %w", s, v.Round, v.PickNo, err)
				}
				progressed = true
			}
			if !progressed {
				return nil, nil, fmt.Errorf("draft stuck in phase %s with no legal pick", e.Phase())
			}
		}
	}

	decks, err := buildDecks(r, e)
	if err != nil {
		return nil, nil, err
	}

	er := &eventResult{decks: decks}
	if r.noGames {
		// Draft-only (the pool-shape tests): stop after the build phase.
		// The series stay pending, no EventEnded is applied and the caller
		// reads the pools and decks off the returned event.
		return e, er, nil
	}
	for {
		if round, ok := e.AwaitingPairing(); ok {
			pairs, err := e.SuggestedPairings()
			if err != nil {
				return nil, nil, err
			}
			if err := e.Apply(event.Entry{Kind: event.KindRoundPaired, Round: round, Pairs: pairs}); err != nil {
				return nil, nil, err
			}
			continue
		}
		pending := e.PendingSeries()
		if len(pending) == 0 {
			break
		}
		for _, sp := range pending {
			rec, err := playSeries(r, e, decks, sp, verify)
			if err != nil {
				return nil, nil, err
			}
			er.games = append(er.games, rec)
		}
	}
	if err := e.Apply(event.Entry{Kind: event.KindEventEnded}); err != nil {
		return nil, nil, err
	}
	er.standings = e.Standings()
	return e, er, nil
}

// podShapeOr returns the runner's resolved pod shape, filling the zero
// shape an options-literal runner carries (the tests build those directly)
// with the legacy defaults: the players list's length (or 4), draft mode,
// round robin, 3 packs per seat.
func (r *runner) podShapeOr() podShape {
	sh := r.shape
	if sh.seats == 0 {
		n := len(r.players)
		if n == 0 {
			n = draft.Seats
		}
		sh.seats = n
	}
	if sh.mode == "" {
		sh.mode = event.ModeDraft
	}
	if sh.structure == "" {
		sh.structure = event.StructureRoundRobin
	}
	if sh.packs == 0 {
		sh.packs = event.DraftPacks
	}
	if sh.rounds == 0 {
		sh.rounds = defaultRounds(sh.structure, sh.seats)
	}
	return sh
}

// drafterLabel names the drafter in the event log's Persona strings; the
// concrete v1 persona types are the only drafter the tool builds, so the
// label is fixed.
func drafterLabel(r *runner) string {
	if _, ok := r.drafter.(*pickbot.RatingColour); ok {
		return "rating-colour"
	}
	return "pickbot"
}

// openPacks opens the pod's packs from the event seed: out[seat][pack] is
// that seat's copy of that pack, IDs assigned once across the whole event
// in seat-major, pack-second order.
func openPacks(r *runner, seed uint64) ([][][]limited.CardRef, error) {
	sh := r.podShapeOr()
	ids := &booster.IDs{}
	return booster.OpenPod(r.spec, seed, sh.seats, sh.packs, ids, colourOfRegistry(r.reg))
}

// colourOfRegistry adapts the card corpus to booster's ColourOf: the
// card's colour identity as W/U/B/R/G bytes, in that order.
func colourOfRegistry(reg *cards.Registry) booster.ColourOf {
	const wubrg = "WUBRG"
	return func(name string) []byte {
		c, ok := reg.Lookup(name)
		if !ok {
			return nil
		}
		mask := c.ColourIdentity()
		if mask == 0 {
			return nil
		}
		out := make([]byte, 0, 5)
		for i := 0; i < len(wubrg); i++ {
			if mask&(1<<i) != 0 {
				out = append(out, wubrg[i])
			}
		}
		return out
	}
}

// buildDecks runs every seat through the builder and saves + readies the
// deck in the event, resolving each deck file into the engine's card
// pointers on the way.
func buildDecks(r *runner, e *event.Event) ([][]*cards.Card, error) {
	n := r.podShapeOr().seats
	decks := make([][]*cards.Card, n)
	for s := 0; s < n; s++ {
		pool := e.Pool(s)
		d := r.builder.Build(r.spec.Code, pool)
		f := build.ToDeckFile(pool, d, fmt.Sprintf("p%d %s draft", s, r.spec.Code))
		if err := e.Apply(event.Entry{Kind: event.KindDeckSaved, Seat: s, Rev: 1, Deck: &d}); err != nil {
			return nil, fmt.Errorf("seat %d deck save: %w", s, err)
		}
		if err := e.Apply(event.Entry{Kind: event.KindReady, Seat: s, Rev: 1}); err != nil {
			return nil, fmt.Errorf("seat %d ready: %w", s, err)
		}
		cs, err := f.Resolve(r.reg)
		if err != nil {
			return nil, fmt.Errorf("seat %d deck resolve: %w", s, err)
		}
		decks[s] = cs
		if r.verbose {
			fmt.Fprintf(r.out, "  seat %d deck: %d cards (%d main, %d basics)\n",
				s, len(cs), len(d.Main), d.Total()-len(d.Main))
		}
	}
	return decks, nil
}

// playSeries plays one Bo1 series end to end: SeriesStarted, the game,
// GameEnded with the chain head, SeriesEnded with the score.
func playSeries(r *runner, e *event.Event, decks [][]*cards.Card, sp event.SeriesSpec, verify bool) (gameRecord, error) {
	if err := e.Apply(event.Entry{Kind: event.KindSeriesStarted, Round: sp.Round, A: sp.A, B: sp.B, BestOf: sp.BestOf}); err != nil {
		return gameRecord{}, err
	}
	rec, err := playGame(r, e.Seed(), sp.Round, sp.A, sp.B, 1, decks[sp.A], decks[sp.B], verify)
	if err != nil {
		return gameRecord{}, fmt.Errorf("round %d seats %d–%d: %w", sp.Round, sp.A, sp.B, err)
	}
	rec.round = sp.Round
	ge := event.Entry{Kind: event.KindGameEnded, Round: sp.Round, A: sp.A, B: sp.B, Game: 1, Head: rec.head}
	if rec.winner != nil {
		w := *rec.winner
		ge.Winner = &w
	}
	if err := e.Apply(ge); err != nil {
		return gameRecord{}, err
	}
	score := [2]int{}
	switch {
	case rec.winner != nil && *rec.winner == sp.A:
		score[0] = 1
	case rec.winner != nil && *rec.winner == sp.B:
		score[1] = 1
	}
	if err := e.Apply(event.Entry{Kind: event.KindSeriesEnded, Round: sp.Round, A: sp.A, B: sp.B, Score: score}); err != nil {
		return gameRecord{}, err
	}
	fmt.Fprintf(r.out, "  round %d seat %d (%s) vs seat %d (%s): %s turns %d head %s%s\n",
		sp.Round, sp.A, r.players[sp.A], sp.B, r.players[sp.B],
		outcomeText(rec, sp), rec.turns, rec.head, verifyNote(rec))
	return rec, nil
}

// outcomeText spells one game's result for the game line: the winning
// seat's policy, "draw" for a drawn game, or the watchdog that capped it.
func outcomeText(rec gameRecord, sp event.SeriesSpec) string {
	switch {
	case rec.stall != "":
		return "stall(" + rec.stall + ")"
	case rec.winner == nil:
		return "draw"
	default:
		return fmt.Sprintf("winner seat %d", *rec.winner)
	}
}

// verifyNote suffixes a game line with its engine-level replay verdict
// when -verify replayed this game's chain.
func verifyNote(rec gameRecord) string {
	if !rec.verified {
		return ""
	}
	return " replay OK"
}
