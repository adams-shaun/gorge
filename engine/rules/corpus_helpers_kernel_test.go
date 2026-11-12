package rules

// Shared fixtures for tests that run on a real engine, where every
// mid-resolution ask is posed by the resolution kernel and answered with
// Submit.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// kernelEngine builds a seats-player engine at seat 0's precombat main phase,
// every hand empty (the dealt cards are put back on the library so a hand
// count reads only what the test seats), with the corpus token table so
// Clue and other token scripts resolve.
func kernelEngine(t *testing.T, seats int) *Engine {
	t.Helper()
	return kernelEngineWith(t, seats, nil)
}

// kernelEngineWith is kernelEngine with a hook to adjust the Config first.
func kernelEngineWith(t *testing.T, seats int, mod func(*Config)) *Engine {
	t.Helper()
	names := make([]string, seats)
	decks := make([][]*cards.Card, seats)
	for i := range names {
		names[i] = string(rune('a' + i))
		decks[i] = mountainDeck(t, 40)
	}
	cfg := Config{Seed: 1, Names: names, Decks: decks, Tokens: testutil.CorpusRegistry(t).Tokens}
	if mod != nil {
		mod(&cfg)
	}
	e := New(cfg)
	for p := state.PlayerID(0); int(p) < seats; p++ {
		hand := e.G.Zone(state.ZHand, p)
		lib := append([]state.ObjID(nil), e.G.Zone(state.ZLibrary, p)...)
		for _, id := range hand {
			e.G.Obj(id).Zone = state.ZLibrary
			lib = append(lib, id)
		}
		e.G.SetZone(state.ZHand, p, nil)
		e.G.SetZone(state.ZLibrary, p, lib)
	}
	e.G.Step = state.StepMain1
	e.G.Active, e.G.Priority = 0, 0
	e.G.Turn = 1
	return e
}

// corpusFixtureCard returns the named corpus card.
func corpusFixtureCard(t *testing.T, name string) *cards.Card {
	t.Helper()
	c, ok := testutil.CorpusRegistry(t).Lookup(name)
	if !ok {
		t.Fatalf("corpus has no %q", name)
	}
	return c
}

// putCardIn adds c for owner p into zone z (appended; for the library, top
// puts it on top instead), eventlessly, and returns its id.
func putCardIn(t *testing.T, e *Engine, p state.PlayerID, c *cards.Card, z state.Zone, top bool) state.ObjID {
	t.Helper()
	o := e.G.AddObject(c, p)
	o.Zone = z
	cur := e.G.Zone(z, p)
	if top {
		e.G.SetZone(z, p, append([]state.ObjID{o.ID}, cur...))
	} else {
		e.G.SetZone(z, p, append(append([]state.ObjID(nil), cur...), o.ID))
	}
	if z == state.ZBattlefield {
		e.G.Clock++
		o.Timestamp = e.G.Clock
		e.staticEpoch = -1
		e.activeEpoch = -1
		e.typesEpoch = -1
		if e.layer4InPool {
			e.refreshDerivedTypes()
		}
	}
	return o.ID
}

// cardOfSrc parses an inline fixture script.
func cardOfSrc(t *testing.T, src string) *cards.Card { t.Helper(); return card(t, src) }

// sorceryCard is an inline sorcery named name whose script lines are body.
func sorceryCard(t *testing.T, name string, body ...string) *cards.Card {
	t.Helper()
	return card(t, "Name:"+name+"\nManaCost:B\nTypes:Sorcery\n"+strings.Join(body, "\n")+"\nOracle:x\n")
}

// fundMana fills p's pool with plenty of every colour.
func fundMana(e *Engine, p state.PlayerID) {
	for _, c := range []int{state.MW, state.MU, state.MB, state.MR, state.MG} {
		e.G.Players[p].Pool[c] = 10
	}
}

// givePriority re-poses p's priority decision over the current state.
func givePriority(e *Engine, p state.PlayerID) {
	e.pending = nil
	e.askPriority(p)
}

// castWithMana funds seat p, gives it priority and casts id; it returns the next
// non-priority decision (cast-time or mid-resolution), or nil once the stack
// has emptied at a priority decision.
func castWithMana(t *testing.T, e *Engine, p state.PlayerID, id state.ObjID) *decision.Decision {
	t.Helper()
	fundMana(e, p)
	givePriority(e, p)
	d := e.Pending()
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == id {
			submitChoices(t, e, o.Index)
			return nextAfterSettle(t, e)
		}
	}
	t.Fatalf("no cast option for %d (%s): %+v", id, e.G.Obj(id).Face().Name, d.Options)
	return nil
}

// activatePermanentAbility funds seat p and activates id's first offered ability.
func activatePermanentAbility(t *testing.T, e *Engine, p state.PlayerID, id state.ObjID) *decision.Decision {
	t.Helper()
	fundMana(e, p)
	givePriority(e, p)
	d := e.Pending()
	for _, o := range d.Options {
		if o.Kind == "ability" && o.Obj == id {
			submitChoices(t, e, o.Index)
			return nextAfterSettle(t, e)
		}
	}
	t.Fatalf("no ability option for %d: %+v", id, d.Options)
	return nil
}

// nextAfterSettle passes priority until a non-priority decision is pending (and
// returns it) or the stack is empty at a priority decision (nil).
func nextAfterSettle(t *testing.T, e *Engine) *decision.Decision {
	t.Helper()
	for i := 0; i < 60; i++ {
		d := e.Pending()
		if d == nil || e.G.Over {
			return nil
		}
		if d.Kind != decision.KPriority {
			return d
		}
		if len(e.G.Stack) == 0 && len(e.pendingTriggers) == 0 {
			return nil
		}
		passFirst(t, e)
	}
	t.Fatal("nextAfterSettle: the stack never settled")
	return nil
}

// answerDecision submits choices to the pending decision d and returns the next
// non-priority decision (nil once the stack is empty).
func answerDecision(t *testing.T, e *Engine, d *decision.Decision, choices ...int) *decision.Decision {
	t.Helper()
	if cur := e.Pending(); cur == nil || cur.Seq != d.Seq {
		t.Fatalf("decision %+v is not the pending one (%+v)", d, cur)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: choices}); err != nil {
		t.Fatalf("submit %v to %s/%s: %v", choices, d.Kind, d.ResumeKind, err)
	}
	return nextAfterSettle(t, e)
}

// optionKindIndex returns the index of d's option whose Kind is kind.
func optionKindIndex(t *testing.T, d *decision.Decision, kind string) int {
	t.Helper()
	for _, o := range d.Options {
		if o.Kind == kind {
			return o.Index
		}
	}
	t.Fatalf("no %q option in %+v", kind, d.Options)
	return -1
}

// objOptionIndex returns the index of d's option naming obj.
func objOptionIndex(t *testing.T, d *decision.Decision, obj state.ObjID) int {
	t.Helper()
	for _, o := range d.Options {
		if o.Obj == obj {
			return o.Index
		}
	}
	t.Fatalf("object %d not offered: %+v", obj, d.Options)
	return -1
}

// playerOptionIndex returns the index of d's option naming player p.
func playerOptionIndex(t *testing.T, d *decision.Decision, p state.PlayerID) int {
	t.Helper()
	for _, o := range d.Options {
		if o.Obj == 0 && o.Player == p {
			return o.Index
		}
	}
	t.Fatalf("player %d not offered: %+v", p, d.Options)
	return -1
}

// requireResumeAsk fails unless d is a decision with the given resume kind.
func requireResumeAsk(t *testing.T, d *decision.Decision, resume string) *decision.Decision {
	t.Helper()
	if d == nil || d.ResumeKind != resume {
		t.Fatalf("decision = %+v, want a %q ask", d, resume)
	}
	return d
}

// eventsOfKindFrom returns the events of kind k logged at or after from.
func eventsOfKindFrom(e *Engine, from int, k events.Kind) []events.Event {
	var out []events.Event
	for _, ev := range e.L.Events[from:] {
		if ev.Kind == k {
			out = append(out, ev)
		}
	}
	return out
}

// secretLooksFrom returns the Secret look Notes carrying ids logged at or
// after from.
func secretLooksFrom(e *Engine, from int) []events.Event {
	var out []events.Event
	for _, ev := range e.L.Events[from:] {
		if ev.Kind == events.Note && ev.Secret && len(ev.IDs) > 0 {
			out = append(out, ev)
		}
	}
	return out
}

// publicRevealsFrom returns the public Notes carrying ids logged at or after
// from.
func publicRevealsFrom(e *Engine, from int) []events.Event {
	var out []events.Event
	for _, ev := range e.L.Events[from:] {
		if ev.Kind == events.Note && !ev.Secret && len(ev.IDs) > 0 {
			out = append(out, ev)
		}
	}
	return out
}

func corpusCardText(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", ".cards", "cardsfolder", rel))
	if err != nil {
		t.Fatalf("read corpus card %s: %v", rel, err)
	}
	return string(b)
}

func corpusCardAtPath(t *testing.T, path string) *cards.Card {
	t.Helper()
	c, ds := cards.Parse(filepath.Join("..", "..", ".cards", "cardsfolder", path))
	if len(ds) != 0 {
		t.Fatalf("parse %s: %v", path, ds)
	}
	if ds = c.Link(); len(ds) != 0 {
		t.Fatalf("link %s: %v", path, ds)
	}
	return c
}

// registryCardOrFatal looks a corpus card up and fails fast when it is absent.
func registryCardOrFatal(t *testing.T, reg *cards.Registry, name string) *cards.Card {
	t.Helper()
	c, ok := reg.Lookup(name)
	if !ok {
		t.Fatalf("corpus missing %s", name)
	}
	return c
}

// mshCorpusCardPath parses a live corpus card at test time (GPL scripts stay
// out of the repo), so the CantBlockBy pin runs against the real Forge script
// whose ValidBlocker$ spec the predicate serves.
func mshCorpusCardPath(t *testing.T, name, path string) *cards.Card {
	t.Helper()
	c, ds := cards.Parse(filepath.Join("..", "..", ".cards", "cardsfolder", path))
	if len(ds) != 0 {
		t.Fatalf("parse %s: %v", name, ds)
	}
	if ds = c.Link(); len(ds) != 0 {
		t.Fatalf("link %s: %v", name, ds)
	}
	return c
}

func corpusCardOrFatal(t *testing.T, name string) *cards.Card {
	t.Helper()
	c, ok := testutil.CorpusRegistry(t).Lookup(name)
	if !ok {
		t.Fatalf("corpus card %q missing", name)
	}
	return c
}

// lookup is reg.Lookup or the test fails: a corpus card these tests
// depend on being absent is a corpus-pin change, not something to paper over.
func lookup(t *testing.T, reg *cards.Registry, name string) *cards.Card {
	t.Helper()
	c, ok := reg.Lookup(name)
	if !ok {
		t.Fatalf("corpus fixture: %q missing from the registry", name)
	}
	return c
}

func registryCardByName(t *testing.T, reg *cards.Registry, name string) *cards.Card {
	t.Helper()
	c, ok := reg.Lookup(name)
	if !ok {
		t.Fatalf("card %q not in the corpus", name)
	}
	return c
}

func corpusAlternativeCard(t *testing.T, name string) *cards.Card {
	t.Helper()
	c, ok := testutil.CorpusRegistry(t).Lookup(name)
	if !ok {
		t.Fatalf("corpus missing %s", name)
	}
	return c
}
