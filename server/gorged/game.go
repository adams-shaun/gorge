package gorged

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"strings"
	"sync"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/host/httpapi"
)

// createGame returns the Options.CreateGame closure gorged arms so the
// browser can seat a human against a bot on demand. It builds a
// fresh single-shot 2-seat table in the requested format — seat 0 human,
// seat 1 bot. Each seat may request a deck from that format's pool; an
// omitted choice is assigned from the same seeded random order as before.
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
func (c config) createGame(r *host.Registry, gate *seatGate, cmdPool, conPool []string, vis view.Visibility) func(httpapi.CreateGameOptions) (httpapi.CreateGameResponse, error) {
	var mu sync.Mutex
	return func(req httpapi.CreateGameOptions) (httpapi.CreateGameResponse, error) {
		mu.Lock()
		defer mu.Unlock()
		seats, humans := req.Seats, req.Humans
		if seats == 0 {
			seats = 2
		}
		if humans == 0 {
			humans = 1
		}
		if err := c.checkPodShape(req, seats, humans); err != nil {
			return httpapi.CreateGameResponse{}, err
		}
		// An omitted policy takes the server's -bot-policy, not the
		// vocabulary's own default, so a demo can change who you play.
		name := req.BotPolicy
		if name == "" {
			// The client names no policy (the web picker's empty "server
			// default" choice), so a deck reserved for one is what picks it:
			// choosing that deck chooses that opponent. Both decks omitted keeps
			// the server's default.
			name = c.botPolicy
			for _, id := range []string{req.HumanDeck, req.BotDeck} {
				if owner := c.reservedPick(id); id != "" && owner != "" {
					name = owner
					break
				}
			}
		}
		policy, err := host.NormalizeBotPolicy(name)
		if err != nil {
			return httpapi.CreateGameResponse{}, err
		}
		// The offered set gates BOTH the default and the client's explicit
		// pick: -bot-policies is the listing this server
		// advertises, so a name it omits must be refused here too, or a
		// client could create a game it would never have been offered. An
		// empty set means every registered entry (the flag's own default and
		// a hand-built config), so this check is a no-op for a test that
		// never set the field.
		if len(c.botPolicies) > 0 && !containsString(c.botPolicies, policy) {
			return httpapi.CreateGameResponse{}, fmt.Errorf("bot policy %q is not offered on this server (offered: %s)", policy, strings.Join(c.botPolicies, ", "))
		}
		entry, _ := bots.Lookup(policy)
		// A policy that cannot seat the requested table (two seats unless the
		// request asks for a pod) could never play the game the request asked
		// for: name the gap rather than building a table the host would then
		// refuse.
		if entry.MaxSeats > 0 && entry.MaxSeats < seats {
			return httpapi.CreateGameResponse{}, fmt.Errorf("bot policy %q seats at most %d, want >= %d", policy, entry.MaxSeats, seats)
		}
		// The policy must play the requested format: offering a constructed-
		// only search entry on a commander request would compile a seat that
		// cannot answer the game.
		if !containsString(entry.Formats, req.Format.String()) {
			return httpapi.CreateGameResponse{}, fmt.Errorf("bot policy %q does not support the %s format (formats: %s)", policy, req.Format, strings.Join(entry.Formats, ", "))
		}
		// A policy that reads opponents'
		// decklists (bots.Info.Env) never plays a vs-bot table — this
		// request always seats a human. It covers both an explicit client
		// pick and the reserved-pool picker above, whose reservedPick hit
		// would otherwise name it here. A closed-list policy
		// (bots.Info.ClosedLists) does not read decklists and passes; the
		// host checks its seats at match start (server/host/closedlist.go).
		if bots.ReadsDecklists(policy) {
			return httpapi.CreateGameResponse{}, fmt.Errorf("bot policy %q reads opponents' decklists and cannot play at a table with a human", policy)
		}
		pool, otherPool := conPool, cmdPool
		otherFormat := host.FormatCommander
		if req.Format == host.FormatCommander {
			pool, otherPool = cmdPool, conPool
			otherFormat = host.FormatConstructed
		}
		// A policy with a reserved pool deals from it and from nothing
		// else, for both seats: the network saw only those decks, and the
		// human's deck shapes the states it reads as much as its own does.
		reservedPool, hasPool := c.policyPools[policy]
		if hasPool {
			pool, otherPool = reservedPool, nil
		}
		if err := checkReservedPool(req, policy, hasPool); err != nil {
			return httpapi.CreateGameResponse{}, err
		}
		// A pod in which every seat submits a deck deals nothing from the pool.
		if len(pool) == 0 && len(req.Decks) < seats {
			return httpapi.CreateGameResponse{}, fmt.Errorf("no %s deck is available to deal (decks dir supplies none)", req.Format)
		}
		for _, pick := range []struct {
			role string
			id   string
		}{{"human", req.HumanDeck}, {"bot", req.BotDeck}} {
			if pick.id == "" {
				continue
			}
			if containsDeck(pool, pick.id) {
				continue
			}
			if why := c.refusedDecks[pick.id]; why != "" {
				return httpapi.CreateGameResponse{}, fmt.Errorf("%s deck %q: %s", pick.role, pick.id, why)
			}
			if hasPool {
				return httpapi.CreateGameResponse{}, fmt.Errorf("%s deck %q is not in the %s pool: that bot plays only the decks it was trained on", pick.role, pick.id, policy)
			}
			if owner := c.reservedFor(pick.id); owner != "" {
				return httpapi.CreateGameResponse{}, fmt.Errorf("%s deck %q is reserved for bot policy %q", pick.role, pick.id, owner)
			}
			if containsDeck(otherPool, pick.id) {
				return httpapi.CreateGameResponse{}, fmt.Errorf("%s deck %q belongs to %s, not requested format %s", pick.role, pick.id, otherFormat, req.Format)
			}
			return httpapi.CreateGameResponse{}, fmt.Errorf("unknown %s deck %q", pick.role, pick.id)
		}
		// A submitted deck is checked before anything is minted or added, so a
		// refusal leaves no table, token or stored deck behind.
		submitted, reports, err := c.importSeatDecks(req)
		if err != nil {
			return httpapi.CreateGameResponse{}, err
		}
		seed, err := randomSeed()
		if err != nil {
			return httpapi.CreateGameResponse{}, err
		}
		shuffled := host.SeededShuffle(seed, pool)
		id := host.NextGameID(r)
		decks := shuffled
		var imported []string
		var route podRoute
		if seats == 2 && len(submitted) == 0 {
			if req.HumanDeck != "" || req.BotDeck != "" {
				human, bot := req.HumanDeck, req.BotDeck
				if human == "" {
					human = randomOpponent(shuffled, bot)
				}
				if bot == "" {
					bot = randomOpponent(shuffled, human)
				}
				// Match 1 rotates TableConfig.Decks by one: with this two-item
				// order seat 0 receives human and seat 1 receives bot.
				decks = []string{bot, human}
			}
		} else {
			want, ids, err := podDecks(req, shuffled, submitted, id, seats)
			if err != nil {
				return httpapi.CreateGameResponse{}, err
			}
			decks, imported = rotateForMatchOne(want), ids
			route = c.routePod(c.diracCheck, req.Format, seats, policy, req.PairedBot, humans > 1 && c.pairedBotShared,
				seatRouteDecks(want, ids, submitted))
		}
		// The request may set its own London allowance; an omitted field keeps the server
		// default. host.TableConfig.Validate still rejects a negative, so a
		// caller that skips the handler's 0..6 cap fails at AddTable.
		mulligans := c.mulligans
		if req.Mulligans != nil {
			mulligans = *req.Mulligans
		}
		if len(submitted) > 0 {
			if err := c.imports.putAll(imported, submitted); err != nil {
				return httpapi.CreateGameResponse{}, err
			}
		}
		cfg := host.TableConfig{
			ID: id, Name: podTableName(req.Format, seats), Seats: seats, Decks: decks,
			Seed: seed, PlayerNames: podPlayerNames(seats, humans), Mulligans: mulligans,
			Spectator: vis, Perpetual: false, Humans: humanSeats(humans), Format: req.Format,
			BotPolicy: policy, BotAutoPayMana: c.botAutoPayMana, BotCommonSense: c.botCommonSense, AutoMana: c.autoMana, OnDemand: true, PairedBot: req.PairedBot,
			AllowPairedBot: humans > 1 && c.pairedBotShared,
		}
		route, err = c.addRoutedTable(r, cfg, route)
		if err != nil {
			c.imports.drop(imported...)
			return httpapi.CreateGameResponse{}, err
		}
		// After AddTable, so a refused request leaves no token behind.
		grants := make([]httpapi.SeatGrant, humans)
		for seat := range grants {
			tok, err := gate.mint(id, state.PlayerID(seat))
			if err != nil {
				return httpapi.CreateGameResponse{}, err
			}
			grants[seat] = httpapi.SeatGrant{Seat: seat, Token: tok, Join: fmt.Sprintf("/t/%s?seat=%d&token=%s", id, seat, tok)}
		}
		if err := r.Start(id); err != nil {
			return httpapi.CreateGameResponse{}, err
		}
		resp := httpapi.CreateGameResponse{Table: string(id), Match: 1, Seed: seed, Seat: 0,
			Token: grants[0].Token, Join: grants[0].Join, BotPolicy: policy, Decks: reports,
			Engine: route.Engine, EngineReason: route.Reason}
		if seats > 2 || humans > 1 || len(submitted) > 0 {
			resp.Seats = grants
		}
		return resp, nil
	}
}

func containsDeck(pool []string, id string) bool {
	for _, candidate := range pool {
		if candidate == id {
			return true
		}
	}
	return false
}

// randomOpponent returns the first shuffled deck different from avoid when
// one exists. A one-deck pool necessarily produces a mirror match, matching
// the host's existing modulo assignment rather than making random unusable.
func randomOpponent(shuffled []string, avoid string) string {
	for _, id := range shuffled {
		if id != avoid {
			return id
		}
	}
	return shuffled[0]
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
