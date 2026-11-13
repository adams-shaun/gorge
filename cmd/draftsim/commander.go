package main

import (
	"fmt"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/bots/heuristic"
	"github.com/adams-shaun/gorge/draft/event"
	"github.com/adams-shaun/gorge/draft/limited"
	"github.com/adams-shaun/gorge/draft/limited/build"
	"github.com/adams-shaun/gorge/draft/limited/pickbot"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/deck"
	"github.com/adams-shaun/gorge/engine/replay"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/testkit/bench"
)

// The Commander-draft pod game's watchdogs: four players at 40 life take
// longer than a limited duel, so the caps are wider. A capped game is a
// drawn pod game.
const (
	podMaxTurns   = 200
	podMaxIntents = 150000
)

// commanderPersona is the -format commander drafter and builder: the
// Commander-draft persona, whatever -drafter and -builder name (they rate
// limited picks and 40-card builds).
func commanderPersona(lookup pickbot.CardLookup) (pickbot.Drafter, pickbot.Builder) {
	return &pickbot.CommanderDrafter{Lookup: lookup}, &pickbot.CommanderBuilder{Lookup: lookup}
}

// podGameSeed is a pod game's engine seed: gameSeed with both seats 0, a
// pairing no series ever plays.
func podGameSeed(eventSeed uint64, round int) uint64 {
	return gameSeed(eventSeed, round, 0, 0, 0)
}

// seatDeck is one seat's resolved Commander-draft deck: its cards and the
// commander indices rules.Config.Commanders wants.
type seatDeck struct {
	cards      []*cards.Card
	commanders []int
}

// resolveCommanderDecks resolves every seat's saved deck, the same path
// (SavedDeck → ToDeckFile → Resolve) live play and -verify share, after
// checking the card facts the event cannot see: the commanders are a legal
// single commander or Commander-draft pair, and every card sits inside
// their colour identity (rules.Config seats only a legal configuration, so
// an illegal deck would quietly play commander-less).
func resolveCommanderDecks(r *runner, ev *event.Event) ([]seatDeck, error) {
	out := make([]seatDeck, ev.NumSeats())
	for s := range out {
		d, ok := ev.SavedDeck(s)
		if !ok {
			return nil, fmt.Errorf("seat %d has no saved deck", s)
		}
		f := build.ToDeckFile(ev.Pool(s), d, fmt.Sprintf("p%d %s commander draft", s, r.spec.Code))
		cs, err := f.Resolve(r.reg)
		if err != nil {
			return nil, fmt.Errorf("seat %d deck resolve: %w", s, err)
		}
		idx := f.CommanderIndices()
		if err := checkCommanderDeck(cs, idx); err != nil {
			return nil, fmt.Errorf("seat %d: %w", s, err)
		}
		out[s] = seatDeck{cards: cs, commanders: idx}
	}
	return out, nil
}

// checkCommanderDeck enforces the Commander-draft card rules on a resolved
// deck: one commander-eligible card or a deck.IsDraftCommanderPair pair,
// and every nonbasic card inside the commanders' colour identity.
func checkCommanderDeck(cs []*cards.Card, idx []int) error {
	var ident uint8
	switch len(idx) {
	case 1:
		if !deck.IsCommanderEligible(cs[idx[0]]) {
			return fmt.Errorf("commander %s is not commander-eligible", cs[idx[0]].Faces[0].Name)
		}
	case 2:
		if !deck.IsDraftCommanderPair(cs[idx[0]], cs[idx[1]]) {
			return fmt.Errorf("commanders %s and %s are not a Commander-draft pair", cs[idx[0]].Faces[0].Name, cs[idx[1]].Faces[0].Name)
		}
	default:
		return fmt.Errorf("%d commanders, want 1 or 2", len(idx))
	}
	for _, i := range idx {
		ident |= cs[i].ColourIdentity()
	}
	for _, c := range cs {
		if len(c.Faces) > 0 && c.Faces[0].IsBasic() && c.Faces[0].IsLand() {
			continue
		}
		if c.ColourIdentity()&^ident != 0 {
			return fmt.Errorf("%s is outside the commanders' colour identity", c.Faces[0].Name)
		}
	}
	return nil
}

// playPodGames plays every pending free-for-all game of a pod event and
// logs each as a PodGameEnded entry.
func playPodGames(r *runner, e *event.Event, verify bool) ([]gameRecord, error) {
	decks, err := resolveCommanderDecks(r, e)
	if err != nil {
		return nil, err
	}
	var recs []gameRecord
	for {
		round, ok := e.PendingPodGame()
		if !ok {
			break
		}
		seed := podGameSeed(e.Seed(), round)
		oc, eng, err := drivePodGame(r, seed, decks)
		if err != nil {
			return nil, fmt.Errorf("pod game %d: %w", round, err)
		}
		rec := gameRecord{round: round, head: eng.L.Head(), turns: int(oc.Turns), intents: oc.Intents}
		if r.valueLog != nil && !bench.IsAbort(oc.StallOn) {
			if err := writeValueLine(r.valueLog, e.Seed(), round, seed, oc, eng, decks); err != nil {
				return nil, fmt.Errorf("pod game %d: -valuelog: %w", round, err)
			}
		}
		switch {
		case bench.IsAbort(oc.StallOn):
			return nil, fmt.Errorf("pod game %d seed %d: engine aborted (%s): %s", round, seed, oc.StallOn, firstLine(oc.Livelock))
		case oc.StallOn != "":
			rec.stall = oc.StallOn
		case oc.Draw:
		default:
			w := oc.WinnerSeat
			rec.winner = &w
		}
		if verify && rec.stall == "" {
			if _, err := replay.Replay(eng.L, podConfig(r, seed, decks)); err != nil {
				return nil, fmt.Errorf("pod game %d seed %d: replay: %w", round, seed, err)
			}
			rec.verified = true
		}
		en := event.Entry{Kind: event.KindPodGameEnded, Round: round, Head: rec.head}
		if rec.winner != nil {
			w := *rec.winner
			en.Winner = &w
		}
		if err := e.Apply(en); err != nil {
			return nil, err
		}
		fmt.Fprintf(r.out, "  game %d (%d-player commander, %s): %s turns %d head %s%s\n",
			round, len(decks), commanderNames(decks), outcomeText(rec, event.SeriesSpec{}), rec.turns, rec.head, verifyNote(rec))
		recs = append(recs, rec)
	}
	return recs, nil
}

// commanderLabel names a built deck's commanders from its pool.
func commanderLabel(pool []limited.CardRef, d build.Deck) string {
	out := ""
	for i, id := range d.Commanders {
		if i > 0 {
			out += " + "
		}
		for _, c := range pool {
			if c.ID == id {
				out += c.Name
				break
			}
		}
	}
	return out
}

// commanderNames lists each seat's commanders for the game line.
func commanderNames(decks []seatDeck) string {
	out := ""
	for s, d := range decks {
		if s > 0 {
			out += "; "
		}
		out += fmt.Sprintf("p%d ", s)
		for i, idx := range d.commanders {
			if i > 0 {
				out += " + "
			}
			out += d.cards[idx].Faces[0].Name
		}
	}
	return out
}

// replayPodGameHead re-plays one logged pod game from the log-rebuilt
// event's saved decks and returns its chain head, for -verify.
func replayPodGameHead(r *runner, ev *event.Event, round int) (string, error) {
	decks, err := resolveCommanderDecks(r, ev)
	if err != nil {
		return "", err
	}
	seed := podGameSeed(ev.Seed(), round)
	oc, eng, err := drivePodGame(r, seed, decks)
	if err != nil {
		return "", err
	}
	if oc.StallOn != "" {
		return "", fmt.Errorf("seed %d: replay ended as a %s stall", seed, oc.StallOn)
	}
	return eng.L.Head(), nil
}

// drivePodGame seats every event seat's policy at one Commander table and
// drives the game to its end or a watchdog cap, the way driveGame drives a
// duel.
func drivePodGame(r *runner, seed uint64, decks []seatDeck) (bench.Outcome, *rules.Engine, error) {
	seats := make([]seat.Seat, len(decks))
	for i := range decks {
		p := r.players[i]
		st, err := bots.New(p, bots.Options{
			Seed:        seed ^ uint64(i+1),
			AutoPayMana: true,
			Deps:        bots.Deps{Cards: r.reg},
		})
		if err != nil {
			return bench.Outcome{}, nil, fmt.Errorf("seat %d policy %q: %w", i, p, err)
		}
		if bot, ok := st.(*heuristic.Bot); ok {
			bot.WithMulligan(botpolicy.MulliganLands)
		}
		seats[i] = st
	}
	oc, eng, err := bench.PlayGame(podConfig(r, seed, decks), seats, podMaxTurns, podMaxIntents, bench.Hooks{
		NewEngine: rules.NewStartingPlayerChoice,
		Setup:     func(e *rules.Engine) { e.AskStartingPlayer() },
	})
	if err != nil {
		return bench.Outcome{}, nil, fmt.Errorf("seed %d: %w", seed, err)
	}
	return oc, eng, nil
}

// podConfig is one pod game's engine config: Commander rules at 40 life
// (CR 903.7; commander damage 21 is the format's own), each seat's
// commanders in the command zone, and the Commander-draft partner rule.
func podConfig(r *runner, seed uint64, decks []seatDeck) rules.Config {
	cfg := rules.Config{
		Seed:          seed,
		Tokens:        r.reg.Tokens,
		NameUniverse:  r.reg.Universe(),
		Mulligans:     draftMulligans,
		Format:        rules.FormatCommander,
		StartingLife:  40,
		DraftPartners: true,
	}
	for s, d := range decks {
		cfg.Names = append(cfg.Names, fmt.Sprintf("p%d", s))
		cfg.Decks = append(cfg.Decks, d.cards)
		cfg.Commanders = append(cfg.Commanders, d.commanders)
	}
	return cfg
}
