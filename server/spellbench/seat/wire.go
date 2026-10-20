package seat

import (
	"crypto/sha256"
	"encoding/binary"

	"github.com/adams-shaun/gorge/engine/deck"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
	"github.com/adams-shaun/gorge/server/spellbench/wire"
)

const (
	protocolName  = protocol.Name
	protocolMinor = 0
)

type helloReq struct {
	RequestType   string `json:"request_type"`
	Protocol      string `json:"protocol"`
	ProtocolMinor uint32 `json:"protocol_minor"`
	RequestID     string `json:"request_id"`
}

type helloResp struct {
	ResponseType       string   `json:"response_type"`
	Protocol           string   `json:"protocol"`
	RequestID          string   `json:"request_id"`
	ExtensionsAccepted []string `json:"extensions_accepted"`
	Bot                struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"bot"`
	Requires struct {
		Observation []string `json:"observation"`
		Extensions  []string `json:"extensions"`
	} `json:"requires"`
}

type gameStartReq struct {
	RequestType   string          `json:"request_type"`
	Protocol      string          `json:"protocol"`
	RequestID     string          `json:"request_id"`
	AgentSeed     uint64          `json:"agent_seed"`
	Engine        protocol.Engine `json:"engine"`
	EngineProfile engineProfile   `json:"engine_profile"`
	Format        string          `json:"format"`
	GameID        string          `json:"game_id"`
	Limits        limits          `json:"limits"`
	OpponentDeck  *deckSpec       `json:"opponent_deck,omitempty"`
	OwnDeck       deckSpec        `json:"own_deck"`
	Resources     resources       `json:"resources"`
	Rules         gameRules       `json:"rules"`
	Seat          string          `json:"seat"`
	TimeControl   timeControl     `json:"time_control"`
}

type engineProfile struct {
	DecisionKinds  []string             `json:"decision_kinds"`
	EngineDefaults map[string]*string   `json:"engine_defaults"`
	Extensions     []protocol.Extension `json:"extensions"`
	Fairness       map[string]bool      `json:"fairness"`
	Observation    map[string]bool      `json:"observation"`
	Rewind         bool                 `json:"rewind"`
	RulesSupported map[string][]string  `json:"rules_supported"`
}

type limits struct {
	MaxDecisions            uint64 `json:"max_decisions"`
	MaxSeatDecisionsPerGame uint64 `json:"max_seat_decisions_per_game"`
	MaxSeatDecisionsPerTurn uint64 `json:"max_seat_decisions_per_turn"`
	MaxSeatStepsPerGame     uint64 `json:"max_seat_steps_per_game"`
	MaxSteps                uint64 `json:"max_steps"`
}

type deckSpec struct {
	DeckID   string             `json:"deck_id"`
	Decklist []protocol.DeckRow `json:"decklist"`
	Name     string             `json:"name"`
}

type resources struct {
	CPUs       int  `json:"cpus"`
	EngineCPUs int  `json:"engine_cpus"`
	GPU        bool `json:"gpu"`
	MemoryMB   int  `json:"memory_mb"`
}

type gameRules struct {
	CardNameDomain   nameDomain `json:"card_name_domain"`
	Extensions       []string   `json:"extensions"`
	Mulligan         string     `json:"mulligan"`
	OpponentDecklist string     `json:"opponent_decklist"`
	Probe            bool       `json:"probe"`
	StartingPlayer   string     `json:"starting_player"`
	StartingSeat     *string    `json:"starting_seat"`
}

type nameDomain struct {
	DomainID string   `json:"domain_id"`
	Names    []string `json:"names"`
}

type timeControl struct {
	BankMS        int64 `json:"bank_ms"`
	EngineStepMS  int64 `json:"engine_step_ms"`
	GameStartMS   int64 `json:"game_start_ms"`
	IncrementMS   int64 `json:"increment_ms"`
	MaxDecisionMS int64 `json:"max_decision_ms"`
	StartupMS     int64 `json:"startup_ms"`
}

type chooseReq struct {
	RequestType string                `json:"request_type"`
	Protocol    string                `json:"protocol"`
	RequestID   string                `json:"request_id"`
	GameID      string                `json:"game_id"`
	Clock       clock                 `json:"clock"`
	Decision    protocol.SeatDecision `json:"decision"`
}

type clock struct {
	MaxDecisionMS int64 `json:"max_decision_ms"`
	RemainingMS   int64 `json:"remaining_ms"`
}

type gameOverReq struct {
	RequestType string         `json:"request_type"`
	Protocol    string         `json:"protocol"`
	RequestID   string         `json:"request_id"`
	GameID      string         `json:"game_id"`
	Terminal    terminalNotice `json:"terminal"`
}

type terminalNotice struct {
	Classification string  `json:"classification"`
	Outcome        string  `json:"outcome"`
	Reason         string  `json:"reason"`
	SeatStepCount  uint64  `json:"seat_step_count"`
	Winner         *string `json:"winner"`
}

type response struct {
	ResponseType string `json:"response_type"`
	Protocol     string `json:"protocol"`
	RequestID    string `json:"request_id"`
	Selection    *struct {
		CandidateID uint32 `json:"candidate_id"`
	} `json:"selection"`
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func deckWire(m *deck.Manifest) deckSpec {
	spec := deckSpec{Name: m.Name}
	rows := make([]wire.DeckRow, 0, len(m.Main))
	for _, r := range m.Main {
		spec.Decklist = append(spec.Decklist, protocol.DeckRow{Name: r.Name, Count: uint32(r.Count)})
		rows = append(rows, wire.DeckRow{Name: r.Name, Count: r.Count})
	}
	spec.DeckID = wire.DeckID(rows)
	return spec
}

func agentSeed(secret []byte, gameID, seat string) uint64 {
	h := sha256.New()
	h.Write(secret)
	h.Write([]byte(gameID))
	h.Write([]byte{0})
	h.Write([]byte(seat))
	var b [8]byte
	copy(b[:], h.Sum(nil))
	return binary.BigEndian.Uint64(b[:])
}

func domainNames(names []string, decks ...*deck.Manifest) []string {
	seen := make(map[string]bool, len(names))
	out := make([]string, 0, len(names))
	add := func(n string) {
		if n != "" && !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	for _, n := range names {
		add(n)
	}
	for _, d := range decks {
		if d == nil {
			continue
		}
		for _, r := range d.Main {
			add(r.Name)
		}
		for _, r := range d.Sideboard {
			add(r.Name)
		}
		for _, n := range d.Commanders {
			add(n)
		}
	}
	return out
}
