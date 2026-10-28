package agent

import (
	"encoding/json"
	"fmt"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/server/spellbench/neutral"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
	"github.com/adams-shaun/gorge/server/spellbench/wire"
	"github.com/adams-shaun/gorge/server/spellbench/xview"
)

// NeutralVersion versions the translation from v2 observations and
// candidates; it joins Version in a neutral agent's identity.
const NeutralVersion = "neutral-0.1.0"

// neutralGame is a seat playing another engine's game: each decision is
// translated from the observation and candidates alone (internal/neutral).
type neutralGame struct {
	session *neutral.Session
	line    []byte
}

// EnableNeutral makes the server read the v2 observation and candidates
// instead of x_gorge_view_v1, so it can play a game hosted by any engine.
// It needs the registry for printed card facts. Env and Search policies need
// an engine root and cannot play another engine's game.
func (s *Server) SetRegistry(reg *cards.Registry) { s.registry = reg }

func (s *Server) EnableNeutral() error {
	if s.identity.outOfScope() {
		return fmt.Errorf("%s", s.identity.scopeRefusal())
	}
	if s.registry == nil {
		return fmt.Errorf("the neutral world needs the pinned registry")
	}
	s.neutral = &neutralGame{}
	return nil
}

func (s *Server) startNeutral(q request) error {
	g := &neutralGame{session: neutral.NewSession(s.registry)}
	// The host's hello declares whether it pays plans itself (Section 7.1).
	// A manual host poses mana abilities as candidates and pays by a lowered
	// plan, never by a plan-only cast, so the session must not invent one.
	autoPay := false
	if m := q.EngineProfile.EngineDefaults["mana_payment"]; m != nil && *m == "engine_autopay" {
		autoPay = true
	}
	g.session.SetAutoPay(autoPay)
	for _, d := range []*searchDeck{q.OwnDeck, q.OpponentDeck} {
		if d == nil {
			continue
		}
		rows := make([]wire.DeckRow, len(d.Decklist))
		copy(rows, d.Decklist)
		if _, err := neutral.ResolveDeck(s.registry, rows); err != nil {
			return fmt.Errorf("deck %s: %w", d.Name, err)
		}
	}
	s.neutral = g
	return nil
}

func (s *Server) translate(q request) (xview.Payload, error) {
	var full struct {
		Decision protocol.SeatDecision `json:"decision"`
	}
	if err := json.Unmarshal(s.neutral.line, &full); err != nil {
		return xview.Payload{}, err
	}
	if s.neutral.session == nil {
		return xview.Payload{}, fmt.Errorf("choose before game_start")
	}
	p, err := s.neutral.session.Payload(&full.Decision)
	if err == nil && len(p.Ops) != len(q.Decision.Candidates) {
		err = fmt.Errorf("%d ops for %d candidates", len(p.Ops), len(q.Decision.Candidates))
	}
	return p, err
}
