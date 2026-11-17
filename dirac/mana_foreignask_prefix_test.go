package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestForeignOptionalPrefixGate drives the scriptDFS gate (mana_window.go)
// end to end for the ask class that is posed at a trigger's resolution: a
// may-election (kop.TfOptional) queued by a foreign control. The board is the
// prefix search's shape (Taps-deferred gap: my Judge of Currents makes my
// Firdoch Core a source the ordinary planner never taps), with the opponent's
// optional watcher queued as a foreign pending trigger, the provoked state of
// a Taps watcher controlled by the opponent -- the class no lowered corpus
// card queues today (Thoughtleech, Verity Circle, Roots of Life, Sanctimony
// and Snowfall are all refused at lowering), so the search is driven at the
// scriptPrefix level with the trigger fabricated, as the kernel's scenario
// tests fabricate every other staged state.
//
// The gate must mirror gorge's potential_script.go visit: a foreign optional
// ask blocks a sorcery-speed play -- the search's waitsForStack branch passes
// to let the trigger resolve and reaches the foreign election there -- and
// does not block an instant-speed play, which is offered at the payer's
// priority with the trigger still on the stack (the election is the
// opponent's to answer later, CR 603.5, 117.1a). The payer's own optional
// trigger (my Judge, the Core's tap queues it) blocks neither.
//
// The Core is a plain Taps-deferred gap, which gorge's planner funds in its
// second pass (engine/rules/pay plandeferred.go): the plain walk lists the
// instant-speed play and the own-election play (availTapsDeferIn), so their
// payment needs no prefix and the subtests below pin that.
func TestForeignOptionalPrefixGate(t *testing.T) {
	deck := []cardset.Entry{
		{Name: "Judge of Currents", Count: 2},
		{Name: "Firdoch Core", Count: 1},
		{Name: "Savannah Lions", Count: 1},
		{Name: "Tragic Slip", Count: 1},
		{Name: "Plains", Count: 15},
	}
	tbl, da, _, err := cardsettest.Set(t, "tapsdefer").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	judge, core, lions, slip := id["Judge of Currents"], id["Firdoch Core"], id["Savannah Lions"], id["Tragic Slip"]

	// board puts my Judge and my Core out, the opponent's Judge beside them,
	// and one of my one-mana plays in hand. The opponent's Judge watches only
	// its controller's Merfolk (whoseOK), so my Core's tap queues my watcher
	// alone; it is the fabricated pending entry's source.
	board := func(t *testing.T, play CardID) (*scen, Obj, Obj, Obj) {
		t.Helper()
		sc := newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true})
		sc.perm(judge, 0)
		co := sc.perm(core, 0)
		opp := sc.perm(judge, 1)
		if !sc.s.scriptTapsDefer(sc.k, 0).has(sc.slot(co)) {
			t.Fatal("precondition: the Core is not a Taps-deferred source under my Judge")
		}
		wo := sc.hand(play, 0)
		// the foreign pending trigger: the opponent's Judge's may-election,
		// queued by a tap it watched. No trigger of the opponent's is queued by
		// this board's taps, so the entry is the only foreign one.
		if sc.s.npend != 0 {
			t.Fatal("precondition: the trigger queue is empty")
		}
		if a := &tbl.Cards[judge].Abilities[0]; a.NTargets != 0 || a.NModes != 0 || a.Trig.Flags&kop.TfOptional == 0 {
			t.Fatal("precondition: the watcher is no optional-only ability")
		}
		sc.s.pend[0] = pendingTrig{obj: opp, card: judge, ab: 0, ctrl: 1}
		sc.s.npend = 1
		return sc, co, opp, wo
	}

	// instantSpeed: Tragic Slip ({B} instant) is offered at my priority with
	// the foreign trigger still pending: the election is posed only when the
	// trigger resolves, and the prefix places it below the play (the applied
	// path, legal.go's flush) without answering the opponent's election.
	t.Run("a foreign election does not block an instant-speed play", func(t *testing.T) {
		sc, co, _, wo := board(t, slip)
		// gorge's planner funds the Core's {B} in its second pass (engine/rules/pay
		// plandeferred.go), so the plain walk lists the play and the payment needs no
		// prefix: the offer holds at the payer's priority with the trigger pending,
		// and the Core stays untapped for the planner to tap at the cast.
		var d Decision
		sc.s.plainOptions(sc.k, &d, 0)
		if !hasOption(&d, MakeAction(AkCast, TCode(wo), 0, 0)) {
			t.Fatal("the instant-speed play is not offered at the payer's priority beside the foreign election")
		}
		sc.s.scriptPrefix(sc.k, MakeAction(AkCast, TCode(wo), 0, 0), 0)
		if sc.tapped(co) || sc.s.pl[0].pool[kop.B] != 0 {
			t.Fatalf("the Core tapped %v, pool %v: the prefix tapped a gap the planner's second pass pays",
				sc.tapped(co), sc.s.pl[0].pool)
		}
		if sc.s.npend != 1 {
			t.Fatalf("the foreign trigger left the queue (npend %d): the prefix placed it", sc.s.npend)
		}
	})

	// sorcerySpeed: the Lions ({W} creature) wait for the stack, so the
	// foreign election ends the search's line: no script, the Core untapped.
	t.Run("a foreign election blocks a stack-waiting play", func(t *testing.T) {
		sc, co, _, wo := board(t, lions)
		sc.s.scriptPrefix(sc.k, MakeAction(AkCast, TCode(wo), 0, 0), 0)
		if sc.tapped(co) || sc.s.pl[0].pool[kop.W] != 0 {
			t.Fatalf("the Core tapped %v, pool %v: the search answered the opponent's election itself",
				sc.tapped(co), sc.s.pl[0].pool)
		}
		if sc.s.npend != 1 {
			t.Fatalf("the foreign trigger left the queue (npend %d): the search resolved it", sc.s.npend)
		}
	})

	// my own election: the Core is a plain gap gorge's planner funds in its second
	// pass, so the plain walk lists the Lions and the payment needs no prefix; the
	// election is posed when the Judge's trigger resolves after the cast.
	t.Run("the payer's own election does not block", func(t *testing.T) {
		sc := newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true})
		sc.perm(judge, 0)
		co := sc.perm(core, 0)
		if !sc.s.scriptTapsDefer(sc.k, 0).has(sc.slot(co)) {
			t.Fatal("precondition: the Core is not a Taps-deferred source under my Judge")
		}
		wo := sc.hand(lions, 0)
		var d Decision
		sc.s.plainOptions(sc.k, &d, 0)
		if !hasOption(&d, MakeAction(AkCast, TCode(wo), 0, 0)) {
			t.Fatal("the sorcery-speed play is not offered beside my own election")
		}
		sc.s.scriptPrefix(sc.k, MakeAction(AkCast, TCode(wo), 0, 0), 0)
		if sc.tapped(co) {
			t.Fatal("the prefix tapped a gap the planner's second pass pays")
		}
	})
}
