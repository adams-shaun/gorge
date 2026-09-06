package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/state"
)

const boltSrc = "Name:Bolt\nManaCost:R\nTypes:Instant\nA:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 1\nOracle:x\n"

// castTarget casts id from the pending priority decision, answers the target
// decision by choosing the offered option whose Player is want, then empties
// the stack. The bare castObj helper always picks options[0] (the first
// living seat), which for a ValidTgts$ Player spell is the caster themselves;
// Storm's copies keep that choice, so the whole burst lands on one player and
// the life assertion here wants it routed to the opponent (seat 1).
func castTarget(t *testing.T, e *Engine, id state.ObjID, want state.PlayerID) {
	t.Helper()
	d := e.Pending()
	if d == nil {
		t.Fatal("no decision pending")
	}
	idx := -1
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == id {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("no cast option for %d: %+v", id, d.Options)
	}
	submitChoices(t, e, idx)
	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("expected a target decision, got %+v", d)
	}
	tIdx := -1
	for _, o := range d.Options {
		if o.Kind == "player" && o.Player == want {
			tIdx = o.Index
		}
	}
	if tIdx < 0 {
		t.Fatalf("seat %d not offered as a target: %+v", want, d.Options)
	}
	submitChoices(t, e, tIdx)
	passUntilStackEmpty(t, e, 20)
}

// TestStormCopiesTheSpellOncePerSpellCastBefore is Task 17's end-to-end
// Storm carrier: two Instants are cast before a Storm sorcery, so Storm's
// amount is two copies on top of the original. Each copy (and the original)
// lives/dies on its own, and every resolved copy sits in exile (CR 707.10a:
// a copy is a different object; CR 608.2m/111.7 settle a resolves-to-copy
// spell there), while the original goes to the graveyard as a plain sorcery.
func TestStormCopiesTheSpellOncePerSpellCastBefore(t *testing.T) {
	tendrils := "Name:Tendrils\nManaCost:2 B B\nTypes:Sorcery\nK:Storm\nA:SP$ LoseLife | ValidTgts$ Player | LifeAmount$ 2 | SubAbility$ DBGainLife\nSVar:DBGainLife:DB$ GainLife | Defined$ You | LifeAmount$ 2\nOracle:x\n"
	e, cfg, td := newFixtureDeck(t, 91, tendrils, boltSrc, boltSrc)
	for i := 0; i < 2; i++ {
		b := addToHand(t, e, 0, boltSrc)
		addMana(t, e, 0, "R")
		castObj(t, e, b)
		passUntilStackEmpty(t, e, 20)
	}
	if e.CastThisTurn() != 2 {
		t.Fatalf("cast this turn %d", e.CastThisTurn())
	}
	addMana(t, e, 0, "BBGG")
	life0, life1 := e.G.Players[0].Life, e.G.Players[1].Life
	castTarget(t, e, td, 1) // target the opponent explicitly
	e.Advance()
	passUntilStackEmpty(t, e, 40)
	if e.G.Players[1].Life != life1-6 || e.G.Players[0].Life != life0+6 {
		t.Fatalf("life %d/%d: want original + two copies", e.G.Players[0].Life, e.G.Players[1].Life)
	}
	copies := 0
	for _, o := range e.G.Objs {
		if o.IsCopy {
			copies++
			if o.Zone != state.ZExile {
				t.Fatalf("a resolved copy sits in %s", o.Zone)
			}
		}
	}
	if copies != 2 {
		t.Fatalf("%d copies", copies)
	}
	replayCheck(t, e, cfg)
}

// TestChainLightningDeclinesTheMayPay is Task 17's R-8 carrier: Chain
// Lightning's CopySpellAbility has an UnlessCost (RR), a rider this build
// cannot present as a player choice mid-resolution, so the build records a
// Note and makes no copy at all.
func TestChainLightningDeclinesTheMayPay(t *testing.T) {
	chain := "Name:Chain\nManaCost:R\nTypes:Sorcery\nA:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 3 | SubAbility$ DBCopy1\n" +
		"SVar:DBCopy1:DB$ CopySpellAbility | Defined$ Parent | Controller$ TargetedOrController | UnlessPayer$ TargetedOrController | UnlessCost$ R R | UnlessSwitched$ True | MayChooseTarget$ True\nOracle:x\n"
	e, cfg, ch := newFixtureDeck(t, 92, chain)
	addMana(t, e, 0, "R")
	addMana(t, e, 1, "RR")
	life := e.G.Players[1].Life
	castTarget(t, e, ch, 1)
	passUntilStackEmpty(t, e, 20)
	if e.G.Players[1].Life != life-3 {
		t.Fatal("chain lightning did not resolve")
	}
	for _, o := range e.G.Objs {
		if o.IsCopy {
			t.Fatal("a may-pay copy was created: UnlessCost is declined in this build (R-8)")
		}
	}
	if !hasNote(e, "declined") {
		t.Fatal("no Note recorded the declined may-pay")
	}
	replayCheck(t, e, cfg)
}
