package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

// An activated ability's XColor$ ("Spend only black mana on X", Crypt Rats):
// the announced {X} is paid in that colour only, so the X ask offers no value
// the pool's black mana cannot cover and the payment spends black mana.

const xColorRatsSrc = "Name:Crypt Rats\nManaCost:2 B\nTypes:Creature Rat\nPT:1/1\n" +
	"A:AB$ DamageAll | Cost$ X | XColor$ Black | NumDmg$ X | ValidCards$ Creature | ValidPlayers$ Player\n" +
	"SVar:X:Count$xPaid\nOracle:x\n"

// xColorRatsAsk activates the rats with pool floating and returns the X
// announcement decision and the engine.
func xColorRatsAsk(t *testing.T, pool string) (*Engine, state.ObjID, *decision.Decision) {
	t.Helper()
	e, _, rats := newFixtureDeck(t, 34, xColorRatsSrc)
	e.emit(events.Event{Kind: events.MoveZone, Obj: rats, From: state.ZHand, To: state.ZBattlefield})
	addMana(t, e, 0, pool)
	if got := e.G.Players[0].Pool.Total(); got != int32(len(pool)) {
		t.Fatalf("precondition: pool holds %d mana, want %d", got, len(pool))
	}
	if c := pay.AbilityCostOf(e.G.Obj(rats).Face().Abilities[0]); c.X != 1 || c.XColor != uint8(state.MB+1) {
		t.Fatalf("precondition: the ability's cost carries X=%d XColor=%d, want one {X} paid in black", c.X, c.XColor)
	}
	opt, ok := findAbilityOption(e, rats, 0)
	if !ok {
		t.Fatal("Crypt Rats' ability is not offered")
	}
	submitChoices(t, e, opt.Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("want the X announcement, got %+v", d)
	}
	return e, rats, d
}

func xAmounts(d *decision.Decision) []int {
	var xs []int
	for _, o := range d.Options {
		if o.Kind == "x" {
			xs = append(xs, o.Amount)
		}
	}
	return xs
}

// TestXColorBoundsTheXAnnouncement: two black and three red mana announce X
// up to 2, not 5 -- the red mana cannot pay the {X}.
func TestXColorBoundsTheXAnnouncement(t *testing.T) {
	t.Parallel()
	_, _, d := xColorRatsAsk(t, "BBRRR")
	if got, want := xAmounts(d), []int{0, 1, 2}; !slices.Equal(got, want) {
		t.Fatalf("X options %v, want %v", got, want)
	}
}

// TestXColorPaysWithBlackMana: X = 2 spends the two black mana and leaves
// the white floating (a generic X would spend the white first).
func TestXColorPaysWithBlackMana(t *testing.T) {
	t.Parallel()
	e, _, d := xColorRatsAsk(t, "WWBB")
	pick := -1
	for _, o := range d.Options {
		if o.Kind == "x" && o.Amount == 2 {
			pick = o.Index
		}
	}
	if pick < 0 {
		t.Fatalf("X = 2 not offered: %+v", d.Options)
	}
	submitChoices(t, e, pick)
	for i := 0; i < 8 && len(e.G.Stack) == 0; i++ {
		d := e.Pending()
		if d == nil || d.Kind != decision.KChoose || len(d.Options) == 0 {
			break
		}
		submitChoices(t, e, d.Options[0].Index) // a payment window: take its first answer
	}
	if len(e.G.Stack) != 1 {
		t.Fatalf("the ability is not on the stack: stack=%v pending=%+v", e.G.Stack, e.Pending())
	}
	pool := e.G.Players[0].Pool
	if pool[state.MB] != 0 || pool[state.MW] != 2 {
		t.Fatalf("pool after paying X=2: W=%d B=%d, want W=2 B=0", pool[state.MW], pool[state.MB])
	}
}
