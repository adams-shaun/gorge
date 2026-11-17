package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestCounterAddedAllGrantedToHost: a Continuous static's AddTrigger$ grant of
// CounterAddedAll (an Equipment's "whenever a counter is put on ...") makes
// the attached host a listener. The host has no Once subscription of its own --
// only the granting card's latent ability subscribes, so the counter sites'
// cheap gate calls fireObj -- and the watcher's high event ordinal is past
// grantTrigEv's 32 bits, so ctrAllNote must walk the granted hosts when the
// table's grantCtrAll flag is set.
func TestCounterAddedAllGrantedToHost(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature, Flags: kop.FAttachedBySrc})
	f := kop.FilterID(len(tbl.Filters) - 1)
	eq, bear := CardID(len(tbl.Cards)), CardID(len(tbl.Cards)+1)
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Watcher Banner", Types: kop.TArtifact, Abilities: []kop.Ability{
			{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatContinuous, Filter: f, Grant: 2}},
			{Kind: kop.AbTriggered, AFlags: kop.AfLatent, Trig: kop.Trigger{Event: kop.TrigCounterAddedAll, N: kop.TrigNAnyCounter}}}},
		kop.CardDef{Name: "Test Bear", Types: kop.TCreature, Power: 2, Toughness: 2})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	s, k := sc.s, sc.k
	host := sc.perm(bear, 0)
	banner := sc.perm(eq, 0)
	s.perms[s.objs[banner].slot].attached = s.objs[host].slot
	s.npend = 0

	s.bolsterPut(k, s.objs[host].slot, int(kop.CtrP1P1), 1)
	if s.npend != 1 {
		t.Fatalf("a counter on the granted host queued %d CounterAddedAll triggers, want 1", s.npend)
	}
	if s.pend[0].trigAmt != 1 {
		t.Fatalf("granted host trigger amount = %d, want 1", s.pend[0].trigAmt)
	}
}
