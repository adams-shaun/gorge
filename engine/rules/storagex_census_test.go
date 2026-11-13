package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// A storage land's only mana ability: remove X storage counters, add X {B}.
// The Amount$ is Count$xPaid, so with no X announced the strict read is zero.
const storageXLandFixtureSrc = `Name:Fixture Storage Vault
ManaCost:no cost
Types:Land
A:AB$ Mana | Cost$ T SubCounter<X/STORAGE> | Produced$ B | Amount$ X | SpellDescription$ Add {B} for each storage counter removed.
SVar:X:Count$xPaid
Oracle:{T}, Remove X storage counters: Add {X}{B}.
`

// The {B} spell the Vault's counters have to pay for.
const storageXSpellFixtureSrc = "Name:Fixture Black One\nManaCost:B\nTypes:Creature Cat\nPT:1/1\nOracle:x\n"

// TestStorageXSourceIsACensusGap: a storage land with counters is a census
// gap, not a source that makes nothing, so the planner scripts its X
// activation for the {B} cast. With no counters the same land really makes
// nothing and the cast stays a proven "insufficient".
func TestStorageXSourceIsACensusGap(t *testing.T) {
	t.Parallel()
	findCast := func(t *testing.T, e *Engine, spell state.ObjID) PotentialPlan {
		t.Helper()
		for _, pp := range e.PotentialPaymentPlans(0) {
			if pp.Action.Kind == "cast" && pp.Action.Obj == spell {
				return pp
			}
		}
		t.Fatalf("the {B} cast is not a potential play")
		return PotentialPlan{}
	}
	setup := func(t *testing.T, counters int32) (*Engine, state.ObjID, state.ObjID) {
		t.Helper()
		e, _, spell := newFixtureDeck(t, 6311, storageXSpellFixtureSrc)
		land := onBoard(t, e, 0, storageXLandFixtureSrc)
		putCountersOn(t, e, 0, "Fixture Storage Vault", "STORAGE", counters)
		driveToStep(t, e, 1, 0, state.StepMain1)
		o := e.G.Obj(land)
		if o == nil || o.Zone != state.ZBattlefield || o.Tapped {
			t.Fatalf("fixture: the storage land is %+v, want an untapped battlefield permanent", o)
		}
		if got := o.Counter("STORAGE"); got != counters {
			t.Fatalf("fixture: the land carries %d storage counters, want %d", got, counters)
		}
		if got := e.G.Obj(spell); got == nil || got.Zone != state.ZHand {
			t.Fatalf("fixture: the {B} spell is %+v, want in hand", got)
		}
		return e, land, spell
	}

	t.Run("with counters", func(t *testing.T) {
		t.Parallel()
		e, land, spell := setup(t, 2)
		// The census must read the source as yielding, and the plan must then
		// activate it (the compared values really differ: 2 counters vs 0).
		if !e.windowSourceYields(0, land) {
			t.Fatal("a storage land with 2 counters reads as yielding nothing")
		}
		pp := findCast(t, e, spell)
		if pp.Reason != "" || len(pp.Script) == 0 {
			t.Fatalf("2 counters: verdict %q %q, script %d steps; want a scripted payment", pp.Reason, pp.Detail, len(pp.Script))
		}
		activated := false
		for _, st := range pp.Script {
			for _, pk := range st.Picks {
				if pk.Kind == "activate" && pk.Obj == land {
					activated = true
				}
			}
		}
		if !activated {
			t.Fatalf("the script does not activate the storage land: %+v", pp.Script)
		}
	})

	t.Run("no counters", func(t *testing.T) {
		t.Parallel()
		e, land, spell := setup(t, 0)
		if e.windowSourceYields(0, land) {
			t.Fatal("a storage land with no counters reads as yielding mana")
		}
		pp := findCast(t, e, spell)
		if pp.Reason != "insufficient" || len(pp.Script) != 0 {
			t.Fatalf("0 counters: verdict %q %q, script %d steps; want insufficient with no script", pp.Reason, pp.Detail, len(pp.Script))
		}
	})
}
