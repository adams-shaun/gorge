package effects

// Task param-adapt (CR 702.35a), the DB-body half: a chained
// `DB$ PutCounter | Adapt$ N` body reads N as the count and its put is
// conditional on the recipient holding no +1/+1 counters. Jetfire,
// Ingenious Scientist // Jetfire, Air Guardian is the measured corpus
// carrier (`SVar:DBAdapt:DB$ PutCounter | Adapt$ 3`) -- "Convert Jetfire,
// then adapt 3. (If it has no +1/+1 counters on it, put three +1/+1
// counters on it.)". A chained body never gets the offer-time activation
// gate rules/legal.go poses for an AB$ Adapt activation, so the effect's own
// if-condition is the only gate it has; the engine-side AB pins live in
// rules/adapt_test.go on the real corpus Pteramander.

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// corpusAdaptSA returns Jetfire's REAL compiled DBAdapt sub-ability,
// asserting it carries Adapt$ and no CounterNum$ -- the caller can never be
// handed an SA that only looks like the shape under test.
func corpusAdaptSA(t *testing.T) (*cards.SA, map[string]string) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	c, ok := reg.Lookup("Jetfire, Ingenious Scientist")
	if !ok {
		t.Fatalf("corpus has no Jetfire, Ingenious Scientist")
	}
	for _, f := range c.Faces {
		for name := range f.SVars {
			sa := cards.ResolveSVar(f.SVars, name)
			if sa == nil || sa.API != "PutCounter" {
				continue
			}
			if sa.Params["Adapt"] != "" && sa.Params["CounterNum"] == "" {
				return sa, f.SVars
			}
		}
	}
	t.Fatal("Jetfire's compiled DBAdapt body not found (Adapt$ without CounterNum$)")
	return nil, nil
}

// TestAdaptDBBodyCountsAndConditionsOnNoP1P1: the body places THREE counters
// on a clean recipient (the old CounterNum$-default read placed one), and
// places NOTHING on a recipient that already carries a +1/+1 counter -- the
// if-condition CR 702.35a puts on the effect itself, the only gate a
// resolution-time body ever gets.
func TestAdaptDBBodyCountsAndConditionsOnNoP1P1(t *testing.T) {
	sa, sv := corpusAdaptSA(t)
	if got := sa.Params["Adapt"]; got != "3" {
		t.Fatalf("precondition: compiled Adapt$ = %q, want 3", got)
	}
	h := &fakeHost{}
	h.g = state.NewGame(names(2))
	src := putCounterObject(t, h)
	Resolve(h, &Ctx{Source: src, Controller: 0, SVars: sv}, sa)
	if got := counterChangeCount(h, src); got != 1 {
		t.Fatalf("clean recipient: %d CounterChange events, want 1", got)
	}
	for _, ev := range h.log {
		if ev.Kind == events.CounterChange {
			if ev.Counter != "P1P1" || ev.Amount != 3 {
				t.Fatalf("put = (%s, %d), want (P1P1, 3) -- Adapt$ not read as the count", ev.Counter, ev.Amount)
			}
		}
	}

	// The conditional half: a recipient already carrying a +1/+1 counter.
	h2 := &fakeHost{}
	h2.g = state.NewGame(names(2))
	src2 := putCounterObject(t, h2)
	h2.g.Obj(src2).AddCounter("P1P1", 1)
	Resolve(h2, &Ctx{Source: src2, Controller: 0, SVars: sv}, sa)
	if got := counterChangeCount(h2, src2); got != 0 {
		t.Fatalf("recipient with a +1/+1 counter received %d put(s), want none (CR 702.35a's if-condition)", got)
	}
}
