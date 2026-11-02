package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestGrantedTriggerToHost: an Equipment's Continuous static granting a
// triggered ability (AddTrigger$, gorge layer 6) makes the creature it is
// attached to trigger ("whenever this creature attacks", Self = the host,
// controlled by the host's controller); unattached, nothing triggers.
func TestGrantedTriggerToHost(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature, Flags: kop.FAttachedBySrc})
	f := kop.FilterID(len(tbl.Filters) - 1)
	gain := []kop.Instr{{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(1)}}
	eq, bear := CardID(len(tbl.Cards)), CardID(len(tbl.Cards)+1)
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Banner", Types: kop.TArtifact, Abilities: []kop.Ability{
			{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatContinuous, Filter: f, Grant: 2}},
			{Kind: kop.AbTriggered, AFlags: kop.AfLatent, Trig: kop.Trigger{Event: kop.TrigAttacks, Flags: kop.TfSelf}, Prog: gain}}},
		kop.CardDef{Name: "Test Bear", Types: kop.TCreature, Power: 2, Toughness: 2})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	a1, a2 := sc.perm(bear, 0), sc.perm(bear, 0)
	e := sc.perm(eq, 0)
	sc.s.perms[sc.slot(e)].attached = sc.slot(a1)
	sc.combat(t, []Obj{a1, a2}, nil)
	// a1 (equipped) triggers once: +1; the two bears deal 4
	if l0, l1 := sc.s.pl[0].life, sc.s.pl[1].life; l0 != 21 || l1 != 16 {
		t.Fatalf("life %d / %d, want 21 / 16", l0, l1)
	}
}
