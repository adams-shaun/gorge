package main

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"sync"

	"github.com/adams-shaun/gorge/host"
	"github.com/adams-shaun/gorge/host/httpapi"
	"github.com/adams-shaun/gorge/view"
)

// createGame returns the Options.CreateGame closure gorged arms so the
// browser can seat a human against a bot on demand (Task ui11). It builds a
// fresh single-shot 2-seat table in the requested format — seat 0 human,
// seat 1 bot — dealing the two seats decks picked at random from the
// format's pool.
//
// Seeding and replayability are the whole point of the "random" here: the
// game's seed is minted once, at create time, and it drives BOTH the deck
// order (host.SeededShuffle over the pool) and the match itself (via
// MatchSeed). The shuffled deck order is persisted verbatim in the
// TableConfig, and the seed is returned in the response and on the match's
// own wire records, so a viewer can replay the game from its configuration
// — the engine's deterministic-replay contract is preserved, not
// undercut, by a randomly chosen starting point.
//
// The closure is serialized: it allocates a table id (host.nextGameID,
// which observes the registry) and mints a token, and two concurrent
// requests must not hand each other the same id.
func (c config) createGame(r *host.Registry, gate *seatGate, cmdPool, conPool []string, vis view.Visibility) func(host.Format) (httpapi.CreateGameResponse, error) {
	var mu sync.Mutex
	return func(f host.Format) (httpapi.CreateGameResponse, error) {
		mu.Lock()
		defer mu.Unlock()
		pool := conPool
		if f == host.FormatCommander {
			pool = cmdPool
		}
		if len(pool) == 0 {
			return httpapi.CreateGameResponse{}, fmt.Errorf("no %s deck is available to deal (decks dir supplies none)", f)
		}
		seed, err := randomSeed()
		if err != nil {
			return httpapi.CreateGameResponse{}, err
		}
		// The shuffled pool is the table's deck order, so seat 0 (human) and
		// seat 1 (bot) of match 1 are dealt two distinct decks and the whole
		// assignment is a pure function of the seed.
		decks := host.SeededShuffle(seed, pool)
		id := host.NextGameID(r)
		tok, err := gate.mint(0)
		if err != nil {
			return httpapi.CreateGameResponse{}, err
		}
		cfg := host.TableConfig{
			ID: id, Name: fmt.Sprintf("Play vs bot (%s)", f), Seats: 2, Decks: decks,
			Seed: seed, PlayerNames: []string{"You", "Bot"}, Mulligans: c.mulligans,
			Spectator: vis, Perpetual: false, Humans: []int{0}, Format: f,
		}
		if err := r.AddTable(cfg); err != nil {
			return httpapi.CreateGameResponse{}, err
		}
		if err := r.Start(id); err != nil {
			return httpapi.CreateGameResponse{}, err
		}
		return httpapi.CreateGameResponse{Table: string(id), Match: 1, Seed: seed, Seat: 0,
			Token: tok, Join: fmt.Sprintf("/t/%s?seat=0&token=%s", id, tok)}, nil
	}
}

// randomSeed mints a fresh 64-bit game seed with crypto/rand, the one
// source of novelty in an otherwise fully deterministic game: everything
// after it is a pure function of the recorded configuration.
func randomSeed() (uint64, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(b[:]), nil
}
