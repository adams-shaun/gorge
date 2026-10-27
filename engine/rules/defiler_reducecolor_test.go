// defiler_reducecolor_test.go — the Defiler cycle's optional cost-for-reduction
// (`Mode$ OptionalCost | ValidCard$ Permanent.<colour> | ValidSA$ Spell |
// Activator$ You | Cost$ PayLife<2> | ReduceColor$ <C>`) end to end on the real
// corpus carriers: the offer gate prices the most-reduced composition, each
// applicable Defiler asks its own yes/no mid-cast, a yes pays the life and
// removes one pip of its colour (never generic), and an unaffordable life
// payment or a missing pip poses no ask.

package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

const (
	defilerBigGreenSrc  = "Name:Big Green\nManaCost:3 G G\nTypes:Creature Beast\nPT:3/3\nOracle:x\n"
	defilerBlueOxSrc    = "Name:Blue Ox\nManaCost:3 U\nTypes:Creature Ox\nPT:2/2\nOracle:x\n"
	defilerGreenZapSrc  = "Name:Green Zap\nManaCost:3 G G\nTypes:Instant\nA:SP$ GainLife | Cost$ 3 G G | LifeAmount$ 1\nOracle:x\n"
	defilerNoPipSrc     = "Name:Greenish Ox\nManaCost:4\nColors:green\nTypes:Creature Ox\nPT:2/2\nOracle:x\n"
	defilerGreenWhiteSr = "Name:Brindle Pair\nManaCost:2 G W\nTypes:Creature Beast\nPT:2/2\nOracle:x\n"
	defilerOneGreenSrc  = "Name:Lone Sprout\nManaCost:1 G\nTypes:Creature Plant\nPT:1/1\nOracle:x\n"
)

// defilerBoard builds seat 0 with the named Defilers on the battlefield and
// spell in hand, then funds the pool.
func defilerBoard(t *testing.T, seed uint64, defilers []string, spellSrc, spell, pool string) (*Engine, Config, state.ObjID) {
	t.Helper()
	e, cfg, _ := altCostEngine(t, seed, defilers, []string{spellSrc}, nil)
	for _, name := range defilers {
		defilerToBattlefield(t, e, name)
	}
	id := findCardObj(t, e, 0, spell, state.ZHand)
	addMana(t, e, 0, pool)
	return e, cfg, id
}

// defilerToBattlefield moves one not-yet-played copy of name onto seat 0's
// battlefield.
func defilerToBattlefield(t *testing.T, e *Engine, name string) {
	t.Helper()
	for _, z := range []state.Zone{state.ZHand, state.ZLibrary} {
		for _, id := range e.G.Zone(z, 0) {
			if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name {
				findCardObjID(t, e, id)
				return
			}
		}
	}
	t.Fatalf("no unplayed %q for seat 0", name)
}

// findCardObjID moves exactly object id to the battlefield (findCardObj
// addresses by name, which cannot tell two copies apart).
func findCardObjID(t *testing.T, e *Engine, id state.ObjID) {
	t.Helper()
	toMain1(t, e)
	o := e.G.Obj(id)
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: o.Zone, To: state.ZBattlefield})
	e.pending = nil
	e.priorityRound()
}

// defilerAsked reports whether the pending decision is a Defiler yes/no.
func defilerAsked(e *Engine) *decision.Decision {
	d := e.Pending()
	if d != nil && d.Kind == decision.KChoose && len(d.Options) == 2 &&
		d.Options[0].Kind == "yes" && d.Options[1].Kind == "no" {
		return d
	}
	return nil
}

// defilerAnswer answers the pending Defiler ask.
func defilerAnswer(t *testing.T, e *Engine, yes bool) {
	t.Helper()
	d := defilerAsked(e)
	if d == nil {
		t.Fatalf("no Defiler yes/no pending: %+v", e.Pending())
	}
	if yes {
		submitChoices(t, e, d.Options[0].Index)
	} else {
		submitChoices(t, e, d.Options[1].Index)
	}
}

// defilerSettle drives the cast to an empty stack, answering the Defilers'
// own SpellCast trigger targets with their first option.
func defilerSettle(t *testing.T, e *Engine) {
	t.Helper()
	for i := 0; i < 60; i++ {
		d := e.Pending()
		switch {
		case d == nil:
			t.Fatal("no decision while settling")
		case d.Kind == decision.KTarget && len(d.Options) > 0:
			submitChoices(t, e, d.Options[0].Index)
		case d.Kind == decision.KTriggerOrder:
			var all []int
			for _, o := range d.Options {
				all = append(all, o.Index)
			}
			submitChoices(t, e, all...)
		case d.Kind == decision.KPriority && len(e.G.Stack) == 0:
			return
		case d.Kind == decision.KPriority:
			passHere(t, e)
		default:
			t.Fatalf("unexpected decision while settling: %+v", d)
		}
	}
	t.Fatal("stack never emptied")
}

func defilerViews(e *Engine, id state.ObjID) int {
	return len(e.reduceColorOptionalViews(e.collectCostStatics(), 0, id))
}

func defilerOnBattlefield(t *testing.T, e *Engine, id state.ObjID) {
	t.Helper()
	if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("object %d did not resolve onto the battlefield: %+v", id, o)
	}
}

func TestDefilerOfVigorEnablesCastWithLessGreen(t *testing.T) {
	t.Parallel()
	e, cfg, spell := defilerBoard(t, 1770, []string{"Defiler of Vigor"}, defilerBigGreenSrc, "Big Green", "CCCG")
	if defilerViews(e, spell) != 1 {
		t.Fatalf("precondition: Defiler of Vigor's ReduceColor$ static does not apply to Big Green")
	}
	if got := e.G.Players[0].Life; got != 20 {
		t.Fatalf("precondition: life = %d, want 20", got)
	}
	// {3}{G}{G} against {3}{G}: only the Defiler's trade makes it castable.
	submitChoices(t, e, castModeOption(t, e, spell, ""))
	defilerAnswer(t, e, true)
	defilerSettle(t, e)
	defilerOnBattlefield(t, e, spell)
	if got := e.G.Players[0].Life; got != 18 {
		t.Fatalf("life = %d, want 18 (2 paid)", got)
	}
	if p := e.G.Players[0].Pool; p != (state.Mana{}) {
		t.Fatalf("pool = %v, want empty", p)
	}
	replayCheck(t, e, cfg)
}

func TestDefilerDeclineCostsFullPrice(t *testing.T) {
	t.Parallel()
	e, cfg, spell := defilerBoard(t, 1771, []string{"Defiler of Vigor"}, defilerBigGreenSrc, "Big Green", "CCCGG")
	if defilerViews(e, spell) != 1 {
		t.Fatalf("precondition: the ReduceColor$ static does not apply")
	}
	submitChoices(t, e, castModeOption(t, e, spell, ""))
	d := defilerAsked(e)
	if d == nil || !strings.Contains(d.Prompt, "{G}") || !strings.Contains(d.Prompt, "2 life") {
		t.Fatalf("precondition: affordable-at-full-price cast was not asked: %+v", e.Pending())
	}
	defilerAnswer(t, e, false)
	defilerSettle(t, e)
	defilerOnBattlefield(t, e, spell)
	if got := e.G.Players[0].Life; got != 20 {
		t.Fatalf("life = %d, want 20 (declined)", got)
	}
	if p := e.G.Players[0].Pool; p != (state.Mana{}) {
		t.Fatalf("pool = %v, want all five mana spent", p)
	}
	replayCheck(t, e, cfg)
}

func TestDefilerReducesOnlyItsColour(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, src, spell, pool string
		applies, wantLife      int
	}{
		// Green permanent whose cost has no {G} pip: the static applies but
		// the reduction would spill to generic, which it must not.
		{"no green pip", defilerNoPipSrc, "Greenish Ox", "CCCC", 1, 20},
		{"non-green", defilerBlueOxSrc, "Blue Ox", "CCCU", 0, 20},
		{"green instant", defilerGreenZapSrc, "Green Zap", "CCCGG", 0, 21},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e, cfg, spell := defilerBoard(t, 1772, []string{"Defiler of Vigor"}, tc.src, tc.spell, tc.pool)
			if got := defilerViews(e, spell); got != tc.applies {
				t.Fatalf("precondition: applicable ReduceColor$ statics = %d, want %d", got, tc.applies)
			}
			submitChoices(t, e, castModeOption(t, e, spell, ""))
			if d := defilerAsked(e); d != nil {
				t.Fatalf("asked a Defiler question for %s: %+v", tc.spell, d)
			}
			defilerSettle(t, e)
			// Green Zap's own GainLife is the instant's +1; no life was paid.
			if got := e.G.Players[0].Life; got != int32(tc.wantLife) {
				t.Fatalf("life = %d, want %d (nothing paid)", got, tc.wantLife)
			}
			if o := e.G.Obj(spell); o == nil || o.Zone == state.ZHand || o.Zone == state.ZStack {
				t.Fatalf("%s was not cast: %+v", tc.spell, o)
			}
			replayCheck(t, e, cfg)
		})
	}
}

func TestDefilerTwoAskedIndependently(t *testing.T) {
	t.Parallel()
	// {2}{G}{W}; paying only Vigor's life leaves {2}{W}.
	e, cfg, spell := defilerBoard(t, 1773, []string{"Defiler of Vigor", "Defiler of Faith"},
		defilerGreenWhiteSr, "Brindle Pair", "CCW")
	if defilerViews(e, spell) != 2 {
		t.Fatalf("precondition: want both Defilers applicable, got %d", defilerViews(e, spell))
	}
	submitChoices(t, e, castModeOption(t, e, spell, ""))
	asked := 0
	for d := defilerAsked(e); d != nil; d = defilerAsked(e) {
		asked++
		green := strings.Contains(d.Prompt, "{G}")
		if !green && !strings.Contains(d.Prompt, "{W}") {
			t.Fatalf("ask names neither {G} nor {W}: %q", d.Prompt)
		}
		defilerAnswer(t, e, green)
	}
	if asked != 2 {
		t.Fatalf("Defiler asks = %d, want 2", asked)
	}
	defilerSettle(t, e)
	defilerOnBattlefield(t, e, spell)
	if got := e.G.Players[0].Life; got != 18 {
		t.Fatalf("life = %d, want 18 (only the green Defiler paid)", got)
	}
	if p := e.G.Players[0].Pool; p != (state.Mana{}) {
		t.Fatalf("pool = %v, want empty", p)
	}
	replayCheck(t, e, cfg)
}

func TestDefilerUnaffordableLifeNotAsked(t *testing.T) {
	t.Parallel()
	// At 1 life CR 119.4 forbids the 2-life payment: the reduced cast is not
	// offered, and a full-price cast is asked nothing.
	e, cfg, spell := defilerBoard(t, 1774, []string{"Defiler of Vigor"}, defilerBigGreenSrc, "Big Green", "CCCG")
	e.emit(events.Event{Kind: events.LifeChange, Player: 0, Amount: -19})
	e.pending = nil
	e.priorityRound()
	if got := e.G.Players[0].Life; got != 1 {
		t.Fatalf("precondition: life = %d, want 1", got)
	}
	if defilerViews(e, spell) != 1 {
		t.Fatalf("precondition: the static does not apply")
	}
	d := e.Pending()
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == spell {
			t.Fatalf("{3}{G}{G} offered with {3}{G} at 1 life: %+v", o)
		}
	}
	// With the full price funded the cast is offered but never asked.
	addMana(t, e, 0, "G")
	submitChoices(t, e, castModeOption(t, e, spell, ""))
	if d := defilerAsked(e); d != nil {
		t.Fatalf("asked to pay 2 life at 1 life: %+v", d)
	}
	defilerSettle(t, e)
	defilerOnBattlefield(t, e, spell)
	if got := e.G.Players[0].Life; got != 1 {
		t.Fatalf("life = %d, want 1", got)
	}
	replayCheck(t, e, cfg)
}
