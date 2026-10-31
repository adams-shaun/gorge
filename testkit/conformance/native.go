package conformance

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/game"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/host/httpapi"
	"github.com/adams-shaun/gorge/server/protocol"
)

const (
	nativeTable = "conformance"
	nativeMatch = "/api/tables/" + nativeTable + "/matches/1"
)

// Native is the HTTP API a browser plays through: a host table in process with
// every seat a HumanSeat answered through /intent, and an omniscient spectator.
type Native struct{}

func (Native) Name() string { return "native" }

// Open starts c on e. A table takes c's deck identity from Names, so two seats
// may share a name only by sharing a deck.
func (Native) Open(t testing.TB, e game.Engine, c game.Config) Session {
	t.Helper()
	if c.StartChooser != nil || c.DisablePaidManaCensus || c.NoObjectCap || c.NameUniverseNames != nil || c.Key != "" {
		t.Fatal("native wire: a table cannot express StartChooser, DisablePaidManaCensus, NoObjectCap, NameUniverseNames or Key")
	}
	seats := len(c.Decks)
	decks := make(map[string]host.Deck, seats)
	tableDecks := make([]string, seats)
	humans := make([]int, seats)
	for i, name := range c.Names {
		if prior, ok := decks[name]; ok && !slices.Equal(prior.Cards, c.Decks[i]) {
			t.Fatalf("native wire: two different decks are named %q", name)
		}
		d := host.Deck{Name: name, Cards: c.Decks[i]}
		if c.Sideboards != nil {
			d.Sideboard = c.Sideboards[i]
		}
		if c.Archetypes != nil {
			d.Archetype = c.Archetypes[i]
		}
		if c.Commanders != nil {
			d.Commanders = c.Commanders[i]
		}
		decks[name] = d
		// Seat i of a table's match k plays Decks[(i+k)%len], and this is match 1.
		tableDecks[(i+1)%seats] = name
		humans[i] = i
	}
	guard := host.DefaultMaxDecisionsPerTurn
	if c.GuardDisabled {
		guard = 0
	}
	r, err := host.New(host.Options{
		LoadDeck: func(name string) (host.Deck, error) {
			d, ok := decks[name]
			if !ok {
				return host.Deck{}, fmt.Errorf("no deck %q", name)
			}
			return d, nil
		},
		Tokens:              c.Tokens,
		NameUniverse:        c.NameUniverse,
		Engines:             map[string]game.Engine{e.Name(): e},
		MaxDecisionsPerTurn: guard,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close() })
	cfg := host.TableConfig{ID: nativeTable, Name: nativeTable, Seats: seats, Decks: tableDecks, Seed: c.Seed,
		PlayerNames: c.PlayerNames, Spectator: view.Omniscient, Humans: humans, AutoMana: true, Mulligans: c.Mulligans,
		WindowDiagnostics: c.WindowDiagnostics, Format: host.Format(c.Format), StartingLife: c.StartingLife, Engine: e.Name()}
	if err := r.AddTable(cfg); err != nil {
		t.Fatal(err)
	}
	if err := r.Start(nativeTable); err != nil {
		t.Fatal(err)
	}
	s := &nativeSession{t: t, spectator: httpapi.NewHandler(r, httpapi.Options{})}
	for i := range seats {
		s.seats = append(s.seats, httpapi.NewHandler(r, httpapi.Options{
			Seat: func(*http.Request) (httpapi.SeatClaim, bool) {
				return httpapi.SeatClaim{Table: nativeTable, Seat: state.PlayerID(i)}, true
			},
		}))
	}
	return s
}

type nativeSession struct {
	t         testing.TB
	seats     []http.Handler
	spectator http.Handler
	pending   *decision.Decision // what the last poll found; nil at the end of the game
	names     *objectNames       // the last frame's renaming
}

func (s *nativeSession) Next() (Frame, bool) {
	s.t.Helper()
	raw := nativeRaw{}
	s.pending, raw.Pending = s.poll()
	for i := range s.seats {
		raw.Seats = append(raw.Seats, s.get(s.seats[i], fmt.Sprintf("%s/view?seat=%d", nativeMatch, i)))
	}
	raw.Spectator = s.get(s.spectator, nativeMatch+"/view")
	f, names, err := normaliseNative(raw)
	if err != nil {
		s.t.Fatalf("native wire: %v", err)
	}
	s.names = names
	return f, s.pending != nil
}

func (s *nativeSession) Answer(key string) error {
	s.t.Helper()
	if s.pending == nil {
		return errors.New("the game is over")
	}
	if in, ok := s.intentFor(key); ok {
		return s.submit(in)
	}
	if s.pending.Kind != decision.KPriority {
		return fmt.Errorf("answer %q is not on offer", key)
	}
	return s.tapToward(key)
}

// tapToward plays what only PotentialActions lists, which the wire offers once
// enough mana is floating: it taps sources in offer order until the play is on
// offer. Tapping cannot be taken back, so from the first tap this counts as an
// accepted answer whatever follows, and the next frame shows the result: the
// spell on the stack, the floating mana, or a choice a mana ability posed.
func (s *nativeSession) tapToward(key string) error {
	i := slices.IndexFunc(s.pending.Options, isManaAbility)
	if i < 0 {
		return fmt.Errorf("play %q needs mana no source can make", key)
	}
	for i >= 0 {
		d := s.pending
		if err := s.submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{d.Options[i].Index}}); err != nil {
			return err
		}
		s.pending, _ = s.poll()
		if s.pending == nil || s.pending.Player != d.Player || s.pending.Kind != decision.KPriority {
			return nil
		}
		if in, ok := s.intentFor(key); ok {
			_ = s.submit(in)
			return nil
		}
		i = slices.IndexFunc(s.pending.Options, isManaAbility)
	}
	return nil
}

// intentFor builds the intent key stands for at the pending decision, if it
// can be answered without tapping mana first.
func (s *nativeSession) intentFor(key string) (decision.Intent, bool) {
	d := s.pending
	in := decision.Intent{Seq: d.Seq, Player: d.Player}
	if d.Kind != decision.KPriority {
		choices, err := parseSelection(key)
		if err != nil {
			s.t.Fatalf("native wire: %v", err)
		}
		in.Choices = choices
		return in, true
	}
	for _, a := range d.PaymentActions {
		if len(a.Plans) > 0 && paymentKey(a, s.names) == key {
			in.Payment = &decision.PaymentSelection{ActionID: a.ID, Plan: a.Plans[0]}
			return in, true
		}
	}
	for _, o := range d.Options {
		if isPlay(o) && optionKey(o, s.names) == key {
			in.Choices = []int{o.Index}
			return in, true
		}
	}
	return decision.Intent{}, false
}

// submit posts in. A refusal leaves the decision pending, once the match has
// parked it again.
func (s *nativeSession) submit(in decision.Intent) error {
	s.t.Helper()
	body, err := json.Marshal(in)
	if err != nil {
		s.t.Fatal(err)
	}
	status, resp := nativeDo(s.seats[in.Player], http.MethodPost, nativeMatch+"/intent", body)
	switch status {
	case http.StatusNoContent:
		return nil
	case http.StatusConflict:
		s.pending, _ = s.poll()
		return fmt.Errorf("refused: %s", resp)
	}
	s.t.Fatalf("native wire: intent status %d: %s", status, resp)
	return nil
}

// poll waits for a seat to be asked something and returns the decision and the
// bytes it was served as, or nil at the end of the game. It answers a
// mana_payment window itself: the window is one way of paying, and an engine
// that pays otherwise never poses it. This package reads no clock, so a hung
// engine is left to go test's -timeout.
func (s *nativeSession) poll() (*decision.Decision, []byte) {
	s.t.Helper()
	for {
		var infos []protocol.MatchInfo
		if err := json.Unmarshal(s.get(s.spectator, "/api/tables/"+nativeTable+"/matches"), &infos); err != nil {
			s.t.Fatal(err)
		}
		if len(infos) == 1 && infos[0].State != protocol.MatchLive {
			if infos[0].State != protocol.MatchFinished {
				s.t.Fatalf("native wire: match ended %s, seed %d: %s", infos[0].State, infos[0].Seed, infos[0].Result)
			}
			return nil, nil
		}
		for i := range s.seats {
			status, body := nativeDo(s.seats[i], http.MethodGet, fmt.Sprintf("%s/pending?seat=%d", nativeMatch, i), nil)
			if status != http.StatusOK {
				continue
			}
			var d decision.Decision
			if err := json.Unmarshal(body, &d); err != nil {
				s.t.Fatal(err)
			}
			if d.ManaPayment == nil {
				return &d, body
			}
			s.settleWindow(&d)
			break
		}
		runtime.Gosched()
	}
}

// settleWindow takes a mana_payment window's auto-fill, or cancels the cast
// when the window has none.
func (s *nativeSession) settleWindow(d *decision.Decision) {
	s.t.Helper()
	for _, kind := range []string{decision.OptAutoFill, decision.OptCancelCast} {
		for _, o := range d.Options {
			if o.Kind == kind {
				if err := s.submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{o.Index}}); err != nil {
					s.t.Fatal(err)
				}
				return
			}
		}
	}
	s.t.Fatal("native wire: a mana_payment window offers neither auto-fill nor cancel")
}

func (s *nativeSession) get(h http.Handler, path string) []byte {
	s.t.Helper()
	status, body := nativeDo(h, http.MethodGet, path, nil)
	if status != http.StatusOK {
		s.t.Fatalf("native wire: GET %s: status %d: %s", path, status, body)
	}
	return body
}

// nativeDo calls the API in process, with no socket.
func nativeDo(h http.Handler, method, path string, body []byte) (int, []byte) {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, path, bytes.NewReader(body)))
	return rec.Code, rec.Body.Bytes()
}
