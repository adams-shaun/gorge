package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// A counter replacement whose recipient list carries a Player alternative
// (Innkeeper's Talent level 3, Vorinclex: ValidObject$
// Permanent.inZoneBattlefield,Player) must also apply to counters put on a
// player; the object-only gate used to reject every PlayerCounterChange.

// TestCounterReplacementPlayerRecipient: Innkeeper's Talent at level 3 doubles
// the poison an infect creature's combat damage deals (2 -> 4), Vorinclex
// doubles a player counter its controller puts, and permanent counters still
// double.
func TestCounterReplacementPlayerRecipient(t *testing.T) {
	t.Parallel()

	t.Run("innkeepers talent doubles infect combat poison", func(t *testing.T) {
		t.Parallel()
		e := combatEngine(t)
		talent := onBoardCard(t, e, 0, tokenReplCorpusCard(t, "Innkeeper's Talent"))
		e.emit(events.Event{Kind: events.ClassLevelChange, Obj: talent, Amount: 2})
		if got := e.G.Obj(talent).ClassLevel(); got != 3 {
			t.Fatalf("precondition: Innkeeper's Talent level = %d, want 3", got)
		}
		inf := onBoard(t, e, 0, "Name:Infecter\nManaCost:1 B\nTypes:Creature Zombie\nPT:2/2\nK:Infect\nOracle:x\n")
		e.G.Obj(inf).SummonSick = false
		if !e.HasKeyword(inf, "Infect") {
			t.Fatal("precondition: attacker does not read infect")
		}
		if got := e.G.Players[1].Counter("POISON"); got != 0 {
			t.Fatalf("precondition: defender already carries %d poison", got)
		}
		e.askAttackers()
		submitAttackers(t, e, inf)
		if got := e.G.Players[1].Counter("POISON"); got != 4 {
			t.Fatalf("defender poison = %d, want 4 (2 combat damage as poison, doubled)", got)
		}
	})

	t.Run("vorinclex doubles a player counter its controller puts", func(t *testing.T) {
		t.Parallel()
		e := layerEngine(t)
		onBoardCard(t, e, 0, tokenReplCorpusCard(t, "Vorinclex, Monstrous Raider"))
		prev := e.SetCounterAdder(0)
		e.emit(events.Event{Kind: events.PlayerCounterChange, Player: 1, Counter: "POISON", Amount: 2})
		e.SetCounterAdder(prev)
		if got := e.G.Players[1].Counter("POISON"); got != 4 {
			t.Fatalf("poison put by Vorinclex's controller = %d, want 4", got)
		}
		// The opponent half of the pair: seat 1 putting counters on a player
		// is halved (2 -> 1).
		prev = e.SetCounterAdder(1)
		e.emit(events.Event{Kind: events.PlayerCounterChange, Player: 0, Counter: "ENERGY", Amount: 2})
		e.SetCounterAdder(prev)
		if got := e.G.Players[0].Counter("ENERGY"); got != 1 {
			t.Fatalf("energy put by the opponent = %d, want 1 (halved)", got)
		}
	})

	t.Run("permanent counters still double", func(t *testing.T) {
		t.Parallel()
		e := layerEngine(t)
		onBoardCard(t, e, 0, tokenReplCorpusCard(t, "Vorinclex, Monstrous Raider"))
		bear := onBoard(t, e, 0, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
		if o := e.G.Obj(bear); o == nil || o.Zone != state.ZBattlefield {
			t.Fatal("precondition: bear not on the battlefield")
		}
		prev := e.SetCounterAdder(0)
		e.emit(events.Event{Kind: events.CounterChange, Obj: bear, Counter: "P1P1", Amount: 2})
		e.SetCounterAdder(prev)
		if got := e.G.Obj(bear).Counter("P1P1"); got != 4 {
			t.Fatalf("P1P1 on a permanent = %d, want 4", got)
		}
	})
}
