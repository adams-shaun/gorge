package extern

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/bots/heuristic"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/draft/draftlab"
	"github.com/adams-shaun/gorge/draft/draftlab/extern/refagent"
	"github.com/adams-shaun/gorge/draft/draftlab/extern/wire"
	"github.com/adams-shaun/gorge/draft/limited/pickbot"
	"github.com/adams-shaun/gorge/draft/limited/setdata"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// testLab opens the lab over the shared corpus; it skips honestly when the
// corpus is absent.
func testLab(t *testing.T) *draftlab.Lab {
	t.Helper()
	l, err := draftlab.Open(testutil.CorpusDirOrSkip(t), draftlab.Options{})
	if err != nil {
		t.Fatalf("draftlab Open: %v", err)
	}
	return l
}

// testLabOpts is testLab with run options.
func testLabOpts(t *testing.T, o draftlab.Options) *draftlab.Lab {
	t.Helper()
	l, err := draftlab.Open(testutil.CorpusDirOrSkip(t), o)
	if err != nil {
		t.Fatalf("draftlab Open: %v", err)
	}
	return l
}

// testBot resolves the lab's deterministic bot player.
func testBot(t *testing.T, l *draftlab.Lab) draftlab.NamedPlayer {
	t.Helper()
	p, err := l.Player(draftlab.AgentRef{Name: "bot"})
	if err != nil {
		t.Fatalf("Player(bot): %v", err)
	}
	return p
}

// catalogEntrant pairs catalog deck i with player p.
func catalogEntrant(t *testing.T, l *draftlab.Lab, i int, p draftlab.NamedPlayer) draftlab.Entrant {
	t.Helper()
	decks, err := l.CatalogDecks("fdn-limited")
	if err != nil {
		t.Fatalf("CatalogDecks: %v", err)
	}
	if i >= len(decks) {
		t.Fatalf("catalog has %d decks, want deck %d", len(decks), i)
	}
	return draftlab.Entrant{Deck: decks[i], Player: p}
}

// itemObs records the items a served refagent observed.
type itemObs struct {
	mu    sync.Mutex
	items []wire.Item
}

func (o *itemObs) record(it wire.Item) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.items = append(o.items, it)
}

func (o *itemObs) got() []wire.Item {
	o.mu.Lock()
	defer o.mu.Unlock()
	return append([]wire.Item(nil), o.items...)
}

// decides filters the observed items to decide kind.
func (o *itemObs) decides() []wire.Item {
	var out []wire.Item
	for _, it := range o.got() {
		if it.Kind == wire.KindDecide {
			out = append(out, it)
		}
	}
	return out
}

// TestExternalSeatShape: an Agent serves all three roles over the same
// session; each adapter implements exactly its tier's interfaces and
// implements no capability it must not.
func TestExternalSeatShape(t *testing.T) {
	l := testLab(t)
	a := dialRefagent(t, newHub(), refagent.Config{}, Options{})

	d, err := a.Drafter(l)
	if err != nil {
		t.Fatalf("Drafter: %v", err)
	}
	if d.Ref != (draftlab.AgentRef{Name: "ext:random", Version: "1"}) {
		t.Errorf("drafter ref = %s, want ext:random@1", d.Ref)
	}
	if _, ok := d.Drafter.(pickbot.Drafter); !ok {
		t.Error("drafter does not implement pickbot.Drafter")
	}
	if _, ok := d.Drafter.(draftlab.FallibleDrafter); !ok {
		t.Error("drafter does not implement draftlab.FallibleDrafter")
	}

	b, err := a.Builder(l)
	if err != nil {
		t.Fatalf("Builder: %v", err)
	}
	if b.Ref != d.Ref {
		t.Errorf("builder ref = %s, want %s", b.Ref, d.Ref)
	}
	if _, ok := b.Builder.(pickbot.Builder); !ok {
		t.Error("builder does not implement pickbot.Builder")
	}
	if _, ok := b.Builder.(draftlab.FallibleBuilder); !ok {
		t.Error("builder does not implement draftlab.FallibleBuilder")
	}

	p, err := a.Player(l)
	if err != nil {
		t.Fatalf("Player: %v", err)
	}
	if p.Ref != d.Ref {
		t.Errorf("player ref = %s, want %s", p.Ref, d.Ref)
	}
	if p.ReadsDecklists {
		t.Error("NamedPlayer.ReadsDecklists = true, want false")
	}
	s, err := p.New(7)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_ = s
	if _, ok := s.(seat.Seat); !ok {
		t.Error("seat does not implement seat.Seat")
	}
	if _, ok := s.(view.LeanReader); !ok {
		t.Error("seat does not implement view.LeanReader")
	}
	if _, ok := s.(seat.PaymentPlanConsumer); !ok {
		t.Error("seat does not implement seat.PaymentPlanConsumer")
	}
	if _, ok := s.(seat.TargetHintConsumer); !ok {
		t.Error("seat does not implement seat.TargetHintConsumer")
	}
	if _, ok := s.(bots.RefusalAnswerer); !ok {
		t.Error("seat does not implement bots.RefusalAnswerer")
	}
	if _, ok := s.(draftlab.GameScoped); !ok {
		t.Error("seat does not implement draftlab.GameScoped")
	}
	if _, ok := s.(heuristic.BoardSeat); ok {
		t.Error("seat implements heuristic.BoardSeat (it must not)")
	}
	if _, ok := s.(bots.EnvSeat); ok {
		t.Error("seat implements bots.EnvSeat (it must not)")
	}
}

// TestExternalPlayerGameReplaysWithoutAgent: a game with an external seat
// produces a result whose recorded tape replays to the same head once the
// agent is closed — the result is agent-free and reproducible.
func TestExternalPlayerGameReplaysWithoutAgent(t *testing.T) {
	l := testLabOpts(t, draftlab.Options{RecordTape: true})
	a := dialRefagent(t, newHub(), refagent.Config{}, Options{})
	p, err := a.Player(l)
	if err != nil {
		t.Fatalf("Player: %v", err)
	}
	bot := testBot(t, l)
	res, err := l.PlayGame(catalogEntrant(t, l, 0, p), catalogEntrant(t, l, 1, bot), 4242)
	if err != nil {
		t.Fatalf("PlayGame: %v", err)
	}
	if a.Stats().Decides == 0 {
		t.Fatal("the agent was never asked a decide (precondition)")
	}
	_ = a.Close()
	head, err := l.ReplayTape(
		catalogEntrant(t, l, 0, bot).Deck,
		catalogEntrant(t, l, 1, bot).Deck,
		res.Seed, res.Tape)
	if err != nil {
		t.Fatalf("ReplayTape: %v", err)
	}
	if head != res.Head {
		t.Errorf("replay head %s != recorded head %s", head, res.Head)
	}
	t.Logf("game: winner=%d stall=%q turns=%d intents=%d decides=%d",
		res.Winner, res.Stall, res.Turns, res.Intents, a.Stats().Decides)
}

// externalMatchups builds four fdn-limited matchups: matchup i's player 0
// is the external agent on even i, the bot otherwise, and the other side
// is the opposite — the same every call.
func externalMatchups(t *testing.T, l *draftlab.Lab, ext, bot draftlab.NamedPlayer) []draftlab.Matchup {
	t.Helper()
	ms := make([]draftlab.Matchup, 4)
	for i := range ms {
		p0, p1 := bot, ext
		if i%2 == 0 {
			p0, p1 = ext, bot
		}
		ms[i] = draftlab.Matchup{
			A: catalogEntrant(t, l, 2*i, p0),
			B: catalogEntrant(t, l, 2*i+1, p1),
		}
	}
	return ms
}

// TestExternalPlayerWorkerCountIndependent: the same 4 matchups over the
// same agent produce byte-identical results at Workers 1 and 4, and again
// under both flush rules.
func TestExternalPlayerWorkerCountIndependent(t *testing.T) {
	run := func(o draftlab.Options, flush Options) []draftlab.GameResult {
		l := testLabOpts(t, o)
		a := dialRefagent(t, newHub(), refagent.Config{}, flush)
		p, err := a.Player(l)
		if err != nil {
			t.Fatalf("Player: %v", err)
		}
		bot := testBot(t, l)
		res, err := l.Games(externalMatchups(t, l, p, bot), 999)
		if err != nil {
			t.Fatalf("Games: %v", err)
		}
		return res
	}
	r1 := run(draftlab.Options{Workers: 1, MaxTurns: 40}, Options{})
	r4 := run(draftlab.Options{Workers: 4, MaxTurns: 40}, Options{})
	rReady := run(draftlab.Options{Workers: 4, MaxTurns: 40}, Options{Flush: FlushReady})
	if !reflect.DeepEqual(r1, r4) {
		t.Error("Workers 1 and Workers 4 results differ")
	}
	if !reflect.DeepEqual(r4, rReady) {
		t.Error("barrier and ready flush results differ")
	}
	for i, r := range rReady {
		t.Logf("game %d: winner=%d stall=%q turns=%d", i, r.Winner, r.Stall, r.Turns)
	}
}

// TestExternalPlayerSeesOnlyItsSeat: every decide item's view is a seat
// view naming the deciding player, with every other seat's hand null and
// no library list anywhere in the raw JSON.
func TestExternalPlayerSeesOnlyItsSeat(t *testing.T) {
	l := testLab(t)
	obs := &itemObs{}
	a := dialRefagent(t, newHub(), refagent.Config{ObserveItem: obs.record}, Options{})
	p, err := a.Player(l)
	if err != nil {
		t.Fatalf("Player: %v", err)
	}
	bot := testBot(t, l)
	_, err = l.PlayGame(catalogEntrant(t, l, 0, p), catalogEntrant(t, l, 1, bot), 99)
	if err != nil {
		t.Fatalf("PlayGame: %v", err)
	}
	items := obs.decides()
	if len(items) == 0 {
		t.Fatal("no decide items observed (precondition)")
	}
	for _, it := range items {
		v, err := wire.DecodeView(it.View)
		if err != nil {
			t.Fatalf("DecodeView: %v", err)
		}
		if v.Visibility != view.Seat.String() {
			t.Fatalf("view visibility = %q, want %q", v.Visibility, view.Seat)
		}
		if v.Decision == nil || int(v.Decision.Player) != int(v.Viewer) {
			t.Fatalf("view decision player %v / viewer %d: not the deciding seat", v.Decision, v.Viewer)
		}
		for i, pv := range v.Players {
			if i == int(v.Viewer) {
				continue
			}
			if pv.Hand != nil {
				t.Fatalf("seat %d's hand is not null in seat %d's view (CR 400.2 leak)", i, v.Viewer)
			}
		}
		// No seat but the viewer carries a library list; the viewer's own
		// unordered library is part of the seat view by design (the raw
		// "library" key belongs to the viewer's PlayerView alone).
		var m map[string]any
		if err := json.Unmarshal(it.View, &m); err != nil {
			t.Fatalf("unmarshal view: %v", err)
		}
		players, _ := m["players"].([]any)
		for i, p := range players {
			if i == int(v.Viewer) {
				continue
			}
			pm, _ := p.(map[string]any)
			if pm["library"] != nil || pm["library_top"] != nil {
				t.Fatalf("seat %d's view carries seat %d's library", v.Viewer, i)
			}
		}
	}
}

// TestLeanViewOmitsParts: with the hello naming every omittable part, no
// decide view carries a library, graveyard or exile list for anyone,
// while the viewer's own battlefield is still projected.
func TestLeanViewOmitsParts(t *testing.T) {
	l := testLab(t)
	var names []string
	for _, e := range wire.ViewOmitNames {
		names = append(names, e.Name)
	}
	obs := &itemObs{}
	a := dialRefagent(t, newHub(), refagent.Config{ViewOmit: names, ObserveItem: obs.record}, Options{})
	p, err := a.Player(l)
	if err != nil {
		t.Fatalf("Player: %v", err)
	}
	bot := testBot(t, l)
	_, err = l.PlayGame(catalogEntrant(t, l, 0, p), catalogEntrant(t, l, 1, bot), 101)
	if err != nil {
		t.Fatalf("PlayGame: %v", err)
	}
	items := obs.decides()
	if len(items) == 0 {
		t.Fatal("no decide items observed (precondition)")
	}
	for _, it := range items {
		v, err := wire.DecodeView(it.View)
		if err != nil {
			t.Fatalf("DecodeView: %v", err)
		}
		if v.Decision == nil {
			t.Fatal("a lean view omitted the decision (OmitDecision is not a name)")
		}
		var m map[string]any
		if err := json.Unmarshal(it.View, &m); err != nil {
			t.Fatalf("unmarshal view: %v", err)
		}
		players, _ := m["players"].([]any)
		viewer, _ := players[int(v.Viewer)].(map[string]any)
		if bf, ok := viewer["battlefield"].([]any); !ok || bf == nil {
			t.Fatalf("viewer %d's own battlefield is not a projected list", v.Viewer)
		}
		for _, p := range players {
			pm, _ := p.(map[string]any)
			for _, key := range []string{"graveyard", "exile", "library", "library_top"} {
				if pm[key] != nil {
					t.Fatalf("lean view carries %s: %v", key, pm[key])
				}
			}
		}
	}
}

// TestSelfPlaySameAgent: both seats of one game sit on the same agent; the
// game completes and the hub holds no live game afterwards (both seats'
// Begin/End balanced on one token).
func TestSelfPlaySameAgent(t *testing.T) {
	l := testLab(t)
	a := dialRefagent(t, procHub, refagent.Config{}, Options{})
	p, err := a.Player(l)
	if err != nil {
		t.Fatalf("Player: %v", err)
	}
	res, err := l.PlayGame(catalogEntrant(t, l, 2, p), catalogEntrant(t, l, 3, p), 555)
	if err != nil {
		t.Fatalf("PlayGame: %v", err)
	}
	procHub.mu.Lock()
	live := len(procHub.games)
	procHub.mu.Unlock()
	if live != 0 {
		t.Errorf("the hub holds %d live games after the game ended", live)
	}
	t.Logf("game: winner=%d stall=%q turns=%d intents=%d", res.Winner, res.Stall, res.Turns, res.Intents)
}

// TestExternalEventAllRoles: one agent serves drafter, builder and player
// of a 4-seat FDN event (seats 0 and 2 external, 1 and 3 the bot); two
// runs produce identical event logs, every game's tape replays, and the
// manifest names the agent under all three roles.
func TestExternalEventAllRoles(t *testing.T) {
	run := func() (draftlab.EventResult, *Agent, error) {
		l := testLabOpts(t, draftlab.Options{MaxTurns: 40, RecordTape: true})
		a := dialRefagent(t, newHub(), refagent.Config{}, Options{})
		d, err := a.Drafter(l)
		if err != nil {
			return draftlab.EventResult{}, a, err
		}
		b, err := a.Builder(l)
		if err != nil {
			return draftlab.EventResult{}, a, err
		}
		p, err := a.Player(l)
		if err != nil {
			return draftlab.EventResult{}, a, err
		}
		bot := testBot(t, l)
		res, err := l.RunEvent(draftlab.EventSpec{
			Set:       "FDN",
			Seats:     4,
			Structure: draftlab.RoundRobin,
			Drafters:  []draftlab.NamedDrafter{d},
			Builders:  []draftlab.NamedBuilder{b},
			Players:   []draftlab.NamedPlayer{p, bot, p, bot},
			Seed:      77,
		})
		return res, a, err
	}
	r1, a1, err := run()
	if err != nil {
		t.Fatalf("RunEvent 1: %v", err)
	}
	r2, _, err := run()
	if err != nil {
		t.Fatalf("RunEvent 2: %v", err)
	}
	if !bytes.Equal(r1.Log, r2.Log) {
		t.Error("the two runs' event logs differ")
	}
	if err := l1Verify(t, r1); err != nil {
		t.Errorf("VerifyEventTapes: %v", err)
	}
	if a1.Stats().Picks == 0 || a1.Stats().Builds == 0 || a1.Stats().Decides == 0 {
		t.Errorf("the agent was not asked all three kinds: picks %d builds %d decides %d",
			a1.Stats().Picks, a1.Stats().Builds, a1.Stats().Decides)
	}
	for _, want := range []string{
		"builder:ext:random@1", "drafter:ext:random@1", "player:ext:random@1", "player:bot@1",
	} {
		if !manifestHas(r1, want) {
			t.Errorf("manifest agents lack %s: %v", want, r1.Manifest.Agents)
		}
	}
	if len(r1.Manifest.Agents) != 4 {
		t.Errorf("manifest has %d agents, want 4: %v", len(r1.Manifest.Agents), r1.Manifest.Agents)
	}
	for i, g := range r1.Games {
		t.Logf("game %d: winner=%d stall=%q turns=%d", i, g.Winner, g.Stall, g.Turns)
	}
}

// l1Verify re-opens a lab and verifies the event's tapes under the event's
// own watchdogs (the manifest's effective values): a capped game's tape
// re-drives only to the same cap, so the replay lab must run them.
func l1Verify(t *testing.T, r draftlab.EventResult) error {
	t.Helper()
	l, err := draftlab.Open(testutil.CorpusDirOrSkip(t), draftlab.Options{
		MaxTurns:   r.Manifest.MaxTurns,
		MaxIntents: r.Manifest.MaxIntents,
	})
	if err != nil {
		return err
	}
	return l.VerifyEventTapes(r)
}

// manifestHas reports whether the event manifest names ref.
func manifestHas(r draftlab.EventResult, ref string) bool {
	for _, a := range r.Manifest.Agents {
		if a == ref {
			return true
		}
	}
	return false
}

// TestRefusalFallsBackAndCounts: the illegal-policy agent refuses every
// kind; in fallback mode the adapters answer with the lab's fallbacks and
// count every refusal, in strict mode the run fails naming ErrRefused.
func TestRefusalFallsBackAndCounts(t *testing.T) {
	l := testLab(t)
	bot := testBot(t, l)

	// decide
	a := dialRefagent(t, newHub(), refagent.Config{Policy: "illegal"}, Options{})
	p, err := a.Player(l)
	if err != nil {
		t.Fatalf("Player: %v", err)
	}
	res, err := l.PlayGame(catalogEntrant(t, l, 0, p), catalogEntrant(t, l, 1, bot), 222)
	if err != nil {
		t.Fatalf("PlayGame: %v", err)
	}
	st := a.Stats()
	if st.Decides == 0 {
		t.Fatal("no decide sent (precondition)")
	}
	if st.Refused != st.Decides {
		t.Errorf("refused %d != decides sent %d", st.Refused, st.Decides)
	}
	t.Logf("decide refusals: %d (game winner=%d stall=%q)", st.Refused, res.Winner, res.Stall)

	// draft
	packs, err := l.OpenPod("FDN", setdata.Collation(""), 1234, 4, 3)
	if err != nil {
		t.Fatalf("OpenPod: %v", err)
	}
	ad := dialRefagent(t, newHub(), refagent.Config{Policy: "illegal"}, Options{})
	d, err := ad.Drafter(l)
	if err != nil {
		t.Fatalf("Drafter: %v", err)
	}
	dr, err := l.Draft(packs, []draftlab.NamedDrafter{d}, 1234)
	if err != nil {
		t.Fatalf("Draft: %v", err)
	}
	for seat, pool := range dr.Pools {
		if len(pool) != 42 {
			t.Errorf("seat %d's pool has %d cards, want 42", seat, len(pool))
		}
	}
	if dst := ad.Stats(); dst.Refused != dst.Picks || dst.Picks != 168 {
		t.Errorf("draft: picks %d refused %d, want 168/168", dst.Picks, dst.Refused)
	}

	// build
	ab := dialRefagent(t, newHub(), refagent.Config{Policy: "illegal"}, Options{})
	b, err := ab.Builder(l)
	if err != nil {
		t.Fatalf("Builder: %v", err)
	}
	fb, ok := b.Builder.(draftlab.FallibleBuilder)
	if !ok {
		t.Fatal("builder does not implement draftlab.FallibleBuilder")
	}
	deck, err := fb.BuildErr("FDN", dr.Pools[0])
	if err != nil {
		t.Fatalf("BuildErr: %v", err)
	}
	if deck.Total() < 40 {
		t.Errorf("fallback deck has %d cards, want >= 40", deck.Total())
	}
	if st := ab.Stats(); st.Refused != 1 || st.Builds != 1 {
		t.Errorf("build: refused %d builds %d, want 1/1", st.Refused, st.Builds)
	}

	// strict: every role fails naming ErrRefused
	as := dialRefagent(t, newHub(), refagent.Config{Policy: "illegal"}, Options{StrictRefusals: true})
	ps, err := as.Player(l)
	if err != nil {
		t.Fatalf("Player: %v", err)
	}
	_, err = l.PlayGame(catalogEntrant(t, l, 0, ps), catalogEntrant(t, l, 1, bot), 223)
	if !errors.Is(err, ErrRefused) {
		t.Errorf("strict decide: error %v does not wrap ErrRefused", err)
	}
	ds, err := as.Drafter(l)
	if err != nil {
		t.Fatalf("Drafter: %v", err)
	}
	_, err = l.Draft(packs, []draftlab.NamedDrafter{ds}, 1234)
	if !errors.Is(err, ErrRefused) {
		t.Errorf("strict draft: error %v does not wrap ErrRefused", err)
	}
	bs, err := as.Builder(l)
	if err != nil {
		t.Fatalf("Builder: %v", err)
	}
	fbs, ok := bs.Builder.(draftlab.FallibleBuilder)
	if !ok {
		t.Fatal("builder does not implement draftlab.FallibleBuilder")
	}
	_, err = fbs.BuildErr("FDN", dr.Pools[0])
	if !errors.Is(err, ErrRefused) {
		t.Errorf("strict build: error %v does not wrap ErrRefused", err)
	}
}

// TestAgentDeathPolicy: an agent that dies mid-game fails the run with
// ErrAgentGone by default, and falls back per request with OnFailure set.
func TestAgentDeathPolicy(t *testing.T) {
	l := testLab(t)
	bot := testBot(t, l)

	a := dialRefagent(t, newHub(), refagent.Config{DieAfter: 2}, Options{MaxBatch: 1})
	p, err := a.Player(l)
	if err != nil {
		t.Fatalf("Player: %v", err)
	}
	_, err = l.PlayGame(catalogEntrant(t, l, 0, p), catalogEntrant(t, l, 1, bot), 321)
	if !errors.Is(err, ErrAgentGone) {
		t.Errorf("FailError: error %v does not wrap ErrAgentGone", err)
	}

	af := dialRefagent(t, newHub(), refagent.Config{DieAfter: 2}, Options{MaxBatch: 1, OnFailure: FailFallback})
	pf, err := af.Player(l)
	if err != nil {
		t.Fatalf("Player: %v", err)
	}
	res, err := l.PlayGame(catalogEntrant(t, l, 0, pf), catalogEntrant(t, l, 1, bot), 322)
	if err != nil {
		t.Fatalf("FailFallback: PlayGame: %v", err)
	}
	if st := af.Stats(); st.Fallbacks == 0 {
		t.Error("FailFallback counted no fallbacks")
	} else {
		t.Logf("fallbacks: %d (game winner=%d stall=%q turns=%d)", st.Fallbacks, res.Winner, res.Stall, res.Turns)
	}
}

// TestForcedDecisionsSkipped: with skip_forced, no decide item the agent
// sees is a forced decision (they were answered locally).
func TestForcedDecisionsSkipped(t *testing.T) {
	l := testLab(t)
	obs := &itemObs{}
	a := dialRefagent(t, newHub(), refagent.Config{SkipForced: true, ObserveItem: obs.record}, Options{})
	p, err := a.Player(l)
	if err != nil {
		t.Fatalf("Player: %v", err)
	}
	bot := testBot(t, l)
	_, err = l.PlayGame(catalogEntrant(t, l, 0, p), catalogEntrant(t, l, 1, bot), 404)
	if err != nil {
		t.Fatalf("PlayGame: %v", err)
	}
	items := obs.decides()
	if len(items) == 0 {
		t.Fatal("no decide items observed (precondition)")
	}
	for _, it := range items {
		d, err := wire.DecisionOf(it.View)
		if err != nil {
			t.Fatalf("DecisionOf: %v", err)
		}
		if forced(d) {
			t.Errorf("the agent was sent a forced decision (seq %d, kind %q)", d.Seq, d.Kind)
		}
	}
	if st := a.Stats(); st.Forced > 0 {
		t.Logf("forced answers: %d", st.Forced)
	}
}

// TestForcedShape: the forced() rule over synthetic decisions.
func TestForcedShape(t *testing.T) {
	one := decision.Decision{Options: []decision.Option{{Index: 0, Kind: "pass"}}, Min: 1, Max: 1}
	two := decision.Decision{Options: []decision.Option{{Index: 0}, {Index: 1}}, Min: 1, Max: 1}
	optional := one
	optional.Min = 0
	manymax := one
	manymax.Max = 2
	paid := one
	paid.PaymentActions = []decision.PaymentAction{{}}
	mana := one
	mana.ManaPayment = &decision.ManaPaymentWindow{}
	for _, tc := range []struct {
		name string
		d    decision.Decision
		want bool
	}{
		{"one option", one, true},
		{"two options", two, false},
		{"min 0", optional, false},
		{"max 2", manymax, false},
		{"payment actions", paid, false},
		{"mana payment", mana, false},
	} {
		if got := forced(&tc.d); got != tc.want {
			t.Errorf("%s: forced = %v, want %v", tc.name, got, tc.want)
		}
	}
}
