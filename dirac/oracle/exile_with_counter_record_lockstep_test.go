package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestExileWithCounterRecordReprosLockstep replays the two recorded games
// where a card exiled with a counter lost it in the kernel: Grolnok's
// "exile it with a croak counter" (the may-play and the projection read it)
// and Etrata's hit counter (the lose-at-three count reads it). The exile move
// kept the counter only for Stash and Time; the per-object exile record now
// holds any modelled kind (repl_exilectr.go).
func TestExileWithCounterRecordReprosLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	stdReproNamed(t, "exile_with_counter_record")
	stdReproNamed(t, "exile_with_counter_record_b")
}

// exileCounterDeck is lands, a body and the exiler, so the exiler is cast and
// reaches the exile move (Grolnok mills on attack; Etrata connects unblocked).
func exileCounterDeck(exiler, land1, land2, body string) []string {
	return deckOf(6, exiler, deckOf(8, body, deckOf(22, land1, deckOf(22, land2)...)...)...)
}

// exileCounterObs counts the cards gorge holds in exile with the counter kind.
type exileCounterObs struct {
	kind  string
	seen  map[state.ObjID]bool
	reads int // priorities with such a card in exile: the later read is evaluated
}

func (o *exileCounterObs) check(g *GorgeSide, _ Side) *Divergence {
	for p := range g.E.G.Players {
		for _, id := range g.E.G.Zone(state.ZExile, state.PlayerID(p)) {
			if ob := g.E.G.Obj(id); ob != nil && ob.Counter(o.kind) > 0 {
				if !o.seen[id] {
					o.seen[id] = true
				}
				o.reads++
			}
		}
	}
	return nil
}

// TestExileWithCounterRecordFourSeat plays four-seat pods whose decks hold the
// exiler: Grolnok with Frogs (itself) and its self-mill. A card
// exiled with a croak counter is later read by the may-play; the
// games play out the same in both engines, and the mechanism is reached.
// Etrata is not in this pod: its four-seat pods are
// TestFourSeatEtrataPodsLockstep (podlimited_cards_mp_test.go); the two-seat
// recording above carries its hit counter.
func TestExileWithCounterRecordFourSeat(t *testing.T) {
	if isolate(t) {
		return
	}
	cases := []struct {
		name, pool, kind string
		deck, foe        []string
	}{
		{"grolnok", "tail-21", "CROAK",
			exileCounterDeck("Grolnok, the Omnivore", "Forest", "Island", "Elvish Berserker"),
			exileCounterDeck("Bellowing Elk", "Forest", "Island", "Elvish Berserker")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			obs := &exileCounterObs{kind: c.kind, seen: map[state.ObjID]bool{}}
			set := cardsettest.Synth(t, c.pool)
			play := fourSeatSetPlay(t, set, [4][]string{c.deck, c.foe, c.deck, c.foe},
				nil, obs.check, nil, func(seed uint64) Policy { return combatPolicy(seed, nil) })
			fourSeatAtkClean(t, play, 20)
			if len(obs.seen) == 0 {
				t.Fatalf("no card was ever exiled with a %s counter: the mechanism was never reached", c.kind)
			}
			t.Logf("%d cards exiled with a %s counter, %d priorities with one in exile", len(obs.seen), c.kind, obs.reads)
		})
	}
}

// TestExileWithCounterRecordMutation restores the old exile move, which kept no
// counter but Stash and Time (dirac.DebugExileCounterDropped), and replays the
// Grolnok recording: it must diverge, so the lockstep test cannot pass on a
// game that never read the counter.
func TestExileWithCounterRecordMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	old := dirac.DebugExileCounterDropped
	dirac.DebugExileCounterDropped = true
	defer func() { dirac.DebugExileCounterDropped = old }()
	r, err := ReadRepro("testdata/repro/exile_with_counter_record.repro.json")
	if err != nil {
		t.Fatal(err)
	}
	res := replayDecodedScript(t, synthReg(t), globSet(t, r.Pool, r), r, &ScriptPolicy{Actions: r.Actions})
	if res.Div == nil {
		t.Fatal("the mutated kernel dropped the exile counter and still matched gorge")
	}
	t.Logf("mutation diverges at %d: %s %s", res.Div.At, res.Div.What, res.Div.Field)
}
