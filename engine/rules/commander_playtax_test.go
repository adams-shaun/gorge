package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestCommanderPlayCastPaysTaxAndReversalUncounts: a Play cast of a commander
// from the command zone, free of its mana cost (Geode Golem), still captures
// the CR 903.8 tax; when that cost cannot be paid the cast is reversed (CR
// 733.1), and the reversed cast is uncounted from CmdCasts and from the
// log-derived CommanderCastsFromCommandZone alike, so the next cast's tax is
// unchanged.
func TestCommanderPlayCastPaysTaxAndReversalUncounts(t *testing.T) {
	t.Parallel()
	e, _, cmd := commanderTaxGame(t, 21)
	castCommanderAndReturn(t, e, cmd)
	if n := e.G.Players[0].CmdCasts[0]; n != 1 {
		t.Fatalf("after one command-zone cast CmdCasts = %d, want 1", n)
	}
	if o := e.G.Obj(cmd); o == nil || o.Zone != state.ZCommand {
		t.Fatal("the commander is not back in the command zone")
	}
	// the precondition: no mana anywhere, so the {2} tax cannot be paid
	if e.G.Players[0].Pool.Total() != 0 || len(e.G.Zone(state.ZBattlefield, 0)) != 0 {
		t.Fatalf("seat 0 has mana or permanents: pool %v, battlefield %v", e.G.Players[0].Pool, e.G.Zone(state.ZBattlefield, 0))
	}
	if tax := e.commanderTaxAmount(0, cmd); tax != 2 {
		t.Fatalf("commander tax %d, want 2", tax)
	}
	mark := len(e.L.Events)
	e.beginPlay(0, cmd, true, "", false, false, "")
	pushed, reversed := false, false
	for _, ev := range e.L.Events[mark:] {
		pushed = pushed || ev.Kind == events.PutOnStack && ev.Obj == cmd && ev.From == state.ZCommand
		reversed = reversed || ev.Kind == events.SpellCastReversed
	}
	if e.cast != nil && e.cast.card == cmd {
		// the cast waits on a payment ask: its pending cost carries the tax
		if e.cast.taxGeneric != 2 {
			t.Fatalf("the Play cast captured tax %d, want 2", e.cast.taxGeneric)
		}
		e.abortCast(e.cast, "cost no longer payable", true)
		reversed = true
	}
	if !pushed || !reversed {
		t.Fatalf("the free Play cast was pushed %v, reversed %v: want both", pushed, reversed)
	}
	if o := e.G.Obj(cmd); o == nil || o.Zone != state.ZCommand {
		t.Fatal("the reversed commander is not back in the command zone")
	}
	if n := e.G.Players[0].CmdCasts[0]; n != 1 {
		t.Fatalf("after the reversed cast CmdCasts = %d, want 1", n)
	}
	if n := e.CommanderCastsFromCommandZone(0); n != 1 {
		t.Fatalf("after the reversed cast the log counts %d command-zone casts, want 1", n)
	}
	if tax := e.commanderTaxAmount(0, cmd); tax != 2 {
		t.Fatalf("commander tax after the reversal %d, want 2", tax)
	}
}
