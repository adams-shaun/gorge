package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Security Rhox's "You may pay {R}{G} rather than pay this spell's mana cost.
// Spend only mana produced by Treasures to cast it this way." is the
// AlternativeCost static's ManaRestriction$ Treasure (pay.Descriptor.OnlyTag):
// the alternative is offered, and paid, from Treasure-produced units only.

// rhoxAltBoard is the probe board: Security Rhox in seat 0's hand at Main1.
func rhoxAltBoard(t *testing.T) (*Engine, Config, state.ObjID) {
	t.Helper()
	rhox := corpusAlternativeCard(t, "Security Rhox")
	e, cfg := tokenReplGame(t, 5, rhox)
	id := moveSeededCard(t, e, 0, rhox, state.ZHand)
	toMain1(t, e)
	return e, cfg, id
}

// rhoxCastOptions returns the printed-cost and alternative-cost cast options
// for Rhox at the pending priority decision.
func rhoxCastOptions(t *testing.T, e *Engine, id state.ObjID) (printed, alt []decision.Option) {
	t.Helper()
	for _, o := range castOptions(t, e) {
		if o.Obj != id {
			continue
		}
		if o.AltCostIndex > 0 {
			alt = append(alt, o)
		} else {
			printed = append(printed, o)
		}
	}
	return printed, alt
}

// treasureMana mints a real Treasure token for seat 0 and sacrifices it for
// one mana of colour (0 = the first offered colour), leaving a typed unit.
func treasureMana(t *testing.T, e *Engine, colour string) {
	t.Helper()
	before := e.G.Zone(state.ZBattlefield, 0)
	e.emit(events.Event{Kind: events.TokenCreate, Player: 0, Text: "c_a_treasure_sac"})
	var tok state.ObjID
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		found := true
		for _, b := range before {
			if b == id {
				found = false
			}
		}
		if found {
			tok = id
		}
	}
	if tok == 0 {
		t.Fatal("the Treasure token was not minted")
	}
	e.pending = nil
	e.priorityRound()
	d := e.Pending()
	act := -1
	for _, o := range d.Options {
		if o.Kind == "activate" && o.Obj == tok {
			act = o.Index
		}
	}
	if act < 0 {
		t.Fatalf("the Treasure's mana ability is not offered: %+v", d.Options)
	}
	submitChoices(t, e, act)
	if cd := e.Pending(); cd != nil && cd.Kind == decision.KChoose {
		pick := -1
		for _, o := range cd.Options {
			if o.ManaSymbol == colour {
				pick = o.Index
			}
		}
		if pick < 0 {
			t.Fatalf("colour %q not offered: %+v", colour, cd.Options)
		}
		submitChoices(t, e, pick)
	}
	e.pending = nil
	e.priorityRound()
}

func TestSecurityRhoxAltCostNeedsTreasureMana(t *testing.T) {
	t.Parallel()
	e, cfg, id := rhoxAltBoard(t)
	addMana(t, e, 0, "RG") // plain mana, no Treasure anywhere
	pl := e.G.Players[0]
	if pl.Pool[state.MR] != 1 || pl.Pool[state.MG] != 1 || pl.TypedMana != [4]state.Mana{} {
		t.Fatalf("precondition: pool %v typed %v, want plain {R}{G}", pl.Pool, pl.TypedMana)
	}
	printed, alt := rhoxCastOptions(t, e, id)
	if len(alt) != 0 {
		t.Fatalf("alternative cost offered on plain mana: %+v", alt)
	}
	if len(printed) != 0 {
		t.Fatalf("printed {2}{R}{G} offered from {R}{G}: %+v", printed)
	}
	// The printed cost still works from enough plain mana.
	addMana(t, e, 0, "CC")
	printed, alt = rhoxCastOptions(t, e, id)
	if len(printed) != 1 || len(alt) != 0 {
		t.Fatalf("with {2}{R}{G}: printed %v alt %v, want one printed option and no alternative", printed, alt)
	}
	replayCheck(t, e, cfg)
}

func TestSecurityRhoxAltCostPaysWithTreasures(t *testing.T) {
	t.Parallel()
	e, cfg, id := rhoxAltBoard(t)
	toMain1(t, e)
	treasureMana(t, e, "R")
	treasureMana(t, e, "G")
	pl := e.G.Players[0]
	if pl.Pool[state.MR] != 1 || pl.Pool[state.MG] != 1 || pl.Pool.Total() != 2 {
		t.Fatalf("precondition: pool %v, want exactly {R}{G}", pl.Pool)
	}
	if pl.TypedMana[state.TypedTreasure][state.MR] != 1 || pl.TypedMana[state.TypedTreasure][state.MG] != 1 {
		t.Fatalf("precondition: Treasure tally %v, want {R}{G} Treasure-produced", pl.TypedMana[state.TypedTreasure])
	}
	_, alt := rhoxCastOptions(t, e, id)
	if len(alt) != 1 {
		t.Fatalf("alternative-cost options = %+v, want exactly one", alt)
	}
	submitChoices(t, e, alt[0].Index)
	drainStack(t, e, 20)
	if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("Security Rhox not on the battlefield: %+v", o)
	}
	if got := e.G.Players[0].Pool.Total(); got != 0 {
		t.Fatalf("pool after the cast = %v, want empty", e.G.Players[0].Pool)
	}
	replayCheck(t, e, cfg)
}

func TestSecurityRhoxAltCostRefusesMixedPool(t *testing.T) {
	t.Parallel()
	e, cfg, id := rhoxAltBoard(t)
	toMain1(t, e)
	treasureMana(t, e, "R")
	addMana(t, e, 0, "G") // plain {G}
	pl := e.G.Players[0]
	if pl.Pool[state.MR] != 1 || pl.Pool[state.MG] != 1 ||
		pl.TypedMana[state.TypedTreasure][state.MR] != 1 || pl.TypedMana[state.TypedTreasure][state.MG] != 0 {
		t.Fatalf("precondition: pool %v typed %v, want Treasure {R} + plain {G}", pl.Pool, pl.TypedMana)
	}
	_, alt := rhoxCastOptions(t, e, id)
	if len(alt) != 0 {
		t.Fatalf("alternative cost offered with a plain {G}: %+v", alt)
	}
	replayCheck(t, e, cfg)
}

// A floating Treasure {R}{G} beside plain {R}{G}: the alternative is paid from
// the Treasure units alone, and the plain pair stays in the pool.
func TestSecurityRhoxAltCostSpendsOnlyTreasureUnits(t *testing.T) {
	t.Parallel()
	e, cfg, id := rhoxAltBoard(t)
	toMain1(t, e)
	treasureMana(t, e, "R")
	treasureMana(t, e, "G")
	addMana(t, e, 0, "RG")
	pl := e.G.Players[0]
	if pl.Pool[state.MR] != 2 || pl.Pool[state.MG] != 2 || pl.TypedMana[state.TypedTreasure].Total() != 2 {
		t.Fatalf("precondition: pool %v typed %v, want {R}{R}{G}{G} with two Treasure units", pl.Pool, pl.TypedMana)
	}
	_, alt := rhoxCastOptions(t, e, id)
	if len(alt) != 1 {
		t.Fatalf("alternative-cost options = %+v, want exactly one", alt)
	}
	submitChoices(t, e, alt[0].Index)
	drainStack(t, e, 20)
	if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("Security Rhox not on the battlefield: %+v", o)
	}
	pl = e.G.Players[0]
	if pl.Pool[state.MR] != 1 || pl.Pool[state.MG] != 1 || pl.Pool.Total() != 2 || pl.TypedMana[state.TypedTreasure].Total() != 0 {
		t.Fatalf("after the cast pool %v typed %v, want the plain {R}{G} left and no Treasure units", pl.Pool, pl.TypedMana)
	}
	replayCheck(t, e, cfg)
}

// The potential-mana bound for a Treasure-restricted payment counts only the
// Treasure producers: an untapped Mountain makes {R} for an unrestricted
// payment and nothing for the restricted one, until a Treasure joins it.
func TestSecurityRhoxAltCostPotentialCountsOnlyTreasures(t *testing.T) {
	t.Parallel()
	e, _, id := rhoxAltBoard(t)
	toMain1(t, e)
	lib := e.G.Zone(state.ZLibrary, 0)
	if len(lib) == 0 {
		t.Fatal("no library card to play as a Mountain")
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: lib[0], From: state.ZLibrary, To: state.ZBattlefield})
	cost := ParseCost("R G")
	bound := func(tag string) state.Mana {
		return e.potentialManaFor(0, &paymentDescriptor{ID: id, Class: paymentSpell, Cost: &cost, OnlyTag: tag})
	}
	if plain := bound(""); plain[state.MR] == 0 {
		t.Fatalf("precondition: the Mountain gives the plain bound no {R}: %v", plain)
	}
	if got := bound("Treasure"); got.Total() != 0 {
		t.Fatalf("restricted bound %v with only a Mountain, want empty", got)
	}
	e.emit(events.Event{Kind: events.TokenCreate, Player: 0, Text: "c_a_treasure_sac"})
	if got := bound("Treasure"); got[state.MR] == 0 || got[state.MG] == 0 {
		t.Fatalf("restricted bound %v with a Treasure, want {R} and {G} reachable", got)
	}
}
