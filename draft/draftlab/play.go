package draftlab

import (
	"fmt"
	"sort"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/replay"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/bench"
)

// Entrant is one seat's input to a game: the deck it plays and the resolved
// player policy that pilots it.
type Entrant struct {
	Deck   Deck
	Player NamedPlayer
}

// Matchup is one game's two entrants, player 0 first.
type Matchup struct {
	A, B Entrant
}

// GameResult is one Bo1 game's result: what a training program needs to
// write a 17Lands-shaped game_data row, plus the engine chain head -verify
// re-checks. Every field is a pure function of the entrants and the seed.
type GameResult struct {
	Seed        uint64
	Round, Game int         // 0 outside a structured Play / event
	A, B        int         // entrant indexes; -1 when not applicable
	Players     [2]string   // AgentRef strings, player 0 first
	Decks       [2]string   // deck names
	DraftIDs    [2]string   // Deck.DraftID per player
	Main        [2][]string // the deck's card names, one per copy, sorted
	Sideboard   [2][]string // pool cards not in the main, sorted; nil for a list deck
	Winner      int         // 0 or 1 (player index); -1 for a draw or a stall
	Stall       string      // "", "turns", "intents"
	Turns       int
	Intents     int
	Starter     int         // the player on the play
	Mulligans   [2]int      // KMulligan asks answered with the "mulligan" option
	OpeningHand [2][]string // the kept hand at the first turn, sorted
	Drawn       [2][]string // cards put into hand from the library after the first turn began, sorted
	Head        string      // engine chain head
	// Tape is the game's recorded accepted intents; nil unless
	// Options.RecordTape. ReplayTape re-drives the game from it with no
	// player policy.
	Tape *Tape
}

// PlayGame plays one Bo1 game between entrants a (player 0) and b (player 1)
// with an explicit engine seed. It names the two players by their deck name
// ("p0"/"p1" for an empty name); E5-c3's events pass "p<seat>" instead. The
// result is a pure function of the entrants and the seed.
func (l *Lab) PlayGame(a, b Entrant, seed uint64) (GameResult, error) {
	names := [2]string{a.Deck.Name, b.Deck.Name}
	for i, n := range names {
		if n == "" {
			names[i] = fmt.Sprintf("p%d", i)
		}
	}
	return l.playGame(a, b, names, seed, false)
}

// playGame is the shared game body: PlayGame (deck-name labels) and Play
// (seat labels) both funnel through it. names is the engine config's deck
// identity for each player, not the GameResult.Decks display name. verbatim
// selects Ruling P2 (ReplayTape's contract): every seat answer is submitted
// verbatim and any Submit rejection is a fatal error, not the refusal
// ladder's fallback re-answer — it rides a parameter, never a Lab field,
// because games run concurrently through runGames pools.
func (l *Lab) playGame(a, b Entrant, names [2]string, seed uint64, verbatim bool) (GameResult, error) {
	deckA, namesA, err := l.resolveDeck(a.Deck)
	if err != nil {
		return GameResult{}, err
	}
	deckB, namesB, err := l.resolveDeck(b.Deck)
	if err != nil {
		return GameResult{}, err
	}
	s0, err := a.Player.New(seed ^ 1)
	if err != nil {
		return GameResult{}, fmt.Errorf("draftlab: seat 0: %w", err)
	}
	s1, err := b.Player.New(seed ^ 2)
	if err != nil {
		return GameResult{}, fmt.Errorf("draftlab: seat 1: %w", err)
	}
	env, err := l.envGate([2]NamedPlayer{a.Player, b.Player}, [2]seat.Seat{s0, s1})
	if err != nil {
		return GameResult{}, err
	}

	// E5-d: the GameScoped seats learn their game's begin here; its end is
	// signalled exactly once — right after the engine stops (before Verify's
	// replay and foldHand, so the barrier is released as soon as the engine
	// stops), and on every other return path through the guarded defer.
	_, endGame := beginGameScoped([2]seat.Seat{s0, s1})
	defer endGame()

	var mulligans [2]int
	cfg := l.gameConfig(names, seed, deckA, deckB)
	hooks := bench.Hooks{
		NewEngine: rules.NewStartingPlayerChoice,
		Setup:     func(e *rules.Engine) { e.AskStartingPlayer() },
		Decision: func(seatIdx int, d *decision.Decision, in decision.Intent, _ *botpolicy.Board) error {
			if d.Kind == decision.KMulligan {
				for _, idx := range in.Choices {
					if optionKind(d, idx) == "mulligan" {
						mulligans[seatIdx]++
					}
				}
			}
			return nil
		},
	}
	if env {
		hooks.EnvSeats = true
		hooks.HonestRoot = true
	}
	if verbatim {
		hooks.Submit = func(e *rules.Engine, _ int, d *decision.Decision, in decision.Intent) (bool, error) {
			if err := e.Submit(in); err != nil {
				return false, fmt.Errorf("tape replay, decision seq %d kind %s: %w", d.Seq, d.Kind, err)
			}
			return true, nil
		}
	}
	oc, eng, err := bench.PlayGame(cfg, []seat.Seat{s0, s1}, l.maxTurns(), l.maxIntents(), hooks)
	if err != nil {
		return GameResult{}, fmt.Errorf("draftlab: seed %d: %w", seed, err)
	}
	endGame()
	if l.opts.OnEngine != nil {
		l.opts.OnEngine(seed, eng)
	}

	res := GameResult{
		Seed:      seed,
		Round:     0,
		Game:      0,
		A:         -1,
		B:         -1,
		Players:   [2]string{a.Player.Ref.String(), b.Player.Ref.String()},
		Decks:     [2]string{a.Deck.Name, b.Deck.Name},
		DraftIDs:  [2]string{a.Deck.DraftID, b.Deck.DraftID},
		Main:      [2][]string{sortedCopy(namesA), sortedCopy(namesB)},
		Sideboard: [2][]string{sideboardOf(a.Deck), sideboardOf(b.Deck)},
		Turns:     int(oc.Turns),
		Intents:   oc.Intents,
		Starter:   oc.Starter,
		Mulligans: mulligans,
		Head:      eng.L.Head(),
	}
	switch {
	case bench.IsAbort(oc.StallOn):
		return GameResult{}, fmt.Errorf("draftlab: seed %d: engine aborted (%s): %s", seed, oc.StallOn, firstLine(oc.Livelock))
	case oc.StallOn != "":
		res.Winner, res.Stall = -1, oc.StallOn
	case oc.Draw:
		res.Winner = -1
	default:
		res.Winner = oc.WinnerSeat
	}
	if l.opts.RecordTape {
		res.Tape = &Tape{
			Names:     names,
			Mulligans: cfg.Mulligans,
			Intents:   append([]decision.Intent(nil), eng.L.Intents...),
		}
	}
	if l.opts.Verify && res.Stall == "" {
		if _, err := replay.Replay(eng.L, cfg); err != nil {
			return GameResult{}, fmt.Errorf("draftlab: seed %d: replay: %w", seed, err)
		}
	}
	res.OpeningHand, res.Drawn = foldHand(eng, [2]map[*cards.Card]string{nameMap(deckA, namesA), nameMap(deckB, namesB)})
	return res, nil
}

// envGate refuses an Env player unless Options.AllowDecklistBots is set, and
// reports whether either player reads every seat's real decklist (a
// NamedPlayer that declares ReadsDecklists, or whose constructed seat
// implements bots.EnvSeat -- fail closed). With the opt-in such a game runs
// through the bench Env driver (Hooks.EnvSeats, E5-c5), so the hosted wrapper
// is driven with its bots.Env instead of silently playing without it; the
// returned bit is what selects those hooks.
func (l *Lab) envGate(players [2]NamedPlayer, seats [2]seat.Seat) (bool, error) {
	return l.envGateResult(players[:], seats[:])
}

// envGateList is envGate over a player list: the event driver gates every
// seat's player before any game runs (seats nil then — no seat is
// constructed yet), while playGame gates the two players it constructed. It
// drops envGate's bit: the event driver only needs the admit-or-refuse
// verdict.
func (l *Lab) envGateList(players []NamedPlayer, seats []seat.Seat) error {
	_, err := l.envGateResult(players, seats)
	return err
}

// envGateResult is the one home for the Env gate: it fails closed on a
// NamedPlayer that declares ReadsDecklists or a constructed seat that
// implements bots.EnvSeat, refuses the first such player without the
// AllowDecklistBots opt-in, and reports whether any player reads every seat's
// real decklist. E5-c5 lifted the refusal: with the opt-in the player is
// admitted, and the returned bit selects the bench Env driver.
func (l *Lab) envGateResult(players []NamedPlayer, seats []seat.Seat) (bool, error) {
	env := false
	for i := 0; i < len(players); i++ {
		reads := players[i].ReadsDecklists
		if i < len(seats) {
			if _, ok := seats[i].(bots.EnvSeat); ok {
				reads = true
			}
		}
		if !reads {
			continue
		}
		env = true
		if !l.opts.AllowDecklistBots {
			return false, fmt.Errorf("%w: player %d (%s) reads every seat's real decklist; the lab's AllowDecklistBots opt-in (operator ruling 2026-10-13) is off", ErrDecklistBot, i, players[i].Ref)
		}
	}
	return env, nil
}

// gameConfig is one game's engine config, exactly cmd/draftsim's
// gameConfig: the two decks, the derived seed, the token table and name
// universe the engine's asks may need, and the lab's London mulligan round.
func (l *Lab) gameConfig(names [2]string, seed uint64, deckA, deckB []*cards.Card) rules.Config {
	return rules.Config{
		Seed:         seed,
		Names:        []string{names[0], names[1]},
		Decks:        [][]*cards.Card{deckA, deckB},
		Tokens:       l.reg.Tokens,
		NameUniverse: l.reg.Universe(),
		Mulligans:    l.mulliganRound(),
		LoopGuard:    l.loopGuard(),
	}
}

// loopGuard is Options.MaxObjs as the engine's watcher config; nil (the
// default thresholds, no cap) when it is off.
func (l *Lab) loopGuard() *rules.LoopGuard {
	if l.opts.MaxObjs <= 0 {
		return nil
	}
	return &rules.LoopGuard{MaxObjs: l.opts.MaxObjs}
}

// maxTurns is the effective turn watchdog: Options.MaxTurns or draftsim's
// default.
func (l *Lab) maxTurns() int {
	if l.opts.MaxTurns == 0 {
		return defaultMaxTurns
	}
	return l.opts.MaxTurns
}

// maxIntents is the effective intent watchdog: Options.MaxIntents or
// draftsim's default.
func (l *Lab) maxIntents() int {
	if l.opts.MaxIntents == 0 {
		return defaultMaxIntents
	}
	return l.opts.MaxIntents
}

// mulliganRound is the London mulligan round count rules.Config wants:
// Options.Mulligans or the draftsim default, with a negative value (no
// round) collapsed to Config's zero.
func (l *Lab) mulliganRound() int {
	m := l.opts.Mulligans
	switch {
	case m < 0:
		return 0
	case m == 0:
		return defaultMulligans
	}
	return m
}

// resolveDeck resolves a deck file into the engine's card pointers and
// returns, parallel to the cards, the deck-file entry name each resolved
// card came from (each entry expanded by Count). The name list is what maps
// a resolved *cards.Card back to the name the deck used — the 17Lands name,
// not the engine face name.
func (l *Lab) resolveDeck(d Deck) ([]*cards.Card, []string, error) {
	cs, err := d.File.Resolve(l.reg)
	if err != nil {
		return nil, nil, fmt.Errorf("draftlab: deck %q: %w", d.Name, err)
	}
	names := make([]string, 0, len(cs))
	for _, e := range d.File.Cards {
		for i := 0; i < e.Count; i++ {
			names = append(names, e.Name)
		}
	}
	if len(names) != len(cs) {
		return nil, nil, fmt.Errorf("draftlab: deck %q: resolved %d cards but named %d", d.Name, len(cs), len(names))
	}
	return cs, names, nil
}

// nameMap maps each resolved card pointer to the deck-file name it came
// from; the first name wins when a card appears more than once.
func nameMap(cs []*cards.Card, names []string) map[*cards.Card]string {
	m := make(map[*cards.Card]string, len(cs))
	for i, c := range cs {
		if _, ok := m[c]; !ok {
			m[c] = names[i]
		}
	}
	return m
}

// sortedCopy returns a sorted copy of names (nil stays nil).
func sortedCopy(names []string) []string {
	if len(names) == 0 {
		return nil
	}
	out := append([]string(nil), names...)
	sort.Strings(out)
	return out
}

// sideboardOf lists a built deck's pool cards not in its main deck, by
// name, sorted; nil for a list deck (no pool). The main deck's basics are
// not in the pool, so they never appear here.
func sideboardOf(d Deck) []string {
	if d.Build == nil || d.Pool == nil {
		return nil
	}
	inMain := make(map[string]bool, len(d.Build.Main))
	for _, id := range d.Build.Main {
		inMain[id] = true
	}
	var out []string
	for _, ref := range d.Pool {
		if !inMain[ref.ID] {
			out = append(out, ref.Name)
		}
	}
	return sortedCopy(out)
}

// foldHand folds the event log into each player's telemetry, splitting
// dzgorge's "cards seen in hand" fold in two: OpeningHand is the held set at
// the first TurnChange (the kept hand after the mulligan round and London
// bottoming), and Drawn is every card that moved from the library to hand
// after that first TurnChange. Both lists are sorted.
func foldHand(eng *rules.Engine, nameOf [2]map[*cards.Card]string) ([2][]string, [2][]string) {
	var opening, drawn [2][]string
	held := [2]map[state.ObjID]bool{{}, {}}
	started := false
	for _, ev := range eng.L.Events {
		switch ev.Kind {
		case events.TurnChange:
			if !started {
				started = true
				for p := 0; p < 2; p++ {
					for id := range held[p] {
						opening[p] = append(opening[p], handCardName(eng, nameOf[p], id))
					}
					sort.Strings(opening[p])
				}
			}
		case events.Draw, events.MoveZone:
			o := eng.G.Obj(ev.Obj)
			if o == nil || o.Card == nil || o.Owner > 1 {
				continue
			}
			if ev.To == state.ZHand && (ev.Kind == events.Draw || ev.From == state.ZLibrary) {
				held[o.Owner][ev.Obj] = true
				if started {
					drawn[o.Owner] = append(drawn[o.Owner], handCardName(eng, nameOf[o.Owner], ev.Obj))
				}
			} else if !started && ev.From == state.ZHand && ev.To == state.ZLibrary {
				delete(held[o.Owner], ev.Obj)
			}
		}
	}
	for p := 0; p < 2; p++ {
		sort.Strings(drawn[p])
	}
	return opening, drawn
}

// handCardName names one object's card by the deck-file name it resolved
// from, falling back to its front face's name.
func handCardName(eng *rules.Engine, m map[*cards.Card]string, id state.ObjID) string {
	c := eng.G.Obj(id).Card
	if n, ok := m[c]; ok {
		return n
	}
	if len(c.Faces) > 0 {
		return c.Faces[0].Name
	}
	return c.Path
}

// optionKind is the Kind of the option at idx in d, "" when idx names no
// option (cmd/dzgorge/play.go's optionKind).
func optionKind(d *decision.Decision, idx int) string {
	for _, o := range d.Options {
		if o.Index == idx {
			return o.Kind
		}
	}
	return ""
}

// firstLine cuts a diagnostic to its first line (cmd/draftsim/game.go's
// firstLine), for the game-abort error.
func firstLine(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			return s[:i]
		}
	}
	return s
}
