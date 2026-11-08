package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestEntryCounterSelfScope covers CR 614.12's own-entry rule for a requested
// entry counter (the count Generous Revival's ChangeZone stages): the
// entrant's own AddCounter line applies only when it selects the entrant alone.
// Mowu's "on CARDNAME" plus-one therefore applies to its own entry, while
// Yoshimaru's general "a creature you control" does not; a self-scoped
// CantPutCounter line still forbids the counter gorge's preview forbids.
func TestEntryCounterSelfScope(t *testing.T) {
	for _, tc := range []struct {
		name string
		want int
	}{
		{"Mowu, Loyal Companion", 2},        // FSelf: its own plus-one applies
		{"Yoshimaru, Beloved Companion", 1}, // general subset: its own plus-one does not
	} {
		t.Run(tc.name, func(t *testing.T) {
			sc, id := replCountersScen(t, []cardset.Entry{
				{Name: "Mowu, Loyal Companion", Count: 1},
				{Name: "Yoshimaru, Beloved Companion", Count: 1},
				{Name: "Grizzly Bears", Count: 1},
				{Name: "Forest", Count: 1},
			})
			// Precondition: the table carries the plus-one replacements, so
			// the assertion below is not vacuous.
			if sc.k.gen == nil || sc.k.gen.anyRepl&rbCtrOn == 0 {
				t.Fatal("prep missed the pool's AddCounter replacements")
			}
			o := sc.obj(id[tc.name], 0)
			sc.s.g.enterCtr.n, sc.s.g.enterCtr.kind = 1, kop.CtrP1P1
			sl := sc.s.enter(sc.k, o, 0, false, kop.ZoneStack)
			// Precondition: the entrant is subscribed before enterGen settles
			// its counters, so its own static is in the matcher's walk.
			if !sc.s.statM.has(sl) {
				t.Fatal("entrant is not in statM")
			}
			if got := sc.s.perms[sl].ctrLow(int(kop.CtrP1P1)); got != tc.want {
				t.Fatalf("entered with %d +1/+1 counters, want %d", got, tc.want)
			}
			if sc.s.g.ctrSettle != 0 {
				t.Fatalf("entry settle slot leaked: %d", sc.s.g.ctrSettle)
			}
		})
	}

	t.Run("self CantPutCounter", func(t *testing.T) {
		set := cardsettest.Set(t, "exileanddoublers")
		tbl, da, _, err := set.Matchup([]cardset.Entry{
			{Name: "Grizzly Bears", Count: 1},
			{Name: "Forest", Count: 1},
		}, []cardset.Entry{
			{Name: "Grizzly Bears", Count: 1},
			{Name: "Forest", Count: 1},
		})
		if err != nil {
			t.Fatal(err)
		}
		self := kop.FilterID(len(tbl.Filters))
		tbl.Filters = append(tbl.Filters, kop.Filter{Flags: kop.FSelf})
		blocked := CardID(len(tbl.Cards))
		tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Self Block", Types: kop.TCreature, Power: 2, Toughness: 2,
			Abilities: []kop.Ability{{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatCantPutCounter, A: int16(kop.CtrP1P1), Filter: self}}}})
		lib := idOf(tbl, da, "Forest")
		sc := newScenTableLib(t, tbl, lib, Config{NoElide: true})
		if sc.k.gen == nil || !sc.k.gen.anyCtrBlock {
			t.Fatal("prep missed the table's CantPutCounter static")
		}
		o := sc.obj(blocked, 0)
		sc.s.g.enterCtr.n, sc.s.g.enterCtr.kind = 1, kop.CtrP1P1
		sl := sc.s.enter(sc.k, o, 0, false, kop.ZoneStack)
		// Precondition: the requested counter is one the entrant's own static
		// forbids, and the static is live after the entry.
		if !sc.s.putCounterBlocked(sc.k, sl, int(kop.CtrP1P1)) {
			t.Fatal("the entrant's own CantPutCounter static is not live")
		}
		if got := sc.s.perms[sl].ctrLow(int(kop.CtrP1P1)); got != 0 {
			t.Fatalf("entered with %d +1/+1 counters, want 0: its own prohibition blocks them", got)
		}
		if sc.s.g.ctrSettle != 0 {
			t.Fatalf("entry settle slot leaked: %d", sc.s.g.ctrSettle)
		}
	})
}

// idOf is the card id of name among a matchup's seat cards.
func idOf(t *kop.Table, ids []CardID, name string) CardID {
	for _, c := range ids {
		if t.Cards[c].Name == name {
			return c
		}
	}
	return CardID(255)
}
