package dirac

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestSagaDepartCasualtyTriggerOrder: a mixed SBA pass (a creature casualty
// and a Saga due its sacrifice, CR 704.3/714.4) queues the pass's triggers in
// gorge's emit order: the casualty's dies trigger at the casualty's own
// departure, and the Sacrificed listener's trigger at the Saga's, where the
// one zone-change event's face walk orders the sources by battlefield
// position (sacLeaveOrder's walk). So with the listener after the Saga on the
// battlefield the queue is [casualty's dies, Saga's leaves, listener's
// Sacrificed] — the Sacrificed trigger is not ahead of the whole pass (the
// injected cards, so the test depends on no printed card).
func TestSagaDepartCasualtyTriggerOrder(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TEnchantment, Owner: kop.CtrlYou})
	yourEnchantment := kop.FilterID(len(tbl.Filters) - 1)
	gain := []kop.Instr{{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(1)}}
	n := CardID(len(tbl.Cards))
	casualty, saga, listener := n, n+1, n+2
	tbl.Cards = append(tbl.Cards,
		// zero toughness: the pass's creature casualty
		kop.CardDef{Name: "Depart Casualty", Types: kop.TCreature, Power: 2, Toughness: 0,
			Abilities: []kop.Ability{{Kind: kop.AbTriggered,
				Trig: kop.Trigger{Event: kop.TrigDies, Flags: kop.TfSelf}, Prog: gain}}},
		// final chapter reached on entry: due its sacrifice in the same pass
		kop.CardDef{Name: "Depart Saga", Types: kop.TEnchantment, SagaN: 1,
			Abilities: []kop.Ability{{Kind: kop.AbTriggered,
				Trig: kop.Trigger{Event: kop.TrigLeaves, Flags: kop.TfSelf}, Prog: gain}}},
		// the event object of its Sacrificed trigger is the Saga (an
		// enchantment you control)
		kop.CardDef{Name: "Depart Listener", Types: kop.TCreature, Power: 1, Toughness: 1,
			Abilities: []kop.Ability{{Kind: kop.AbTriggered,
				Trig: kop.Trigger{Event: kop.TrigSacrificed, Filter: yourEnchantment}, Prog: gain}}})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	s, k := sc.s, sc.k
	c, sg, _ := sc.perm(casualty, 0), sc.perm(saga, 0), sc.perm(listener, 0) // listener after the Saga: gorge's walk orders the Saga's leaves line first
	if !s.subs[kop.TrigSacrificed].any() {
		t.Fatal("nothing listens for sacrifices: the test sees nothing")
	}
	if !s.sagaDue(k, sc.slot(sg)) {
		t.Fatal("the Saga is not due its sacrifice: the test sees nothing")
	}
	s.sba(k) // returns whether a further pass or an ask is owed, not whether it applied
	if sc.onBF(c) || sc.onBF(sg) {
		t.Fatal("the casualty and the Saga must both have left")
	}
	var got []string
	for i := 0; i < int(s.npend); i++ {
		got = append(got, tbl.Cards[s.pend[i].card].Name)
	}
	want := []string{"Depart Casualty", "Depart Saga", "Depart Listener"}
	if !slices.Equal(got, want) {
		t.Fatalf("pending trigger order %v, want %v", got, want)
	}
}
