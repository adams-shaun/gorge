package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// bretagardChapterTwo drives the real corpus Battle for Bretagard through its
// first two chapters, leaving seat 0 with the chapter I Human Warrior token
// and the chapter II Elf Warrior token, and returns the engine, the replay
// config and the Saga's id so a test can add the chapter III lore counter.
func bretagardChapterTwo(t *testing.T) (*Engine, Config, state.ObjID) {
	t.Helper()
	reg := searchTestRegistry(t)
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{lookup(t, reg, "Battle for Bretagard")}, nil)
	saga := moveByName(t, e, 0, "Battle for Bretagard", state.ZBattlefield)
	if o := e.G.Obj(saga); o == nil || o.Zone != state.ZBattlefield || o.Counter("LORE") != 1 {
		t.Fatalf("precondition: Battle for Bretagard entered as %+v, want battlefield with 1 lore", o)
	}
	// Chapter I: create a 1/1 white Human Warrior token.
	answerQuiet(t, e, 80)
	if n := countTokensNamedOnSeat(t, e, 0, "Human Warrior Token"); n != 1 {
		t.Fatalf("precondition: chapter I created %d Human Warrior tokens, want 1", n)
	}
	// Chapter II: create a 1/1 green Elf Warrior token.
	e.emit(events.Event{Kind: events.CounterChange, Obj: saga, Counter: "LORE", Amount: 1})
	answerQuiet(t, e, 80)
	if n := countTokensNamedOnSeat(t, e, 0, "Elf Warrior Token"); n != 1 {
		t.Fatalf("precondition: chapter II created %d Elf Warrior tokens, want 1", n)
	}
	return e, cfg, saga
}

// bretagardDifferentNameAsk starts chapter III (the third lore counter) and
// drives to the first WithDifferentNames$ pick. It asserts the ask's shape
// (an optional single pick with the multi-pick resume marker) and returns it.
func bretagardDifferentNameAsk(t *testing.T, e *Engine, saga state.ObjID) *decision.Decision {
	t.Helper()
	e.emit(events.Event{Kind: events.CounterChange, Obj: saga, Counter: "LORE", Amount: 1})
	d := drainToTargetAsk(t, e, 200)
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "copypermanent_choice" {
		t.Fatalf("chapter III did not pose the WithDifferentNames$ pick: %+v", d)
	}
	if d.Min != 0 || d.Max != 1 {
		t.Fatalf("WithDifferentNames$ ask Min/Max = %d/%d, want 0/1 (accepting an empty stop)", d.Min, d.Max)
	}
	return d
}

// copyNamedOnSeat counts battlefield copy tokens named name on seat p.
func copyNamedOnSeat(t *testing.T, e *Engine, p state.PlayerID, name string) int {
	t.Helper()
	n := 0
	for _, id := range e.G.Zone(state.ZBattlefield, p) {
		o := e.G.Obj(id)
		if o != nil && o.IsCopy && o.Face() != nil && o.Face().Name == name {
			n++
		}
	}
	return n
}

// optionName reads the derived name of a pick option's object.
func optionName(t *testing.T, e *Engine, o decision.Option) string {
	t.Helper()
	if o.Obj == 0 {
		return ""
	}
	if got := e.G.Obj(o.Obj); got == nil || got.Face() == nil {
		t.Fatalf("pick option %+v names no object", o)
	}
	return e.G.Obj(o.Obj).Face().Name
}

// optionForName returns the option index of the first offer whose derived name
// is name, or -1.
func optionForName(t *testing.T, e *Engine, d *decision.Decision, name string) int {
	t.Helper()
	for _, o := range d.Options {
		if optionName(t, e, o) == name {
			return o.Index
		}
	}
	return -1
}

// noDifferentNamesNote fails if the log carries the retired
// WithDifferentNames$ skip Note or an unimplemented-API note.
func noDifferentNamesNote(t *testing.T, e *Engine) {
	t.Helper()
	for _, ev := range e.L.Events {
		if ev.Kind != events.Note {
			continue
		}
		if strings.Contains(ev.Text, "WithDifferentNames$") ||
			strings.Contains(ev.Text, "unimplemented API CopyPermanent") {
			t.Fatalf("chapter III still degrades loudly: %q", ev.Text)
		}
	}
}

// TestCopyPermanentWithDifferentNamesBretagard is the ticket's headline probe:
// seat 0 controls the chapter I Human Warrior token, the chapter II Elf
// Warrior token and a Treasure token; chapter III offers all three, seat 0
// picks all three, and one copy of each is created with no skip Note.
func TestCopyPermanentWithDifferentNamesBretagard(t *testing.T) {
	t.Parallel()
	e, cfg, saga := bretagardChapterTwo(t)
	e.emit(events.Event{Kind: events.TokenCreate, Player: 0, Text: "c_a_treasure_sac"})
	if n := countTokensNamedOnSeat(t, e, 0, "Treasure Token"); n != 1 {
		t.Fatalf("precondition: %d Treasure tokens, want 1", n)
	}
	d := bretagardDifferentNameAsk(t, e, saga)
	// Every one of the three distinct names is offered by the first pick.
	for _, name := range []string{"Human Warrior Token", "Elf Warrior Token", "Treasure Token"} {
		if idx := optionForName(t, e, d, name); idx < 0 {
			t.Fatalf("first pick did not offer %q: %+v", name, d.Options)
		}
	}
	// Pick all three, one per ask.
	for picked := 0; picked < 3; picked++ {
		cur := e.Pending()
		if cur == nil || cur.Kind != decision.KChoose || cur.ResumeKind != "copypermanent_choice" {
			t.Fatalf("pick %d: no WithDifferentNames$ ask pending, got %+v", picked, cur)
		}
		if len(cur.Options) == 0 {
			t.Fatalf("pick %d: the ask offered no options", picked)
		}
		submitChoices(t, e, cur.Options[0].Index)
	}
	answerQuiet(t, e, 120)
	for _, name := range []string{"Human Warrior Token", "Elf Warrior Token", "Treasure Token"} {
		if n := copyNamedOnSeat(t, e, 0, name); n != 1 {
			t.Fatalf("copies of %q = %d, want exactly 1", name, n)
		}
	}
	noDifferentNamesNote(t, e)
	replayCheck(t, e, cfg)
}

// TestCopyPermanentWithDifferentNamesExcludesSameName: with two Human Warrior
// tokens and one Elf Warrior, picking a Human Warrior removes BOTH Humans from
// the remaining pool (Forge's sharesNameWith negation), so the second ask
// offers only the Elf and at most two copies exist.
func TestCopyPermanentWithDifferentNamesExcludesSameName(t *testing.T) {
	t.Parallel()
	e, cfg, saga := bretagardChapterTwo(t)
	// A second Human Warrior token: same name as the chapter I token.
	e.emit(events.Event{Kind: events.TokenCreate, Player: 0, Text: "w_1_1_human_warrior"})
	if n := countTokensNamedOnSeat(t, e, 0, "Human Warrior Token"); n != 2 {
		t.Fatalf("precondition: %d Human Warrior tokens, want 2", n)
	}
	d := bretagardDifferentNameAsk(t, e, saga)
	human := optionForName(t, e, d, "Human Warrior Token")
	if human < 0 {
		t.Fatalf("first pick offered no Human Warrior: %+v", d.Options)
	}
	submitChoices(t, e, human)

	d = e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "copypermanent_choice" {
		t.Fatalf("second WithDifferentNames$ ask missing: %+v", d)
	}
	if len(d.Options) != 1 || optionName(t, e, d.Options[0]) != "Elf Warrior Token" {
		t.Fatalf("after picking a Human Warrior the pool = %+v, want only the Elf Warrior (the other Human must be excluded)", d.Options)
	}
	submitChoices(t, e, d.Options[0].Index)
	answerQuiet(t, e, 120)

	if n := copyNamedOnSeat(t, e, 0, "Human Warrior Token"); n != 1 {
		t.Fatalf("Human Warrior copies = %d, want 1 (the second Human was not offered)", n)
	}
	if n := copyNamedOnSeat(t, e, 0, "Elf Warrior Token"); n != 1 {
		t.Fatalf("Elf Warrior copies = %d, want 1", n)
	}
	noDifferentNamesNote(t, e)
	replayCheck(t, e, cfg)
}

// TestCopyPermanentWithDifferentNamesStopEarly: an empty answer to the second
// pick stops the loop after exactly one copy.
func TestCopyPermanentWithDifferentNamesStopEarly(t *testing.T) {
	t.Parallel()
	e, cfg, saga := bretagardChapterTwo(t)
	e.emit(events.Event{Kind: events.TokenCreate, Player: 0, Text: "c_a_treasure_sac"})
	d := bretagardDifferentNameAsk(t, e, saga)
	if len(d.Options) == 0 {
		t.Fatal("first ask offered no options")
	}
	submitChoices(t, e, d.Options[0].Index)

	stop := e.Pending()
	if stop == nil || stop.Kind != decision.KChoose || stop.ResumeKind != "copypermanent_choice" {
		t.Fatalf("second WithDifferentNames$ ask missing: %+v", stop)
	}
	if len(stop.Options) == 0 {
		t.Fatal("precondition: the stop ask offered no further picks")
	}
	submitChoices(t, e) // the empty answer stops the loop
	answerQuiet(t, e, 120)

	total := 0
	for _, name := range []string{"Human Warrior Token", "Elf Warrior Token", "Treasure Token"} {
		total += copyNamedOnSeat(t, e, 0, name)
	}
	if total != 1 {
		t.Fatalf("copies after stopping early = %d, want exactly 1", total)
	}
	if d := e.Pending(); d != nil && d.Kind == decision.KChoose && d.ResumeKind == "copypermanent_choice" {
		t.Fatalf("an ask was posed after the player stopped: %+v", d)
	}
	noDifferentNamesNote(t, e)
	replayCheck(t, e, cfg)
}
