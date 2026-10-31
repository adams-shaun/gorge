// Package wire is the gorge-agent/1 wire: the JSON-lines messages an
// out-of-process drafter, builder or player speaks with gorge. gorge drives
// every game and calls OUT to the agent over the child's stdin/stdout or a
// Unix socket; the agent only answers. One JSON object per line: gorge sends
// hello, batch, stream_end and bye; the agent answers hello, result and
// error. A batch carries pick, build or decide items whose payload is a
// protocol.DraftCard list, a seat's pool, or one deciding seat's view.View
// with the decision to answer; a result answers every item id exactly once
// with a pick, a protocol.DeckRequest, a decision.Intent or a per-item
// error. The protocol of record is
// docs/design/2026-10-14-draft-events-e5d-plan.md §4.
package wire

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/server/protocol"
)

const (
	// Protocol is the wire protocol's name; a hello whose protocol field
	// differs ends the session.
	Protocol = "gorge-agent/1"
	// Version is the wire version. It bumps only on a breaking change —
	// additive fields do not bump it, the same rule protocol.Version's
	// doc states for gorge's own wire.
	Version = 1
	// MaxLine is the most bytes one JSON line may carry; a longer line is a
	// protocol violation the reader refuses without buffering it whole.
	MaxLine = 64 << 20

	// Message types. gorge is the only writer of hello, batch, stream_end
	// and bye; the agent answers with hello, result and error.
	THello     = "hello"
	TBatch     = "batch"
	TResult    = "result"
	TStreamEnd = "stream_end"
	TBye       = "bye"
	TError     = "error"

	// Item kinds.
	KindPick   = "pick"
	KindBuild  = "build"
	KindDecide = "decide"

	// Per-item error codes. refused is an ordinary, answer-shaped decline of
	// one item; the other three appear only on a fatal error message.
	CodeRefused     = "refused"
	CodeBadRequest  = "bad_request"
	CodeUnsupported = "unsupported"
	CodeInternal    = "internal"
)

// Message is the one flat envelope every line carries. Every field is
// omitted when empty except V and Type; unknown fields are ignored by both
// sides, so additive evolution never bumps Version.
type Message struct {
	V    int    `json:"v"`
	Type string `json:"type"`
	// hello (both directions)
	Protocol      string `json:"protocol,omitempty"`
	GorgeRevision string `json:"gorge_revision,omitempty"`
	// hello (agent only)
	Agent          string   `json:"agent,omitempty"`
	ModelVersion   string   `json:"model_version,omitempty"`
	Kinds          []string `json:"kinds,omitempty"`
	MaxBatch       int      `json:"max_batch,omitempty"`
	SkipForced     bool     `json:"skip_forced,omitempty"`
	PaymentActions bool     `json:"payment_actions,omitempty"`
	TargetHints    bool     `json:"target_hints,omitempty"`
	ViewOmit       []string `json:"view_omit,omitempty"`
	// batch (gorge) / result (agent)
	Batch   uint64   `json:"batch,omitempty"`
	Items   []Item   `json:"items,omitempty"`
	Answers []Answer `json:"answers,omitempty"`
	// stream_end
	Stream string `json:"stream,omitempty"`
	// error
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

// Item is one request inside a batch. Which fields carry meaning is a
// function of Kind: pick reads set, round, pick_no, pack, picks and seen;
// build reads set, pool and deck_size; decide reads stream and view. Seed
// is always present: every policy seeds its rng from it.
type Item struct {
	ID   uint64 `json:"id"`
	Kind string `json:"kind"`
	Seed uint64 `json:"seed"`
	// pick
	Set    string                 `json:"set,omitempty"`
	Round  int                    `json:"round,omitempty"`
	PickNo int                    `json:"pick_no,omitempty"`
	Pack   []protocol.DraftCard   `json:"pack,omitempty"`
	Picks  []protocol.DraftCard   `json:"picks,omitempty"`
	Seen   [][]protocol.DraftCard `json:"seen,omitempty"`
	// build
	Pool     []protocol.DraftCard `json:"pool,omitempty"`
	DeckSize int                  `json:"deck_size,omitempty"`
	// decide
	Stream string          `json:"stream,omitempty"`
	View   json.RawMessage `json:"view,omitempty"`
}

// Answer is one reply inside a result: exactly one of Pick, Deck, Intent or
// Error. Pick is a card instance id from the item's pack; Deck is the built
// deck; Intent answers a decide item (gorge fills Seq and Player from the
// decision); Error declines the one item.
type Answer struct {
	ID     uint64                `json:"id"`
	Pick   string                `json:"pick,omitempty"`
	Deck   *protocol.DeckRequest `json:"deck,omitempty"`
	Intent *decision.Intent      `json:"intent,omitempty"`
	Error  *ItemError            `json:"error,omitempty"`
}

// ItemError declines one item inside an otherwise well-formed result.
type ItemError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Direction says who sent a message; Validate checks the shape for that
// sender.
type Direction int

const (
	// FromGorge marks a message gorge sent (hello, batch, stream_end, bye).
	FromGorge Direction = iota
	// FromAgent marks a message the agent sent (hello, result, error).
	FromAgent
)

// ErrProtocol is the wrap of every shape violation Validate reports; the
// message names the offending field.
var ErrProtocol = errors.New("wire: protocol violation")

// ErrLineTooLong is the Reader's refusal of a line over the cap.
var ErrLineTooLong = errors.New("wire: line exceeds the line cap")

var (
	agentNameRe    = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)
	modelVersionRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+-]{0,63}$`)
)

func dirName(d Direction) string {
	if d == FromAgent {
		return "agent"
	}
	return "gorge"
}

// Validate checks the message's shape for the direction it arrived from,
// wrapping ErrProtocol and naming the offending field in every error. It
// checks the message in isolation: whether a result's answer ids and answer
// kinds match the batch they answer is the session's check.
func (m Message) Validate(dir Direction) error {
	if m.V != Version {
		return fmt.Errorf("%w: field %q: got %d, want %d", ErrProtocol, "v", m.V, Version)
	}
	gorgeSends := map[string]bool{THello: true, TBatch: true, TStreamEnd: true, TBye: true}
	agentSends := map[string]bool{THello: true, TResult: true, TError: true}
	legal := gorgeSends
	if dir == FromAgent {
		legal = agentSends
	}
	if !legal[m.Type] {
		return fmt.Errorf("%w: field %q: type %q is not a %s message", ErrProtocol, "type", m.Type, dirName(dir))
	}
	switch m.Type {
	case THello:
		if m.Protocol != Protocol {
			return fmt.Errorf("%w: field %q: got %q, want %q", ErrProtocol, "protocol", m.Protocol, Protocol)
		}
		if dir == FromAgent {
			return m.validateAgentHello()
		}
	case TBatch:
		return m.validateBatch()
	case TResult:
		return m.validateResult()
	case TStreamEnd:
		if m.Stream == "" {
			return fmt.Errorf("%w: field %q: stream_end carries no stream", ErrProtocol, "stream")
		}
	case TError:
		if m.Code == "" {
			return fmt.Errorf("%w: field %q: error carries no code", ErrProtocol, "code")
		}
	}
	return nil
}

func (m Message) validateAgentHello() error {
	if !agentNameRe.MatchString(m.Agent) {
		return fmt.Errorf("%w: field %q: %q does not match %s", ErrProtocol, "agent", m.Agent, agentNameRe)
	}
	if !modelVersionRe.MatchString(m.ModelVersion) {
		return fmt.Errorf("%w: field %q: %q does not match %s", ErrProtocol, "model_version", m.ModelVersion, modelVersionRe)
	}
	if len(m.Kinds) == 0 {
		return fmt.Errorf("%w: field %q: agent hello declares no kinds", ErrProtocol, "kinds")
	}
	seen := map[string]bool{}
	for _, k := range m.Kinds {
		if k != KindPick && k != KindBuild && k != KindDecide {
			return fmt.Errorf("%w: field %q: unknown kind %q", ErrProtocol, "kinds", k)
		}
		if seen[k] {
			return fmt.Errorf("%w: field %q: kind %q declared twice", ErrProtocol, "kinds", k)
		}
		seen[k] = true
	}
	if m.MaxBatch < 0 {
		return fmt.Errorf("%w: field %q: %d is negative", ErrProtocol, "max_batch", m.MaxBatch)
	}
	if _, err := ParseViewOmit(m.ViewOmit); err != nil {
		return err
	}
	return nil
}

func (m Message) validateBatch() error {
	if m.Batch < 1 {
		return fmt.Errorf("%w: field %q: batch numbers start at 1, got %d", ErrProtocol, "batch", m.Batch)
	}
	if len(m.Items) == 0 {
		return fmt.Errorf("%w: field %q: batch carries no items", ErrProtocol, "items")
	}
	seen := map[uint64]bool{}
	for i, it := range m.Items {
		if seen[it.ID] {
			return fmt.Errorf("%w: items[%d]: field %q: %d appears twice", ErrProtocol, i, "id", it.ID)
		}
		seen[it.ID] = true
		if err := it.validate(); err != nil {
			return err
		}
	}
	return nil
}

func (it Item) validate() error {
	bad := func(format string, args ...any) error {
		return fmt.Errorf("%w: item %d (%s): %s", ErrProtocol, it.ID, it.Kind, fmt.Sprintf(format, args...))
	}
	switch it.Kind {
	case KindPick:
		if it.Set == "" {
			return bad("field %q is empty", "set")
		}
		if it.Round < 1 {
			return bad("field %q: got %d, want >= 1", "round", it.Round)
		}
		if it.PickNo < 1 {
			return bad("field %q: got %d, want >= 1", "pick_no", it.PickNo)
		}
		if len(it.Pack) == 0 {
			return bad("field %q is empty", "pack")
		}
	case KindBuild:
		if it.Set == "" {
			return bad("field %q is empty", "set")
		}
		if len(it.Pool) == 0 {
			return bad("field %q is empty", "pool")
		}
		if it.DeckSize < 40 {
			return bad("field %q: got %d, want >= 40", "deck_size", it.DeckSize)
		}
	case KindDecide:
		if it.Stream == "" {
			return bad("field %q is empty", "stream")
		}
		if len(it.View) == 0 {
			return bad("field %q is empty", "view")
		}
	default:
		return bad("unknown kind %q", it.Kind)
	}
	return nil
}

func (m Message) validateResult() error {
	if m.Batch < 1 {
		return fmt.Errorf("%w: field %q: batch numbers start at 1, got %d", ErrProtocol, "batch", m.Batch)
	}
	if len(m.Answers) == 0 {
		return fmt.Errorf("%w: field %q: result carries no answers", ErrProtocol, "answers")
	}
	seen := map[uint64]bool{}
	for i, a := range m.Answers {
		if seen[a.ID] {
			return fmt.Errorf("%w: answers[%d]: field %q: %d appears twice", ErrProtocol, i, "id", a.ID)
		}
		seen[a.ID] = true
		n := 0
		for _, set := range []bool{a.Pick != "", a.Deck != nil, a.Intent != nil, a.Error != nil} {
			if set {
				n++
			}
		}
		if n != 1 {
			return fmt.Errorf("%w: answers[%d] (id %d): carries %d of pick/deck/intent/error, want exactly one", ErrProtocol, i, a.ID, n)
		}
	}
	return nil
}

// Seed53 folds a 64-bit rng word into the integers below 2^53 every JSON
// implementation reads exactly.
func Seed53(x uint64) uint64 { return x >> 11 }
