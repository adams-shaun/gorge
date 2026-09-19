package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// kw:Replicate (CR 702.55a) pinned end to end on the REAL corpus carriers.
// The deck is built from compiled corpus cards only, so no Forge script text
// is committed. None of the three carriers (Changing Loyalty, Pyromatics —
// and the declined shape reuses Changing Loyalty) is in any legacy golden
// deck, so no chain head depends on these cards.
//
// The helpers come from search_library_test.go (searchTestRegistry,
// searchCorpusCard, searchMoveByName), cast_test.go (addMana, submitChoices,
// castObj) and replacement_updated_test.go (passUntilStackEmpty) — all the
// same package.

// replicateEngine deals seat 0 a 40-card deck whose first card is the named
// replicate spell, then Mountains, Forests and Grizzly Bears (the creature
// the Changing Loyalty tests enchant); the opponent's deck is all Mountains.
// The seed is advanced to start seat 0.
func replicateEngine(t *testing.T, hero string) (*Engine, Config, *cards.Registry) {
	t.Helper()
	reg := searchTestRegistry(t)
	mountain := searchCorpusCard(t, reg, "Mountain")
	forest := searchCorpusCard(t, reg, "Forest")
	bear := searchCorpusCard(t, reg, "Grizzly Bears")
	deck := []*cards.Card{searchCorpusCard(t, reg, hero)}
	for i := 0; i < 6; i++ {
		deck = append(deck, mountain)
	}
	for i := 0; i < 6; i++ {
		deck = append(deck, forest)
	}
	for len(deck) < 40 {
		deck = append(deck, bear)
	}
	opp := make([]*cards.Card, 40)
	for i := range opp {
		opp[i] = mountain
	}
	cfg := seatZeroStart(Config{Seed: 9311, Names: []string{"rep", "opp"},
		Decks: [][]*cards.Card{deck, opp}, Tokens: reg.Tokens})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	return e, cfg, reg
}

// castBear puts a Grizzly Bears on the battlefield under seat 0 the ordinary
// way — moved to hand, funded, cast through the pending decision — so every
// event in the log is engine-produced and replayCheck stays honest.
func castBear(t *testing.T, e *Engine) state.ObjID {
	t.Helper()
	bear := searchMoveByName(t, e, "Grizzly Bears", state.ZHand)
	addMana(t, e, 0, "GG") // Grizzly Bears is {1}{G}
	castObj(t, e, bear)
	if e.G.Obj(bear).Zone != state.ZBattlefield {
		t.Fatalf("bear did not enter the battlefield: %s", e.G.Obj(bear).Zone)
	}
	return bear
}

// replicateOption returns the priority option casting id in the given mode.
func replicateOption(t *testing.T, e *Engine, id state.ObjID, mode string) decision.Option {
	t.Helper()
	d := e.Pending()
	if d == nil {
		t.Fatal("no decision pending")
	}
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == id && o.Mode == mode {
			return o
		}
	}
	t.Fatalf("no %q cast option for %d: %+v", mode, id, d.Options)
	return decision.Option{}
}

// chooseReplicate submits the count decision's option whose Amount is want.
func chooseReplicate(t *testing.T, e *Engine, want int) {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("expected a KChoose replicate-count decision, got %+v", d)
	}
	for _, o := range d.Options {
		if o.Kind == "replicate" && o.Amount == want {
			submitChoices(t, e, o.Index)
			return
		}
	}
	t.Fatalf("no replicate option for %d: %+v", want, d.Options)
}

// chooseTargetObject answers a pending target decision with the option whose
// Obj is want.
func chooseTargetObject(t *testing.T, e *Engine, want state.ObjID) {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("expected a target decision, got %+v", d)
	}
	for _, o := range d.Options {
		if o.Obj == want {
			submitChoices(t, e, o.Index)
			return
		}
	}
	t.Fatalf("target %d not offered: %+v", want, d.Options)
}

// logHasFlag reports whether the log carries a CastInfo for obj whose flag
// list names name.
func logHasFlag(e *Engine, obj state.ObjID, name string) bool {
	for _, ev := range e.L.Events {
		if ev.Kind == events.CastInfo && ev.Obj == obj {
			for _, part := range splitCSV(ev.Counter) {
				if part == name {
					return true
				}
			}
		}
	}
	return false
}

func splitCSV(s string) []string {
	out := []string{}
	cur := ""
	for _, r := range s {
		if r == ',' {
			out = append(out, cur)
			cur = ""
			continue
		}
		cur += string(r)
	}
	return append(out, cur)
}

// TestReplicateChangingLoyaltyPaidOnceAttachesTheCopy is the Changing Loyalty
// carrier (K:Replicate:2, CR 702.55a): the replicated cast option is offered,
// the count ask is answered once, exactly one IsCopy stack object resolves on
// top of the original, and the copy — a permanent-spell copy — enters the
// battlefield attached to the SAME creature the original enchants. A copy of
// a permanent spell is a different object that resolves as itself (the
// CR 706.10 token question is an engine-wide Storm-era gap, out of scope).
func TestReplicateChangingLoyaltyPaidOnceAttachesTheCopy(t *testing.T) {
	e, cfg, _ := replicateEngine(t, "Changing Loyalty")
	bear := castBear(t, e)
	hero := searchMoveByName(t, e, "Changing Loyalty", state.ZHand)
	addMana(t, e, 0, "BCCC") // base {1}{B} plus one {2}: the B pip and 3 generic

	// The plain and replicated cast options are both offered.
	replicateOption(t, e, hero, "")
	opt := replicateOption(t, e, hero, "replicated")
	submitChoices(t, e, opt.Index)

	// The count ask: ascending 0..max, here exactly 0..1 (the pool affords
	// exactly one payment).
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || len(d.Options) != 2 ||
		d.Options[0].Kind != "replicate" || d.Options[0].Amount != 0 ||
		d.Options[1].Amount != 1 {
		t.Fatalf("replicate count decision %+v", d)
	}
	chooseReplicate(t, e, 1)

	// The target decision, answered at the bear; then both auras resolve.
	chooseTargetObject(t, e, bear)
	passUntilStackEmpty(t, e, 30)

	o := e.G.Obj(hero)
	if o.Zone != state.ZBattlefield || o.AttachedTo != bear {
		t.Fatalf("original: zone=%s attachedTo=%d", o.Zone, o.AttachedTo)
	}
	copies := 0
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		c := e.G.Obj(id)
		if c.IsCopy {
			copies++
			if c.AttachedTo != bear {
				t.Fatalf("copy %d attached to %d, want the same bear %d", id, c.AttachedTo, bear)
			}
			if c.Zone != state.ZBattlefield {
				t.Fatalf("copy in %s", c.Zone)
			}
		}
	}
	if copies != 1 {
		t.Fatalf("%d copies on the battlefield, want 1", copies)
	}
	if !logHasFlag(e, hero, "replicated") {
		t.Fatal("the pay-time CastInfo carries no replicated flag")
	}
	replayCheck(t, e, cfg)
}

// TestReplicateDeclinedIsAPlainCast: the count ask answered 0 declines — the
// cast resolves exactly like the pre-existing plain cast: no copies, no
// replicated flag, no CastInfo event at all (the declined shape emits nothing
// the plain cast would not).
func TestReplicateDeclinedIsAPlainCast(t *testing.T) {
	e, cfg, _ := replicateEngine(t, "Changing Loyalty")
	bear := castBear(t, e)
	hero := searchMoveByName(t, e, "Changing Loyalty", state.ZHand)
	addMana(t, e, 0, "BCCC")
	opt := replicateOption(t, e, hero, "replicated")
	submitChoices(t, e, opt.Index)
	chooseReplicate(t, e, 0)
	chooseTargetObject(t, e, bear)
	passUntilStackEmpty(t, e, 30)

	o := e.G.Obj(hero)
	if o.Zone != state.ZBattlefield || o.AttachedTo != bear {
		t.Fatalf("original: zone=%s attachedTo=%d", o.Zone, o.AttachedTo)
	}
	copies := 0
	for _, id := range e.G.Objs {
		if id.IsCopy {
			copies++
		}
	}
	if copies != 0 {
		t.Fatalf("%d copies, want 0", copies)
	}
	if logHasFlag(e, hero, "replicated") {
		t.Fatal("a declined replicate must not carry the flag")
	}
	for _, ev := range e.L.Events {
		if ev.Kind == events.CastInfo && ev.Obj == hero {
			t.Fatalf("declined replicate emitted a CastInfo: %+v", ev)
		}
	}
	replayCheck(t, e, cfg)
}

// TestReplicatePyromaticsPaidTwiceBoundedByAffordability: the pool affords
// exactly two {1}{R} payments, so the count ask offers exactly 0..2 — not
// more — and the answered 2 yields two copies that resolve alongside the
// original (3 damage instances to the targeted opponent), the instant copies
// resting in exile.
func TestReplicatePyromaticsPaidTwiceBoundedByAffordability(t *testing.T) {
	e, cfg, _ := replicateEngine(t, "Pyromatics")
	hero := searchMoveByName(t, e, "Pyromatics", state.ZHand)
	addMana(t, e, 0, "RRRCCC") // base {1}{R} + two {1}{R}: 3 R pips + 3 generic
	life1 := e.G.Players[1].Life

	opt := replicateOption(t, e, hero, "replicated")
	submitChoices(t, e, opt.Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("expected the replicate count decision, got %+v", d)
	}
	if len(d.Options) != 3 || d.Options[0].Amount != 0 ||
		d.Options[1].Amount != 1 || d.Options[2].Amount != 2 {
		t.Fatalf("count ask not bounded by affordability (want 0..2): %+v", d)
	}
	chooseReplicate(t, e, 2)

	// Pyromatics targets any: aim the opponent.
	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("expected a target decision, got %+v", d)
	}
	tIdx := -1
	for _, o := range d.Options {
		if o.Kind == "player" && o.Player == 1 {
			tIdx = o.Index
		}
	}
	if tIdx < 0 {
		t.Fatalf("seat 1 not offered: %+v", d.Options)
	}
	submitChoices(t, e, tIdx)
	passUntilStackEmpty(t, e, 30)

	if got := life1 - e.G.Players[1].Life; got != 3 {
		t.Fatalf("seat 1 lost %d life, want 3 (original + two copies)", got)
	}
	copies := 0
	for _, o := range e.G.Objs {
		if o.IsCopy {
			copies++
			if o.Zone != state.ZExile {
				t.Fatalf("a resolved instant copy sits in %s", o.Zone)
			}
		}
	}
	if copies != 2 {
		t.Fatalf("%d copies, want 2", copies)
	}
	if !logHasFlag(e, hero, "replicated") {
		t.Fatal("the pay-time CastInfo carries no replicated flag")
	}
	replayCheck(t, e, cfg)
}
