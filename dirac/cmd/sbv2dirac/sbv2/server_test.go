package sbv2

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"

	_ "github.com/adams-shaun/gorge/bots/all"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/oracle"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/server/spellbench/agent"
	"github.com/adams-shaun/gorge/server/spellbench/catalog"
	"github.com/adams-shaun/gorge/server/spellbench/env"
	"github.com/adams-shaun/gorge/server/spellbench/minihost"
	"github.com/adams-shaun/gorge/server/spellbench/observe"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
	"github.com/adams-shaun/gorge/server/spellbench/validate"
	"github.com/adams-shaun/gorge/server/spellbench/wire"
)

var (
	regOnce sync.Once
	regVal  *cards.Registry
	regErr  error
)

// registry is the full gorge corpus, opened once for the test binary.
func registry(t testing.TB) *cards.Registry {
	t.Helper()
	regOnce.Do(func() {
		dir, err := cardset.CorpusDir()
		if err != nil {
			regErr = err
			return
		}
		regVal, regErr = cards.OpenCorpus(dir)
	})
	if regErr != nil {
		t.Skip("no gorge corpus: ", regErr)
	}
	return regVal
}

var rowRE = regexp.MustCompile(`(.+?) (\d+)(?:, |$)`)

// rows reads "Name N, Name N" (a name may hold a comma).
func rows(spec string) []wire.DeckRow {
	var out []wire.DeckRow
	for _, m := range rowRE.FindAllStringSubmatch(spec, -1) {
		n, _ := strconv.Atoi(m[2])
		out = append(out, wire.DeckRow{Name: m[1], Count: n})
	}
	return out
}

// Two FDN pool decks (17Lands FDN_top_00001_WB and FDN_top_00004_RG) whose
// every card the kernel's FDN set supports.
var (
	deckWB = catalog.Deck{CatalogID: "FDN_WB", Name: "FDN_WB", Rows: rows("Arbiter of Woe 1, Burglar Rat 2, Cathar Commando 1, Dazzling Angel 1, Eaten Alive 1, Fiendish Panda 1, Healer's Hawk 1, Helpful Hunter 2, Hero's Downfall 1, Hungry Ghoul 1, Infernal Vessel 1, Liliana, Dreadhorde General 1, Macabre Waltz 1, Plains 9, Revenge of the Rats 1, Swamp 9, Tinybones, Bauble Burglar 1, Tragic Banshee 1, Vampire Nighthawk 1, Vampire Soulcaller 2, Vanguard Seraph 1")}
	deckRG = catalog.Deck{CatalogID: "FDN_RG", Name: "FDN_RG", Rows: rows("Affectionate Indrik 1, Axgard Cavalry 1, Beast-Kin Ranger 1, Abrade 1, Burst Lightning 3, Courageous Goblin 3, Eager Trufflesnout 1, Elvish Regrower 2, Evolving Wilds 1, Felling Blow 1, Forest 8, Giant Growth 2, Gorehorn Raider 2, Heartfire Immolator 1, Mountain 8, Quilled Greatwurm 1, Shivan Dragon 1, Strongbox Raider 1, Wary Thespian 1")}
)

func newServer(t testing.TB) *Server {
	t.Helper()
	s, err := New(registry(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	s.AddDeck(deckWB)
	s.AddDeck(deckRG)
	t.Cleanup(s.Close)
	return s
}

// link is a minihost.Link over the server.
type link struct{ s *Server }

func (l link) Round(req []byte) ([]byte, error) { return l.s.Handle(req), nil }

func host(s *Server) *minihost.Host {
	kinds := map[string]bool{}
	for _, k := range env.DecisionKinds {
		kinds[k] = true
	}
	return &minihost.Host{RunSecret: make([]byte, 32), Engine: link{s},
		Profile:  validate.Profile{Kinds: kinds, Flags: observe.Flags, Extensions: map[string]bool{}},
		MaxSteps: 20000, MaxDecisions: 9999}
}

func names(decks ...catalog.Deck) []string {
	seen := map[string]bool{}
	var out []string
	for _, d := range decks {
		for _, r := range d.Rows {
			if !seen[r.Name] {
				seen[r.Name] = true
				out = append(out, r.Name)
			}
		}
	}
	return out
}

// bot is gorge's own agent (policy "bot") reading only the v2 observation and
// candidates, the neutral world.
func bot(t testing.TB) minihost.Link {
	t.Helper()
	a, err := agent.New("bot")
	if err != nil {
		t.Fatal(err)
	}
	a.SetRegistry(registry(t))
	if err := a.EnableNeutral(); err != nil {
		t.Fatal(err)
	}
	return a
}

// TestDiracEnvFirstGames plays 20 seeded games of the two lockstep-clean
// decks, gorge's neutral-world bot against a seeded uniform agent (seats
// alternating), through the kernel-authority environment. Every message goes
// through gorge's v2 validator (minihost); a game that ends in anything but
// a win or a draw, or any protocol error, fails the test.
func TestDiracEnvFirstGames(t *testing.T) {
	s := newServer(t)
	h := host(s)
	decks := [2]catalog.Deck{deckWB, deckRG}
	nm := names(deckWB, deckRG)
	outcomes := map[string]int{}
	for i := uint64(0); i < 20; i++ {
		agents := [2]minihost.Link{bot(t), &minihost.Uniform{}}
		if i%2 == 1 {
			agents[0], agents[1] = agents[1], agents[0]
		}
		res, err := h.PlayDecks(i, decks, nm, "london", nil, agents)
		if err != nil {
			t.Fatalf("game %d: protocol error: %v", i, err)
		}
		if res.Terminal.Classification != "natural" {
			t.Fatalf("game %d: %s %s (%s)", i, res.Terminal.Outcome, res.Terminal.Classification, res.Terminal.Reason)
		}
		outcomes[res.Terminal.Outcome]++
	}
	if d := s.Divergences(); len(d) != 0 {
		t.Fatalf("%d divergences, first: %s", len(d), d[0])
	}
	t.Logf("20 games, 0 protocol errors, 0 divergences: %v", outcomes)
}

// TestDiracEnvRefusesUncleanDeck: validate_deck names the cards of a deck the
// kernel does not play in lockstep, and reset refuses it the same way.
func TestDiracEnvRefusesUncleanDeck(t *testing.T) {
	s := newServer(t)
	bad := catalog.Deck{CatalogID: "Unclean", Name: "Unclean", Rows: append(rows("Lightning Bolt 4, Mountain 16, Fireblast 4"), deckRG.Rows...)}
	// a card the corpus holds and the FDN set lacks must be among the unclean
	if got := s.Unclean(bad); len(got) == 0 {
		t.Fatal("test deck is clean; pick a card the FDN set does not support")
	}
	s.AddDeck(bad)
	want := s.Unclean(bad)
	resp := s.Handle([]byte(fmt.Sprint(`{"request_type":"validate_deck","protocol":"spellbench/v2","request_id":"v-1","format":"pauper-bo1","deck":{"catalog_id":"Unclean"}}`)))
	var e protocol.ErrorResponse
	if err := json.Unmarshal(resp, &e); err != nil {
		t.Fatal(err)
	}
	if e.ResponseType != "error" || e.Error.Code != protocol.CodeUnsupportedDeck {
		t.Fatalf("validate_deck of an unclean deck: %s", resp)
	}
	for _, n := range want {
		if !strings.Contains(e.Error.Message, n) {
			t.Fatalf("message %q does not name %q", e.Error.Message, n)
		}
	}
	// the clean decks pass, and the unclean deck is not advertised
	ok := s.Handle([]byte(fmt.Sprint(`{"request_type":"validate_deck","protocol":"spellbench/v2","request_id":"v-2","format":"pauper-bo1","deck":{"catalog_id":"FDN_RG"}}`)))
	if !strings.Contains(string(ok), `"deck_ok"`) {
		t.Fatalf("validate_deck of a clean deck: %s", ok)
	}
	var hello protocol.HelloOK
	if err := json.Unmarshal(s.Handle([]byte(`{"request_type":"hello","protocol":"spellbench/v2","request_id":"h-1","protocol_minor":0}`)), &hello); err != nil {
		t.Fatal(err)
	}
	for _, d := range hello.Catalog {
		if d.CatalogID == "Unclean" {
			t.Fatal("hello advertises the unclean deck")
		}
	}
}

// TestDiracEnvDeterministic: one reset, the same agents, the same game byte
// for byte (the host's chained digest), with and without a mulligan round.
func TestDiracEnvDeterministic(t *testing.T) {
	decks := [2]catalog.Deck{deckRG, deckWB}
	nm := names(deckWB, deckRG)
	for _, mull := range []string{"london", "none"} {
		var digest [2]string
		for k := range digest {
			s := newServer(t)
			res, err := host(s).PlayDecks(0, decks, nm, mull, nil, [2]minihost.Link{&minihost.Uniform{}, &minihost.Uniform{}})
			if err != nil {
				t.Fatalf("%s: %v", mull, err)
			}
			if res.Terminal.Classification != "natural" {
				t.Fatalf("%s: %s (%s)", mull, res.Terminal.Classification, res.Terminal.Reason)
			}
			digest[k] = res.Digest
		}
		if digest[0] != digest[1] {
			t.Fatalf("%s: rerun digest %s != %s", mull, digest[1], digest[0])
		}
	}
}

// TestDiracEnvDivergenceHalts: when the kernel and the shadow disagree the
// game ends in a terminal halted engine_contract_failure:divergence (not a
// protocol error), and the oracle's report is kept. The kernel here plays the
// full surface against the shadow's autopay surface.
func TestDiracEnvDivergenceHalts(t *testing.T) {
	s := newServer(t)
	s.kernelConfig = func(c *oracle.Config) { c.GorgeAutopaySurface = false }
	decks := [2]catalog.Deck{deckWB, deckRG}
	nm := names(deckWB, deckRG)
	for i := uint64(0); i < 6; i++ {
		res, err := host(s).PlayDecks(i, decks, nm, "none", nil, [2]minihost.Link{&minihost.Uniform{}, &minihost.Uniform{}})
		if err != nil {
			t.Fatalf("game %d: protocol error: %v", i, err)
		}
		if res.Terminal.Reason != "engine_contract_failure:divergence" {
			continue
		}
		if res.Terminal.Classification != "halted" || len(s.Divergences()) == 0 {
			t.Fatalf("divergence terminal %+v, %d reports", res.Terminal, len(s.Divergences()))
		}
		d := s.Divergences()[0]
		t.Logf("game %d: %s", i, d)
		if d.Lead != "kernel" || d.What == "" {
			t.Fatalf("report %+v", d)
		}
		return
	}
	t.Fatal("no game diverged with the kernel on the other surface")
}

// quitter is an agent that walks away after n decisions.
type quitter struct {
	minihost.Uniform
	n int
}

func (q *quitter) Round(req []byte) ([]byte, error) {
	if strings.Contains(string(req), `"request_type":"choose"`) {
		if q.n--; q.n < 0 {
			return nil, fmt.Errorf("agent left")
		}
	}
	return q.Uniform.Round(req)
}

// TestDiracEnvCloseMidGame: closing the server with a game in progress
// releases the lockstep goroutine (the call returns) and ends the game.
func TestDiracEnvCloseMidGame(t *testing.T) {
	s := newServer(t)
	_, err := host(s).PlayDecks(0, [2]catalog.Deck{deckWB, deckRG}, names(deckWB, deckRG), "london", nil,
		[2]minihost.Link{&quitter{n: 5}, &quitter{n: 5}})
	if err == nil || !strings.Contains(err.Error(), "agent left") {
		t.Fatalf("PlayDecks: %v", err)
	}
	if s.game == nil || s.game.over() {
		t.Fatal("expected a game in progress")
	}
	s.Close()
	if !s.game.over() {
		t.Fatal("Close left the game running")
	}
}
