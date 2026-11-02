// Package sbv2 serves the ENGINE role of SpellBench protocol v2 with the
// dirac kernel (dirac.State) as the authority for game flow.
//
// It mirrors gorge's cmd/spellbench-gorge-env (hello, validate_deck, reset,
// step over NDJSON on stdin/stdout) so an agent that plays through gorge's
// v2 environment plays here unchanged. Each game runs in lockstep
// (dirac/oracle.LockstepLed): the kernel leads, a gorge shadow
// (oracle.GorgeSide) follows it action by action, and gorge's
// server/spellbench mapping and observe layers build every v2 observation
// and candidate list from the SHADOW's pending decision. The agent's pick is
// translated to the canonical action it stands for (GorgeSide.CanonOfIntent
// and friends), applied to the kernel and mirrored into the shadow; any
// disagreement between the two sides (the decision set, the projection, the
// result) ends the game with the terminal reason
// engine_contract_failure:divergence, the oracle's report going to stderr and
// Server.Divergences.
//
// Decks are the engine catalog (catalog.Decks) plus any a caller adds
// (-decks DIR, Server.AddDeck), and only decks whose every card the FDN
// kernel set supports (cardset "fdn" Supported); validate_deck and reset
// refuse any other, naming the cards. Mana is paid by the engine planner
// with its plans exposed, the default of gorge's env.
//
//	sbv2dirac [-corpus DIR] [-decks DIR] [-source-revision REV]
package sbv2

import (
	"crypto/sha256"
	"errors"
	"io"
	"slices"
	"sort"
	"strings"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/oracle"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/server/spellbench/catalog"
	"github.com/adams-shaun/gorge/server/spellbench/env"
	"github.com/adams-shaun/gorge/server/spellbench/observe"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
	"github.com/adams-shaun/gorge/server/spellbench/secrets"
	"github.com/adams-shaun/gorge/server/spellbench/wire"
)

// adapterVersion is this environment's own version, beside gorge's.
const adapterVersion = "0.1.0"

// cached is one answered request: the SHA-256 of its line and the response.
type cached struct {
	sum  [32]byte
	resp []byte
}

// Server is the v2 engine role over the dirac kernel.
type Server struct {
	reg    *cards.Registry
	set    cardset.Set
	engine protocol.Engine
	game   *diracGame
	// gameIDs and cache follow gorge's env.Server: a game id is used once,
	// and every response since the last accepted reset is kept by request id.
	gameIDs map[string]bool
	cache   map[string]cached
	// extra holds decks a Go caller registered beside the catalog.
	extra map[string]catalog.Deck
	// divergences are the oracle's reports of the games that ended in one.
	divergences []*oracle.Divergence
	// kernelConfig, when set, edits the kernel side's config (not the
	// shadow's): a test's way to make the two sides disagree.
	kernelConfig func(*oracle.Config)
	// recordChance turns on the kernel's chance recording (SetRecordChance).
	recordChance bool
	// clean memoises card name -> lockstep-clean.
	clean map[string]bool
}

// New returns a server over reg, the full gorge corpus registry (a lockstep
// game needs the whole name universe, so not a subset).
func New(reg *cards.Registry, sourceRevision *string) (*Server, error) {
	set, err := cardset.ByName("fdn")
	if err != nil {
		return nil, err
	}
	return NewSet(reg, set, sourceRevision)
}

// NewSet is New over set instead of the supported FDN set: a parity harness plays
// the unverified cards too, through cardset.AllowUnverified.
func NewSet(reg *cards.Registry, set cardset.Set, sourceRevision *string) (*Server, error) {
	s := &Server{reg: reg, set: set, gameIDs: map[string]bool{}, cache: map[string]cached{}, clean: map[string]bool{}}
	e := env.EngineIdentity(sourceRevision, reg)
	e.Name = "dirac"
	e.Version = "dirac-" + e.Version + "/sbv2dirac-" + adapterVersion + "/autopay"
	e.RulesSnapshotID = "dirac+" + e.RulesSnapshotID
	e.CardPoolIdentity = "fdn-kernel+" + e.CardPoolIdentity
	s.engine = e
	return s, nil
}

// AddDeck makes d playable by its catalog_id beside the engine catalog. It is
// not part of the protocol.
func (s *Server) AddDeck(d catalog.Deck) {
	if s.extra == nil {
		s.extra = map[string]catalog.Deck{}
	}
	s.extra[d.CatalogID] = d
}

// Divergences are the oracle reports of the games that ended in a lockstep
// divergence, oldest first.
func (s *Server) Divergences() []*oracle.Divergence { return s.divergences }

// SetRecordChance records the kernel's random outcomes for the games that
// start after it (dirac.Config.RecordChance); Chance returns them. Recording
// changes no game.
func (s *Server) SetRecordChance(on bool) { s.recordChance = on }

// Chance is the current game's kernel random outcomes, in draw order, once the
// game is over (the lockstep goroutine has stopped). It is nil while the game
// runs, when recording is off, and when the kernel's transcript overflowed
// dirac.MaxChanceRuns. It is not part of the protocol: the transcript
// harness feeds it to a gorge host's ShufflePlanner.
func (s *Server) Chance() []oracle.ChanceDraw {
	if s.game == nil || !s.game.over() {
		return nil
	}
	return s.game.chance
}

// Close abandons a game in progress, releasing its goroutine and kernel table.
func (s *Server) Close() {
	if s.game != nil {
		s.game.abandon()
	}
}

func (s *Server) deckByID(id string) (catalog.Deck, bool) {
	if d, ok := catalog.ByID(id); ok {
		return d, true
	}
	d, ok := s.extra[id]
	return d, ok
}

// Decks are the catalog decks beside the added ones whose every card is
// lockstep-clean, in catalog_id order.
func (s *Server) Decks() []catalog.Deck {
	all := append([]catalog.Deck(nil), catalog.Decks()...)
	for _, d := range s.extra {
		all = append(all, d)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].CatalogID < all[j].CatalogID })
	var out []catalog.Deck
	for _, d := range all {
		if len(s.Unclean(d)) == 0 {
			out = append(out, d)
		}
	}
	return out
}

// Unclean lists the distinct cards of d the kernel set does not support (the
// lockstep-clean set), sorted. A card the corpus lacks counts as unclean.
func (s *Server) Unclean(d catalog.Deck) []string {
	supported := s.set.Supported()
	var bad []string
	for _, r := range d.Rows {
		ok, seen := s.clean[r.Name]
		if !seen {
			c, found := s.reg.Lookup(r.Name)
			if found {
				i, in := s.set.Lookup(frontName(c))
				ok = in && supported[i]
			}
			s.clean[r.Name] = ok
		}
		if !ok && !slices.Contains(bad, r.Name) {
			bad = append(bad, r.Name)
		}
	}
	sort.Strings(bad)
	return bad
}

// frontName is a compiled card's front face name (the kernel set's key).
func frontName(c *cards.Card) string {
	if len(c.Faces) > 0 && c.Faces[0] != nil && c.Faces[0].Name != "" {
		return c.Faces[0].Name
	}
	return c.Path
}

func marshal(v any) []byte {
	b, _ := wire.Canonical(v) // any valid layout is allowed; canonical keeps goldens stable
	return b
}

func errResp(id string, e *protocol.Error) []byte {
	return marshal(protocol.ErrorResponse{ResponseType: "error", Protocol: protocol.Name, RequestID: id,
		Error: protocol.ErrorBody{Code: e.Code, Message: e.Message}})
}

// Handle answers one request line.
func (s *Server) Handle(line []byte) []byte {
	req, perr := protocol.Decode(line)
	if perr != nil {
		return errResp(req.ID, perr)
	}
	sum := sha256.Sum256(line)
	if c, ok := s.cache[req.ID]; ok {
		if c.sum == sum {
			return c.resp
		}
		return errResp(req.ID, protocol.Errf(protocol.CodeRequestIDReuseMismatch, "request_id reused with a different payload"))
	}
	prev := s.game
	out := s.dispatch(req)
	if s.game != prev { // an accepted reset starts a new game
		clear(s.cache)
	}
	s.cache[req.ID] = cached{sum: sum, resp: out}
	return out
}

func (s *Server) dispatch(req protocol.Request) []byte {
	switch req.Type {
	case "hello":
		return marshal(s.hello(req.ID))
	case "reset":
		return s.reset(req)
	case "step":
		return s.stepReq(req)
	case "validate_deck":
		v := req.ValidateDeck
		if v.Format != "pauper-bo1" {
			return errResp(req.ID, protocol.Errf(protocol.CodeUnsupportedFormat, v.Format))
		}
		d, ok := s.deckByID(v.Deck.CatalogID)
		if v.Deck.IsDecklist || !ok {
			return errResp(req.ID, protocol.Errf(protocol.CodeUnsupportedDeck, "only the engine catalog decks are playable"))
		}
		if bad := s.Unclean(d); len(bad) > 0 {
			return errResp(req.ID, protocol.Errf(protocol.CodeUnsupportedDeck, "cards the kernel does not play in lockstep: "+strings.Join(bad, ", ")))
		}
		return marshal(protocol.DeckOK{ResponseType: "deck_ok", Protocol: protocol.Name, RequestID: req.ID})
	default: // probe_resample: this engine has no probe (Section 9.7)
		return errResp(req.ID, protocol.Errf(protocol.CodeUnsupportedRequest, req.Type+" is not implemented"))
	}
}

func (s *Server) hello(id string) protocol.HelloOK {
	cat := []protocol.CatalogDeck{}
	for _, d := range s.Decks() {
		cd := protocol.CatalogDeck{CatalogID: d.CatalogID, Name: d.Name}
		for _, r := range d.Rows {
			cd.Decklist = append(cd.Decklist, protocol.DeckRow{Name: r.Name, Count: uint32(r.Count)})
		}
		cat = append(cat, cd)
	}
	engineOrder, autopay := "engine_order", "engine_autopay"
	return protocol.HelloOK{ResponseType: "hello_ok", Protocol: protocol.Name, RequestID: id, ProtocolMinor: 0, Engine: s.engine,
		Formats: []string{"pauper-bo1"}, DeckSources: []string{"catalog"}, Catalog: cat,
		RulesSupported: map[string][]string{"mulligan": {"london", "none"}, "starting_player": {"host_assigned"}},
		Observation:    observe.Flags, DecisionKinds: env.DecisionKinds,
		EngineDefaults: map[string]*string{"trigger_order": nil, "replacement_order": nil, "combat_damage_assignment": &engineOrder, "mana_payment": &autopay},
		Rewind:         false, Fairness: map[string]bool{"noninterference_probe": false},
		Extensions: []protocol.Extension{}}
}

func (s *Server) respond(id string, g *diracGame) []byte {
	dec, term := g.Pending()
	if term != nil {
		t := *term
		t.RequestID = id
		return marshal(t)
	}
	d := *dec
	d.RequestID = id
	return marshal(d)
}

func (s *Server) reset(req protocol.Request) []byte {
	r := req.Reset
	switch {
	case s.game != nil && !s.game.over():
		return errResp(req.ID, protocol.Errf(protocol.CodeGameAlreadyActive, "a game is active"))
	case s.gameIDs[r.GameID]:
		return errResp(req.ID, protocol.Errf(protocol.CodeMalformedRequest, "game_id reused"))
	case r.Format != "pauper-bo1":
		return errResp(req.ID, protocol.Errf(protocol.CodeUnsupportedFormat, r.Format))
	}
	var decks [2][]*cards.Card
	var names [2]string
	for i, spec := range r.Decks {
		d, ok := s.deckByID(spec.CatalogID)
		if spec.IsDecklist || !ok {
			return errResp(req.ID, protocol.Errf(protocol.CodeUnsupportedDeck, "only the engine catalog decks are playable"))
		}
		if spec.DeckID != d.DeckID() {
			return errResp(req.ID, protocol.Errf(protocol.CodeDeckIDMismatch, "deck_id does not match "+d.CatalogID))
		}
		if bad := s.Unclean(d); len(bad) > 0 {
			return errResp(req.ID, protocol.Errf(protocol.CodeUnsupportedDeck, "cards the kernel does not play in lockstep: "+strings.Join(bad, ", ")))
		}
		cs, err := catalog.Resolve(s.reg, d)
		if err != nil {
			return errResp(req.ID, protocol.Errf(protocol.CodeUnsupportedDeck, err.Error()))
		}
		decks[i], names[i] = cs, d.CatalogID
	}
	switch {
	case r.Rules.StartingPlayer != "host_assigned":
		return errResp(req.ID, protocol.Errf(protocol.CodeUnsupportedRule, "starting_player"))
	case r.Rules.Probe:
		return errResp(req.ID, protocol.Errf(protocol.CodeUnsupportedRule, "probe"))
	}
	if dom := wire.DomainID(r.Rules.Names); dom != r.Rules.DomainID {
		return errResp(req.ID, protocol.Errf(protocol.CodeMalformedRequest, "card_name_domain.domain_id does not hash its distinct names"))
	}
	if len(r.Rules.Extensions) > 0 {
		return errResp(req.ID, protocol.Errf(protocol.CodeUnsupportedRule, "extension "+r.Rules.Extensions[0]))
	}
	sec, err := secrets.ParseGame(r.GameSecret)
	if err != nil {
		return errResp(req.ID, protocol.Errf(protocol.CodeMalformedRequest, err.Error()))
	}
	// Section 9.2: a game the engine cannot start is unsupported_deck, and
	// the failure consumes neither the game id nor a slot.
	g, err := startGame(s, r, sec, decks, names)
	if err != nil {
		return errResp(req.ID, protocol.Errf(protocol.CodeUnsupportedDeck, "engine could not start: "+err.Error()))
	}
	s.game = g
	s.gameIDs[r.GameID] = true
	return s.respond(req.ID, g)
}

func (s *Server) stepReq(req protocol.Request) []byte {
	switch {
	case s.game == nil:
		return errResp(req.ID, protocol.Errf(protocol.CodeStepBeforeReset, "no game"))
	case req.Step.GameID != s.game.id:
		return errResp(req.ID, protocol.Errf(protocol.CodeGameIDMismatch, "another game is active"))
	}
	if perr := s.game.step(req.Step); perr != nil {
		return errResp(req.ID, perr)
	}
	return s.respond(req.ID, s.game)
}

// Serve runs the stdio loop until stdin closes.
func Serve(r io.Reader, w io.Writer, s *Server) error {
	in := wire.NewReader(r)
	for {
		line, err := in.ReadLine()
		switch {
		case errors.Is(err, io.EOF):
			return nil
		case errors.Is(err, wire.ErrLineTooLong):
			if _, err := w.Write(append(errResp("", protocol.Errf(protocol.CodeMalformedJSON, "line exceeds 8 MiB")), '\n')); err != nil {
				return err
			}
			continue
		case err != nil:
			return err
		}
		if _, err := w.Write(append(s.Handle(line), '\n')); err != nil {
			return err
		}
	}
}
