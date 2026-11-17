package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestForeignPendAsksCatchesOptionalAsk pins the predicate the prefix search
// uses to end a line at another player's ask: a queued may-election
// (Judge of Currents' "you may gain 1 life") poses no target and no mode, so
// it is a choice only through kop.TfOptional, and it is posed only when the
// trigger resolves (CR 603.5). A foreign one blocks a play that waits for the
// stack, and nothing else: the payer's own is the search's to answer, an
// instant-speed play is offered with the trigger still pending
// (mana_foreignask_prefix_test.go drives both through scriptDFS), and a
// trigger that asks nothing is placed and the search goes on.
func TestForeignPendAsksCatchesOptionalAsk(t *testing.T) {
	deck := []cardset.Entry{
		{Name: "Judge of Currents", Count: 1},
		{Name: "Firdoch Core", Count: 1},
		{Name: "Plains", Count: 18},
	}
	tbl, da, _, err := cardsettest.Set(t, "tapsdefer").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true})
	judge, core := id["Judge of Currents"], id["Firdoch Core"]
	if !sc.k.gen.anyTapsDefer || !sc.k.gen.has(judge, gfTapsDefer) {
		t.Fatalf("precondition: Judge of Currents is not a deferring Taps watcher")
	}
	if a := &tbl.Cards[judge].Abilities[0]; a.NTargets != 0 || a.NModes != 0 || a.Trig.Flags&kop.TfOptional == 0 {
		t.Fatal("precondition: the watcher is no optional-only ability")
	}
	queued := func(c CardID, ab uint8, ctrl uint8) {
		sc.s.npend = 0
		sc.s.pend[0] = pendingTrig{card: c, ab: ab, ctrl: ctrl}
		sc.s.npend = 1
	}
	queued(judge, 0, 1)
	if !sc.s.foreignPendAsks(sc.k, 0, true) {
		t.Fatal("a foreign optional trigger is no foreign ask of a stack-waiting play: the search would answer the opponent's election itself")
	}
	if sc.s.foreignPendAsks(sc.k, 0, false) {
		t.Fatal("a foreign optional trigger blocks an instant-speed play: the election is posed only at resolution, the play is offered at the payer's priority")
	}
	queued(judge, 0, 0)
	if sc.s.foreignPendAsks(sc.k, 0, true) {
		t.Fatal("the payer's own optional trigger is a foreign ask: gorge's search answers the payer's own asks")
	}
	queued(core, 0, 1)
	if sc.s.foreignPendAsks(sc.k, 0, true) {
		t.Fatal("a foreign trigger that asks nothing is a foreign ask: the search places it and goes on")
	}
}

// TestForeignPendAsksOptionalAsker pins the seat the predicate reads for a
// foreign may-election: optTrigAsker's, not the trigger's controller's -- with
// Tf4OptTrigPlayer the election is the trigger's player's, with Tf5OptSrcCtrl
// the trigger object's live controller's, either of which can be the payer.
// The flags are stamped on the scenario's table copy: no lowered corpus card
// carries them on a Taps watcher (OptionalDecider$ lowered forms ask at other
// events), so the scenario fabricates them as the lockstep's mutation tests
// fabricate theirs.
func TestForeignPendAsksOptionalAsker(t *testing.T) {
	deck := []cardset.Entry{
		{Name: "Judge of Currents", Count: 2},
		{Name: "Firdoch Core", Count: 1},
		{Name: "Plains", Count: 17},
	}
	tbl, da, _, err := cardsettest.Set(t, "tapsdefer").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	judge := id["Judge of Currents"]
	sc := newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true})
	mine, theirs := sc.perm(judge, 0), sc.perm(judge, 1)
	run := func(f4 kop.TrigFlags4, f5 kop.TrigFlags5, trigObj Obj, trigPl uint8, wantForeign bool, why string) {
		t.Helper()
		sc.s.npend = 0
		sc.s.pend[0] = pendingTrig{card: judge, ab: 0, ctrl: 1, trigObj: trigObj, trigPl: trigPl}
		sc.s.npend = 1
		a := &tbl.Cards[judge].Abilities[0]
		f4o, f5o := a.Trig.Flags4, a.Trig.Flags5
		a.Trig.Flags4, a.Trig.Flags5 = f4, f5
		got := sc.s.foreignPendAsks(sc.k, 0, true)
		a.Trig.Flags4, a.Trig.Flags5 = f4o, f5o
		if got != wantForeign {
			t.Fatalf("%s: foreign %v, want %v", why, got, wantForeign)
		}
	}
	run(kop.Tf4OptTrigPlayer, 0, NoObj, 0, false, "Tf4OptTrigPlayer poses the election to the trigger's player, the payer")
	run(kop.Tf4OptTrigPlayer, 0, NoObj, 1, true, "Tf4OptTrigPlayer poses the election to the trigger's player, the opponent")
	run(0, kop.Tf5OptSrcCtrl, mine, 0, false, "Tf5OptSrcCtrl poses the election to the trigger object's controller, the payer")
	run(0, kop.Tf5OptSrcCtrl, theirs, 1, true, "Tf5OptSrcCtrl poses the election to the trigger object's controller, the opponent")
	run(0, 0, NoObj, 0, true, "no override: the election is the trigger's controller's, the opponent")
}
