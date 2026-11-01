package conformance

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"runtime"
	"testing"

	"github.com/adams-shaun/gorge/engine/game"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/server/host/httpapi"
	"github.com/adams-shaun/gorge/server/host/manabrewhttp"
	"github.com/adams-shaun/gorge/server/manabrew/mbtest"
	mb "github.com/adams-shaun/gorge/server/protocol/manabrew"
)

const manaBrewMatch = "/" + tableName + "/matches/1"

// ManaBrew is the ManaBrew HTTP transport: a host table in process with every
// seat human, read through GET .../state and answered through POST .../send. It
// polls rather than reading the stream because stream framing depends on
// goroutine timing by design, and both routes share one translation.
type ManaBrew struct{}

func (ManaBrew) Name() string { return "manabrew" }

// Open starts c on e.
func (ManaBrew) Open(t testing.TB, e game.Engine, c game.Config) Session {
	t.Helper()
	r := startTable(t, "manabrew", e, c)
	s := &ManaBrewSession{t: t, spectator: httpapi.NewHandler(r, httpapi.Options{}), gameOver: make([][]byte, len(c.Decks))}
	for i := range c.Decks {
		s.seats = append(s.seats, manabrewhttp.NewHandler(r, manabrewhttp.Options{
			Seat: func(*http.Request) (manabrewhttp.SeatClaim, bool) {
				return manabrewhttp.SeatClaim{Table: tableName, Seat: state.PlayerID(i)}, true
			},
		}))
	}
	return s
}

// ManaBrewSession is one ManaBrew client per seat of a game.
type ManaBrewSession struct {
	// Synthetic counts the reveal and dice prompts delivered and acknowledged.
	// They are not in any frame: a client answers them and nothing changes.
	Synthetic int
	// Windows counts the payManaCost prompts this session answered itself.
	Windows int

	t         testing.TB
	seats     []http.Handler
	spectator http.Handler
	gameOver  [][]byte // each seat's terminal prompt, once delivered

	asked   int               // the seat asked in the last frame
	prompt  *mb.PromptMessage // what it was asked; nil at the end of the game
	answers manaBrewAnswers   // what the last frame's Answers stand for
}

func (s *ManaBrewSession) Next() (Frame, bool) {
	s.t.Helper()
	raw := s.settle()
	f, answers, err := normaliseManaBrew(raw)
	if err != nil {
		s.t.Fatalf("manabrew wire: %v", err)
	}
	s.answers = answers
	return f, s.prompt != nil
}

func (s *ManaBrewSession) Answer(key string) error {
	s.t.Helper()
	if s.prompt == nil {
		return errors.New("the game is over")
	}
	out, ok := s.answers[key]
	if !ok {
		return fmt.Errorf("answer %q is not on offer", key)
	}
	if err := s.send(s.asked, s.prompt, out); err != nil {
		for s.prompt = nil; s.prompt == nil; runtime.Gosched() {
			s.read(s.asked)
		}
		return err
	}
	return nil
}

// settle waits for a seat to be asked something or for the match to finish, and
// returns that point's read. It reads every seat again once one is asked, since
// a seat read earlier in the pass may have been read before the engine parked.
// This package reads no clock, so a hung engine is left to go test's -timeout.
func (s *ManaBrewSession) settle() manaBrewRaw {
	s.t.Helper()
	for {
		over := tableFinished(s.t, "manabrew", s.spectator)
		s.prompt = nil
		raw := manaBrewRaw{GameOver: s.gameOver}
		for i := range s.seats {
			raw.Seats = append(raw.Seats, s.read(i))
		}
		switch {
		case raw.Seats[0] == nil:
			// The match has not begun.
		case s.prompt != nil:
			for i := range s.seats {
				raw.Seats[i] = s.read(i)
			}
			raw.Asked, raw.Prompt = s.asked, s.prompt
			return raw
		case over:
			return raw
		}
		runtime.Gosched()
	}
}

// read polls one seat and returns its state, or nil before the match begins. It
// acknowledges what is ack-only, answers a payment window, and records an
// ordinary prompt as what the seat is asked.
func (s *ManaBrewSession) read(seat int) []byte {
	s.t.Helper()
	path := manaBrewMatch + "/state"
	status, body := nativeDo(s.seats[seat], http.MethodGet, path, nil)
	if status == http.StatusNotFound {
		return nil
	}
	if status != http.StatusOK {
		s.t.Fatalf("manabrew wire: GET %s: status %d: %s", path, status, body)
	}
	var msgs []json.RawMessage
	if err := json.Unmarshal(body, &msgs); err != nil {
		s.t.Fatal(err)
	}
	if len(msgs) == 0 {
		s.t.Fatalf("manabrew wire: seat %d: an empty reply", seat)
	}
	for _, m := range msgs[1:] {
		var em mb.EngineMessage
		if _, err := mb.Decode(m, &em); err != nil {
			s.t.Fatalf("manabrew wire: seat %d: %v: %s", seat, err, m)
		}
		pm, ok := em.Value.(mb.PromptMessage)
		if !ok {
			s.t.Fatalf("manabrew wire: seat %d: %T after the state", seat, em.Value)
		}
		switch in := pm.Input.Value.(type) {
		case mb.GameOverInput:
			if s.gameOver[seat] != nil {
				s.t.Fatalf("manabrew wire: seat %d: a second gameOver prompt", seat)
			}
			s.gameOver[seat] = m
		case mb.PayManaCostInput:
			if !s.settleWindow(seat, &pm, in) {
				s.asked, s.prompt = seat, &pm
			}
		default:
			if pm.PromptID < 0 {
				s.acknowledge(seat, pm)
				continue
			}
			s.asked, s.prompt = seat, &pm
		}
	}
	return msgs[0]
}

func (s *ManaBrewSession) acknowledge(seat int, pm mb.PromptMessage) {
	s.t.Helper()
	msg, err := mbtest.NewFirstLegalClient().Answer(pm)
	if err != nil {
		s.t.Fatalf("manabrew wire: synthetic prompt: %v", err)
	}
	if err := s.post(seat, msg); err != nil {
		s.t.Fatalf("manabrew wire: synthetic prompt refused: %v", err)
	}
	s.Synthetic++
}

// settleWindow takes a payManaCost window's auto-fill, or cancels the cast when
// the window has none: a window is one way of paying, and an engine that pays
// otherwise never poses it. It reports false on a window that offers neither but
// can confirm from the pool: gorge's window for a cast with no plan, tapped by
// hand and refusing cancel, which the native wire poses as an ordinary choice.
func (s *ManaBrewSession) settleWindow(seat int, pm *mb.PromptMessage, in mb.PayManaCostInput) bool {
	s.t.Helper()
	var out mb.PromptOutputValue = mb.CancelOutput{}
	for _, a := range in.Actions {
		if a.Type == "autofill" {
			out = mb.PayOutput{Auto: true}
		}
	}
	if _, cancel := out.(mb.CancelOutput); cancel && in.CanConfirmFromPool {
		return false
	}
	if err := s.send(seat, pm, out); err != nil {
		s.t.Fatalf("manabrew wire: payManaCost answer refused: %v", err)
	}
	s.Windows++
	return true
}

func (s *ManaBrewSession) send(seat int, pm *mb.PromptMessage, out mb.PromptOutputValue) error {
	s.t.Helper()
	return s.post(seat, mb.ClientMessage{Value: mb.ClientResponse{
		Kind:     "response",
		PromptID: pm.PromptID,
		Action:   mb.PromptOutput{Type: pm.Input.Value.PromptType(), Output: mb.PromptOutputData{Value: out}},
	}})
}

// post sends msg. A protocol error is a refusal, and anything else the transport
// answers is a failure.
func (s *ManaBrewSession) post(seat int, msg mb.ClientMessage) error {
	s.t.Helper()
	body, err := mb.Encode(msg)
	if err != nil {
		s.t.Fatal(err)
	}
	status, resp := nativeDo(s.seats[seat], http.MethodPost, manaBrewMatch+"/send", body)
	if status == http.StatusNoContent {
		return nil
	}
	var pe mb.ProtocolError
	dec := json.NewDecoder(bytes.NewReader(resp))
	if err := dec.Decode(&pe); err != nil || pe.Code == "" || pe.Code == "bad_request" {
		s.t.Fatalf("manabrew wire: send status %d: %s", status, resp)
	}
	return fmt.Errorf("refused: %s", resp)
}
