package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Effect-delivered Event$ Moved redirects whose ReplaceWith$ body carries a
// SubAbility$ rider or RememberChanged$: Lilah, Feather,
// Gandalf of the Secret Fire, Forger's Foundry and Soulfire Grand Master.
// A bare-exile-only effects.replacementRedirectsToExile would emit the loud
// "unsupported" Note for each and send the resolving spell to the graveyard.

// lbResolve drains the stack, answering nothing.
func lbResolve(t *testing.T, e *Engine) {
	t.Helper()
	for i := 0; i < 40 && e.Pending() != nil && len(e.G.Stack) > 0; i++ {
		d := e.Pending()
		if d.Kind == decision.KPriority {
			submitPass(t, e)
			continue
		}
		var all []int
		for _, o := range d.Options {
			all = append(all, o.Index)
		}
		submitChoices(t, e, all...)
	}
	if len(e.G.Stack) != 0 {
		t.Fatalf("stack did not drain: %d items, pending %v", len(e.G.Stack), e.Pending().Kind)
	}
}

func lbNotes(t *testing.T, e *Engine) {
	t.Helper()
	for _, ev := range e.L.Events {
		if ev.Kind == events.Note {
			t.Logf("note: %s", ev.Text)
		}
	}
}

func lbNoUnsupportedNote(t *testing.T, e *Engine) {
	t.Helper()
	for _, ev := range e.L.Events {
		if ev.Kind == events.Note && (strings.Contains(ev.Text, "not implemented") || strings.Contains(ev.Text, "unsupported")) {
			t.Fatalf("loud Note survived: %s", ev.Text)
		}
	}
}

// lbCast casts id and picks the target option whose object is tgt
// (or the first when tgt is 0).
func lbCast(t *testing.T, e *Engine, id, tgt state.ObjID) {
	t.Helper()
	d := e.Pending()
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
	if d := e.Pending(); d != nil && d.Kind == decision.KTarget {
		pick := d.Options[0].Index
		for _, o := range d.Options {
			if tgt != 0 && o.Obj == tgt {
				pick = o.Index
			}
		}
		submitChoices(t, e, pick)
	}
}

func TestLilahExilesMulticolouredSpellAndPlotsIt(t *testing.T) {
	t.Parallel()
	e, cfg := b5Engine(t, "Lilah, Undefeated Slickshot", "Lightning Helix", "Shock")
	searchMoveByName(t, e, "Lilah, Undefeated Slickshot", state.ZBattlefield)
	helix := searchMoveByName(t, e, "Lightning Helix", state.ZHand)
	shock := searchMoveByName(t, e, "Shock", state.ZHand)
	addMana(t, e, 0, "RRW")
	lbCast(t, e, helix, 0)
	lbResolve(t, e)
	o := e.G.Obj(helix)
	if o.Zone != state.ZExile {
		t.Fatalf("Lightning Helix rests in %v, want exile (Lilah's redirect)", o.Zone)
	}
	if o.PlottedTurn == 0 {
		t.Fatal("the exiled spell did not become plotted (Lilah's DBPlot rider)")
	}
	// A monocoloured spell is not Lilah's and must still be graveyarded.
	lbCast(t, e, shock, 0)
	lbResolve(t, e)
	if z := e.G.Obj(shock).Zone; z != state.ZGraveyard {
		t.Fatalf("Shock rests in %v, want graveyard (not multicoloured)", z)
	}
	lbNoUnsupportedNote(t, e)
	replayCheck(t, e, cfg)
}

func TestFeatherReturnsTheExiledSpellAtEndStep(t *testing.T) {
	t.Parallel()
	e, cfg := b5Engine(t, "Feather, the Redeemed", "Giant Growth")
	feather := searchMoveByName(t, e, "Feather, the Redeemed", state.ZBattlefield)
	growth := searchMoveByName(t, e, "Giant Growth", state.ZHand)
	addMana(t, e, 0, "G")
	lbCast(t, e, growth, feather)
	lbResolve(t, e)
	if z := e.G.Obj(growth).Zone; z != state.ZExile {
		t.Fatalf("Giant Growth rests in %v, want exile (Feather's redirect)", z)
	}
	driveToStepAll(t, e, e.G.Turn, e.G.Active, state.StepEnd)
	for i := 0; i < 20 && e.G.Obj(growth).Zone == state.ZExile && len(e.G.Stack) > 0; i++ {
		submitPass(t, e)
	}
	if z := e.G.Obj(growth).Zone; z != state.ZHand {
		t.Fatalf("Giant Growth is in %v after the end step, want hand (Feather's delayed return)", z)
	}
	lbNoUnsupportedNote(t, e)
	replayCheck(t, e, cfg)
}

func TestGandalfExilesWithThreeTimeCounters(t *testing.T) {
	t.Parallel()
	e, cfg := b5Engine(t, "Gandalf of the Secret Fire", "Shock")
	searchMoveByName(t, e, "Gandalf of the Secret Fire", state.ZBattlefield)
	shock := searchMoveByName(t, e, "Shock", state.ZHand)
	driveToStepAll(t, e, e.G.Turn+1, 1, state.StepUpkeep)
	submitPass(t, e) // the active opponent passes; seat 0 gets priority
	if d := e.Pending(); d == nil || d.Player != 0 {
		t.Fatalf("seat 0 not holding priority: %+v", d)
	}
	e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "R", Amount: 1})
	e.pending = nil
	e.priorityRound()
	for e.Pending().Player != 0 {
		submitPass(t, e)
	}
	lbCast(t, e, shock, 0)
	lbResolve(t, e)
	o := e.G.Obj(shock)
	if o.Zone != state.ZExile {
		t.Fatalf("Shock rests in %v, want exile (Gandalf's redirect)", o.Zone)
	}
	if n := o.Counter("TIME"); n != 3 {
		t.Fatalf("exiled Shock carries %d time counters, want 3 (counters=%v)", n, o.Counters)
	}
	if !o.SuspendGranted {
		t.Fatal("exiled Shock did not gain suspend (Gandalf's DBPump rider)")
	}
	lbNoUnsupportedNote(t, e)
	replayCheck(t, e, cfg)
}

func TestSoulfireGrandMasterPutsTheNextSpellIntoHand(t *testing.T) {
	t.Parallel()
	e, cfg := b5Engine(t, "Soulfire Grand Master", "Shock", "Lightning Bolt")
	sgm := searchMoveByName(t, e, "Soulfire Grand Master", state.ZBattlefield)
	shock := searchMoveByName(t, e, "Shock", state.ZHand)
	addMana(t, e, 0, "RRRR")
	d := e.Pending()
	activated := false
	for _, o := range d.Options {
		if o.Kind == "ability" && o.Obj == sgm {
			submitChoices(t, e, o.Index)
			activated = true
			break
		}
	}
	if !activated {
		t.Fatalf("no activation offered for Soulfire Grand Master: %+v", d.Options)
	}
	for i := 0; i < 6 && e.Pending() != nil && e.Pending().Kind != decision.KPriority; i++ {
		submitChoices(t, e, 0)
	}
	lbResolve(t, e)
	addMana(t, e, 0, "R")
	lbCast(t, e, shock, 0)
	lbResolve(t, e)
	if z := e.G.Obj(shock).Zone; z != state.ZHand {
		lbNotes(t, e)
		for _, ce := range e.active() {
			t.Logf("active: src=%d ev=%q body=%q", ce.Source, ce.ReplacementEvent, ce.ReplacementBody)
		}
		for _, ev := range e.L.Events {
			t.Logf("ev: %v obj=%d from=%v to=%v %s", ev.Kind, ev.Obj, ev.From, ev.To, ev.Text)
		}
		t.Fatalf("Shock rests in %v, want hand (Soulfire Grand Master's buyback)", z)
	}
	// "The NEXT time": the promise is spent, so the following spell is an
	// ordinary graveyard one.
	bolt := searchMoveByName(t, e, "Lightning Bolt", state.ZHand)
	addMana(t, e, 0, "R")
	lbCast(t, e, bolt, 0)
	lbResolve(t, e)
	if z := e.G.Obj(bolt).Zone; z != state.ZGraveyard {
		t.Fatalf("second spell Lightning Bolt rests in %v, want graveyard (the buyback was one-shot)", z)
	}
	lbNoUnsupportedNote(t, e)
	replayCheck(t, e, cfg)
}

// Forger's Foundry: "When you spend this mana to cast an instant or sorcery
// spell with mana value 3 or less, you may exile that spell instead of
// putting it into its owner's graveyard as it resolves."
func TestForgersFoundryMayExileTheSpellItPaidFor(t *testing.T) {
	t.Parallel()
	for _, accept := range []bool{true, false} {
		e, cfg := b5Engine(t, "Forger's Foundry", "Opt")
		foundry := searchMoveByName(t, e, "Forger's Foundry", state.ZBattlefield)
		opt := searchMoveByName(t, e, "Opt", state.ZHand)
		if o := e.G.Obj(foundry); o.Zone != state.ZBattlefield || o.Tapped {
			t.Fatalf("precondition: Foundry zone=%v tapped=%v", o.Zone, o.Tapped)
		}
		if e.G.Obj(opt).Zone != state.ZHand {
			t.Fatalf("precondition: Opt in %v", e.G.Obj(opt).Zone)
		}
		activateMana(t, e, foundry)
		castSeeded(t, e, opt)
		for i := 0; i < 40 && len(e.G.Stack) > 0; i++ {
			d := e.Pending()
			switch d.Kind {
			case decision.KPriority:
				submitPass(t, e)
			default:
				ans := 0
				for _, o := range d.Options {
					if (accept && o.Kind == "apply") || (!accept && o.Kind == "decline") {
						ans = o.Index
					}
				}
				submitChoices(t, e, ans)
			}
		}
		want := state.ZGraveyard
		if accept {
			want = state.ZExile
			if got := e.G.Obj(opt).ExiledWith; got != foundry {
				t.Fatalf("Opt's ExiledWith = %d, want the Foundry %d (ExiledWithEffectSource$)", got, foundry)
			}
		}
		if z := e.G.Obj(opt).Zone; z != want {
			lbNotes(t, e)
			t.Fatalf("accept=%v: Opt rests in %v, want %v", accept, z, want)
		}
		lbNoUnsupportedNote(t, e)
		replayCheck(t, e, cfg)
	}
}
