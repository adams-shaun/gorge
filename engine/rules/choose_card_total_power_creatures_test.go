package rules

// Slaughter the Strong's Oracle text ("Each player chooses any number of
// creatures they control with total power 4 or less") bounds the pool to
// creatures. The corpus script carries no Choices$, so the implicit pool is
// each chooser's own battlefield objects -- but WithTotalPower$ is a POWER
// budget, and CR 208.3 gives a noncreature permanent no power to spend. A
// land must never be offered: Forge's ChooseCardEffect draws its
// WithTotalPower$ pool from getCreaturesInPlay() only, and XMage asks for
// creatures. These cases pin both halves: a chooser with only lands gets no
// ask at all, and a chooser with lands plus creatures sees only the
// creatures in the offer.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

const sttPlainsSrc = "Name:Stt Plains\nTypes:Basic Land Plains\nOracle:x\n"

// sttCountCreatures counts seat p's DERIVED battlefield creatures, unlike
// countBattlefieldCreatures (which counts every battlefield object).
func sttCountCreatures(e *Engine, p state.PlayerID) int {
	n := 0
	for _, id := range e.G.Zone(state.ZBattlefield, p) {
		if o := e.G.Obj(id); o != nil && o.Zone == state.ZBattlefield && e.IsCreature(id) {
			n++
		}
	}
	return n
}

// sttLandsBoard seeds the real corpus Slaughter the Strong plus seat 0's
// noncreature lands and seat 0's creature fixtures ([name, PT] pairs),
// moves them all to seat 0's battlefield, and asserts each placement --
// the offer assertions below depend on both kinds of permanent being there.
func sttLandsBoard(t *testing.T, reg *cards.Registry, spell *cards.Card, lands int, creatures [][2]string) (*Engine, []state.ObjID, []state.ObjID) {
	t.Helper()
	extras := []*cards.Card{spell}
	for i := 0; i < lands; i++ {
		extras = append(extras, card(t, sttPlainsSrc))
	}
	for _, pr := range creatures {
		extras = append(extras, card(t, "Name:"+pr[0]+"\nTypes:Creature\nPT:"+pr[1]+"\nOracle:x\n"))
	}
	e, _ := corpusEngineCfg(t, reg, extras, nil)
	landIDs := make([]state.ObjID, 0, lands)
	for i := 0; i < lands; i++ {
		id := moveByName(t, e, 0, "Stt Plains", state.ZBattlefield)
		if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield {
			t.Fatalf("precondition: land %d not on seat 0's battlefield", i)
		}
		landIDs = append(landIDs, id)
	}
	creatureIDs := make([]state.ObjID, 0, len(creatures))
	for _, pr := range creatures {
		id := moveByName(t, e, 0, pr[0], state.ZBattlefield)
		if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield {
			t.Fatalf("precondition: creature %q not on seat 0's battlefield", pr[0])
		}
		creatureIDs = append(creatureIDs, id)
	}
	return e, landIDs, creatureIDs
}

func TestSlaughterTheStrongOffersOnlyCreatures(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	spell := lookup(t, reg, "Slaughter the Strong")

	// A chooser whose battlefield is all lands has no power to budget, so
	// the choice resolves silently: any KChoose here is the bug. The drain
	// fatals on every unexpected non-priority decision, so a land offer
	// fails this case even before the board assertions.
	t.Run("only lands pose no choice", func(t *testing.T) {
		e, lands, _ := sttLandsBoard(t, reg, spell, 3, nil)
		// The pre-sweep board is what makes the no-power premise real: three
		// noncreature permanents and nothing with power.
		if n := sttCountCreatures(e, 0); n != 0 {
			t.Fatalf("precondition: seat 0 has %d creatures, want none", n)
		}
		sttCast(t, e, "Slaughter the Strong", "1WW")
		passUntilStackEmpty(t, e, 200)
		for _, id := range lands {
			o := e.G.Obj(id)
			if o == nil || o.Zone != state.ZBattlefield {
				t.Fatalf("land %d was not offered (no choice), so it must survive", id)
			}
		}
	})

	// Lands and creatures together: the offer is the creatures only, priced
	// by power, and the budget still binds (3/3 and 2/2 are both affordable
	// alone; the single optional pick fits either).
	t.Run("lands and creatures offer only the creatures", func(t *testing.T) {
		e, lands, creatures := sttLandsBoard(t, reg, spell, 3,
			[][2]string{{"Stt Two", "2/2"}, {"Stt Three", "3/3"}})
		// Precondition: the lands are the decoys the old pool would have
		// offered, and both creatures are really on the battlefield.
		if len(lands) != 3 || len(creatures) != 2 {
			t.Fatalf("precondition: %d lands, %d creatures", len(lands), len(creatures))
		}
		for _, id := range creatures {
			if o := e.G.Obj(id); o == nil || !e.IsCreature(id) {
				t.Fatalf("precondition: creature %d is not a battlefield creature", id)
			}
		}
		sttCast(t, e, "Slaughter the Strong", "1WW")
		d := passUntilAskKind(t, e, decision.KChoose, 200)
		if d.ResumeKind != "choice" || d.Player != 0 || !d.HasBudget() || d.MaxSum != 4 {
			t.Fatalf("seat 0 ask = %+v, want an optional budgeted choice", d)
		}
		ids, _ := sttAskOptions(d)
		if len(ids) != len(creatures) {
			t.Fatalf("seat 0 options = %v, want only the creatures %v (lands are not creatures and have no power to budget)", ids, creatures)
		}
		for i, want := range creatures {
			if ids[i] != want {
				t.Fatalf("seat 0 option %d = %d, want creature %d", i, ids[i], want)
			}
		}
		for _, id := range lands {
			for _, got := range ids {
				if got == id {
					t.Fatalf("land %d was offered alongside the creatures: %v", id, ids)
				}
			}
		}
		submitCardChoice(t, e, d, creatures[1]) // keep the 3/3
		passUntilStackEmpty(t, e, 200)
		sttCreature(t, e, 0, "Stt Three")
		if n := sttCountCreatures(e, 0); n != 1 {
			t.Fatalf("seat 0 has %d creatures after the sweep, want 1", n)
		}
	})
}
