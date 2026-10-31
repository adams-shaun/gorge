package host

// RunClosedPrivacyContract, the privacy proof a closed-list hosted policy
// (bots.Info.ClosedLists) owes before it may seat against a human.

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/bots/heuristic"
	"github.com/adams-shaun/gorge/bots/searchprobe"
	"github.com/adams-shaun/gorge/bots/searchseat"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/protocol"
)

// ClosedPrivacyContract configures RunClosedPrivacyContract.
type ClosedPrivacyContract struct {
	// Options is the registry's base options; the contract sets LoadDeck
	// (decks "bot" and "human"), Seats, ThinkTimeout, MaxIntents and the
	// unbounded-search knobs over it.
	Options Options
	// Policy is the registered closed-list policy under test; its seats are
	// built with bots.New(Policy, ...).
	Policy string
	// Belief is the belief the policy was enabled with: its corpus is
	// public knowledge, so a card in it is never secret.
	Belief *searchprobe.Belief
	// BotDeck is the policy's own deck; HumanDeck and HumanSideboard the
	// human's. Every card of the human's that is in neither the belief nor
	// BotDeck is secret; the fixture must yield at least 8 secret names.
	BotDeck, HumanDeck, HumanSideboard []*cards.Card
	// Seeds are the games played, one single-shot human table each.
	Seeds []uint64
}

// privacySpy wraps the real closed-list seat at the human's opponent slot.
// Before every answer it hands check what the host handed it; it forwards
// every interface the host dispatches on (Env, Board, closed list).
type privacySpy struct {
	inner seat.Seat
	check func(kind string, d *decision.Decision, env *bots.Env, b *botpolicy.Board, v *view.View)
}

var (
	_ bots.EnvSeat              = (*privacySpy)(nil)
	_ heuristic.BoardSeat       = (*privacySpy)(nil)
	_ searchseat.ClosedListSeat = (*privacySpy)(nil)
)

func (s *privacySpy) Decide(ctx context.Context, v view.View, d decision.Decision) (decision.Intent, error) {
	s.check("view", &d, nil, nil, &v)
	return s.inner.Decide(ctx, v, d)
}

func (s *privacySpy) DecideBoard(ctx context.Context, b botpolicy.Board, d decision.Decision) (decision.Intent, error) {
	s.check("board", &d, nil, &b, nil)
	return s.inner.(heuristic.BoardSeat).DecideBoard(ctx, b, d)
}

func (s *privacySpy) WantsEnv(d *decision.Decision) bool { return s.inner.(bots.EnvSeat).WantsEnv(d) }

func (s *privacySpy) DecideEnv(ctx context.Context, env bots.Env, d decision.Decision) (decision.Intent, error) {
	s.check("env", &d, &env, &env.Board, &env.View)
	return s.inner.(bots.EnvSeat).DecideEnv(ctx, env, d)
}

func (s *privacySpy) SearchBelief() *searchprobe.Belief {
	return s.inner.(searchseat.ClosedListSeat).SearchBelief()
}

// publicZone is a zone whose cards every seat sees face up.
func publicZone(z state.Zone) bool {
	switch z {
	case state.ZBattlefield, state.ZGraveyard, state.ZExile, state.ZStack, state.ZCommand:
		return true
	}
	return false
}

// RunClosedPrivacyContract plays whole hosted games, the policy (seat 0)
// against a human (seat 1, answered by the host's own timeout caretaker),
// and proves the human's unrevealed cards never reach the bot.
//
// revealed, the ground truth, is built from the LIVE game independently of
// the feed and the redealer: a secret card counts as revealed once its object
// has been the subject of an event into a public zone or onto the stack, or
// named in a public reveal (a non-secret Note's IDs), or shown in a View the
// bot seat was handed. The assertion, at every decision of the bot seat: the
// secret names found in what the host handed the seat -- the Env's Setup,
// root engine, Board, feed and refusal reason; the Board and View the
// non-searched paths get -- are a subset of revealed. The root engine is
// walked whole, through unexported fields, except the fields named in
// skipFields (each is a known residual of the leak audit).
// Every searched decision must get a root (bar posed-resolution refusals),
// and the games must give real evidence: at least 10 roots, and at least 10
// decisions at which a secret card was still hidden.
func RunClosedPrivacyContract(t ContractT, c ClosedPrivacyContract) {
	t.Helper()
	policy := c.Policy
	known := map[string]bool{}
	for _, n := range c.Belief.Cards().Names() {
		known[n] = true
	}
	for _, cd := range c.BotDeck {
		known[cd.Faces[0].Name] = true
	}
	var secret []*cards.Card
	for _, cd := range append(append([]*cards.Card(nil), c.HumanDeck...), c.HumanSideboard...) {
		if !known[cd.Faces[0].Name] {
			secret = append(secret, cd)
		}
	}
	// Fields the walk does not enter, each a residual of the audit:
	// compiledText is the engine's immutable card-text cache, built at
	// genesis from every configured deck and shared by every clone; it is
	// read only by lookups keyed on a card the engine already holds.
	skipFields := []string{"Engine.compiledText"}
	scanner := newLeakScan(secret, skipFields...)
	if len(scanner.names) < 8 {
		t.Fatalf("only %d secret names (%v): the fixture does not test much", len(scanner.names), scanner.names)
	}
	t.Logf("secret names (%d): %v", len(scanner.names), scanner.names)

	opts := c.Options
	opts.LoadDeck = func(name string) (Deck, error) {
		switch name {
		case "bot":
			return Deck{Name: "bot", Cards: c.BotDeck}, nil
		case "human":
			return Deck{Name: "human", Cards: c.HumanDeck, Sideboard: c.HumanSideboard}, nil
		}
		return Deck{}, ErrNotFound
	}
	opts.ThinkTimeout = time.Millisecond // the caretaker answers for the human
	opts.BotUnboundedDecisions = true
	opts.BotSearchParallelism = 4
	opts.MaxIntents = 900

	var (
		mu                     sync.Mutex
		r                      *Registry
		tid                    TableID
		revealed               map[string]bool
		evDone                 int
		kinds                  = map[string]int{}
		decisions, envs, roots int
		posed                  int // roots refused at a posed resolution (posedRefusal)
		otherRefusals          []string
		secretsHidden          int // decisions at which some secret card was still hidden in the live game
		leaks                  []string
		walks                  int
		walkTime               time.Duration
	)
	secretName := map[string]bool{}
	for _, n := range scanner.names {
		secretName[n] = true
	}
	nameOf := func(e *rules.Engine, id state.ObjID) string {
		if o := e.G.Obj(id); o != nil && o.Card != nil && len(o.Card.Faces) > 0 {
			return o.Card.Faces[0].Name
		}
		return ""
	}
	// fold advances revealed over the live log since the last call and
	// reports whether any secret card is still hidden.
	fold := func(e *rules.Engine) bool {
		evs := e.L.Events
		for _, ev := range evs[evDone:] {
			if ev.Obj != 0 && (publicZone(ev.To) || ev.Kind == events.PutOnStack) {
				revealed[nameOf(e, ev.Obj)] = true
			}
			if ev.Kind == events.Note && !ev.Secret {
				for _, id := range ev.IDs {
					revealed[nameOf(e, id)] = true
				}
			}
		}
		evDone = len(evs)
		for i := range e.G.Objs {
			o := &e.G.Objs[i]
			if o.Card != nil && publicZone(o.Zone) {
				revealed[nameOf(e, o.ID)] = true
			}
		}
		hidden := false
		for _, z := range []state.Zone{state.ZHand, state.ZLibrary} {
			for _, id := range e.G.Zone(z, 1) {
				if n := nameOf(e, id); secretName[n] && !revealed[n] {
					hidden = true
				}
			}
		}
		return hidden
	}
	viewNames := func(v *view.View) {
		if v == nil {
			return
		}
		vs := newLeakScan(secret)
		for n := range vs.scan("view", v) {
			revealed[n] = true
		}
	}
	check := func(kind string, d *decision.Decision, env *bots.Env, b *botpolicy.Board, v *view.View) {
		mu.Lock()
		defer mu.Unlock()
		r.mu.RLock()
		tbl := r.tables[tid]
		r.mu.RUnlock()
		tbl.mu.RLock()
		m := tbl.cur
		tbl.mu.RUnlock()
		if m == nil {
			t.Errorf("the bot was asked with no live match")
			return
		}
		m.mu.RLock()
		if fold(m.e) {
			secretsHidden++
		}
		m.mu.RUnlock()
		viewNames(v)
		decisions++
		kinds[kind+":"+string(d.Kind)]++
		inputs := map[string]any{}
		if b != nil {
			inputs["board"] = b
		}
		if env != nil {
			envs++
			if env.Search.Engine != nil {
				roots++
			} else if env.RootRefused == posedRefusal {
				posed++
			} else {
				otherRefusals = append(otherRefusals, "seq "+strconv.FormatUint(d.Seq, 10)+" "+string(d.Kind)+": "+env.RootRefused)
			}
			inputs["setup"] = env.Search.Setup
			inputs["root"] = env.Search.Engine
			inputs["searchBoard"] = env.Search.Board
			inputs["feed"] = env.Search.Feed
			inputs["refused"] = env.RootRefused
			if setup := env.Search.Setup; setup.Belief == nil || len(setup.Decks) != 2 || setup.Decks[1] != nil || setup.Decks[0] == nil {
				t.Errorf("seq %d: the Setup is not closed (belief %v)", d.Seq, setup.Belief != nil)
			}
		}
		start := time.Now()
		for label, in := range inputs {
			for name, paths := range scanner.scan(label, in) {
				if !revealed[name] {
					leaks = append(leaks, kind+" seq "+strconv.FormatUint(d.Seq, 10)+": "+name+" at "+strings.Join(paths, " | "))
				}
			}
		}
		walks++
		walkTime += time.Since(start)
	}
	opts.Seats = func(names []string, seed uint64) []seat.Seat {
		inner, err := bots.New(policy, bots.Options{Seed: seed ^ 1, AutoPayMana: true, SearchParallelism: 4})
		if err != nil {
			panic(err)
		}
		return []seat.Seat{&privacySpy{inner: inner, check: check}, heuristic.NewBot(seed ^ 2)}
	}
	var err error
	r, err = New(opts)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer r.Close()
	// One single-shot human table per seed, with the London mulligan on so
	// the non-searched Board path is asked too.
	for g, seed := range c.Seeds {
		mu.Lock()
		tid, revealed, evDone = TableID("g"+strconv.Itoa(g+1)), map[string]bool{}, 0
		mu.Unlock()
		cfg := TableConfig{ID: tid, Name: policy + " vs a human", Seats: 2,
			Decks: []string{"human", "bot"}, // match 1 deals seat i Decks[(i+1)%2]
			Seed:  seed, Spectator: view.Public, Humans: []int{1}, BotPolicy: policy,
			AutoMana: true, BotAutoPayMana: true, Mulligans: 2}
		if err := r.AddTable(cfg); err != nil {
			t.Fatalf("AddTable refused %s at a human table without the decklist opt-in: %v", policy, err)
		}
		if err := r.Start(tid); err != nil {
			t.Fatalf("Start: %v", err)
		}
		r.Wait(tid)
		ms, err := r.Matches(tid)
		if err != nil || len(ms) != 1 {
			t.Fatalf("matches %v, %v", ms, err)
		}
		if got := []string{ms[0].Seats[0].Deck, ms[0].Seats[1].Deck}; got[0] != "bot" || got[1] != "human" {
			t.Fatalf("seat decks %v, want the bot deck at the %s seat and the human deck at the human's", got, policy)
		}
		if ms[0].State == protocol.MatchCrashed {
			t.Fatalf("game %d crashed", g+1)
		}
		mu.Lock()
		nRevealed := 0
		for n := range secretName {
			if revealed[n] {
				nRevealed++
			}
		}
		t.Logf("game %d: %s after %d turns (%s); %d of %d secret names revealed by the end", g+1, ms[0].State, ms[0].Turns, ms[0].Result, nRevealed, len(secretName))
		mu.Unlock()
	}
	mu.Lock()
	defer mu.Unlock()
	t.Logf("bot decisions %d (by path:kind %v), Env %d, roots %d, refused at a posed resolution %d; secrets still hidden at %d decisions; %d walks, %v",
		decisions, kinds, envs, roots, posed, secretsHidden, walks, walkTime)
	for k, n := range scanner.skipped {
		t.Logf("not walked: %s (%d times)", k, n)
	}
	// Every searched decision got a root, bar the posed-resolution refusals:
	// a feed that stopped shows up here as "no live observation feed".
	if len(otherRefusals) > 0 {
		t.Errorf("%d roots refused for other reasons, first: %s", len(otherRefusals), otherRefusals[0])
	}
	// Non-vacuity: the bot searched real roots while the human held cards
	// it had not seen.
	if roots < 10 || secretsHidden < 10 {
		t.Fatalf("too little evidence: %d roots, %d decisions with hidden secrets", roots, secretsHidden)
	}
	if len(leaks) > 0 {
		sort.Strings(leaks)
		if len(leaks) > 40 {
			leaks = leaks[:40]
		}
		t.Fatalf("%d leak(s) of the human's unrevealed cards into %s's inputs:\n%s", len(leaks), policy, strings.Join(leaks, "\n"))
	}
}
